<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import logoUrl from "../assets/plugin.svg";
import { describeError, jenkinsApi, setConnectionId } from "./api";
import BatchBuildDialog from "./components/BatchBuildDialog.vue";
import BuildView from "./components/BuildView.vue";
import ConfirmAction from "./components/ConfirmAction.vue";
import ConfirmDelete from "./components/ConfirmDelete.vue";
import JobEditor from "./components/JobEditor.vue";
import JobList from "./components/JobList.vue";
import JobView from "./components/JobView.vue";
import NodesView from "./components/NodesView.vue";
import OverviewView from "./components/OverviewView.vue";
import ParamDialog from "./components/ParamDialog.vue";
import QueueView from "./components/QueueView.vue";
import StageView from "./components/StageView.vue";
import ToastHost from "./components/ToastHost.vue";
import ViewEditor from "./components/ViewEditor.vue";
import { locale, syncLocale, t } from "./i18n";
import {
  MAX_BATCH_JOBS,
  abortedChunk,
  batchTargets,
  buildParamValues,
  chunkJobs,
  collectSharedParameters,
  countOutsideCurrent,
  failedChunk,
  rememberTargets,
  selectVisible,
  shouldAbortChunks,
  summarizeBatch,
  toggleJobSelection,
  type BatchTargetMap,
} from "./lib/batchBuild";
import {
  createCancelRequest,
  createStopRequest,
  isStopTarget,
  type CancelRequest,
  type ConfirmRequest,
  type StopRequest,
} from "./lib/confirmRequest";
import { resolveEnterActions, shouldRefreshOnEnter, type AppView } from "./lib/enterRefresh";
import { errorText } from "./lib/errors";
import { createHistoryBridge } from "./lib/historyBridge";
import {
  NAV_MOUSE_FALLBACK_MS,
  createNavDeduper,
  resolveMouseButton,
  type NavDirection,
} from "./lib/mouseNav";
import { createNavStack, type NavEntry } from "./lib/navStack";
import { diffBuildRows, type RefreshUi } from "./lib/refreshStatus";
import { createToastQueue, type ToastItem, type ToastTone } from "./lib/toast";
import {
  AUTO_REFRESH_INTERVAL_MS,
  OVERVIEW_AUTO_REFRESH_INTERVAL_MS,
  OVERVIEW_IDLE_REFRESH_INTERVAL_MS,
  POLL_TIMEOUT_MS,
  QUEUE_AUTO_REFRESH_INTERVAL_MS,
  STAGE_AUTO_REFRESH_INTERVAL_MS,
  createAutoRefresher,
  createPendingBuild,
  evaluatePendingBuild,
  hasRunningBuild,
  hasRunningStage,
  resolveOverviewRefresh,
  shouldPollBuildDetail,
  shouldRefreshJobsAfterPending,
  type AutoRefresher,
  type PendingBuild,
} from "./lib/refresh";
import {
  appendJobName,
  containsJob,
  isNamedView,
  needsJoinView,
  resolveJoinAction,
} from "./lib/viewRefresh";
import type {
  BatchBuildResult,
  BatchTargetInfo,
  BuildDetail,
  BuildSummary,
  CreateJobPayload,
  CreateViewPayload,
  ExecutorInfo,
  JobConfigResponse,
  JobDetail,
  JobSummary,
  JobUpdateFields,
  NodeInfo,
  Overview,
  ParameterDefinition,
  QueueItem,
  StageInfo,
  StageMatrix,
  ViewConfigResponse,
  ViewInfo,
  ViewUpdateFields,
} from "./types";

interface Identity {
  name: string;
  version: string;
  mode: string;
}

const connectionName = ref("");
const identity = ref<Identity | null>(null);
const readOnly = ref(true);

/** 构建时注入的插件版本，用于一眼识别当前加载的是哪一版 UI。 */
const appVersion = __APP_VERSION__;

const view = ref<AppView>("overview");
const folder = ref("");
const jobs = ref<JobSummary[]>([]);
const views = ref<ViewInfo[]>([]);
const activeView = ref("");
const selectedJob = ref<JobDetail | null>(null);
const builds = ref<BuildSummary[]>([]);
/** 正在构建的那一次构建的阶段快照，供列表进度条按阶段步进。 */
const buildStageSnapshot = ref<{ number: number; stages: StageInfo[] } | null>(null);
const selectedBuild = ref<BuildDetail | null>(null);
const stageMatrix = ref<StageMatrix | null>(null);
const loadingStages = ref(false);
const buildReturnView = ref<"jobs" | "stages">("jobs");
const overview = ref<Overview | null>(null);
const queueItems = ref<QueueItem[]>([]);
const nodes = ref<NodeInfo[]>([]);
/** 概览页的执行器槽位明细。 */
const executors = ref<ExecutorInfo[]>([]);

/**
 * 页面导航历史：鼠标侧键、宿主的前进后退、页面内「返回」共用这一条路径。
 *
 * 内部栈是唯一真相；宿主 history 只做 1:1 镜像，好让系统级的 back/forward
 * 能落到我们的 popstate 上。镜像写不进去（file:// 等）时自动降级为内部栈。
 */
const navStack = createNavStack({
  view: "overview",
  activeView: "",
  folder: "",
  job: "",
  build: null,
});
/** 同一次按键可能派发多个事件，也可能同时命中宿主与 DOM 两条通道。 */
const navDedupe = createNavDeduper();
const navHistory = createHistoryBridge();
const canNavBack = ref(false);
const canNavForward = ref(false);
/** 回放历史期间为 true：期间的内部状态变化不再记入历史。 */
let navApplying = false;
/** DOM 通道的待决项：popstate 一到就说明宿主接了这一下，取消自己的兜底。 */
let navAwaitingPop: { direction: NavDirection; timer: number } | null = null;

const loadingJobs = ref(false);
const loadingJob = ref(false);
const loadingBuilds = ref(false);
const loadingView = ref(false);
/** 执行器面板独立加载态：和概览主数据并行拉，互不阻塞。 */
const loadingExecutors = ref(false);
const showParams = ref(false);
const errorMessage = ref("");
/** 连续刷新失败被护栏暂停时置位，用独立提示条引导手动恢复。 */
const refreshPaused = ref(false);
/** 当前这一轮自动刷新是否正在请求（驱动徽章的「请求中」态）。 */
const pollInFlight = ref(false);
/** 上一次成功刷新的时间戳，徽章显示成 HH:mm:ss。 */
const lastRefreshAt = ref<number | null>(null);
/** 手动刷新按钮的三态：进行中 / 刚完成。 */
const manualRefreshing = ref(false);
const manualJustDone = ref(false);
/** 刚发生变化的构建号，用于表格行短暂高亮。 */
const changedBuilds = ref<number[]>([]);
/** 操作提示改为浮层 toast：只占视觉，不挤压页面布局。 */
const toasts = ref<ToastItem[]>([]);
const toastQueue = createToastQueue({
  onChange: (items) => (toasts.value = items),
});

type JobEditorState =
  | { mode: "create" }
  | { mode: "edit"; config: JobConfigResponse };

type ViewEditorState =
  | { mode: "create" }
  | { mode: "edit"; view: ViewInfo; config: ViewConfigResponse };

interface DeleteTarget {
  kind: "job" | "view";
  name: string;
  fullName?: string;
}

const jobEditor = ref<JobEditorState | null>(null);
const viewEditor = ref<ViewEditorState | null>(null);
const deleteTarget = ref<DeleteTarget | null>(null);
/** 停止构建 / 取消排队项的二次确认目标（宿主 webview 会吞掉原生 confirm）。 */
const pendingConfirm = ref<ConfirmRequest | null>(null);
const confirmBusy = ref(false);
const editorBusy = ref(false);
const viewJobPool = ref<string[]>([]);

/** 批量构建：多选模式、选中集合、确认弹窗（预检 → 确认 → 执行 → 结果）。 */
const batchMode = ref(false);
const batchSelected = ref<string[]>([]);
/** 跨视图的已选任务快照：切到别的视图后仍能展示名字与状态。 */
const batchTargetMap = ref<BatchTargetMap>({});
/** 弹窗里「跳过正在构建 / 排队中」的勾选值，默认开启。 */
const batchSkipBusy = ref(true);
/** 本批共用参数：并集定义（仅 Boolean 开关）与用户填写值。 */
const batchParamDefs = ref<ParameterDefinition[]>([]);
const batchParamValues = ref<Record<string, unknown>>({});
const batchDialog = ref<BatchDialogState | null>(null);

interface BatchDialogState {
  phase: "inspecting" | "confirm" | "running" | "done";
  /** 预检结果（降级时为缓存快照）。 */
  targets: BatchTargetInfo[];
  results: BatchBuildResult[];
  error: string;
  /** 执行进度：已返回条数 / 总条数。 */
  done: number;
  total: number;
}

/** 已选任务里有多少不在当前列表（跨视图提示）。 */
const batchOutsideCount = computed(() =>
  countOutsideCurrent(batchSelected.value, jobs.value),
);

const canManageView = computed(() => isNamedView(activeView.value));

const confirmTitle = computed(() => {
  const request = pendingConfirm.value;
  if (!request) return "";
  return request.kind === "stop"
    ? t("stopConfirm", { number: request.number })
    : t("cancelConfirm", { id: request.id });
});

