<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { describeError, jenkinsApi } from "../api";
import { locale, t } from "../i18n";
import { copyText } from "../lib/clipboard";
import { countMatches, highlightSegments, tailLines, trimLogTail } from "../lib/console";
import { errorText } from "../lib/errors";
import { formatBytes, shortenPath } from "../lib/format";
import { isTaskFinished, taskPercent } from "../lib/taskProgress";
import type { ToastTone } from "../lib/toast";
import {
  countLevels,
  filterLines,
  firstVisibleLine,
  matchRows,
  parseAnsi,
  stripAnsi,
  type LogRow,
} from "../lib/logLines";

const props = defineProps<{
  job: string;
  number: number;
}>();

const emit = defineEmits<{
  /** 导出完成/失败的消息提示，由 BuildView 转发给 App 的 toast。 */
  notify: [text: string, tone?: ToastTone];
}>();

const MAX_LINES = 5000;
/** 日志文本内存上限：约 2MB，超出只留尾部（显示本来就只截 5000 行）。 */
const MAX_TEXT_CHARS = 2_000_000;
/** 增量拉取间隔。 */
const POLL_DELAY_MS = 1500;
/** 距底部超过这个距离就认为用户在看历史，自动取消跟随。 */
const FOLLOW_SLOP_PX = 48;

const text = ref("");
const nextStart = ref(0);
const moreData = ref(false);
const error = ref("");
const loading = ref(true);
const follow = ref(true);
const query = ref("");
const mode = ref<"all" | "problems">("all");
const wrap = ref(true);
const fullscreen = ref(false);
const copied = ref(false);
const copiedLine = ref(0);
/** 日志导出走 sidecar 落盘：DBX 沙箱没有 allow-downloads，浏览器下载会被静默拦截。 */
const exporting = ref(false);
/** 导出进度（0-100），总量未知时保持 0。 */
const exportPercent = ref(0);
const exportedPath = ref("");
const exportError = ref("");
let exportTimer: number | undefined;
const activeMatch = ref(0);
const logEl = ref<HTMLElement | null>(null);

let timer: number | undefined;
let generation = 0;
/** 正在飞行中的轮次所属 generation，-1 表示空闲；按代际记录，reset 后新轮不受旧请求阻挡。 */
let inFlight = -1;
/**
 * 程序化滚动的代际：> 0 表示正在滚到底，期间忽略 scroll 事件。
 * 首屏 5000 行刚插入时滚动高度还在变，滚到底会被误判成「用户上翻」而关掉跟随。
 */
let scrollGuard = 0;

const lines = computed(() => tailLines(text.value, MAX_LINES));
const totalLines = computed(() => {
  if (!text.value) return 0;
  const parts = text.value.split(/\r?\n/);
  return parts[parts.length - 1] === "" ? parts.length - 1 : parts.length;
});
const truncated = computed(() => totalLines.value > MAX_LINES);
/** 缓冲区被截断时，第一条可见行的行号（行号仅在当前缓冲区内有效）。 */
const firstLine = computed(() => firstVisibleLine(totalLines.value, lines.value.length));

const allRows = computed(() => filterLines(lines.value, "all", firstLine.value));
const rows = computed(() =>
  mode.value === "all"
    ? allRows.value
    : allRows.value.filter((row) => row.level === "error" || row.level === "warn"),
);
const renderRows = computed(() =>
  rows.value.map((row) => ({ ...row, spans: parseAnsi(row.text) })),
);
const levels = computed(() => countLevels(lines.value));
const issueTotal = computed(() => levels.value.error + levels.value.warn);
const levelsTitle = computed(
  () =>
    `${t("logErrors", { count: levels.value.error })} · ${t("logWarnings", {
      count: levels.value.warn,
    })}`,
);

const matchList = computed(() => matchRows(rows.value.map((row) => row.text), query.value));
const matchTotal = computed(() =>
  countMatches(
    rows.value.map((row) => stripAnsi(row.text)),
    query.value,
  ),
);
const activeLine = computed(() => {
  if (!matchList.value.length) return 0;
  const index = Math.min(activeMatch.value, matchList.value.length - 1);
  return rows.value[matchList.value[index]]?.number ?? 0;
});

/**
 * 滚到底并等布局稳定：首屏 5000 行带 content-visibility，一次设置 scrollTop 可能
 * 停在「估算高度」的位置，随后真实行高撑开就又不到底了。连补两帧直到距离归零。
 */
