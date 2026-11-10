import type { BuildParameter } from "../types";

/**
 * 构建参数行数据清洗：名称去空白、丢弃空名称，其余保持 Jenkins 返回顺序。
 * 后端已经做过同名合并，这里只负责界面层的兜底。
 */
export function buildParameterRows(
  parameters: BuildParameter[] | null | undefined,
): BuildParameter[] {
  return (parameters ?? [])
    .map((parameter) => ({
      name: parameter.name.trim(),
      value: parameter.value ?? "",
    }))
    .filter((parameter) => parameter.name !== "");
}

/** 空值（未填 / null）统一显示占位符，避免出现只有名称没有值的空卡片。 */
export function parameterValueLabel(value: string): string {
  return value.trim() === "" ? "—" : value;
}