/** 弹窗里点明操作对象，避免在多个任务间切来切去时停错构建。 */
const confirmDetail = computed(() => {
  const request = pendingConfirm.value;
  if (!request) return "";
  return request.kind === "stop" ? request.job : request.jobName;
});

const confirmHint = computed(() => {
  const request = pendingConfirm.value;
  if (!request) return "";
  return request.kind === "stop" ? t("stopConfirmHint") : t("cancelConfirmHint");
});

const confirmActionLabel = computed(() => {
  const request = pendingConfirm.value;
  if (!request) return "";
  return request.kind === "stop" ? t("stopAction") : t("cancelAction");
});

/** 已有视图名，用于编辑视图时的重名提示（Jenkins 判重区分大小写）。 */
const viewNames = computed(() =>
  views.value.map((item) => item.name).filter((item) => item !== ""),
);

const connectionLabel = computed(() => {
  if (identity.value?.version) return `Jenkins ${identity.value.version}`;
  return connectionName.value || t("unknownConnection");
});

const isPipeline = computed(() =>
  (selectedJob.value?.class ?? "").toLowerCase().includes("workflowjob"),
);

/** 刚触发、还没在构建列表里出现的构建；存在时构建列表保持自动刷新。 */
const pendingBuild = ref<PendingBuild | null>(null);

type AutoRefreshMode =
  | "builds"
  | "stages"
  | "build"
  | "queue"
  | "overview"
  | "overviewIdle";

/** 当前视图是否需要自动刷新：构建详情在跑、构建列表有构建中/刚触发、阶段有进行中、概览常驻双档，或队列页常驻。 */
const autoRefreshMode = computed<AutoRefreshMode | null>(() => {
  if (view.value === "build" && shouldPollBuildDetail(selectedBuild.value)) {
    return "build";
  }
  if (view.value === "queue") return "queue";
  // 概览页常驻：执行器被占用或队列非空走 5s 活跃档，否则 30s 低频兜底，
  // 这样外部触发的构建、排队项进出、节点上下线都能自己冒出来。
  if (view.value === "overview") {
    return resolveOverviewRefresh(overview.value, executors.value) === "active"
      ? "overview"
      : "overviewIdle";
  }
  if (view.value === "jobs" && selectedJob.value) {
    if (hasRunningBuild(builds.value)) return "builds";
    if (pendingBuild.value?.job === selectedJob.value.fullName) return "builds";
  }
  if (view.value === "stages" && hasRunningStage(stageMatrix.value)) {
    return "stages";
  }
  return null;
});

let autoRefresher: AutoRefresher | null = null;

let jobsToken = 0;
let selectToken = 0;
let stageToken = 0;
let buildToken = 0;
/** 概览页两个数据源各留一个 token：轮询返回时若已被手动刷新超越就作废。 */
let overviewToken = 0;
let executorsToken = 0;

function setError(error: unknown): void {
  errorMessage.value = errorText(describeError(error), locale.value);
}

/** 记录一次成功刷新，供徽章显示「上次更新」时间。 */
function markRefreshed(): void {
  lastRefreshAt.value = Date.now();
}

let lastBuildJob = "";
let lastBuildRows: BuildSummary[] | null = null;
let changedTimer: number | undefined;

/**
 * 写入构建列表并算出本次变化了哪些行。
 * 只比较同一任务的前后两次数据，切任务时重置基线，避免整表高亮。
 */
function applyBuilds(job: string, next: BuildSummary[]): void {
  const baseline = lastBuildJob === job ? lastBuildRows : null;
  const changed = diffBuildRows(baseline, next);
  lastBuildJob = job;
  lastBuildRows = next;
  builds.value = next;
  if (!changed.length) return;
  changedBuilds.value = changed;
  if (changedTimer) window.clearTimeout(changedTimer);
  changedTimer = window.setTimeout(() => {
    changedBuilds.value = [];
  }, 1600);
}

/** 清空构建列表与变化基线。 */
function resetBuilds(): void {
  builds.value = [];
  buildStageSnapshot.value = null;
  lastBuildJob = "";
  lastBuildRows = null;
  changedBuilds.value = [];
}

/**
 * 拉取正在构建的那一次构建的阶段，供进度条按阶段步进。
 * 只有 Pipeline 且确实有构建在跑时才发请求；拿不到阶段（Freestyle、未装
 * pipeline-rest-api）就清空快照，进度回退到时长估算。
 */
async function loadBuildStages(job: string, list: BuildSummary[]): Promise<void> {
  const running = isPipeline.value ? list.find((build) => build.building) : undefined;
  if (!running) {
    buildStageSnapshot.value = null;
    return;
  }
  try {
    const result = await jenkinsApi.buildStages(job, running.number, POLL_TIMEOUT_MS);
    if (selectedJob.value?.fullName !== job) return;
    buildStageSnapshot.value = { number: running.number, stages: result.stages ?? [] };
  } catch {
    // 阶段只是进度的加分项：失败要静默回退，不能冒泡给轮询护栏，
    // 否则一个非核心请求会把整个自动刷新停掉。
    if (selectedJob.value?.fullName === job) buildStageSnapshot.value = null;
  }
}

/** 成功提示走 success，纯提示（只读拦截等）走 info。 */
function showToast(message: string, tone: ToastTone = "success"): void {
  toastQueue.push(message, tone);
}

function dismissToast(id: number): void {
  toastQueue.dismiss(id);
}

/**
 * 刷新左侧任务列表。
 * silent = true 时不置加载态、不清错误横幅，失败也不弹错，用于构建轮询与新建后的静默补刷。
 * autoSelect = false 时不会自动选中第一个任务——历史回放要还原「这一屏本来没有选中」。
 */
async function loadJobs(path: string, silent = false, autoSelect = true): Promise<void> {
  const token = ++jobsToken;
  if (!silent) {
    loadingJobs.value = true;
    errorMessage.value = "";
  }
  try {
    const useView = path === "" && activeView.value !== "";
    const result = useView
      ? await jenkinsApi.jobs("", activeView.value)
      : await jenkinsApi.jobs(path, "");
    if (token !== jobsToken) return;

    jobs.value = result.jobs;
    folder.value = path;
    // 把当前列表并入跨视图缓存：已选任务切到别的视图后仍有名字与状态可展示。
    batchTargetMap.value = rememberTargets(batchTargetMap.value, result.jobs);
    markRefreshed();

    const current = selectedJob.value?.fullName;
    const stillThere = current
      ? result.jobs.some((job) => job.fullName === current)
      : false;
    if (!stillThere) {
      const first = autoSelect ? result.jobs.find((job) => !job.isFolder) : undefined;
      if (first) {
        void selectJob(first, silent);
      } else {
        selectedJob.value = null;
        resetBuilds();
        selectedBuild.value = null;
        stageMatrix.value = null;
      }
    }
  } catch (error) {
    if (token === jobsToken && !silent) setError(error);
  } finally {
    // 静默请求也可能持有最新 token，必须一并关掉上一个请求留下的加载态。
    if (token === jobsToken) loadingJobs.value = false;
  }
}

async function loadViews(): Promise<void> {
  try {
    const result = await jenkinsApi.views();
    views.value = result.views.length
      ? result.views
      : [{ name: "", description: "", url: "" }];
  } catch {
    // 旧 Jenkins 或权限受限时回退为单个「全部任务」。
    views.value = [{ name: "", description: "", url: "" }];
  }

  if (!views.value.some((item) => item.name === activeView.value)) {
    const preferred =
      views.value.find((item) => item.name.toLowerCase() === "all") ?? views.value[0];
    activeView.value = preferred.name;
  }
}

async function activateView(name: string): Promise<void> {
  activeView.value = name;
  folder.value = "";
  selectedJob.value = null;
  selectedBuild.value = null;
  stageMatrix.value = null;
  resetBuilds();
  pendingBuild.value = null;
  // 批量选择跨视图保留（需求 3）：切视图不再清空，底部条会提示有多少项不在当前视图。
  view.value = "jobs";
  await loadJobs("");
}

async function loadOverview(): Promise<void> {
  const token = ++overviewToken;
  loadingView.value = true;
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.overview();
    if (token !== overviewToken) return;
    overview.value = result;
    markRefreshed();
  } catch (error) {
    if (token === overviewToken) setError(error);
  } finally {
    if (token === overviewToken) loadingView.value = false;
  }
}

/**
 * 拉取执行器槽位明细，供概览页「构建执行状态」面板使用。
 * 独立加载态：面板失败不该把整页概览数字一起拖成错误页。
 */
async function loadExecutors(): Promise<void> {
  const token = ++executorsToken;
  loadingExecutors.value = true;
  try {
    const result = await jenkinsApi.executors();
    if (token !== executorsToken) return;
    executors.value = result.executors;
    markRefreshed();
  } catch (error) {
    if (token === executorsToken) setError(error);
  } finally {
    if (token === executorsToken) loadingExecutors.value = false;
  }
}

/** 概览页刷新入口：聚合数字与执行器明细一起刷。 */
async function refreshOverview(): Promise<void> {
  // 先重置倒计时：否则手动刷新后轮询可能紧接着又发一轮。
  autoRefresher?.restart();
  await Promise.all([loadOverview(), loadExecutors()]);
}

async function loadQueue(): Promise<void> {
  loadingView.value = true;
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.queue();
    queueItems.value = result.items;
    markRefreshed();
  } catch (error) {
    setError(error);
  } finally {
    loadingView.value = false;
  }
}

