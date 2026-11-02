import type { ParameterDefinition } from "../types";

/** 把 "a,b" 形式的参数值拆成已选项；空白项忽略。 */
export function parseSelection(raw: string): string[] {
  return raw
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item !== "");
}

/**
 * 多选参数的值按 Jenkins 约定用逗号拼接，顺序跟 choices 保持一致。
 * choices 里没有的旧值保留在末尾，避免静默丢值。
 */
export function toggleSelection(
  choices: readonly string[],
  raw: string,
  choice: string,
  checked: boolean,
): string {
  const selected = new Set(parseSelection(raw));
  if (checked) {
    selected.add(choice);
  } else {
    selected.delete(choice);
  }
  const ordered = choices.filter((item) => selected.has(item));
  const extra = [...selected].filter((item) => !choices.includes(item));
  return [...ordered, ...extra].join(",");
}

/** uno-choice 的 PT_CHECKBOX 是多选；单选下拉与普通参数都不走多选分支。 */
export function isMultiSelect(parameter: ParameterDefinition): boolean {
  return parameter.multiSelect === true && (parameter.choices?.length ?? 0) > 0;
}
