export type UiLocale = "zh" | "en";

/** 把毫秒时长格式化为 1h 2m / 3m 4s / 12s。 */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "—";
  const totalSeconds = Math.floor(ms / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  if (hours > 0) return `${hours}h ${minutes}m`;
  if (minutes > 0) return `${minutes}m ${seconds}s`;
  return `${seconds}s`;
}

/** 把 Jenkins 的 epoch 毫秒格式化成本地时间。 */
export function formatTimestamp(ms: number, locale: UiLocale = "zh"): string {
  if (!ms || !Number.isFinite(ms)) return "—";
  return new Date(ms).toLocaleString(locale === "zh" ? "zh-CN" : "en-US", {
    hour12: false,
  });
}

/** 绝对路径太长时只留末尾几段，完整路径由调用方放进 title。 */
export function shortenPath(path: string, keepSegments = 2): string {
  if (!path) return "";
  const parts = path.split(/[\\/]/g).filter(Boolean);
  if (parts.length <= keepSegments) return path;
  return `…\\${parts.slice(-keepSegments).join("\\")}`;
}

const BYTE_UNITS = ["B", "KB", "MB", "GB", "TB"];

/** 文件大小：1024 进制、一位小数；非法值给破折号。 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  if (bytes < 1024) return `${Math.floor(bytes)} B`;

  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024;
    unit += 1;
  }
  return `${value.toFixed(1)} ${BYTE_UNITS[unit]}`;
}