async function loadNodes(): Promise<void> {
  loadingView.value = true;
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.nodes();
    nodes.value = result.nodes;
    markRefreshed();
  } catch (error) {
    setError(error);
  } finally {
    loadingView.value = false;
  }
}

/** 上一次执行进入刷新的页面：只有同一页重复进入才节流。 */
let lastEnterView: AppView | null = null;
/** 进入刷新进行中：防止连点侧栏叠出多轮请求。 */
let enterRefreshBusy = false;

/**
 * 进入页面时补刷一次当前页的数据。
 *
 * - 节流：换页必刷；同一页距上次成功刷新（含自动轮询）不足 2s 才跳过；
 * - 任务页保留当前文件夹位置，任务列表与选中任务的构建列表都静默刷新，
 *   不闪加载骨架；静默失败的提示交给手动刷新，不打断浏览；
 * - 结束时重置轮询倒计时，避免紧接着又排一轮、两个请求撞在一起。
 */
async function refreshOnEnter(): Promise<void> {
  if (enterRefreshBusy) return;
  const target = view.value;
  const actions = resolveEnterActions(target, {
    hasViews: views.value.length > 0,
    hasSelection: selectedJob.value !== null,
  });
  if (!actions.length) return;
  if (
    !shouldRefreshOnEnter(target, { view: lastEnterView, at: lastRefreshAt.value }, Date.now())
  ) {
    return;
  }

  enterRefreshBusy = true;
  try {
    // loadJobs 可能因选中任务已消失而自动重选：那种情况详情已经拉过，不再重复请求。
    const detailToken = selectToken;
    for (const action of actions) {
      if (action === "views") await loadViews();
      else if (action === "jobs") await loadJobs(folder.value, true);
      else if (action === "overview") await loadOverview();
      else if (action === "executors") await loadExecutors();
      else if (action === "queue") await loadQueue();
      else if (action === "nodes") await loadNodes();
      else if (action === "detail" && selectedJob.value && selectToken === detailToken) {
        await selectJob({ fullName: selectedJob.value.fullName }, true);
      }
    }
  } finally {
    enterRefreshBusy = false;
    lastEnterView = target;
    autoRefresher?.restart();
  }

  // 刷新期间用户又切了页：刚刷完的是旧页，给新页补一次。
  if (target !== view.value) void refreshOnEnter();
}

/* ── 页面导航历史（鼠标侧键 / 宿主前进后退） ───────────────── */

/** 当前状态快照；overrides 用来记录「已经设置好」的目标状态。 */
function navEntry(overrides: Partial<NavEntry> = {}): NavEntry {
  return {
    view: view.value,
    activeView: activeView.value,
    folder: folder.value,
    job: selectedJob.value?.fullName ?? "",
    build: view.value === "build" ? (selectedBuild.value?.number ?? null) : null,
    ...overrides,
  };
}

function syncNavFlags(): void {
  canNavBack.value = navStack.canBack();
  canNavForward.value = navStack.canForward();
}

/** 记录一次用户导航：内部栈压一条，宿主栈跟着压一条（重复条目不写）。 */
function recordNav(entry: NavEntry = navEntry()): void {
  if (navApplying) return;
  const result = navStack.push(entry);
  if (result.added) navHistory.push(result.index);
  syncNavFlags();
}

/** 取消 DOM 通道的待决兜底。 */
function cancelAwaitingPop(): void {
  if (!navAwaitingPop) return;
  window.clearTimeout(navAwaitingPop.timer);
  navAwaitingPop = null;
}

/**
 * 关掉盖在页面最上面的一层弹窗：后退优先收 UI，不消耗页面历史。
 * 写入进行中时只吞掉这次按键——强行关窗会丢掉正在跑的请求状态。
 */
function closeTopLayer(): boolean {
  const layer =
    batchDialog.value ||
    jobEditor.value ||
    viewEditor.value ||
    deleteTarget.value ||
    pendingConfirm.value ||
    showParams.value;
  if (!layer) return false;
  if (editorBusy.value || confirmBusy.value || batchDialog.value?.phase === "running") {
    return true;
  }
  if (batchDialog.value) {
    batchDialog.value = null;
    return true;
  }
  if (jobEditor.value) {
    jobEditor.value = null;
    return true;
  }
  if (viewEditor.value) {
    viewEditor.value = null;
    return true;
  }
  if (deleteTarget.value) {
    deleteTarget.value = null;
    return true;
  }
  if (pendingConfirm.value) {
    pendingConfirm.value = null;
    return true;
  }
  showParams.value = false;
  return true;
}

/**
 * 回放一条历史记录。只恢复状态，不再写新历史。
 *
 * 数据按页补齐：切页走进入刷新；任务 / 构建 / 阶段必须精确定位到
 * 具体任务与构建号，不能依赖「自动选中第一个」，否则退回去看到的
 * 可能是另一条任务。
 */
async function applyEntry(entry: NavEntry): Promise<void> {
  navApplying = true;
  try {
    if (entry.activeView !== activeView.value) activeView.value = entry.activeView;

    if (entry.view === "overview" || entry.view === "queue" || entry.view === "nodes") {
      view.value = entry.view;
      await refreshOnEnter();
      return;
    }

    // 构建详情与阶段视图都挂在某个任务上：先把左侧定位过去。
    view.value = "jobs";
    await loadJobs(entry.folder, true, entry.job !== "");
    if (!entry.job) {
      selectedJob.value = null;
      selectedBuild.value = null;
      stageMatrix.value = null;
      resetBuilds();
      return;
    }
    if (selectedJob.value?.fullName !== entry.job) {
      await selectJob({ fullName: entry.job }, true);
    }
    if (entry.view === "stages") {
      view.value = "stages";
      await loadStages(entry.job);
      return;
    }
    if (entry.view === "build" && entry.build !== null) {
      // rememberReturn = false：回放不算「用户新开详情」，不重复记历史。
      await openBuild(entry.build, false);
    }
  } finally {
    navApplying = false;
    syncNavFlags();
  }
}

/** 回放串行化：连续两次 popstate 不能并行拉数据，否则旧响回填会盖掉新状态。 */
let navQueue: Promise<void> = Promise.resolve();

function enqueueEntry(entry: NavEntry): Promise<void> {
  navQueue = navQueue.then(() => applyEntry(entry)).catch(() => undefined);
  return navQueue;
}

/**
 * 后退一步。
 *
 * - `moved`：已经接管（数据可能还在异步加载）
 * - `closed`：只关了一层弹窗，页面没动
 * - `none`：无处可退，调用方可以走自己的兜底
 */
async function navBack(): Promise<"moved" | "closed" | "none"> {
  if (navApplying) return "none";
  if (closeTopLayer()) return "closed";
  if (!navStack.canBack()) return "none";
  // 宿主栈可用就交给它走一格：popstate 回来时统一回放，避免两套游标各走各的。
  if (navHistory.go(-1)) return "moved";
  navStack.back();
  await enqueueEntry(navStack.current());
  return "moved";
}

async function navForward(): Promise<"moved" | "closed" | "none"> {
  if (navApplying) return "none";
  if (closeTopLayer()) return "closed";
  if (!navStack.canForward()) return "none";
  if (navHistory.go(1)) return "moved";
  navStack.forward();
  await enqueueEntry(navStack.current());
  return "moved";
}

/**
 * 鼠标侧键。宿主有两种处理方式，必须都覆盖：
 *
 * - 当浏览器命令：宿主先发 popstate，我们什么都不做；
 * - 不处理：页面能收到 mouseup（button 3 / 4），由我们发起导航。
 *
 * 所以收到 DOM 事件后先空等 NAV_MOUSE_FALLBACK_MS，期间 popstate 先到就作废自己，
 * 否则一次按键会退两步。
 */
function handleMouseNavButton(event: MouseEvent): void {
  const direction = resolveMouseButton(event.button);
  if (!direction) return;
  // 侧键自身没有默认行为，但宿主可能把它映射成滚动之类，一律拦掉。
  event.preventDefault();
  if (!navDedupe.accept(direction)) return;

  cancelAwaitingPop();
  const timer = window.setTimeout(() => {
    navAwaitingPop = null;
    void (direction === "back" ? navBack() : navForward());
  }, NAV_MOUSE_FALLBACK_MS);
  navAwaitingPop = { direction, timer };
}

/** 宿主或浏览器历史变了：按它给的游标对齐内部栈并回放。 */
function handlePopState(): void {
  // 宿主已经接了这一下，DOM 兜底不必再补一次。
  cancelAwaitingPop();
  const index = navHistory.index();
  if (index === null) {
    // 不是我们写的条目（用户退到了宿主自己的页面）：只校正按钮状态。
    syncNavFlags();
    return;
  }
  void enqueueEntry(navStack.restore(index));
}

/** 侧栏导航：切页后按当前页补刷一次。 */
async function switchView(next: "overview" | "jobs" | "queue" | "nodes"): Promise<void> {
  view.value = next;
  recordNav(navEntry({ view: next }));
  await refreshOnEnter();
}

/** 点任务：切到该任务并记入历史。 */
async function openJob(job: JobSummary): Promise<void> {
  await selectJob(job);
  // 请求期间用户可能已经切走：那一屏不是当前页，不该记进历史。
  if (view.value !== "jobs") return;
  recordNav(navEntry({ job: job.fullName }));
}

