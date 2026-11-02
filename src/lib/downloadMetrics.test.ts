import { describe, expect, it } from "vitest";
import { averageSpeed, remainingSeconds, transferSummary } from "./downloadMetrics";

describe("averageSpeed", () => {
  it("按样本区间算平均速度（字节/秒）", () => {
    expect(
      averageSpeed([
        { done: 0, at: 0 },
        { done: 1000, at: 1000 },
      ]),
    ).toBe(1000);
  });

  it("只统计最近窗口内的样本", () => {
    expect(
      averageSpeed(
        [
          { done: 0, at: 0 },
          { done: 10_000, at: 10_000 },
          { done: 20_000, at: 11_000 },
        ],
        3000,
      ),
    ).toBe(10_000);
  });

  it("样本不足或进度没前进时返回 0", () => {
    expect(averageSpeed([])).toBe(0);
    expect(averageSpeed([{ done: 100, at: 1000 }])).toBe(0);
    expect(
      averageSpeed([
        { done: 100, at: 0 },
        { done: 100, at: 1000 },
      ]),
    ).toBe(0);
  });
});

describe("remainingSeconds", () => {
  it("按剩余字节与速度估算秒数，向上取整", () => {
    expect(remainingSeconds(50, 100, 10)).toBe(5);
    expect(remainingSeconds(0, 100, 30)).toBe(4);
  });

  it("已完成返回 0", () => {
    expect(remainingSeconds(100, 100, 10)).toBe(0);
    expect(remainingSeconds(120, 100, 10)).toBe(0);
  });

  it("总量或速度未知时返回 null", () => {
    expect(remainingSeconds(10, 0, 10)).toBeNull();
    expect(remainingSeconds(10, 100, 0)).toBeNull();
    expect(remainingSeconds(10, 100, Number.NaN)).toBeNull();
  });
});

describe("transferSummary", () => {
  it("给出已下载/总量、速度与剩余时间", () => {
    const summary = transferSummary(8.5 * 1024 * 1024, 20 * 1024 * 1024, 3.2 * 1024 * 1024);
    expect(summary.bytes).toBe("8.5 MB / 20.0 MB");
    expect(summary.speed).toBe("3.2 MB/s");
    expect(summary.etaSeconds).toBe(4);
  });

  it("总量未知时只给已下载量，速度未知时为空串", () => {
    expect(transferSummary(1024, 0, 0)).toEqual({
      bytes: "1.0 KB",
      speed: "",
      etaSeconds: null,
    });
  });
});
