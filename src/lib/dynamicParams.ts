import type { ParameterDefinition } from "../types";
import { parseSelection } from "./paramValue";

export function hasDynamicParameters(parameters: ParameterDefinition[]): boolean {
  return parameters.some((parameter) => parameter.dynamic === true);
}

/** 保留仍有效的用户选择；失效的旧值/提示文字不能作为新选项提交。 */
export function reconcileParameterValues(
  parameters: ParameterDefinition[],
  values: Record<string, unknown>,
): Record<string, unknown> {
  const next = { ...values };
  for (const parameter of parameters) {
    if (!parameter.dynamic) continue;
    if (!parameter.choices?.length) {
      next[parameter.name] = "";
      continue;
    }
    const disabled = new Set(parameter.disabledChoices ?? []);
    const choices = parameter.choices.filter((choice) => !disabled.has(choice));
    if (parameter.multiSelect) {
      const selected = parseSelection(String(next[parameter.name] ?? "")).filter((value) =>
        choices.includes(value),
      );
      if (selected.length === 0) {
        for (const value of parameter.selectedValues ?? []) {
          if (choices.includes(value) && !selected.includes(value)) selected.push(value);
        }
      }
      next[parameter.name] = choices
        .filter((choice) => selected.includes(choice))
        .join(",");
      continue;
    }
    const current = String(next[parameter.name] ?? "");
    if (choices.includes(current)) continue;
    const selected = (parameter.selectedValues ?? []).find((value) => choices.includes(value));
    next[parameter.name] = selected ?? choices[0] ?? "";
  }
  return next;
}