function scrollToBottom(): void {
  const element = logEl.value;
  if (!element) return;
  scrollGuard += 1;
  const guard = scrollGuard;
  const settle = (attempt: number): void => {
    if (guard !== scrollGuard || !element.isConnected) return;
    element.scrollTop = element.scrollHeight;
    if (attempt >= 2) {
      requestAnimationFrame(() => {
        if (guard === scrollGuard) scrollGuard = 0;
      });
      return;
    }
    requestAnimationFrame(() => settle(attempt + 1));
  };
  settle(0);
}

function handleScroll(): void {
  if (scrollGuard > 0) return;
  const element = logEl.value;
  if (!element) return;
  const distance = element.scrollHeight - element.scrollTop - element.clientHeight;
  if (distance > FOLLOW_SLOP_PX) follow.value = false;
}

function jumpToLatest(): void {
  follow.value = true;
  void nextTick(scrollToBottom);
}

async function poll(current: number): Promise<void> {
  if (inFlight === current) return;
  inFlight = current;
  try {
    const chunk = await jenkinsApi.console(props.job, props.number, nextStart.value);
    if (current !== generation) return;
    if (chunk.text) {
      text.value = trimLogTail(text.value + chunk.text, MAX_TEXT_CHARS);
    }
    nextStart.value = chunk.nextStart;
    moreData.value = chunk.moreData;
    error.value = "";
    await nextTick();
    if (follow.value) scrollToBottom();
    // 后台不续排：回到前台由 visibilitychange 接着拉，避免后台空转。
    if (chunk.moreData && !document.hidden) {
      timer = window.setTimeout(() => void poll(current), POLL_DELAY_MS);
    }
  } catch (caught) {
    if (current !== generation) return;
    error.value = errorText(describeError(caught), locale.value);
  } finally {
    if (current === generation) {
      inFlight = -1;
      loading.value = false;
    }
  }
}

/** 页面隐藏时取消已排的下一轮；回到前台且还有增量就立刻续上。 */
function handleVisibility(): void {
  if (document.hidden) {
    if (timer !== undefined) {
      window.clearTimeout(timer);
      timer = undefined;
    }
    return;
  }
  if (moreData.value && inFlight !== generation) void poll(generation);
}

function reset(): void {
  generation += 1;
  if (timer) window.clearTimeout(timer);
  text.value = "";
  nextStart.value = 0;
  moreData.value = false;
  error.value = "";
  loading.value = true;
  copied.value = false;
  copiedLine.value = 0;
  activeMatch.value = 0;
  // 切构建时恢复跟随：上一次手动上翻不该影响新打开的这一份日志。
  follow.value = true;
  scrollGuard = 0;
  void poll(generation);
}

/** 定位到第 index 个命中行（按下标循环），并把它滚到视口中间。 */
function focusMatch(index: number): void {
  const list = matchList.value;
  if (!list.length) return;
  const normalized = ((index % list.length) + list.length) % list.length;
  activeMatch.value = normalized;
  const lineNumber = rows.value[list[normalized]]?.number;
  if (!lineNumber) return;
  void nextTick(() => {
    const element = logEl.value?.querySelector<HTMLElement>(`[data-line="${lineNumber}"]`);
    element?.scrollIntoView({ block: "center" });
  });
}

function stepMatch(delta: number): void {
  focusMatch(activeMatch.value + delta);
}

function onSearchEnter(event: KeyboardEvent): void {
  stepMatch(event.shiftKey ? -1 : 1);
}

async function copyLog(): Promise<void> {
  const ok = await copyText(text.value);
  copied.value = ok;
  if (ok) window.setTimeout(() => (copied.value = false), 1500);
}

/** 点行号复制这一行（已剥离 ANSI，粘出去可读）。 */
async function copyLine(row: LogRow): Promise<void> {
  const ok = await copyText(stripAnsi(row.text));
  if (!ok) return;
  copiedLine.value = row.number;
  window.setTimeout(() => {
    if (copiedLine.value === row.number) copiedLine.value = 0;
  }, 1500);
}

function stopExportPoll(): void {
  if (exportTimer !== undefined) {
    window.clearTimeout(exportTimer);
    exportTimer = undefined;
  }
}

