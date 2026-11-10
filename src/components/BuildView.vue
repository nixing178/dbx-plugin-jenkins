<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, jenkinsApi } from "../api";
import { t } from "../i18n";
import { buildParameterRows, parameterValueLabel } from "../lib/buildParameters";
import {
  CHANGE_PREVIEW_LIMIT,
  changeBody,
  changeSummary,
  formatChangeTime,
  hasBody,
  sortChanges,
  visibleChanges,
} from "../lib/changes";
import {
  averageSpeed,
  remainingSeconds,
  transferSummary,
  type ProgressSample,
} from "../lib/downloadMetrics";
import { errorText } from "../lib/errors";
import { formatBytes, formatDuration, formatTimestamp, shortenPath } from "../lib/format";
import type { ToastTone } from "../lib/toast";
import type { RefreshUi } from "../lib/refreshStatus";
import { shortCommitId } from "../lib/stage";
import { statusMeta } from "../lib/status";
import { formatTaskProgress, isTaskFinished, taskPercent } from "../lib/taskProgress";
import type { Artifact, ArtifactTaskSnapshot, BuildDetail } from "../types";
import ConsoleView from "./ConsoleView.vue";
import RefreshBadge from "./RefreshBadge.vue";

const props = defineProps<{
  build: BuildDetail;
  jobFullName: string;
  readOnly: boolean;
  locale: "zh" | "en";
  refresh: RefreshUi;
}>();

const emit = defineEmits<{
  back: [];
  stop: [number: number];
  refresh: [];
  /** 下载完成/失败的消息提示，由 App 统一渲染成 toast。 */
  notify: [text: string, tone?: ToastTone];
}>();

/** 变更列表默认折叠，避免几十条提交把控制台挤到页面底部。 */
const expanded = ref(false);
/** 已展开正文的提交号。用 Set 存，3s 轮询换数据时不会把展开状态冲掉。 */
const expandedBodies = ref(new Set<string>());
const copiedId = ref("");

const orderedChanges = computed(() => sortChanges(props.build.changeSet ?? []));
const summary = computed(() => changeSummary(orderedChanges.value));
const rows = computed(() =>
  visibleChanges(orderedChanges.value, expanded.value, CHANGE_PREVIEW_LIMIT),
);
const summaryText = computed(() =>
  t("changeSummary", {
    commits: summary.value.commits,
    authors: summary.value.authors,
    files: summary.value.files,
  }),
);

/** 构建参数顺序保持 Jenkins 返回顺序，不做排序；空名称在这里兜底过滤。 */
const parameterRows = computed(() => buildParameterRows(props.build.parameters));

function toggleBody(commitId: string): void {
  const next = new Set(expandedBodies.value);
  if (next.has(commitId)) next.delete(commitId);
  else next.add(commitId);
  expandedBodies.value = next;
}

function isBodyOpen(commitId: string): boolean {
  return expandedBodies.value.has(commitId);
}

/** 点哈希复制完整 commit id：宿主有 copy 能力时优先用，兜底走浏览器剪贴板。 */
async function copyCommit(commitId: string): Promise<void> {
  if (!commitId) return;
  try {
    if (window.dbxPlugin?.copy) {
      await window.dbxPlugin.copy(commitId);
    } else {
      await navigator.clipboard.writeText(commitId);
    }
    copiedId.value = commitId;
    window.setTimeout(() => {
      if (copiedId.value === commitId) copiedId.value = "";
    }, 1500);
  } catch {
    // 复制失败不打断阅读，静默即可。
  }
}

/** 单个制品的落盘态：idle → saving → saved / failed / canceled。 */
interface ArtifactRowState {
  status: "idle" | "saving" | "saved" | "failed" | "canceled";
  /** 后台任务 id，用于取消。 */
  taskId: string;
  done: number;
  total: number;
  /** 最近窗口内的即时速度（字节/秒），0 表示还没算出来。 */
  speed: number;
  /** 按当前速度估算的剩余秒数，null 表示未知。 */
  etaSeconds: number | null;
  /** 已落盘字节数（终态用）。 */
  size: number;
  /** sidecar 返回的绝对路径。 */
  path: string;
  error: string;
  copied: boolean;
}

