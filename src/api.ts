import type {
  ArtifactChunk,
  ArtifactInfo,
  ArtifactTaskSnapshot,
  BatchBuildResult,
  BatchTargetInfo,
  BuildDetail,
  BuildSummary,
  ConsoleChunk,
  CreateJobPayload,
  CreateViewPayload,
  ExecutorInfo,
  JobConfigResponse,
  JobDetail,
  JobSummary,
  JobUpdateFields,
  MutationResult,
  NodeInfo,
  Overview,
  QueueItem,
  SavedFile,
  StageInfo,
  StageMatrix,
  TriggerResult,
  ViewConfigResponse,
  ViewInfo,
  ViewUpdateFields,
  WhoAmIResponse,
} from "./types";

let currentConnectionId = "";

export function setConnectionId(id: string | undefined): void {
  currentConnectionId = id ?? "";
}

export function getConnectionId(): string {
  return currentConnectionId;
}

/**
 * 调用本插件 sidecar 的 RPC。
 * 宿主不会自动补 connectionId，必须由 UI 显式携带（与 DBX 官方插件一致）。
 */
export async function invoke<T>(
  method: string,
  params: Record<string, unknown> = {},
  timeoutMs = 30_000,
): Promise<T> {
  const plugin = window.dbxPlugin;
  if (!plugin) {
    throw new Error("DBX host bridge is not available.");
  }
  return plugin.invoke<T>(
    method,
    { connectionId: currentConnectionId, ...params },
    { timeoutMs },
  );
}

export function describeError(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  try {
    return JSON.stringify(error);
  } catch {
    return "Unknown error";
  }
}

