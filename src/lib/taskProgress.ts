/** 下载任务状态机（与后端 ArtifactTaskSnapshot.state 一致）。 */
export type DownloadTaskState = "running" | "saved" | "failed" | "canceled";

/** 已完成比例：0-100 四舍五入；总量未知时给 0，避免出现 100% 假象。 */
export function taskPercent(done: number, total: number): number {
  if (!Number.isFinite(done) || !Number.isFinite(total) || total <= 0 || done <= 0) {
    return 0;
  }
  return Math.min(100, Math.round((done / total) * 100));
}

/** 按钮上的进度文案；总量未知时返回空串，由界面显示「保存中…」。 */
export function formatTaskProgress(done: number, total: number): string {
  if (!Number.isFinite(total) || total <= 0) return "";
  return `${taskPercent(done, total)}%`;
}

/** 只有明确终态才停轮询；未知状态按进行中处理。 */
export function isTaskFinished(state: string): boolean {
  return state === "saved" || state === "failed" || state === "canceled";
}