const POLL_INTERVAL_MS = 500;
/** 速度采样窗口：4s 足以抹平轮询抖动，又不会慢半拍。 */
const SPEED_WINDOW_MS = 4000;

const artifactRows = ref<Record<string, ArtifactRowState>>({});
/** relativePath → 字节数，来自后台 HEAD 或任务终态。 */
const artifactSizes = ref<Record<string, number>>({});
/** 已尝试过 HEAD 的制品，避免 3s 轮询把同一个 404 反复打。 */
const sizeAttempts = new Set<string>();
/** 当前连接实际使用的下载目录。 */
const downloadDir = ref("");
/** 「全部下载」的进度。 */
const batch = ref({ running: false, done: 0, total: 0 });
/** relativePath → 轮询定时器。 */
const pollTimers = new Map<string, number>();
/** relativePath → 进度采样，用于算速度；不参与渲染，避免无意义的重渲染。 */
const progressSamples = new Map<string, ProgressSample[]>();

function sizeAttemptKey(artifact: Artifact): string {
  return `${props.jobFullName}#${props.build.number}#${artifact.relativePath}`;
}

function rowOf(relativePath: string): ArtifactRowState {
  return (
    artifactRows.value[relativePath] ?? {
      status: "idle",
      taskId: "",
      done: 0,
      total: 0,
      speed: 0,
      etaSeconds: null,
      size: 0,
      path: "",
      error: "",
      copied: false,
    }
  );
}

/** 记一次进度采样并算出速度/剩余时间。 */
function recordProgress(
  relativePath: string,
  done: number,
  total: number,
): { speed: number; etaSeconds: number | null } {
  const samples = progressSamples.get(relativePath) ?? [];
  const at = Date.now();

  // 同一毫秒的重复轮询会让时间跨度为 0，直接覆盖最后一个样本。
  if (samples.length > 0 && samples[samples.length - 1].at === at) {
    samples[samples.length - 1] = { done, at };
  } else {
    samples.push({ done, at });
  }
  while (samples.length > 16) samples.shift();
  progressSamples.set(relativePath, samples);

  const speed = averageSpeed(samples, SPEED_WINDOW_MS);
  return { speed, etaSeconds: remainingSeconds(done, total, speed) };
}

function patchRow(relativePath: string, patch: Partial<ArtifactRowState>): void {
  artifactRows.value = {
    ...artifactRows.value,
    [relativePath]: { ...rowOf(relativePath), ...patch },
  };
}

function artifactSizeLabel(artifact: Artifact): string {
  const known = artifactSizes.value[artifact.relativePath] ?? rowOf(artifact.relativePath).size;
  return known > 0 ? formatBytes(known) : "";
}

function artifactButtonLabel(artifact: Artifact): string {
  const row = rowOf(artifact.relativePath);
  if (row.status === "saving") {
    const percent = formatTaskProgress(row.done, row.total);
    return percent
      ? t("artifactSavingProgress", { percent })
      : t("artifactSaving");
  }
  if (row.status === "saved") return t("artifactSaved");
  if (row.status === "canceled") return t("artifactCanceled");
  if (row.status === "failed") return t("retry");
  return t("download");
}

function artifactPercent(artifact: Artifact): number {
  const row = rowOf(artifact.relativePath);
  return taskPercent(row.done, row.total);
}

/** 副行文案：`8.5 MB / 20.0 MB · 3.2 MB/s · 剩余 4s`，未知项自动省略。 */
function transferText(artifact: Artifact): string {
  const row = rowOf(artifact.relativePath);
  const summary = transferSummary(row.done, row.total, row.speed);
  const parts = [summary.bytes];
  if (summary.speed) parts.push(summary.speed);
  if (summary.etaSeconds !== null) {
    parts.push(t("etaSeconds", { seconds: summary.etaSeconds }));
  }
  return parts.join(" · ");
}