/** 文件夹导航（进入子目录 / 点面包屑上跳），也占一格历史。 */
async function navigateFolder(path: string): Promise<void> {
  await loadJobs(path);
  if (view.value !== "jobs") return;
  recordNav(navEntry({ folder: path }));
}

/** 切换视图 tab：同样占一格。 */
async function chooseView(name: string): Promise<void> {
  await activateView(name);
  if (view.value !== "jobs") return;
  recordNav(navEntry());
}

/** 队列「取消」只负责发起确认，真正调用在 confirmPending 里执行。 */
function cancelQueueItem(id: number, jobName: string): void {
  const request = createCancelRequest(id, jobName);
  if (!request) return;
  pendingConfirm.value = request;
}

async function loadIdentity(): Promise<void> {
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.whoami();
    readOnly.value = result.readOnly;
    // 连接变成只读就退出多选：底部操作条留着只会点到「被后端拒绝」。
    if (result.readOnly) {
      batchMode.value = false;
      batchSelected.value = [];
    }
    identity.value = {
      name: result.user?.name ?? "",
      version: result.user?.version ?? "",
      mode: result.user?.mode ?? "",
    };
    await Promise.all([refreshOverview(), loadViews()]);
  } catch (error) {
    setError(error);
  }
}

/**
 * 加载任务详情 + 构建列表。
 * silent = true 时不置加载态、不清错误横幅，用于触发构建后的快速补刷。
 */
async function selectJob(
  job: Pick<JobSummary, "fullName">,
  silent = false,
): Promise<void> {
  const token = ++selectToken;
  view.value = "jobs";
  if (!silent) {
    selectedBuild.value = null;
    stageMatrix.value = null;
    loadingJob.value = true;
    loadingBuilds.value = true;
    errorMessage.value = "";
  }
  try {
    const [detail, list] = await Promise.all([
      jenkinsApi.job(job.fullName),
      jenkinsApi.builds(job.fullName, 20),
    ]);
    if (token !== selectToken) return;
    selectedJob.value = detail;
    applyBuilds(job.fullName, list.builds);
    markRefreshed();
    settlePendingBuild(job.fullName, list.builds);
    void loadBuildStages(job.fullName, list.builds);
  } catch (error) {
    if (token === selectToken && !silent) setError(error);
  } finally {
    // 静默补刷可能覆盖上一个非静默请求的 token，这里必须负责收尾。
    if (token === selectToken) {
      loadingJob.value = false;
      loadingBuilds.value = false;
    }
  }
}

/**
 * 点执行器行跳到对应任务。
 * 任务不在左侧当前视图的过滤结果里也能打开：selectJob 按 fullName 直取，不依赖列表。
 */
async function openExecutorJob(row: ExecutorInfo): Promise<void> {
  if (!row.jobName) return;
  await selectJob({ fullName: row.jobName });
  if (view.value !== "jobs") return;
  recordNav(navEntry({ job: row.jobName }));
}

async function refreshJob(silent = false): Promise<void> {
  if (!selectedJob.value) {
    await loadJobs(folder.value);
    return;
  }
  // 先重置倒计时再发请求：否则请求窗口内还会排一轮，两个请求撞在一起。
  autoRefresher?.restart();
  await selectJob({ fullName: selectedJob.value.fullName }, silent);
}

async function openBuild(number: number, rememberReturn = true): Promise<void> {
  if (!selectedJob.value) return;
  const job = selectedJob.value.fullName;
  const token = ++buildToken;
  if (rememberReturn) {
    buildReturnView.value = view.value === "stages" ? "stages" : "jobs";
  }
  loadingBuilds.value = true;
  errorMessage.value = "";
  try {
    const detail = await jenkinsApi.build(job, number);
    if (token !== buildToken) return;
    // 用户主动点开、但响应回来时已经在回放另一屏：别再把他拉回详情。
    // （回放自己调 openBuild 时 rememberReturn=false，不受这条影响。）
    if (rememberReturn && navApplying) return;
    selectedBuild.value = detail;
    view.value = "build";
    // 只有用户主动点开才算一次导航；轮询 / 手动刷新（rememberReturn=false）不记历史。
    if (rememberReturn) recordNav(navEntry({ view: "build", job, build: number }));
  } catch (error) {
    if (token === buildToken) setError(error);
  } finally {
    if (token === buildToken) loadingBuilds.value = false;
  }
}

async function refreshBuild(): Promise<void> {
  if (!selectedBuild.value || !selectedJob.value) return;
  await openBuild(selectedBuild.value.number, false);
  autoRefresher?.restart();
}

/**
 * 构建详情「返回」= 后退一步，和鼠标侧键同义。
 * 回放会走 applyEntry，列表顺带静默刷新，圆点/构建号不会停在旧值。
 */
async function closeBuild(): Promise<void> {
  if ((await navBack()) !== "none") return;
  // 兜底：历史栈里没有上一条（正常路径不会——打开详情必留一条）时退回进入前的页面。
  view.value = buildReturnView.value === "stages" ? "stages" : "jobs";
  if (view.value === "stages") {
    void refreshStages();
    return;
  }
  void refreshJob(true);
}

/** 阶段视图返回任务列表：同样走后退，列表刷新由回放负责。 */
async function backToJobs(): Promise<void> {
  if ((await navBack()) !== "none") return;
  view.value = "jobs";
  await refreshJob(true);
}

async function loadStages(job: string): Promise<void> {
  const token = ++stageToken;
  loadingStages.value = true;
  errorMessage.value = "";
  try {
    const matrix = await jenkinsApi.stages(job, 10);
    if (token !== stageToken) return;
    stageMatrix.value = matrix;
    markRefreshed();
  } catch (error) {
    if (token !== stageToken) return;
    stageMatrix.value = null;
    setError(error);
  } finally {
    if (token === stageToken) loadingStages.value = false;
  }
}

function pageHidden(): boolean {
  return typeof document !== "undefined" && document.hidden;
}

/** 判定刚触发的构建是否已出现在列表里；settled / expired / stale 都清掉跟踪。 */
function settlePendingBuild(job: string, list: BuildSummary[]): void {
  const pending = pendingBuild.value;
  if (!pending) return;
  const status = evaluatePendingBuild(pending, job, list);
  if (status === "watching") return;
  pendingBuild.value = null;
  // 新构建出现或跟踪超时都要补刷：否则圆点与 #构建号 会停在旧值。
  if (shouldRefreshJobsAfterPending(status)) void loadJobs(folder.value, true);
}

/**
 * 静默刷新构建列表 + 任务详情：不置 loading，失败不弹错误。
 * 错误必须抛给轮询器：护栏靠它计数，徽章也靠它退出「请求中」。
 */
async function pollBuilds(): Promise<void> {
  const job = selectedJob.value?.fullName;
  if (!job || view.value !== "jobs" || pageHidden()) return;
  const token = selectToken;
  const [detail, list] = await Promise.all([
    jenkinsApi.job(job, POLL_TIMEOUT_MS),
    jenkinsApi.builds(job, 20, POLL_TIMEOUT_MS),
  ]);
  if (view.value !== "jobs" || selectedJob.value?.fullName !== job) return;
  // 期间若手动刷新过，已有更新的一份落了盘，别用旧响应覆盖。
  if (token !== selectToken) return;
  selectedJob.value = detail;
  applyBuilds(job, list.builds);
  settlePendingBuild(job, list.builds);
  void loadBuildStages(job, list.builds);
}

/** 静默刷新阶段矩阵；不抢占 stageToken，手动刷新会让本轮旧响应作废。错误交给轮询器计数。 */
async function pollStages(): Promise<void> {
  const job = selectedJob.value?.fullName;
  if (!job || view.value !== "stages" || pageHidden()) return;
  const token = stageToken;
  const matrix = await jenkinsApi.stages(job, 10, POLL_TIMEOUT_MS);
  if (
    token !== stageToken ||
    view.value !== "stages" ||
    selectedJob.value?.fullName !== job
  ) {
    return;
  }
  stageMatrix.value = matrix;
}

/** 静默刷新构建详情；构建结束后 autoRefreshMode 自然变 null，轮询自动停。 */
async function pollBuildDetail(): Promise<void> {
  const job = selectedJob.value?.fullName;
  const number = selectedBuild.value?.number;
  if (!job || number === undefined || view.value !== "build" || pageHidden()) return;
  const token = buildToken;
  const detail = await jenkinsApi.build(job, number, POLL_TIMEOUT_MS);
  if (token !== buildToken || view.value !== "build") return;
  if (selectedBuild.value?.number !== number) return;
  selectedBuild.value = detail;
}

/** 静默刷新构建队列：排队项随时进出，固定轮询不看内容。 */
async function pollQueue(): Promise<void> {
  if (view.value !== "queue" || pageHidden()) return;
  const result = await jenkinsApi.queue(POLL_TIMEOUT_MS);
  if (view.value !== "queue") return;
  queueItems.value = result.items;
}

/**
 * 静默刷新概览：聚合数字与执行器明细并行拉，一轮里两侧一起收敛。
 * 一个接口挂掉不丢另一份数据，但仍抛出失败交给轮询护栏计数。
 */
