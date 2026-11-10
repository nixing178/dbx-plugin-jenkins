export type JenkinsStatus =
  | "success"
  | "failure"
  | "unstable"
  | "building"
  | "aborted"
  | "disabled"
  | "not_built"
  | "unknown";

export interface ViewInfo {
  name: string;
  description: string;
  url: string;
}

export interface BuildRef {
  number: number;
  result: string;
  timestamp: number;
  duration: number;
  building: boolean;
}

export interface JobSummary {
  name: string;
  fullName: string;
  url: string;
  color: string;
  isFolder: boolean;
  status: JenkinsStatus;
  building: boolean;
  lastBuild?: BuildRef | null;
}

export interface ParameterDefinition {
  name: string;
  type: string;
  description: string;
  defaultValue: string;
  choices?: string[] | null;
  /** uno-choice 的 PT_CHECKBOX 为多选，值按逗号拼接。 */
  multiSelect?: boolean | null;
}

export interface JobDetail {
  name: string;
  fullName: string;
  description: string;
  url: string;
  color: string;
  class?: string;
  buildable: boolean;
  inQueue: boolean;
  lastBuild?: BuildRef | null;
  lastSuccessfulBuild?: BuildRef | null;
  lastFailedBuild?: BuildRef | null;
  parameters: ParameterDefinition[] | null;
  status: JenkinsStatus;
}

export interface Cause {
  shortDescription: string;
  userId?: string;
  userName?: string;
}

export interface BuildSummary {
  number: number;
  result: string;
  building: boolean;
  timestamp: number;
  duration: number;
  /** Jenkins 侧估算耗时，用于构建中的进度条；缺失时为 0。 */
  estimatedDuration: number;
  displayName: string;
  causes: Cause[] | null;
  status: JenkinsStatus;
}

export interface ChangeSetItem {
  commitId: string;
  message: string;
  author: string;
  /** 提交时间（epoch 毫秒）。后端归一化后下发，缺失为 0/undefined。 */
  timestamp?: number;
  /** 改动文件数，来自 Jenkins affectedPaths。 */
  fileCount?: number;
  /** 提交正文（已去掉首行 subject）。只有构建详情会返回。 */
  body?: string;
}

export interface Artifact {
  fileName: string;
  relativePath: string;
}

/** 制品元信息（后端 HEAD 制品 URL 得到，缺失长度时回退到下载后 stat）。 */
export interface ArtifactInfo {
  fileName: string;
  size: number;
  contentType?: string;
}

/** 制品分块：dataBase64 为原始字节的 base64，单块上限 4 MiB。 */
export interface ArtifactChunk {
  offset: number;
  length: number;
  total: number;
  eof: boolean;
  dataBase64: string;
}

/** sidecar 落盘结果：path 是用户机器上的绝对路径。 */
export interface SavedFile {
  path: string;
  fileName: string;
  size: number;
}

/** 后台下载任务快照；state 为 running / saved / failed / canceled。 */
export interface ArtifactTaskSnapshot {
  taskId: string;
  state: string;
  done: number;
  total: number;
  fileName?: string;
  path?: string;
  error?: string;
}

/** 本次构建实际使用的参数值（区别于任务上的参数定义）。 */
export interface BuildParameter {
  name: string;
  value: string;
}

export interface BuildDetail extends BuildSummary {
  estimatedDuration: number;
  fullDisplayName: string;
  url: string;
  parameters?: BuildParameter[] | null;
  changeSet: ChangeSetItem[] | null;
  artifacts: Artifact[] | null;
}

export interface ConsoleChunk {
  text: string;
  nextStart: number;
  moreData: boolean;
}

export interface StageInfo {
  id: string;
  name: string;
  status: string;
  startTimeMillis: number;
  durationMillis: number;
  pauseDurationMillis: number;
}

export interface StageBuildInfo {
  number: number;
  result: string;
  status: string;
  timestamp: number;
  duration: number;
  commitCount: number;
  changes?: ChangeSetItem[] | null;
  stages: StageInfo[] | null;
}

export interface StageAverageInfo {
  name: string;
  duration: number;
}

export interface StageMatrix {
  job: string;
  stageNames: string[];
  builds: StageBuildInfo[];
  averages: StageAverageInfo[];
  averageRun: number;
}