/** 进页面就量一下大小，让用户在点下载前知道有多大；失败静默，点下载时再报原因。 */
function prefetchArtifactSizes(): void {
  for (const artifact of props.build.artifacts ?? []) {
    const key = sizeAttemptKey(artifact);
    if (sizeAttempts.has(key)) continue;
    sizeAttempts.add(key);
    void jenkinsApi
      .artifactInfo(props.jobFullName, props.build.number, artifact.relativePath)
      .then((info) => {
        artifactSizes.value = {
          ...artifactSizes.value,
          [artifact.relativePath]: info.size,
        };
      })
      .catch(() => {
        // 制品尚未生成 / 已被清理：保持无大小，下载时再给出具体错误。
      });
  }
}

watch(() => props.build.artifacts, prefetchArtifactSizes, { immediate: true });

function clearPoll(relativePath: string): void {
  const timer = pollTimers.get(relativePath);
  if (timer !== undefined) {
    window.clearTimeout(timer);
    pollTimers.delete(relativePath);
  }
  progressSamples.delete(relativePath);
}

function clearAllPolls(): void {
  for (const timer of pollTimers.values()) window.clearTimeout(timer);
  pollTimers.clear();
  progressSamples.clear();
}

// 换构建时清空行状态与轮询，避免把上一个构建的进度带到新页面。
watch(
  () => [props.jobFullName, props.build.number],
  () => {
    clearAllPolls();
    artifactRows.value = {};
    batch.value = { running: false, done: 0, total: 0 };
  },
);

onMounted(() => {
  void jenkinsApi
    .downloadDir()
    .then(({ dir }) => (downloadDir.value = dir))
    .catch(() => {
      // 拿不到目录不影响下载本身。
    });
});

onBeforeUnmount(clearAllPolls);

/**
 * 落盘只能由 sidecar 完成：DBX 工作台的 iframe 没有 allow-downloads，
 * 浏览器下载与宿主保存对话框都不可靠，所以后端启动任务、前端轮询进度。
 */
async function runDownload(
  artifact: Artifact,
  options: { silent?: boolean } = {},
): Promise<void> {
  const relativePath = artifact.relativePath;
  if (rowOf(relativePath).status === "saving") return;
  clearPoll(relativePath);

  patchRow(relativePath, {
    status: "saving",
    taskId: "",
    done: 0,
    total: artifactSizes.value[relativePath] ?? 0,
    speed: 0,
    etaSeconds: null,
    path: "",
    error: "",
    copied: false,
  });

  let taskId = "";
  try {
    const started = await jenkinsApi.artifactStart(
      props.jobFullName,
      props.build.number,
      relativePath,
    );
    taskId = started.taskId;
    patchRow(relativePath, {
      taskId,
      done: started.done,
      total: started.total || rowOf(relativePath).total,
    });
  } catch (error) {
    const message = errorText(describeError(error), props.locale);
    patchRow(relativePath, { status: "failed", error: message });
    if (!options.silent) {
      emit("notify", t("downloadFailedToast", { message }), "info");
    }
    return;
  }

  await waitForTask(relativePath, taskId, options.silent === true);
}

function waitForTask(relativePath: string, taskId: string, silent: boolean): Promise<void> {
  return new Promise((resolve) => {
    const tick = async (): Promise<void> => {
      try {
        const snapshot = await jenkinsApi.artifactStatus(taskId);
        const total = snapshot.total || rowOf(relativePath).total;
        const { speed, etaSeconds } = recordProgress(relativePath, snapshot.done, total);
        patchRow(relativePath, { done: snapshot.done, total, speed, etaSeconds });

        if (isTaskFinished(snapshot.state)) {
          finishRow(relativePath, snapshot, total, silent);
          resolve();
          return;
        }
        pollTimers.set(
          relativePath,
          window.setTimeout(() => void tick(), POLL_INTERVAL_MS),
        );
      } catch (error) {
        const message = errorText(describeError(error), props.locale);
        patchRow(relativePath, { status: "failed", error: message });
        if (!silent) {
          emit("notify", t("downloadFailedToast", { message }), "info");
        }
        resolve();
      }
    };
    void tick();
  });
}

