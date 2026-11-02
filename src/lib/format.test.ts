import { describe, expect, it } from "vitest";
import { formatBytes, formatDuration, formatTimestamp, shortenPath } from "./format";

describe("formatDuration", () => {
  it("formats sub-minute durations", () => {
    expect(formatDuration(0)).toBe("—");
    expect(formatDuration(-5)).toBe("—");
    expect(formatDuration(1_500)).toBe("1s");
    expect(formatDuration(59_000)).toBe("59s");
  });

  it("formats minutes and hours", () => {
    expect(formatDuration(65_000)).toBe("1m 5s");
    expect(formatDuration(3_600_000)).toBe("1h 0m");
    expect(formatDuration(3_725_000)).toBe("1h 2m");
  });
});

describe("formatTimestamp", () => {
  it("returns a dash for empty values", () => {
    expect(formatTimestamp(0, "zh")).toBe("—");
  });

  it("formats a valid epoch in local time", () => {
    const text = formatTimestamp(1_700_000_000_000, "zh");
    expect(text).not.toBe("—");
    expect(text).toMatch(/\d{4}/);
  });
});

describe("formatBytes", () => {
  it("按 1024 进制给一位小数", () => {
    expect(formatBytes(0)).toBe("0 B");
    expect(formatBytes(1023)).toBe("1023 B");
    expect(formatBytes(1024)).toBe("1.0 KB");
    expect(formatBytes(1536)).toBe("1.5 KB");
    expect(formatBytes(5 * 1024 * 1024)).toBe("5.0 MB");
    expect(formatBytes(1024 * 1024 * 1024)).toBe("1.0 GB");
    expect(formatBytes(-1)).toBe("—");
    expect(formatBytes(Number.NaN)).toBe("—");
  });
});

describe("shortenPath", () => {
  it("保留末尾两段并省略盘符与中间目录", () => {
    expect(shortenPath("C:\\Users\\anjun\\Downloads\\app.jar")).toBe("…\\Downloads\\app.jar");
    expect(shortenPath("/home/anjun/Downloads/app.jar")).toBe("…\\Downloads\\app.jar");
  });

  it("路径本来就很短时原样返回", () => {
    expect(shortenPath("Downloads\\app.jar")).toBe("Downloads\\app.jar");
    expect(shortenPath("")).toBe("");
  });
});