/** 轮询导出任务：全量日志可能有几十 MB，按百分比显示进度。 */
function pollExport(taskId: string): Promise<void> {
  return new Promise((resolve) => {
    const tick = async (): Promise<void> => {
      try {
        const snapshot = await jenkinsApi.artifactStatus(taskId);
        exportPercent.value = taskPercent(snapshot.done, snapshot.total);

        if (!isTaskFinished(snapshot.state)) {
          exportTimer = window.setTimeout(() => void tick(), 500);
          return;
        }
        if (snapshot.state === "saved") {
          const saved = snapshot.path ?? "";
          exportedPath.value = saved;
          emit("notify", t("downloadDoneToast", {
            name: snapshot.fileName || `${props.number}.log`,
            size: formatBytes(snapshot.total),
          }));
          window.setTimeout(() => {
            if (exportedPath.value === saved) exportedPath.value = "";
          }, 6000);
        } else if (snapshot.state === "failed") {
          exportError.value = errorText(snapshot.error || t("artifactFailed"), locale.value);
          emit("notify", t("downloadFailedToast", { message: exportError.value }), "info");
        }
        resolve();
      } catch (error) {
        exportError.value = describeError(error);
        emit("notify", t("downloadFailedToast", { message: exportError.value }), "info");
        resolve();
      }
    };
    void tick();
  });
}

async function downloadLog(): Promise<void> {
  if (exporting.value) return;
  exporting.value = true;
  exportPercent.value = 0;
  exportedPath.value = "";
  exportError.value = "";
  stopExportPoll();

  try {
    // 导出完整日志：后端直接拉 Jenkins 的 consoleText 落盘，不受前端 2MB 缓冲限制。
    const task = await jenkinsApi.consoleSave(props.job, props.number);
    await pollExport(task.taskId);
  } catch (error) {
    // 老 sidecar 没有 consoleSave 时退回内存文本导出，至少能拿到当前已加载的内容。
    try {
      const saved = await jenkinsApi.saveText(
        `${props.job.replaceAll("/", "-")}-${props.number}.log`,
        text.value,
      );
      exportedPath.value = saved.path;
      emit("notify", t("downloadDoneToast", {
        name: saved.fileName,
        size: formatBytes(saved.size),
      }));
    } catch {
      exportError.value = describeError(error);
      emit("notify", t("downloadFailedToast", { message: exportError.value }), "info");
    }
  } finally {
    exporting.value = false;
  }
}

async function openExportedLog(): Promise<void> {
  if (!exportedPath.value) return;
  try {
    await jenkinsApi.revealFile(exportedPath.value);
  } catch {
    // 文件可能已被移走：保留路径提示即可。
  }
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape" && fullscreen.value) fullscreen.value = false;
}

watch(
  () => [props.job, props.number],
  () => reset(),
  { immediate: true },
);

watch(query, () => {
  if (!query.value.trim()) {
    activeMatch.value = 0;
    return;
  }
  focusMatch(0);
});

watch(mode, () => {
  activeMatch.value = 0;
});

watch(follow, (enabled) => {
  if (enabled) void nextTick(scrollToBottom);
});