function finishRow(
  relativePath: string,
  snapshot: ArtifactTaskSnapshot,
  total: number,
  silent: boolean,
): void {
  progressSamples.delete(relativePath);

  if (snapshot.state === "saved") {
    const size = snapshot.total || total;
    if (size > 0) {
      artifactSizes.value = { ...artifactSizes.value, [relativePath]: size };
    }
    patchRow(relativePath, {
      status: "saved",
      size,
      total: size,
      speed: 0,
      etaSeconds: null,
      path: snapshot.path ?? "",
    });
    if (!silent) {
      emit(
        "notify",
        t("downloadDoneToast", {
          name: snapshot.fileName || relativePath,
          size: formatBytes(size),
        }),
      );
    }
    return;
  }
  if (snapshot.state === "canceled") {
    patchRow(relativePath, { status: "canceled", done: 0, speed: 0, etaSeconds: null });
    return;
  }
  const detail = snapshot.error || t("artifactFailed");
  const message = errorText(detail, props.locale) || detail;
  patchRow(relativePath, { status: "failed", error: message, speed: 0, etaSeconds: null });
  if (!silent) {
    emit("notify", t("downloadFailedToast", { message }), "info");
  }
}

async function startArtifactDownload(artifact: Artifact): Promise<void> {
  await runDownload(artifact);
}

async function cancelArtifactDownload(artifact: Artifact): Promise<void> {
  const { status, taskId } = rowOf(artifact.relativePath);
  if (status !== "saving" || !taskId) return;
  try {
    await jenkinsApi.artifactCancel(taskId);
  } catch {
    // 轮询会拿到终态，取消失败不额外提示。
  }
}

async function downloadAllArtifacts(): Promise<void> {
  const artifacts = props.build.artifacts ?? [];
  if (batch.value.running || artifacts.length === 0) return;

  batch.value = { running: true, done: 0, total: artifacts.length };
  let succeeded = 0;
  try {
    for (const artifact of artifacts) {
      await runDownload(artifact, { silent: true });
      if (rowOf(artifact.relativePath).status === "saved") succeeded += 1;
      batch.value = {
        running: true,
        done: batch.value.done + 1,
        total: batch.value.total,
      };
    }
  } finally {
    batch.value = { running: false, done: batch.value.done, total: batch.value.total };
    // 批量时逐条通知会刷屏，收尾给一条汇总。
    emit(
      "notify",
      t("downloadBatchDoneToast", { done: succeeded, total: artifacts.length }),
      succeeded === artifacts.length ? "success" : "info",
    );
  }
}

/** 在系统文件管理器里定位已落盘的文件；失败静默，避免打断阅读。 */
async function openArtifactFolder(artifact: Artifact): Promise<void> {
  const { path } = rowOf(artifact.relativePath);
  if (!path) return;
  try {
    await jenkinsApi.revealFile(path);
  } catch {
    // 文件可能已被移走：保留行内路径，用户可自行打开。
  }
}

async function copyArtifactPath(artifact: Artifact): Promise<void> {
  const { path } = rowOf(artifact.relativePath);
  if (!path) return;
  try {
    if (window.dbxPlugin?.copy) {
      await window.dbxPlugin.copy(path);
    } else {
      await navigator.clipboard.writeText(path);
    }
    patchRow(artifact.relativePath, { copied: true });
    window.setTimeout(() => patchRow(artifact.relativePath, { copied: false }), 1500);
  } catch {
    // 复制失败不打断已完成的下载。
  }
}
</script>