export const jenkinsApi = {
  whoami(): Promise<WhoAmIResponse> {
    return invoke<WhoAmIResponse>("jenkins/whoami", {}, 15_000);
  },

  jobs(folder = "", view = ""): Promise<{ jobs: JobSummary[] }> {
    return invoke<{ jobs: JobSummary[] }>("jenkins/jobs", { folder, view });
  },

  views(): Promise<{ views: ViewInfo[] }> {
    return invoke<{ views: ViewInfo[] }>("jenkins/views", {}, 20_000);
  },

  /**
   * 任务详情。`timeoutMs` 供轮询传更短的超时：
   * 轮询是串行链，单轮挂太久会让后续轮次全部停摆。
   */
  job(job: string, timeoutMs = 30_000): Promise<JobDetail> {
    return invoke<JobDetail>("jenkins/job", { job }, timeoutMs);
  },

  builds(job: string, limit = 20, timeoutMs = 30_000): Promise<{ builds: BuildSummary[] }> {
    return invoke<{ builds: BuildSummary[] }>("jenkins/builds", { job, limit }, timeoutMs);
  },

  build(job: string, number: number, timeoutMs = 30_000): Promise<BuildDetail> {
    return invoke<BuildDetail>("jenkins/build", { job, number }, timeoutMs);
  },

  /** 单次构建的阶段，供构建列表的进度条按阶段步进；比 jenkins/stages 矩阵轻得多。 */
  buildStages(
    job: string,
    number: number,
    timeoutMs = 15_000,
  ): Promise<{ stages: StageInfo[] }> {
    return invoke<{ stages: StageInfo[] }>(
      "jenkins/buildStages",
      { job, number },
      timeoutMs,
    );
  },

  console(job: string, number: number, start: number): Promise<ConsoleChunk> {
    return invoke<ConsoleChunk>(
      "jenkins/console",
      { job, number, start },
      60_000,
    );
  },

  /** 制品大小与类型；HEAD 请求，通常几十毫秒。 */
  artifactInfo(
    job: string,
    number: number,
    path: string,
    timeoutMs = 30_000,
  ): Promise<ArtifactInfo> {
    return invoke<ArtifactInfo>("jenkins/artifactInfo", { job, number, path }, timeoutMs);
  },

  /**
   * 制品分块。首块会触发后端把整份制品落盘缓存，所以超时给到 10 分钟；
   * 下载完成后每块只读本地缓存，剩余分块都在几十毫秒内返回。
   */
  artifactChunk(
    job: string,
    number: number,
    path: string,
    offset: number,
    length: number,
    timeoutMs = 600_000,
  ): Promise<ArtifactChunk> {
    return invoke<ArtifactChunk>(
      "jenkins/artifactChunk",
      { job, number, path, offset, length },
      timeoutMs,
    );
  },

  /**
   * 启动后台下载任务，立即返回 taskId。
   * 落盘只能由 sidecar 完成：DBX 工作台的 iframe 没有 allow-downloads。
   */
  artifactStart(
    job: string,
    number: number,
    path: string,
    timeoutMs = 30_000,
  ): Promise<ArtifactTaskSnapshot> {
    return invoke<ArtifactTaskSnapshot>("jenkins/artifactStart", { job, number, path }, timeoutMs);
  },

  /** 轮询下载任务进度；终态后前端停止轮询。 */
  artifactStatus(taskId: string, timeoutMs = 15_000): Promise<ArtifactTaskSnapshot> {
    return invoke<ArtifactTaskSnapshot>("jenkins/artifactStatus", { taskId }, timeoutMs);
  },

  artifactCancel(taskId: string): Promise<{ success: boolean }> {
    return invoke<{ success: boolean }>("jenkins/artifactCancel", { taskId }, 15_000);
  },

  /** 在系统文件管理器里定位已落盘的文件。 */
  revealFile(path: string): Promise<{ success: boolean }> {
    return invoke<{ success: boolean }>("jenkins/revealFile", { path }, 15_000);
  },

  /** 当前连接实际使用的下载目录（用于面板提示）。 */
  downloadDir(): Promise<{ dir: string }> {
    return invoke<{ dir: string }>("jenkins/downloadDir", {}, 15_000);
  },

  /** 把文本写成文件放到下载目录（控制台日志导出兜底）。 */
  saveText(name: string, content: string, timeoutMs = 60_000): Promise<SavedFile> {
    return invoke<SavedFile>("jenkins/saveText", { name, content }, timeoutMs);
  },

  /**
   * 导出**完整**控制台日志：后端直接拉 Jenkins 的 consoleText 流式落盘，
   * 不受前端内存里那份 2 MB 截断影响；返回任务快照，用 artifactStatus 轮询。
   */
  consoleSave(
    job: string,
    number: number,
    timeoutMs = 30_000,
  ): Promise<ArtifactTaskSnapshot> {
    return invoke<ArtifactTaskSnapshot>("jenkins/consoleSave", { job, number }, timeoutMs);
  },

  trigger(job: string, params: Record<string, unknown> = {}): Promise<TriggerResult> {
    return invoke<TriggerResult>("jenkins/trigger", { job, params }, 60_000);
  },

  /**
   * 批量预检：一次拿到所选任务的实时状态与参数定义。
   * 只读；单个任务读取失败会在该条带 error 返回，不影响其余任务。
   */
  batchInspect(jobs: string[]): Promise<{ targets: BatchTargetInfo[] }> {
    return invoke<{ targets: BatchTargetInfo[] }>("jenkins/batchInspect", { jobs }, 60_000);
  },

  /**
   * 批量触发构建：一次 RPC 返回逐任务结果。
   * 前端按 10 个一批调用，单批 60s 够用；超时只影响这一批，其余批次照常。
   * `params` 为本批共用参数，后端只把它应用到任务自己声明过的参数名上。
   */
  batchBuild(
    jobs: string[],
    skipBusy = true,
    params: Record<string, unknown> = {},
  ): Promise<{ results: BatchBuildResult[] }> {
    return invoke<{ results: BatchBuildResult[] }>(
      "jenkins/batchBuild",
      { jobs, skipBusy, params },
      60_000,
    );
  },

  stop(job: string, number: number): Promise<{ success: boolean }> {
    return invoke<{ success: boolean }>("jenkins/stop", { job, number }, 30_000);
  },

  overview(timeoutMs = 30_000): Promise<Overview> {
    return invoke<Overview>("jenkins/overview", {}, timeoutMs);
  },

  queue(timeoutMs = 30_000): Promise<{ items: QueueItem[] }> {
    return invoke<{ items: QueueItem[] }>("jenkins/queue", {}, timeoutMs);
  },

  cancelQueue(id: number): Promise<{ success: boolean }> {
    return invoke<{ success: boolean }>("jenkins/cancelQueue", { id }, 30_000);
  },

  nodes(): Promise<{ nodes: NodeInfo[] }> {
    return invoke<{ nodes: NodeInfo[] }>("jenkins/nodes", {}, 30_000);
  },

  /** 执行器槽位明细；`timeoutMs` 供轮询传更短的超时。 */
  executors(timeoutMs = 30_000): Promise<{ executors: ExecutorInfo[] }> {
    return invoke<{ executors: ExecutorInfo[] }>("jenkins/executors", {}, timeoutMs);
  },

  stages(job: string, limit = 10, timeoutMs = 120_000): Promise<StageMatrix> {
    return invoke<StageMatrix>("jenkins/stages", { job, limit }, timeoutMs);
  },

  jobConfig(job: string): Promise<JobConfigResponse> {
    return invoke<JobConfigResponse>("jenkins/jobConfig", { job });
  },

  createJob(payload: CreateJobPayload): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/createJob", { ...payload }, 60_000);
  },

  updateJob(job: string, fields: JobUpdateFields): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/updateJob", { job, fields }, 60_000);
  },

  saveJob(job: string, xml: string): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/saveJob", { job, xml }, 60_000);
  },

  renameJob(job: string, newName: string): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/renameJob", { job, newName }, 60_000);
  },

  deleteJob(job: string): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/deleteJob", { job }, 60_000);
  },

  viewConfig(view: string): Promise<ViewConfigResponse> {
    return invoke<ViewConfigResponse>("jenkins/viewConfig", { view });
  },

  createView(payload: CreateViewPayload): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/createView", { ...payload }, 60_000);
  },

  updateView(view: string, fields: ViewUpdateFields): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/updateView", { view, fields }, 60_000);
  },

  saveView(view: string, xml: string): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/saveView", { view, xml }, 60_000);
  },

  deleteView(view: string): Promise<MutationResult> {
    return invoke<MutationResult>("jenkins/deleteView", { view }, 60_000);
  },
};