onMounted(() => {
  document.addEventListener("visibilitychange", handleVisibility);
  document.addEventListener("keydown", handleKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener("visibilitychange", handleVisibility);
  document.removeEventListener("keydown", handleKeydown);
  stopExportPoll();
  generation += 1;
  if (timer) window.clearTimeout(timer);
});
</script>

<template>
  <section class="console" :class="{ fullscreen }">
    <div class="console-bar">
      <strong>{{ t("console") }} · #{{ number }}</strong>

      <div class="seg">
        <button type="button" :class="{ on: mode === 'all' }" @click="mode = 'all'">
          {{ t("allLines") }}
        </button>
        <button
          type="button"
          :class="{ on: mode === 'problems' }"
          :title="levelsTitle"
          @click="mode = 'problems'"
        >
          {{ t("problemsOnly") }}<span v-if="issueTotal" class="badge">{{ issueTotal }}</span>
        </button>
      </div>

      <input
        v-model="query"
        type="search"
        class="search"
        :placeholder="t('searchLog')"
        @keydown.enter.prevent="onSearchEnter"
      />
      <span v-if="query" class="matches" :title="t('matchCount', { count: matchTotal })">
        {{ matchList.length ? activeMatch + 1 : 0 }}/{{ matchList.length }}
      </span>
      <button
        type="button"
        class="ghost icon"
        :disabled="!matchList.length"
        :title="t('searchPrev')"
        @click="stepMatch(-1)"
      >
        ↑
      </button>
      <button
        type="button"
        class="ghost icon"
        :disabled="!matchList.length"
        :title="t('searchNext')"
        @click="stepMatch(1)"
      >
        ↓
      </button>

      <label class="toggle">
        <input v-model="follow" type="checkbox" />
        <span>{{ t("follow") }}</span>
      </label>

      <div class="tools">
        <button type="button" class="ghost" :class="{ on: wrap }" @click="wrap = !wrap">
          {{ t("wrapLines") }}
        </button>
        <button type="button" class="ghost" @click="copyLog">
          {{ copied ? t("copied") : t("copy") }}
        </button>
        <button
          type="button"
          class="ghost"
          :title="exportedPath || t('downloadBuffer')"
          :disabled="exporting"
          @click="downloadLog"
        >
          {{
            exporting
              ? t("artifactSavingProgress", { percent: exportPercent })
              : t("download")
          }}
        </button>
        <button
          v-if="exportedPath"
          type="button"
          class="ghost"
          :title="exportedPath"
          @click="openExportedLog"
        >
          {{ t("artifactOpenFolder") }}
        </button>
        <span v-if="exportedPath" class="export-note" :title="exportedPath">
          {{ shortenPath(exportedPath) }}
        </span>
        <span v-else-if="exportError" class="export-note export-failed" :title="exportError">
          {{ exportError }}
        </span>
        <button type="button" class="ghost" @click="fullscreen = !fullscreen">
          {{ fullscreen ? t("exitFullscreen") : t("enterFullscreen") }}
        </button>
      </div>
    </div>

    <p v-if="error" class="banner">
      <span>{{ error }}</span>
      <button type="button" @click="reset">{{ t("retry") }}</button>
    </p>
    <p v-if="truncated" class="hint">{{ t("logTruncated") }}</p>

    <div ref="logEl" class="log-body" :class="{ nowrap: !wrap }" @scroll.passive="handleScroll">
      <div v-if="!rows.length" class="empty">
        <p v-if="loading">{{ t("logLoading") }}</p>
        <p v-else-if="error">{{ t("consoleLoadFailed") }}</p>
        <p v-else-if="mode === 'problems'">{{ t("logNoProblems") }}</p>
        <p v-else>{{ t("logEmpty") }}</p>
      </div>
      <div v-else class="log-list">
        <div
          v-for="row in renderRows"
          :key="row.number"
          class="log-row"
          :class="{ active: row.number === activeLine }"
          :data-line="row.number"
          :data-level="row.level"
        >
          <button
            type="button"
            class="ln"
            :class="{ copied: copiedLine === row.number }"
            :title="t('copyLine')"
            @click="copyLine(row)"
          >
            {{ copiedLine === row.number ? "✓" : row.number }}
          </button>
          <span class="msg"><template v-for="(span, spanIndex) in row.spans" :key="spanIndex"><span
                :class="[span.tone ? `tone-${span.tone}` : '', { bold: span.bold }]"
              ><template
                v-for="(segment, segmentIndex) in highlightSegments(span.text, query)"
                :key="segmentIndex"
              ><span :class="{ hit: segment.match }">{{ segment.text }}</span></template></span></template></span>
        </div>
      </div>
    </div>

    <footer v-if="lines.length" class="log-foot">
      <span class="dot" :class="{ live: moreData }"></span>
      <span>{{ moreData ? t("buildingLive") : t("logFinished") }}</span>
      <span class="foot-count">
        {{ t("lineCount", { total: totalLines, shown: lines.length }) }}
      </span>
      <button v-if="!follow" type="button" class="latest" @click="jumpToLatest">
        ↓ {{ t("jumpToLatest") }}
      </button>
    </footer>
  </section>
</template>

<style scoped>
.console {
  /* BuildView 是 flex column：不锁住的话面板会被压短，日志的 min-height
     又顶住不让缩，底部（footer / 末行）就会被 overflow:hidden 裁掉。 */
  flex: 0 0 auto;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--panel);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 320px;
}

.console.fullscreen {
  position: fixed;
  inset: 0;
  z-index: 60;
  border-radius: 0;
  min-height: 0;
}

.console-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border);
  flex-wrap: wrap;
}

.console-bar strong {
  font-size: 13px;
  margin-right: 4px;
  white-space: nowrap;
}

.seg {
  display: inline-flex;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.seg button {
  border: 0;
  padding: 4px 9px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
}

.seg button + button {
  border-left: 1px solid var(--border);
}

.seg button.on {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}

.badge {
  margin-left: 5px;
  padding: 0 5px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--danger) 16%, transparent);
  color: var(--danger);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.search {
  width: 150px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 5px 8px;
  background: var(--panel-soft);
  color: var(--text);
  font: inherit;
  font-size: 12px;
  outline: none;
}