async function pollOverview(): Promise<void> {
  if (view.value !== "overview" || pageHidden()) return;
  const headToken = overviewToken;
  const slotToken = executorsToken;
  const [head, slots] = await Promise.allSettled([
    jenkinsApi.overview(POLL_TIMEOUT_MS),
    jenkinsApi.executors(POLL_TIMEOUT_MS),
  ]);
  if (view.value !== "overview") return;
  // 期间手动刷新过：它写的是更新的数据，本轮整体作废，别用旧响应覆盖。
  if (headToken !== overviewToken || slotToken !== executorsToken) return;
  if (head.status === "fulfilled") overview.value = head.value;
  if (slots.status === "fulfilled") executors.value = slots.value.executors;
  if (head.status === "rejected") throw head.reason;
  if (slots.status === "rejected") throw slots.reason;
}

/** 连续失败多少次后判定 Jenkins 不可用并暂停轮询。 */
const AUTO_REFRESH_MAX_FAILURES = 3;

function stopAutoRefresh(): void {
  autoRefresher?.stop();
  autoRefresher = null;
}

const AUTO_REFRESH_TASKS: Record<AutoRefreshMode, () => Promise<void>> = {
  builds: pollBuilds,
  build: pollBuildDetail,
  stages: pollStages,
  queue: pollQueue,
  overview: pollOverview,
  overviewIdle: pollOverview,
};

const AUTO_REFRESH_INTERVALS: Record<AutoRefreshMode, number> = {
  builds: AUTO_REFRESH_INTERVAL_MS,
  build: AUTO_REFRESH_INTERVAL_MS,
  stages: STAGE_AUTO_REFRESH_INTERVAL_MS,
  queue: QUEUE_AUTO_REFRESH_INTERVAL_MS,
  overview: OVERVIEW_AUTO_REFRESH_INTERVAL_MS,
  overviewIdle: OVERVIEW_IDLE_REFRESH_INTERVAL_MS,
};

function startAutoRefresh(mode: AutoRefreshMode): void {
  // 同一时刻只允许一个轮询器：重复调用（模式切换与首屏补启动撞车）会叠出第二个定时器。
  if (autoRefresher?.active) return;
  const refresher = createAutoRefresher({
    intervalMs: AUTO_REFRESH_INTERVALS[mode],
    task: AUTO_REFRESH_TASKS[mode],
    shouldContinue: () => autoRefreshMode.value === mode,
    maxConsecutiveFailures: AUTO_REFRESH_MAX_FAILURES,
    onPaused: () => {
      // 不做自动恢复：Jenkins 不可用时继续打只会刷屏，交给用户手动确认。
      refreshPaused.value = true;
    },
    onRunStart: () => {
      pollInFlight.value = true;
    },
    onRunEnd: (ok) => {
      pollInFlight.value = false;
      if (ok) markRefreshed();
    },
  });
  autoRefresher = refresher;
  refreshPaused.value = false;
  refresher.start();
}

const autoRefreshInterval = computed(() =>
  autoRefreshMode.value ? AUTO_REFRESH_INTERVALS[autoRefreshMode.value] : 0,
);

/** 传给各视图的刷新 UI 状态：徽章与手动刷新按钮三态共用。 */
const refreshUi = computed<RefreshUi>(() => ({
  mode: autoRefreshMode.value,
  paused: refreshPaused.value,
  inFlight: pollInFlight.value,
  lastAt: lastRefreshAt.value,
  intervalMs: autoRefreshInterval.value,
  refreshing: manualRefreshing.value,
  refreshed: manualJustDone.value,
}));

/**
 * 页面隐藏时停轮询（后台空转无意义），回到前台立即补一轮。
 * 没有轮询的页面（概览、节点、空闲的任务页）也走进入刷新：
 * 后台期间 Jenkins 上的变化不该等用户手动刷才可见。
 */
function handleVisibility(): void {
  if (pageHidden()) {
    stopAutoRefresh();
    return;
  }
  const mode = autoRefreshMode.value;
  if (!mode) {
    void refreshOnEnter();
    return;
  }
  if (!autoRefresher?.active) startAutoRefresh(mode);
  autoRefresher?.trigger();
}

watch(autoRefreshMode, (mode, previous) => {
  stopAutoRefresh();
  if (mode) {
    // 后台不重建轮询：可见时 handleVisibility 会补上并立即跑一轮。
    if (!pageHidden()) startAutoRefresh(mode);
    return;
  }
  // 构建结束、轮询停止时补一次静默刷新，让列表圆点与构建号收敛到最终状态。
  if (previous === "builds" && view.value === "jobs") {
    void loadJobs(folder.value, true);
  }
  // 已经不需要轮询了，暂停提示也没有意义。
  refreshPaused.value = false;
});

async function openStages(): Promise<void> {
  if (!selectedJob.value || !isPipeline.value) return;
  view.value = "stages";
  recordNav(navEntry({ view: "stages" }));
  await loadStages(selectedJob.value.fullName);
}

async function refreshStages(): Promise<void> {
  if (!selectedJob.value) return;
  autoRefresher?.restart();
  await loadStages(selectedJob.value.fullName);
}

async function triggerBuild(params: Record<string, unknown> = {}): Promise<void> {
  const job = selectedJob.value;
  if (!job) return;
  try {
    const result = await jenkinsApi.trigger(job.fullName, params);
    showToast(t("buildQueued", { queueId: result.queueId || "?" }));
    // 参数化构建要排队后才分配构建号：先立即补刷一次，再跟踪到新构建出现为止。
    pendingBuild.value = createPendingBuild(job.fullName, job.lastBuild?.number ?? 0);
    void refreshJob(true);
    // 列表里的圆点与构建号也要跟着变，否则左侧看起来像没触发。
    void loadJobs(folder.value, true);
    window.setTimeout(() => {
      if (pendingBuild.value?.job === job.fullName) {
        void refreshJob(true);
        void loadJobs(folder.value, true);
      }
    }, 800);
  } catch (error) {
    setError(error);
  }
}

/** 进入/退出多选模式；退出时清空选择，下次进来不带旧任务。 */
function toggleBatchMode(): void {
  if (readOnly.value) {
    showToast(t("readOnlyManageHint"), "info");
    return;
  }
  batchMode.value = !batchMode.value;
  if (!batchMode.value) batchSelected.value = [];
}

function toggleBatchJob(fullName: string): void {
  const next = toggleJobSelection(batchSelected.value, fullName);
  // 去掉勾选会变短，只有「撞上限」才会原样返回同一个数组。
  if (next === batchSelected.value) {
    showToast(t("batchLimit", { max: MAX_BATCH_JOBS }), "info");
    return;
  }
  batchSelected.value = next;
}

/** 全选当前可见任务：文件夹不参与，仍受上限约束。 */
function selectAllVisibleBatch(list: JobSummary[]): void {
  batchSelected.value = selectVisible(batchSelected.value, list);
  if (batchSelected.value.length >= MAX_BATCH_JOBS) {
    showToast(t("batchLimit", { max: MAX_BATCH_JOBS }), "info");
  }
}

function clearBatchSelection(): void {
  batchSelected.value = [];
}

/** 预检失败时的降级清单：用缓存快照，状态可能过期但还能继续构建。 */
function fallbackTargets(names: string[]): BatchTargetInfo[] {
  const cached = batchTargets(names, batchTargetMap.value);
  const byName = new Map(cached.map((item) => [item.fullName, item]));
  return names.map((fullName) => {
    const target = byName.get(fullName);
    return {
      name: target?.name ?? fullName.split("/").pop() ?? fullName,
      fullName,
      status: target?.status ?? "unknown",
      building: target?.building ?? false,
      buildable: true,
      inQueue: false,
      parameters: null,
    };
  });
}

/** 重算共享参数（仅 Boolean 开关），保留用户已填过的值。 */
function applySharedParameters(targets: BatchTargetInfo[]): void {
  const shared = collectSharedParameters(targets);
  const defaults = buildParamValues(shared.parameters);
  const values: Record<string, unknown> = {};
  for (const parameter of shared.parameters) {
    values[parameter.name] =
      parameter.name in batchParamValues.value
        ? batchParamValues.value[parameter.name]
        : defaults[parameter.name];
  }
  batchParamDefs.value = shared.parameters;
  batchParamValues.value = values;
}

/** 点「构建所选」：先预检拿实时状态与参数定义，再进确认态。 */
async function openBatchBuild(): Promise<void> {
  const names = [...batchSelected.value];
  if (!names.length) {
    showToast(t("batchEmpty"), "info");
    return;
  }
  batchParamDefs.value = [];
  batchParamValues.value = {};
  batchDialog.value = {
    phase: "inspecting",
    targets: fallbackTargets(names),
    results: [],
    error: "",
    done: 0,
    total: names.length,
  };

  try {
    const result = await jenkinsApi.batchInspect(names);
    applySharedParameters(result.targets);
    batchDialog.value = {
      phase: "confirm",
      targets: result.targets,
      results: [],
      error: "",
      done: 0,
      total: result.targets.length,
    };
  } catch (error) {
    // 预检失败不堵死构建：用缓存状态 + 默认参数继续，弹窗里给出提示。
    batchDialog.value = {
      phase: "confirm",
      targets: fallbackTargets(names),
      results: [],
      error: describeError(error),
      done: 0,
      total: names.length,
    };
  }
}

/**
 * 分批执行批量构建：每批 10 个串行调用，回来一批推进一次进度。
 * 单批失败只标记这一批，其余批次继续；连续 2 批失败才中止剩余（标记「未执行」）。
 * 这样整批超时不会让「已触发的部分」变成不可知状态。
 */