<template>
  <section class="build-view">
    <header class="build-head">
      <div class="title-block">
        <button type="button" class="back" @click="emit('back')">← {{ t("backToJob") }}</button>
        <div class="title-row">
          <h1>#{{ build.number }}</h1>
          <span class="status-pill" :data-tone="statusMeta(build.status).tone">
            {{ t(statusMeta(build.status).key) }}
          </span>
        </div>
        <p class="full-name">{{ build.fullDisplayName || jobFullName }}</p>
        <p v-if="build.causes?.length" class="cause">
          {{ t("cause") }}: {{ build.causes.map((cause) => cause.shortDescription).join(" · ") }}
        </p>
      </div>
      <div class="actions">
        <RefreshBadge
          :mode="refresh.mode"
          :paused="refresh.paused"
          :in-flight="refresh.inFlight"
          :last-at="refresh.lastAt"
          :interval-ms="refresh.intervalMs"
        />
        <button
          type="button"
          class="ghost"
          :class="{ done: refresh.refreshed }"
          :disabled="refresh.refreshing"
          @click="emit('refresh')"
        >
          {{
            refresh.refreshing
              ? t("refreshing")
              : refresh.refreshed
                ? t("refreshed")
                : t("refresh")
          }}
        </button>
        <button
          v-if="build.building && !readOnly"
          type="button"
          class="danger"
          @click="emit('stop', build.number)"
        >
          {{ t("stopBuild") }}
        </button>
      </div>
    </header>

    <div class="meta-grid">
      <div class="meta">
        <span class="meta-label">{{ t("status") }}</span>
        <strong>{{ t(statusMeta(build.status).key) }}</strong>
      </div>
      <div class="meta">
        <span class="meta-label">{{ t("startedAt") }}</span>
        <strong>{{ formatTimestamp(build.timestamp, locale) }}</strong>
      </div>
      <div class="meta">
        <span class="meta-label">{{ t("duration") }}</span>
        <strong>{{ formatDuration(build.duration) }}</strong>
      </div>
      <div class="meta">
        <span class="meta-label">ETA</span>
        <strong>{{ formatDuration(build.estimatedDuration) }}</strong>
      </div>
    </div>

    <section v-if="parameterRows.length" class="panel">
      <div class="panel-head">
        <h2>{{ t("buildParameters") }}</h2>
        <span class="count">{{ parameterRows.length }}</span>
      </div>
      <dl class="params">
        <div v-for="parameter in parameterRows" :key="parameter.name" class="param">
          <dt :title="parameter.name">{{ parameter.name }}</dt>
          <dd :title="parameter.value">{{ parameterValueLabel(parameter.value) }}</dd>
        </div>
      </dl>
    </section>

    <section v-if="build.changeSet?.length" class="panel">
      <div class="panel-head">
        <h2>{{ t("changes") }}</h2>
        <span class="count">{{ summaryText }}</span>
      </div>
      <table class="changes">
        <thead>
          <tr>
            <th class="col-commit">{{ t("commit") }}</th>
            <th class="col-time">{{ t("committedAt") }}</th>
            <th class="col-author">{{ t("author") }}</th>
            <th>{{ t("message") }}</th>
            <th class="col-files">{{ t("changedFiles") }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(item, index) in rows" :key="item.commitId || index">
            <td class="col-commit">
              <button
                type="button"
                class="commit-id"
                :title="t('copyCommit')"
                @click="copyCommit(item.commitId)"
              >
                <code>{{ shortCommitId(item.commitId) }}</code>
                <span v-if="copiedId === item.commitId" class="copied">{{ t("copied") }}</span>
              </button>
            </td>
            <td class="col-time">{{ formatChangeTime(item.timestamp ?? 0, locale) }}</td>
            <td class="col-author" :title="item.author">{{ item.author || "—" }}</td>
            <td class="message">
              <span class="subject">{{ item.message || "—" }}</span>
              <button
                v-if="hasBody(item)"
                type="button"
                class="toggle-body"
                @click="toggleBody(item.commitId)"
              >
                {{ isBodyOpen(item.commitId) ? t("hideFullMessage") : t("showFullMessage") }}
              </button>
              <pre v-if="hasBody(item) && isBodyOpen(item.commitId)" class="body">{{
                changeBody(item)
              }}</pre>
            </td>
            <td class="col-files">{{ item.fileCount || "—" }}</td>
          </tr>
        </tbody>
      </table>
      <div v-if="orderedChanges.length > CHANGE_PREVIEW_LIMIT" class="more">
        <button type="button" class="ghost" @click="expanded = !expanded">
          {{ expanded ? t("collapseAll") : t("expandAll", { count: orderedChanges.length }) }}
        </button>
      </div>
    </section>

    <section v-else class="panel">
      <div class="panel-head">
        <h2>{{ t("changes") }}</h2>
        <span class="count">{{ t("noChanges") }}</span>
      </div>
      <p class="empty-hint">{{ t("noChangesHint") }}</p>
    </section>

    <section v-if="build.artifacts?.length" class="panel">
      <div class="panel-head">
        <h2>{{ t("artifacts") }}</h2>
        <div class="artifact-head-actions">
          <code v-if="downloadDir" class="artifact-dir" :title="downloadDir">
            {{ shortenPath(downloadDir, 1) }}
          </code>
          <button
            v-if="build.artifacts.length > 1"
            type="button"
            class="ghost artifact-batch"
            :disabled="batch.running"
            @click="downloadAllArtifacts"
          >
            {{
              batch.running
                ? t("artifactBatchRunning", { done: batch.done, total: batch.total })
                : t("artifactBatch")
            }}
          </button>
          <span class="count">{{ build.artifacts.length }}</span>
        </div>
      </div>
      <ul class="artifacts">
        <li
          v-for="artifact in build.artifacts"
          :key="artifact.relativePath"
          :data-state="rowOf(artifact.relativePath).status"
        >
          <div class="artifact-row">
            <div class="artifact-info">
              <span class="artifact-name">{{ artifact.fileName }}</span>
              <code :title="artifact.relativePath">{{ artifact.relativePath }}</code>
            </div>
            <div class="artifact-actions">
              <span v-if="artifactSizeLabel(artifact)" class="artifact-size">
                {{ artifactSizeLabel(artifact) }}
              </span>
              <span
                v-if="rowOf(artifact.relativePath).status === 'failed'"
                class="artifact-error"
                :title="rowOf(artifact.relativePath).error"
              >
                {{ rowOf(artifact.relativePath).error }}
              </span>
              <button
                v-if="rowOf(artifact.relativePath).status === 'saving'"
                type="button"
                class="ghost artifact-download"
                @click="cancelArtifactDownload(artifact)"
              >
                {{ t("cancel") }}
              </button>
              <button
                v-else
                type="button"
                class="ghost artifact-download"
                @click="startArtifactDownload(artifact)"
              >
                {{ artifactButtonLabel(artifact) }}
              </button>
            </div>
          </div>
          <span
            v-if="rowOf(artifact.relativePath).status === 'saving'"
            class="artifact-progress"
            :title="artifactButtonLabel(artifact)"
          >
            <span
              class="artifact-progress-fill"
              :style="{ width: artifactPercent(artifact) + '%' }"
            ></span>
          </span>
          <div v-if="rowOf(artifact.relativePath).status === 'saving'" class="artifact-transfer">
            {{ transferText(artifact) }}
          </div>
          <div
            v-if="rowOf(artifact.relativePath).status === 'saved' && rowOf(artifact.relativePath).path"
            class="artifact-saved"
          >
            <code :title="rowOf(artifact.relativePath).path">
              {{ shortenPath(rowOf(artifact.relativePath).path) }}
            </code>
            <button type="button" class="artifact-copy" @click="openArtifactFolder(artifact)">
              {{ t("artifactOpenFolder") }}
            </button>
            <button type="button" class="artifact-copy" @click="copyArtifactPath(artifact)">
              {{ rowOf(artifact.relativePath).copied ? t("copied") : t("copy") }}
            </button>
          </div>
        </li>
      </ul>
    </section>

    <section v-else class="panel">
      <div class="panel-head">
        <h2>{{ t("artifacts") }}</h2>
        <span class="count">0</span>
      </div>
      <p class="empty-hint">{{ t("noArtifactsHint") }}</p>
    </section>

    <ConsoleView
      :job="jobFullName"
      :number="build.number"
      @notify="(text, tone) => emit('notify', text, tone)"
    />
  </section>
</template>

<style scoped>
.build-view {
  flex: 1;
  min-width: 0;
  overflow: auto;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.build-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.back {
  border: 0;
  background: transparent;
  color: var(--muted);
  padding: 0;
  margin-bottom: 8px;
  font-size: 12px;
}

.back:hover {
  color: var(--accent);
}

.title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

h1 {
  margin: 0;
  font-size: 22px;
}

.full-name {
  margin: 4px 0 0;
  color: var(--muted);
  font-size: 12px;
}

.cause {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 12px;
}

.danger {
  border: 1px solid color-mix(in srgb, var(--danger) 45%, transparent);
  border-radius: var(--radius-sm);
  padding: 8px 13px;
  background: transparent;
  color: var(--danger);
  font-weight: 600;
}

.ghost {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 8px 13px;
  background: transparent;
  color: var(--muted);
  font-weight: 600;
}

.ghost:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.ghost:disabled {
  opacity: 0.55;
  cursor: default;
  border-color: var(--border);
  color: var(--muted);
}

.ghost.done {
  border-color: var(--success);
  color: var(--success);
}

.status-pill {
  padding: 3px 9px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  background: var(--panel-soft);
  color: var(--muted);
}

.status-pill[data-tone="success"] {
  background: color-mix(in srgb, var(--success) 14%, transparent);
  color: var(--success);
}

.status-pill[data-tone="danger"] {
  background: color-mix(in srgb, var(--danger) 14%, transparent);
  color: var(--danger);
}

.status-pill[data-tone="warning"] {
  background: #d99b1a1f;
  color: #b47d0d;
}

.status-pill[data-tone="info"] {
  background: var(--accent-soft);
  color: var(--accent);
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--panel);
}

.meta-label {
  color: var(--muted);
  font-size: 11px;
}

.meta strong {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}

.panel {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 16px;
}

.params {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 8px;
  margin: 0;
  padding: 0;
}

.param {
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  background: var(--panel-soft);
}

.param dt {
  overflow: hidden;
  color: var(--muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.param dd {
  margin: 4px 0 0;
  font-family: var(--mono);
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.panel-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 10px;
}

h2 {
  margin: 0;
  font-size: 14px;
}

.count {
  color: var(--muted);
  font-size: 12px;
}

.changes {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  font-size: 12px;
}

.changes th {
  text-align: left;
  color: var(--muted);
  font-weight: 600;
  padding: 7px 8px;
  border-bottom: 1px solid var(--border);
}

.changes td {
  padding: 8px;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}

.col-commit {
  width: 116px;
}

.col-time {
  width: 104px;
  white-space: nowrap;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.col-author {
  width: 104px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.col-files {
  width: 56px;
  text-align: right;
  font-variant-numeric: tabular-nums;
  color: var(--muted);
}

.commit-id {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 0;
  padding: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
}

.commit-id code {
  font-size: 11px;
  color: var(--text);
}

.commit-id:hover code {
  color: var(--accent);
}

.copied {
  color: var(--success);
  font-size: 11px;
}

.message {
  line-height: 1.5;
}

.subject {
  word-break: break-word;
}

.toggle-body {
  margin-left: 6px;
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--accent);
  font-size: 11px;
  cursor: pointer;
}

.toggle-body:hover {
  text-decoration: underline;
}

.body {
  margin: 6px 0 0;
  color: var(--muted);
  font-family: inherit;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.more {
  margin-top: 10px;
  text-align: center;
}

.empty-hint {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.artifacts {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
}

.artifacts li {
  padding: 7px 9px;
  border-radius: var(--radius-sm);
  background: var(--panel-soft);
}

.artifact-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
}

.artifact-info {
  display: flex;
  align-items: baseline;
  gap: 10px;
  min-width: 0;
}

.artifact-name {
  font-weight: 600;
  white-space: nowrap;
}

.artifacts code {
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.artifact-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.artifact-size {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.artifact-error {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--danger);
  font-size: 11px;
}

.artifact-head-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.artifact-dir {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--muted);
  font-size: 11px;
}

.artifact-batch {
  padding: 4px 10px;
  font-size: 11px;
}

.artifact-download {
  padding: 4px 10px;
  font-size: 11px;
}

.artifact-progress {
  display: block;
  margin-top: 6px;
  height: 3px;
  border-radius: 999px;
  background: var(--border);
  overflow: hidden;
}

.artifact-progress-fill {
  display: block;
  height: 100%;
  background: var(--accent);
  transition: width 200ms linear;
}

.artifact-transfer {
  margin-top: 4px;
  color: var(--muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.artifact-saved {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  padding-left: 2px;
}

.artifact-saved code {
  flex: 1;
  min-width: 0;
}

.artifact-copy {
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--accent);
  font-size: 11px;
  cursor: pointer;
}

.artifact-copy:hover {
  text-decoration: underline;
}
</style>