export interface QueueItem {
  id: number;
  jobName: string;
  jobUrl: string;
  why: string;
  stuck: boolean;
  blocked: boolean;
  buildableIn: number;
  inQueueSince: number;
  cause: string;
}

export interface NodeInfo {
  name: string;
  offline: boolean;
  temporarilyOffline: boolean;
  numExecutors: number;
  idle: number;
  busy: number;
  labels: string[] | null;
  offlineReason: string;
}

/** 节点上的一个执行器槽位；空闲槽位也会返回，用于显示占位。 */
export interface ExecutorInfo {
  /** 节点显示名；控制器为「内置节点」，后端可能返回空串。 */
  node: string;
  nodeOffline: boolean;
  /** 槽位序号，从 1 开始。 */
  index: number;
  idle: boolean;
  /** 任务全名；占位中（Queue$Executable）为空。 */
  jobName: string;
  buildNumber: number;
  building: boolean;
  timestamp: number;
  /** 单位毫秒；构建期间由服务端实时推进。 */
  estimatedDuration: number;
  duration: number;
  result: string;
  /** 执行器已占位，但构建对象还没建好。 */
  pending: boolean;
}

export interface Overview {
  version: string;
  user: string;
  mode: string;
  nodeDescription: string;
  quietingDown: boolean;
  queueLength: number;
  nodesOnline: number;
  nodesTotal: number;
  executorsBusy: number;
  executorsTotal: number;
}

export interface TriggerResult {
  success: boolean;
  queueId: number;
}

/** 批量构建里单个任务的结果，切片顺序与请求的任务顺序一致。 */
export interface BatchBuildResult {
  job: string;
  success: boolean;
  skipped: boolean;
  queueId: number;
  /** 跳过/失败原因：not_buildable | in_queue | building | 错误 Kind（forbidden / read_only …）。 */
  reason: string;
  error: string;
}

/** 批量预检结果：确认弹窗用它展示实时状态与参数定义。 */
export interface BatchTargetInfo {
  name: string;
  fullName: string;
  status: JenkinsStatus;
  building: boolean;
  buildable: boolean;
  inQueue: boolean;
  parameters: ParameterDefinition[] | null;
}

export interface WhoAmIResponse {
  success: boolean;
  readOnly: boolean;
  message: string;
  user: {
    name: string;
    authenticated: boolean;
    authorities?: string[] | null;
    mode?: string;
    version?: string;
  };
}

export type JobKind = "freestyle" | "pipeline";

export interface JobFields {
  title: string;
  kind: JobKind;
  description: string;
  disabled: boolean;
  script: string;
  /** 「归档产物」配置；未配置或 Pipeline 任务为 null。 */
  archive?: ArtifactArchive | null;
}

/** 归档产物（Archive the artifacts）的可编辑字段。 */
export interface ArtifactArchive {
  enabled: boolean;
  /** 文件模式（Ant 通配符），例如 dist 目录下所有 jar。 */
  files: string;
  /** 排除模式，逗号分隔。 */
  excludes: string;
  fingerprint: boolean;
  onlyIfSuccessful: boolean;
  allowEmptyArchive: boolean;
  defaultExcludes: boolean;
  caseSensitive: boolean;
}

export interface JobConfigResponse {
  xml: string;
  fields: JobFields;
}

export interface ListViewFields {
  name: string;
  description: string;
  jobNames: string[] | null;
  includeRegex: string;
  recurse: boolean;
}

export interface ViewConfigResponse {
  xml: string;
  fields: ListViewFields | null;
}

export interface JobUpdateFields {
  title?: string;
  description?: string;
  disabled?: boolean;
  script?: string;
  archive?: ArtifactArchive;
}

export interface ViewUpdateFields {
  name?: string;
  description?: string;
  jobNames?: string[];
  includeRegex?: string;
  recurse?: boolean;
}

export interface CreateJobPayload {
  folder: string;
  name: string;
  kind: JobKind;
  description: string;
  script: string;
}

export interface CreateViewPayload {
  name: string;
  description: string;
  jobs: string[];
  includeRegex: string;
  recurse: boolean;
}

export interface MutationResult {
  success: boolean;
  fullName?: string;
}
