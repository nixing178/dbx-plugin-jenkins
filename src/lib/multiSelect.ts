import { toggleSelection } from "./paramValue";

export function filterParameterChoices(
  choices: readonly string[],
  query: string,
): string[] {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return [...choices];
  return choices.filter((choice) => choice.toLocaleLowerCase().includes(needle));
}

/** 全选当前筛选结果，不影响筛选之外或 choices 之外的旧选择。 */
export function selectFilteredParameterChoices(
  choices: readonly string[],
  filteredChoices: readonly string[],
  current: string,
  disabledChoices: readonly string[] = [],
): string {
  const disabled = new Set(disabledChoices);
  return filteredChoices.reduce(
    (value, choice) => disabled.has(choice) ? value : toggleSelection(choices, value, choice, true),
    current,
  );
}

/** 在当前筛选结果中移动键盘高亮，跳过 Jenkins 禁用的选项。 */
export function nextParameterChoiceIndex(
  choices: readonly string[],
  current: number,
  delta: 1 | -1,
  disabledChoices: readonly string[] = [],
): number {
  const disabled = new Set(disabledChoices);
  for (let index = current + delta; index >= 0 && index < choices.length; index += delta) {
    if (!disabled.has(choices[index])) return index;
  }
  const start = delta === 1 ? 0 : choices.length - 1;
  const end = delta === 1 ? choices.length : -1;
  for (let index = start; index !== end; index += delta) {
    if (!disabled.has(choices[index])) return index;
  }
  return -1;
}