async function executeBatch(
  names: string[],
  skipBusy: boolean,
  params: Record<string, unknown>,
): Promise<void> {
  const targets = [...names];
  const total = targets.length;
  if (!total) {
    batchDialog.value = null;
    return;
  }
  batchSkipBusy.value = skipBusy;
  batchDialog.value = {
    phase: "running",
    targets: batchDialog.value?.targets ?? [],
    results: [],
    error: "",
    done: 0,
    total,
  };

  const collected: BatchBuildResult[] = [];
  let consecutiveFailures = 0;
  const chunks = chunkJobs(targets);

  for (let index = 0; index < chunks.length; index += 1) {
    const chunk = chunks[index];
    try {
      const result = await jenkinsApi.batchBuild(chunk, skipBusy, params);
      collected.push(...result.results);
      consecutiveFailures = 0;
    } catch (error) {
      collected.push(...failedChunk(chunk, describeError(error)));
      consecutiveFailures += 1;
    }

    batchDialog.value = {
      phase: "running",
      targets: batchDialog.value?.targets ?? [],
      results: [...collected],
      error: "",
      done: collected.length,
      total,
    };

    if (shouldAbortChunks(consecutiveFailures)) {
      collected.push(...abortedChunk(chunks.slice(index + 1).flat()));
      break;
    }
  }

  const summary = summarizeBatch(collected);
  batchDialog.value = {
    phase: "done",
    targets: batchDialog.value?.targets ?? [],
    results: collected,
    error: "",
    done: total,
    total,
  };
  showToast(
    t("batchResultSummary", {
      queued: summary.queued,
      skipped: summary.skipped,
      failed: summary.failed,
    }),
    summary.failed ? "info" : "success",
  );

  // 触发后补刷：左侧圆点/构建号立即更新，当前任务在批内则跟踪它的新构建；
  // 800ms 再补一次，让刚进队列的构建尽快反映到列表（与单任务触发一致）。
  const current = selectedJob.value;
  if (current && targets.includes(current.fullName)) {
    pendingBuild.value = createPendingBuild(
      current.fullName,
      current.lastBuild?.number ?? 0,
    );
    void refreshJob(true);
  }
  void loadJobs(folder.value, true);
  window.setTimeout(() => void loadJobs(folder.value, true), 800);
}

/** 确认弹窗 → 用确认清单里的任务执行，并退出多选模式（需求 2）。 */
async function runBatchBuild(
  skipBusy: boolean,
  values: Record<string, unknown>,
): Promise<void> {
  const dialog = batchDialog.value;
  if (!dialog) return;
  const names = dialog.targets.map((target) => target.fullName);
  batchParamValues.value = values;
  // 结果弹窗只用 results 与 targets 快照，不依赖选择，所以可以立刻退出多选。
  batchMode.value = false;
  batchSelected.value = [];
  await executeBatch(names, skipBusy, values);
}

/** 结果页「重试失败项」：只重跑真失败的任务，不再弹确认。 */
function retryFailedBatch(): void {
  const dialog = batchDialog.value;
  if (!dialog) return;
  const failed = dialog.results
    .filter((item) => !item.success && !item.skipped)
    .map((item) => item.job);
  if (!failed.length) return;
  void executeBatch(failed, batchSkipBusy.value, batchParamValues.value);
}

/** 确认弹窗里就地剔除一个任务；剔空即关闭弹窗。 */
function removeBatchTarget(fullName: string): void {
  batchSelected.value = batchSelected.value.filter((name) => name !== fullName);
  const dialog = batchDialog.value;
  if (!dialog) return;
  const targets = dialog.targets.filter((target) => target.fullName !== fullName);
  if (!targets.length) {
    batchDialog.value = null;
    return;
  }
  applySharedParameters(targets);
  batchDialog.value = { ...dialog, targets, total: targets.length };
}

/** Shift 区间连选：JobList 已按可见顺序算好整份集合。 */
function replaceBatchSelection(names: string[]): void {
  batchSelected.value = names;
}

function closeBatchDialog(): void {
  batchDialog.value = null;
}

/** 两个「停止」入口（任务页 / 构建页）都先弹确认，不直接发请求。 */
function stopBuild(number: number): void {
  const request = createStopRequest(selectedJob.value?.fullName, number);
  if (!request) return;
  pendingConfirm.value = request;
}

function stopLastBuild(): void {
  const number = selectedJob.value?.lastBuild?.number;
  if (number) stopBuild(number);
}

async function runStop(request: StopRequest): Promise<void> {
  try {
    await jenkinsApi.stop(request.job, request.number);
    showToast(t("stopSuccess"));
    // Jenkins 要过一小会儿才把状态改过来；延迟补刷，且只在目标未变时才刷。
    window.setTimeout(() => {
      // 用户中途切了任务就别刷错对象。
      if (!isStopTarget(request, selectedJob.value?.fullName)) return;
      void refreshJob();
      if (view.value === "build" && selectedBuild.value?.number === request.number) {
        void refreshBuild();
      }
    }, 800);
  } catch (error) {
    setError(error);
  }
}

async function runCancel(request: CancelRequest): Promise<void> {
  try {
    await jenkinsApi.cancelQueue(Number(request.id));
    showToast(t("cancelSuccess"));
    await loadQueue();
  } catch (error) {
    setError(error);
  }
}

/** 确认弹窗的唯一执行入口：busy 期间挡住重复提交，结束后关窗。 */
async function confirmPending(): Promise<void> {
  const request = pendingConfirm.value;
  if (!request || confirmBusy.value) return;
  confirmBusy.value = true;
  try {
    if (request.kind === "stop") await runStop(request);
    else await runCancel(request);
  } finally {
    confirmBusy.value = false;
    pendingConfirm.value = null;
  }
}

function cancelPending(): void {
  if (confirmBusy.value) return;
  pendingConfirm.value = null;
}

function submitParams(params: Record<string, unknown>): void {
  showParams.value = false;
  void triggerBuild(params);
}

/** 只读连接下所有 CRUD 入口统一拦截。 */
function guardWrite(): boolean {
  if (!readOnly.value) return true;
  showToast(t("readOnlyManageHint"), "info");
  return false;
}

async function loadViewJobPool(): Promise<void> {
  try {
    const result = await jenkinsApi.jobs("", "");
    viewJobPool.value = result.jobs
      .filter((job) => !job.isFolder)
      .map((job) => job.fullName);
  } catch {
    viewJobPool.value = jobs.value
      .filter((job) => !job.isFolder)
      .map((job) => job.fullName);
  }
}

async function selectByFullName(fullName: string): Promise<void> {
  const entry = jobs.value.find((job) => job.fullName === fullName);
  if (entry) await selectJob(entry);
}

async function openCreateJob(): Promise<void> {
  if (!guardWrite()) return;
  errorMessage.value = "";
  jobEditor.value = { mode: "create" };
}

