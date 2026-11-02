import { describe, expect, it } from "vitest";
import { isMultiSelect, parseSelection, toggleSelection } from "./paramValue";

const choices = ["t6-auth", "t6-gateway", "t6-base"];

describe("parseSelection", () => {
  it("拆分逗号分隔的值并去掉空白", () => {
    expect(parseSelection("a, b ,,c ")).toEqual(["a", "b", "c"]);
  });

  it("空串得到空数组", () => {
    expect(parseSelection("")).toEqual([]);
  });
});

describe("toggleSelection", () => {
  it("勾选后按 choices 顺序拼接", () => {
    expect(toggleSelection(choices, "t6-base", "t6-auth", true)).toBe("t6-auth,t6-base");
  });

  it("取消勾选后移除该值", () => {
    expect(toggleSelection(choices, "t6-auth,t6-gateway", "t6-auth", false)).toBe("t6-gateway");
  });

  it("全不选得到空串", () => {
    expect(toggleSelection(choices, "t6-auth", "t6-auth", false)).toBe("");
  });

  it("choices 之外的旧值保留在末尾", () => {
    expect(toggleSelection(choices, "legacy", "t6-auth", true)).toBe("t6-auth,legacy");
  });
});

describe("isMultiSelect", () => {
  it("multiSelect 且有序项才算多选", () => {
    expect(isMultiSelect({ name: "p", type: "ChoiceParameter", description: "", defaultValue: "", choices, multiSelect: true })).toBe(true);
  });

  it("没有选项时退回单选", () => {
    expect(isMultiSelect({ name: "p", type: "ChoiceParameter", description: "", defaultValue: "", choices: [], multiSelect: true })).toBe(false);
  });

  it("普通参数不是多选", () => {
    expect(isMultiSelect({ name: "p", type: "ChoiceParameterDefinition", description: "", defaultValue: "", choices })).toBe(false);
  });
});