.search:focus {
  border-color: var(--accent);
}

.matches {
  color: var(--muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.tools {
  margin-left: auto;
  display: flex;
  gap: 6px;
}

.toggle {
  display: flex;
  align-items: center;
  gap: 5px;
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}

.ghost {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 4px 9px;
  background: transparent;
  color: var(--text);
  font-size: 12px;
  white-space: nowrap;
}

.ghost.icon {
  padding: 4px 7px;
  line-height: 1;
}

.ghost:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
}

.ghost.on {
  border-color: var(--accent);
  color: var(--accent);
  background: var(--accent-soft);
}

.ghost:disabled {
  opacity: 0.45;
  cursor: default;
}

.export-note {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.export-failed {
  color: var(--danger);
}

.banner {
  margin: 0;
  padding: 8px 12px;
  background: color-mix(in srgb, var(--danger) 12%, transparent);
  color: var(--danger);
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.banner button {
  border: 1px solid currentColor;
  border-radius: var(--radius-sm);
  padding: 2px 8px;
  background: transparent;
  color: inherit;
  font-size: 11px;
}

.hint {
  margin: 0;
  padding: 6px 12px;
  color: var(--muted);
  font-size: 11px;
  border-bottom: 1px solid var(--border);
}

.log-body {
  flex: 1;
  min-height: 260px;
  max-height: clamp(420px, 64vh, 900px);
  overflow: auto;
  background: #101319;
}

.console.fullscreen .log-body {
  max-height: none;
}

.log-list {
  min-width: 100%;
  padding: 8px 0;
  font-family: var(--mono);
  font-size: 12px;
  line-height: 1.65;
  color: #d7dce5;
}

.log-row {
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr);
  align-items: start;
  border-left: 2px solid transparent;
  /* 不用 content-visibility：它按 contain-intrinsic-size 估算未渲染行的高度，
     折行行实际更高，scrollHeight 会偏小 —— 滚动条拖到底也看不到最后几行。 */
}

.log-row[data-level="error"] {
  border-left-color: #ff6b6b;
  background: #1c1114;
}

.log-row[data-level="warn"] {
  border-left-color: #e0a04a;
  background: #1a1510;
}

.log-row:hover {
  background: #171b22;
}

.log-row.active {
  background: #1d1a12;
}

.ln {
  position: sticky;
  left: 0;
  z-index: 1;
  border: 0;
  padding: 0 8px 0 0;
  background: inherit;
  color: #4f5765;
  font: inherit;
  font-variant-numeric: tabular-nums;
  text-align: right;
  user-select: none;
  cursor: pointer;
}

.ln:hover {
  color: #9aa3b2;
  text-decoration: underline;
}

.ln.copied {
  color: #1a9e6d;
}

.msg {
  display: block;
  padding-left: 1.1em;
  padding-right: 12px;
  text-indent: -1.1em;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-body.nowrap .msg {
  white-space: pre;
  word-break: normal;
  text-indent: 0;
}

.log-body.nowrap .log-row {
  grid-template-columns: 52px minmax(max-content, 1fr);
}

.tone-red {
  color: #ff8f8f;
}

.tone-green {
  color: #7fd6a5;
}

.tone-yellow {
  color: #e8c06a;
}

.tone-blue {
  color: #7fb3e8;
}

.tone-magenta {
  color: #e39bd8;
}

.tone-cyan {
  color: #79d2d2;
}

.tone-gray {
  color: #8b93a1;
}

.bold {
  font-weight: 600;
}

.hit {
  background: #ffd54f;
  color: #101319;
  border-radius: 2px;
}

.log-row.active .hit {
  background: #ff9f43;
}

.empty {
  color: #8b93a1;
  font-size: 12px;
  text-align: center;
}

.empty p {
  margin: 0;
  padding: 28px 12px;
}

.log-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 12px;
  border-top: 1px solid var(--border);
  color: var(--muted);
  font-size: 11px;
  background: var(--panel);
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--muted);
  flex: none;
}

.dot.live {
  background: var(--success);
}

.foot-count {
  margin-left: auto;
  font-variant-numeric: tabular-nums;
}

.latest {
  border: 1px solid var(--border);
  border-radius: 999px;
  padding: 1px 9px;
  background: transparent;
  color: var(--text);
  font-size: 11px;
  white-space: nowrap;
}

.latest:hover {
  border-color: var(--accent);
  color: var(--accent);
}
</style>