async function openEditJob(): Promise<void> {
  if (!selectedJob.value || !guardWrite()) return;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    const config = await jenkinsApi.jobConfig(selectedJob.value.fullName);
    jobEditor.value = { mode: "edit", config };
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

type JoinViewResult = "added" | "skipped" | "failed";

/**
 * 新建任务后把它写进当前命名视图。
 * Jenkins 的 ListView 是显式任务集合，不写回的话刷新也看不到新任务。
 * 只做追加：以服务端刚读到的 jobNames 为基线，不动描述、列与过滤规则。
 */
async function joinActiveView(
  fullName: string,
  createdIn: string,
): Promise<JoinViewResult> {
  if (
    !needsJoinView({
      folder: createdIn,
      activeView: activeView.value,
      fullName,
      jobs: jobs.value,
    })
  ) {
    return "skipped";
  }

  const view = activeView.value;
  try {
    const config = await jenkinsApi.viewConfig(view);
    if (resolveJoinAction(config.fields) !== "add" || !config.fields) {
      return "failed";
    }
    await jenkinsApi.updateView(view, {
      jobNames: appendJobName(config.fields.jobNames, fullName),
    });
    // 请求期间用户可能切走视图，此时不再按旧视图校验。
    if (activeView.value !== view) return "added";
    await loadJobs("", true);
    return containsJob(jobs.value, fullName) ? "added" : "failed";
  } catch {
    // 任务已经创建成功，写视图失败不回滚，交给提示引导用户手动添加。
    return "failed";
  }
}

async function createJob(payload: CreateJobPayload): Promise<void> {
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.createJob(payload);
    const fullName = result.fullName ?? payload.name;
    jobEditor.value = null;

    // 先刷新列表：正则视图或全部视图能立即包含新任务，避免多余的写请求。
    await loadJobs(folder.value, true);
    const join = await joinActiveView(fullName, payload.folder);
    await selectByFullName(fullName);

    if (join === "added") {
      showToast(
        t("jobCreatedInView", { name: fullName, view: activeView.value }),
      );
    } else if (join === "failed") {
      // 任务建好了但没进视图，属于部分成功：用 info 语气提示，别给绿色勾。
      showToast(
        t("jobCreatedNotInView", { name: fullName, view: activeView.value }),
        "info",
      );
    } else {
      showToast(t("jobCreated", { name: fullName }));
    }
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

async function updateJob(fields: JobUpdateFields): Promise<void> {
  if (!selectedJob.value) return;
  const previous = selectedJob.value.fullName;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.updateJob(previous, fields);
    jobEditor.value = null;
    showToast(t("jobUpdated"));
    await loadJobs(folder.value);
    await selectByFullName(result.fullName ?? previous);
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

async function saveJobXml(xml: string): Promise<void> {
  if (!selectedJob.value) return;
  const fullName = selectedJob.value.fullName;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    await jenkinsApi.saveJob(fullName, xml);
    jobEditor.value = null;
    showToast(t("jobUpdated"));
    await loadJobs(folder.value);
    await selectByFullName(fullName);
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

async function openCreateView(): Promise<void> {
  if (!guardWrite()) return;
  errorMessage.value = "";
  await loadViewJobPool();
  viewEditor.value = { mode: "create" };
}

async function openEditView(): Promise<void> {
  if (!canManageView.value || !guardWrite()) return;
  const name = activeView.value;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    const [config] = await Promise.all([jenkinsApi.viewConfig(name), loadViewJobPool()]);
    const view = views.value.find((item) => item.name === name) ?? {
      name,
      description: "",
      url: "",
    };
    viewEditor.value = { mode: "edit", view, config };
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

async function createView(payload: CreateViewPayload): Promise<void> {
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    await jenkinsApi.createView(payload);
    viewEditor.value = null;
    showToast(t("viewCreated", { name: payload.name }));
    await loadViews();
    activeView.value = payload.name;
    folder.value = "";
    await loadJobs("");
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

async function updateView(fields: ViewUpdateFields): Promise<void> {
  const name = activeView.value;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    const result = await jenkinsApi.updateView(name, fields);
    const nextName = result.fullName || name;
    viewEditor.value = null;
    showToast(
      nextName !== name ? t("viewRenamed", { name: nextName }) : t("viewUpdated"),
    );
    await loadViews();
    if (nextName !== name) {
      // 视图改名后 URL 变了，必须先切到新名再拉任务，否则会 404。
      activeView.value = nextName;
      folder.value = "";
    }
    await loadJobs("");
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

async function saveViewXml(xml: string): Promise<void> {
  const name = activeView.value;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    await jenkinsApi.saveView(name, xml);
    viewEditor.value = null;
    showToast(t("viewUpdated"));
    await loadViews();
    await loadJobs("");
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

function openDeleteJob(): void {
  if (!selectedJob.value || !guardWrite()) return;
  deleteTarget.value = {
    kind: "job",
    name: selectedJob.value.name,
    fullName: selectedJob.value.fullName,
  };
}

function openDeleteView(): void {
  if (!canManageView.value || !guardWrite()) return;
  deleteTarget.value = { kind: "view", name: activeView.value };
}

async function confirmDelete(): Promise<void> {
  if (!deleteTarget.value) return;
  const target = deleteTarget.value;
  editorBusy.value = true;
  errorMessage.value = "";
  try {
    if (target.kind === "job") {
      await jenkinsApi.deleteJob(target.fullName ?? target.name);
      showToast(t("jobDeleted"));
      deleteTarget.value = null;
      selectedJob.value = null;
      resetBuilds();
      selectedBuild.value = null;
      stageMatrix.value = null;
      await loadJobs(folder.value);
    } else {
      await jenkinsApi.deleteView(target.name);
      showToast(t("viewDeleted"));
      deleteTarget.value = null;
      await loadViews();
      await loadJobs("");
    }
  } catch (error) {
    setError(error);
  } finally {
    editorBusy.value = false;
  }
}

let manualDoneTimer: number | undefined;

/**
 * 手动刷新：按钮三态（刷新 → 刷新中… → 已更新）。
 * 手动刷新视为用户确认环境已恢复：清暂停提示，必要时重建被护栏停掉的轮询器。
 */
async function refreshCurrent(): Promise<void> {
  if (manualRefreshing.value) return;
  manualRefreshing.value = true;
  manualJustDone.value = false;
  try {
    refreshPaused.value = false;
    const mode = autoRefreshMode.value;
    if (mode && !autoRefresher?.active) startAutoRefresh(mode);

    if (view.value === "build") {
      await refreshBuild();
    } else if (view.value === "stages") {
      await refreshStages();
    } else if (view.value === "overview") {
      await refreshOverview();
    } else if (view.value === "queue") {
      await loadQueue();
    } else if (view.value === "nodes") {
      await loadNodes();
    } else {
      await refreshJob();
    }

    markRefreshed();
    manualJustDone.value = true;
    if (manualDoneTimer) window.clearTimeout(manualDoneTimer);
    manualDoneTimer = window.setTimeout(() => {
      manualJustDone.value = false;
    }, 1500);
  } finally {
    manualRefreshing.value = false;
  }
}

function applyTheme(): void {
  const plugin = window.dbxPlugin;
  if (!plugin) return;
  const theme = plugin.theme;
  if (theme?.appearance) {
    document.documentElement.dataset.theme = theme.appearance;
  }
  for (const [key, value] of Object.entries(theme?.tokens ?? {})) {
    if (/^--(color|radius|font)-/.test(key)) {
      document.documentElement.style.setProperty(key, value);
    }
  }
}

function handleEnv(): void {
  syncLocale(window.dbxPlugin?.locale);
  applyTheme();
}

onMounted(async () => {
  const plugin = window.dbxPlugin;
  if (!plugin) {
    errorMessage.value = "window.dbxPlugin is undefined";
    return;
  }
  await plugin.ready;
  setConnectionId(plugin.context?.connectionId);
  connectionName.value = String(plugin.context?.connectionName ?? plugin.context?.name ?? "");
  syncLocale(plugin.locale);
  applyTheme();
  window.addEventListener("dbx-plugin-env", handleEnv);
  document.addEventListener("visibilitychange", handleVisibility);
  // 两个都监听：不同宿主在不同阶段派发侧键事件，去抖器保证一次按键只走一次。
  window.addEventListener("mousedown", handleMouseNavButton);
  window.addEventListener("mouseup", handleMouseNavButton);
  window.addEventListener("popstate", handlePopState);
  await loadIdentity();
  // 首屏游标用 replace 写进当前条目（不新增记录）：放在 loadIdentity 之后，
  // 是因为 activeView 要等 loadViews 拉完才定下来，否则初始条目会留下空视图名。
  // 之后每次导航都 push，宿主栈与内部栈保持 1:1 对应。
  navStack.reset(navEntry());
  navHistory.replace(0);
  syncNavFlags();
  // 概览页初始 mode 就是 overviewIdle，没有「null → 非 null」的变化过程，
  // watch 不会触发；这里补一次首次启动，否则全空闲时要等到切页才开始轮询。
  const initialMode = autoRefreshMode.value;
  if (initialMode && !pageHidden()) startAutoRefresh(initialMode);
});

onBeforeUnmount(() => {
  window.removeEventListener("dbx-plugin-env", handleEnv);
  document.removeEventListener("visibilitychange", handleVisibility);
  window.removeEventListener("mousedown", handleMouseNavButton);
  window.removeEventListener("mouseup", handleMouseNavButton);
  window.removeEventListener("popstate", handlePopState);
  cancelAwaitingPop();
  if (changedTimer) window.clearTimeout(changedTimer);
  if (manualDoneTimer) window.clearTimeout(manualDoneTimer);
  toastQueue.clear();
  pendingBuild.value = null;
  stopAutoRefresh();
});
</script>

<template>
  <div class="app">
    <header class="topbar">
      <div class="brand">
        <img class="logo" :src="logoUrl" alt="" />
        <div class="brand-text">
          <strong>{{ t("appTitle") }}</strong>
          <span class="subtle" :title="connectionName">{{ connectionLabel }}</span>
        </div>
      </div>
      <span class="app-version" :title="t('uiVersionHint')">v{{ appVersion }}</span>

      <div class="nav-history">
        <button
          type="button"
          class="nav-step"
          :disabled="!canNavBack"
          :title="t('navBack')"
          :aria-label="t('navBack')"
          @click="navBack()"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M10 3 5 8l5 5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        <button
          type="button"
          class="nav-step"
          :disabled="!canNavForward"
          :title="t('navForward')"
          :aria-label="t('navForward')"
          @click="navForward()"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M6 3l5 5-5 5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
      </div>

      <div class="topbar-right">
        <span v-if="identity" class="who">
          {{ identity.name || t("anonymous") }}
          <template v-if="identity.version"> · {{ identity.version }}</template>
        </span>
        <span class="chip" :data-write="!readOnly">
          {{ readOnly ? t("readOnlyBadge") : t("writeBadge") }}
        </span>
        <button type="button" class="refresh" @click="refreshCurrent">{{ t("refresh") }}</button>
      </div>
    </header>

    <div class="body">
      <nav class="sidebar">
        <button
          type="button"
          class="nav-item"
          :class="{ active: view === 'overview' }"
          @click="switchView('overview')"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M3 3h4v4H3zM9 3h4v4H9zM3 9h4v4H3zM9 9h4v4H9z" fill="none" stroke="currentColor" stroke-width="1.3" />
          </svg>
          <span>{{ t("overview") }}</span>
        </button>
        <button
          type="button"
          class="nav-item"
          :class="{ active: view === 'jobs' || view === 'build' || view === 'stages' }"
          @click="switchView('jobs')"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M3 4h10M3 8h10M3 12h6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
          </svg>
          <span>{{ t("jobs") }}</span>
        </button>
        <button
          type="button"
          class="nav-item"
          :class="{ active: view === 'queue' }"
          @click="switchView('queue')"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M3 4h10M3 8h10M3 12h10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
          </svg>
          <span>{{ t("queue") }}</span>
        </button>
        <button
          type="button"
          class="nav-item"
          :class="{ active: view === 'nodes' }"
          @click="switchView('nodes')"
        >
          <svg viewBox="0 0 16 16" aria-hidden="true">
            <path d="M8 2v4M8 10v4M4 8H2M14 8h-2M8 6a2 2 0 1 0 0 4 2 2 0 0 0 0-4z" fill="none" stroke="currentColor" stroke-width="1.3" />
          </svg>
          <span>{{ t("nodes") }}</span>
        </button>
      </nav>

      <div class="workspace">
        <div v-if="errorMessage" class="banner error">
          <span>{{ errorMessage }}</span>
          <button type="button" @click="loadIdentity">{{ t("retry") }}</button>
        </div>

        <div v-if="refreshPaused" class="banner warn">
          <span>{{ t("autoRefreshPaused") }}</span>
          <button type="button" @click="refreshCurrent">{{ t("refresh") }}</button>
        </div>

        <OverviewView
          v-if="view === 'overview'"
          :overview="overview"
          :loading="loadingView"
          :executors="executors"
          :loading-executors="loadingExecutors"
          :refresh="refreshUi"
          @refresh="refreshCurrent"
          @open-executor="openExecutorJob"
        />

        <QueueView
          v-else-if="view === 'queue'"
          :items="queueItems"
          :loading="loadingView"
          :read-only="readOnly"
          :locale="locale"
          :refresh="refreshUi"
          @cancel="cancelQueueItem"
          @refresh="refreshCurrent"
        />

        <NodesView
          v-else-if="view === 'nodes'"
          :nodes="nodes"
          :loading="loadingView"
          @refresh="loadNodes"
        />

        <StageView
          v-else-if="view === 'stages' && selectedJob"
          :job="selectedJob.fullName"
          :matrix="stageMatrix"
          :loading="loadingStages"
          :locale="locale"
          :refresh="refreshUi"
          @back="backToJobs"
          @refresh="refreshCurrent"
          @open-build="openBuild"
        />

        <div v-else-if="view === 'build' && selectedBuild && selectedJob" class="build-wrap">
          <BuildView
            :build="selectedBuild"
            :job-full-name="selectedJob.fullName"
            :read-only="readOnly"
            :locale="locale"
            :refresh="refreshUi"
            @back="closeBuild"
            @stop="stopBuild"
            @refresh="refreshCurrent"
            @notify="showToast"
          />
        </div>

        <div v-else class="jobs-wrap">
          <JobList
            :jobs="jobs"
            :selected="selectedJob?.fullName ?? ''"
            :folder="folder"
            :loading="loadingJobs"
            :views="views"
            :active-view="activeView"
            :read-only="readOnly"
            :batch-mode="batchMode"
            :batch-selected="batchSelected"
            :batch-outside-count="batchOutsideCount"
            @select="openJob"
            @open-folder="navigateFolder($event.fullName)"
            @navigate="navigateFolder"
            @select-view="chooseView"
            @refresh="loadJobs(folder)"
            @create-job="openCreateJob"
            @create-view="openCreateView"
            @edit-view="openEditView"
            @delete-view="openDeleteView"
            @toggle-batch-mode="toggleBatchMode"
            @toggle-batch-job="toggleBatchJob"
            @select-all-visible="selectAllVisibleBatch"
            @replace-batch-selection="replaceBatchSelection"
            @clear-batch="clearBatchSelection"
            @build-batch="openBatchBuild"
          />

          <JobView
            v-if="selectedJob"
            :job="selectedJob"
            :builds="builds"
            :loading-builds="loadingJob || loadingBuilds"
            :read-only="readOnly"
            :is-pipeline="isPipeline"
            :locale="locale"
            :refresh="refreshUi"
            :changed-builds="changedBuilds"
            :stage-snapshot="buildStageSnapshot"
            @build="triggerBuild()"
            @build-with-params="showParams = true"
            @stop="stopLastBuild"
            @open-build="openBuild"
            @open-stages="openStages"
            @refresh="refreshCurrent"
            @edit-job="openEditJob"
            @delete-job="openDeleteJob"
          />
          <div v-else class="placeholder">
            <p>{{ t("selectJob") }}</p>
          </div>
        </div>
      </div>
    </div>

    <ParamDialog
      v-if="showParams && selectedJob"
      :job="selectedJob"
      @submit="submitParams"
      @cancel="showParams = false"
    />

    <JobEditor
      v-if="jobEditor"
      :mode="jobEditor.mode"
      :folder="folder"
      :config="jobEditor.mode === 'edit' ? jobEditor.config : null"
      :job="selectedJob"
      :busy="editorBusy"
      @create="createJob"
      @update="updateJob"
      @save-xml="saveJobXml"
      @cancel="jobEditor = null"
    />

    <ViewEditor
      v-if="viewEditor"
      :mode="viewEditor.mode"
      :view="viewEditor.mode === 'edit' ? viewEditor.view : null"
      :config="viewEditor.mode === 'edit' ? viewEditor.config : null"
      :available-jobs="viewJobPool"
      :existing-names="viewNames"
      :busy="editorBusy"
      @create="createView"
      @update="updateView"
      @save-xml="saveViewXml"
      @cancel="viewEditor = null"
    />

    <ConfirmDelete
      v-if="deleteTarget"
      :target="deleteTarget.name"
      :kind="deleteTarget.kind"
      :full-name="deleteTarget.fullName"
      :busy="editorBusy"
      @confirm="confirmDelete"
      @cancel="deleteTarget = null"
    />

    <ConfirmAction
      v-if="pendingConfirm"
      :title="confirmTitle"
      :detail="confirmDetail"
      :hint="confirmHint"
      :confirm-label="confirmActionLabel"
      :busy="confirmBusy"
      @confirm="confirmPending"
      @cancel="cancelPending"
    />

    <BatchBuildDialog
      v-if="batchDialog"
      :targets="batchDialog.targets"
      :phase="batchDialog.phase"
      :skip-busy="batchSkipBusy"
      :results="batchDialog.results"
      :progress="{ done: batchDialog.done, total: batchDialog.total }"
      :error="batchDialog.error"
      :parameters="batchParamDefs"
      :param-values="batchParamValues"
      @confirm="runBatchBuild"
      @update:param-values="batchParamValues = $event"
      @retry="retryFailedBatch"
      @remove="removeBatchTarget"
      @close="closeBatchDialog"
    />

    <ToastHost :items="toasts" @dismiss="dismissToast" />
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.topbar {
  display: flex;
  align-items: center;
  gap: 16px;
  height: 56px;
  padding: 0 18px;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
  flex: none;
}

/* 前进/后退箭头：鼠标侧键的可视化等价物，同时让禁用态一眼可见。 */
.nav-history {
  display: flex;
  align-items: center;
  gap: 2px;
  flex: none;
}

.nav-step {
  display: grid;
  place-items: center;
  width: 26px;
  height: 26px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--muted);
}

.nav-step svg {
  width: 15px;
  height: 15px;
}

.nav-step:hover:not(:disabled) {
  border-color: var(--border);
  background: var(--panel-soft);
  color: var(--text);
}

.nav-step:disabled {
  opacity: 0.32;
  cursor: default;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.logo {
  width: 28px;
  height: 28px;
  border-radius: 8px;
}

.brand-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
  line-height: 1.25;
}

.brand-text strong {
  font-size: 14px;
}

.subtle {
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.app-version {
  flex: none;
  padding: 2px 7px;
  border: 1px solid var(--border);
  border-radius: 999px;
  color: var(--muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: none;
  margin-left: auto;
}

.who {
  color: var(--muted);
  font-size: 12px;
}

.chip {
  padding: 4px 10px;
  border-radius: 999px;
  background: var(--accent-soft);
  color: var(--accent);
  font-size: 12px;
  font-weight: 600;
}

.chip[data-write="true"] {
  background: color-mix(in srgb, var(--success) 14%, transparent);
  color: var(--success);
}

.refresh {
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  padding: 6px 11px;
  background: transparent;
  color: var(--text);
  font-size: 12px;
  font-weight: 600;
}

.refresh:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.sidebar {
  width: 196px;
  flex: none;
  padding: 12px;
  border-right: 1px solid var(--border);
  background: var(--panel);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 9px 10px;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--muted);
  text-align: left;
}

.nav-item svg {
  width: 16px;
  height: 16px;
  flex: none;
}

.nav-item.active {
  background: var(--accent-soft);
  color: var(--accent);
  font-weight: 600;
}

.nav-item:disabled {
  opacity: 0.72;
  cursor: default;
}

.soon {
  margin-left: auto;
  font-size: 9px;
  padding: 2px 5px;
  border-radius: 4px;
  background: var(--panel-soft);
  color: var(--muted);
}

.workspace {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 16px;
  font-size: 12px;
  flex: none;
}

.banner.error {
  background: color-mix(in srgb, var(--danger) 12%, transparent);
  color: var(--danger);
}

.banner.warn {
  background: color-mix(in srgb, var(--warning) 12%, transparent);
  color: var(--warning);
}

.banner button {
  border: 1px solid currentColor;
  border-radius: var(--radius-sm);
  background: transparent;
  color: inherit;
  padding: 3px 9px;
  font-size: 12px;
}

.jobs-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
}

.build-wrap {
  flex: 1;
  min-height: 0;
  display: flex;
}

.placeholder {
  flex: 1;
  display: grid;
  place-items: center;
  color: var(--muted);
  font-size: 13px;
}
</style>
