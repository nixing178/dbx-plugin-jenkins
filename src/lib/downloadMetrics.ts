import { formatBytes } from "./format";

export interface ProgressSample {
  done: number;
  at: number;
}

export interface TransferSummary {
  /** 已下载 / 总量，总量未知时只有已下载量。 */
  bytes: string;
  /** 速度文案，未知时为空串。 */
  speed: string;
  /** 剩余秒数，未知时为 null。 */
  etaSeconds: number | null;
}

/**
 * 用最近 windowMs 内的样本算平均速度（字节/秒）。
 * 轮询间隔 500ms，窗口默认 3s，能抹平单次抖动又跟得上速度变化。
 */
export function averageSpeed(samples: ProgressSample[], windowMs = 3000): number {
  if (samples.length < 2) return 0;

  const latest = samples[samples.length - 1];
  const cutoff = latest.at - Math.max(1, windowMs);
  const recent = samples.filter((sample) => sample.at >= cutoff);
  if (recent.length < 2) return 0;

  const first = recent[0];
  const span = latest.at - first.at;
  const delta = latest.done - first.done;
  if (span <= 0 || delta <= 0) return 0;

  return (delta * 1000) / span;
}

/** 按剩余字节与速度估算剩余秒数；未知返回 null。 */
export function remainingSeconds(done: number, total: number, speed: number): number | null {
  if (!Number.isFinite(done) || !Number.isFinite(total) || total <= 0) return null;
  if (!Number.isFinite(speed) || speed <= 0) return null;

  const remaining = Math.max(0, total - done);
  return Math.ceil(remaining / speed);
}

/** 组装副行文案需要的数据：已下载/总量、速度、剩余时间。 */
export function transferSummary(
  done: number,
  total: number,
  speed: number,
): TransferSummary {
  const downloaded = Number.isFinite(done) && done > 0 ? done : 0;
  const knownTotal = Number.isFinite(total) && total > 0 ? total : 0;

  return {
    bytes:
      knownTotal > 0
        ? `${formatBytes(downloaded)} / ${formatBytes(knownTotal)}`
        : formatBytes(downloaded),
    speed: Number.isFinite(speed) && speed > 0 ? `${formatBytes(speed)}/s` : "",
    etaSeconds: remainingSeconds(downloaded, knownTotal, speed),
  };
}
