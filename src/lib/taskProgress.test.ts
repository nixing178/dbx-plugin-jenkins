import { describe, expect, it } from "vitest";
import { formatTaskProgress, isTaskFinished, taskPercent } from "./taskProgress";

describe("taskPercent", () => {
  it("按已完成比例四舍五入并夹在 0-100", () => {
    expect(taskPercent(0, 10)).toBe(0);
    expect(taskPercent(3, 5)).toBe(60);
    expect(taskPercent(10, 10)).toBe(100);
    expect(taskPercent(11, 10)).toBe(100);
    expect(taskPercent(-1, 10)).toBe(0);
  });

  it("总长度未知时返回 0，避免出现 100% 假象", () => {
    expect(taskPercent(1024, 0)).toBe(0);
    expect(taskPercent(Number.NaN, 10)).toBe(0);
  });
});

describe("formatTaskProgress", () => {
  it("已知总量时给出百分比", () => {
    expect(formatTaskProgress(5, 10)).toBe("50%");
  });

  it("总量未知时返回空串，由界面显示「保存中…」", () => {
    expect(formatTaskProgress(1024, 0)).toBe("");
  });
});

describe("isTaskFinished", () => {
  it("running 之外都算结束", () => {
    expect(isTaskFinished("running")).toBe(false);
    expect(isTaskFinished("saved")).toBe(true);
    expect(isTaskFinished("failed")).toBe(true);
    expect(isTaskFinished("canceled")).toBe(true);
  });

  it("未知状态按未结束处理，避免提前停轮询", () => {
    expect(isTaskFinished("")).toBe(false);
    expect(isTaskFinished("queued")).toBe(false);
  });
});
