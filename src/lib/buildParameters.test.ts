import { describe, expect, it } from "vitest";
import { buildParameterRows, parameterValueLabel } from "./buildParameters";
import type { BuildParameter } from "../types";

describe("buildParameterRows", () => {
  it("returns an empty list for missing parameters", () => {
    expect(buildParameterRows(null)).toEqual([]);
    expect(buildParameterRows(undefined)).toEqual([]);
    expect(buildParameterRows([])).toEqual([]);
  });

  it("trims names and drops blank rows", () => {
    const parameters: BuildParameter[] = [
      { name: " ENV ", value: "prod" },
      { name: "   ", value: "ignored" },
      { name: "BRANCH_NAME", value: "release/1.4" },
    ];

    expect(buildParameterRows(parameters)).toEqual([
      { name: "ENV", value: "prod" },
      { name: "BRANCH_NAME", value: "release/1.4" },
    ]);
  });

  it("keeps the backend order instead of sorting", () => {
    const parameters: BuildParameter[] = [
      { name: "ENV", value: "prod" },
      { name: "BRANCH_NAME", value: "main" },
      { name: "SKIP_TESTS", value: "false" },
    ];

    expect(buildParameterRows(parameters).map((row) => row.name)).toEqual([
      "ENV",
      "BRANCH_NAME",
      "SKIP_TESTS",
    ]);
  });
});

describe("parameterValueLabel", () => {
  it("uses a placeholder for empty values", () => {
    expect(parameterValueLabel("")).toBe("—");
    expect(parameterValueLabel("   ")).toBe("—");
  });

  it("keeps non-empty values verbatim", () => {
    expect(parameterValueLabel("release/1.4")).toBe("release/1.4");
    expect(parameterValueLabel("false")).toBe("false");
  });
});
