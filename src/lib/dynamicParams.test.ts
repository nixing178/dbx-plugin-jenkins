import { describe, expect, it } from "vitest";
import type { ParameterDefinition } from "../types";
import { changedReferencedParameters, hasDynamicParameters, reconcileParameterValues } from "./dynamicParams";

function parameter(overrides: Partial<ParameterDefinition>): ParameterDefinition {
  return {
    name: "VALUE",
    type: "StringParameterDefinition",
    description: "",
    defaultValue: "",
    ...overrides,
  };
}

describe("dynamic parameter options", () => {
  it("detects only parameters marked dynamic by Jenkins", () => {
    expect(hasDynamicParameters([parameter({ name: "ENV", choices: ["dev"] })])).toBe(false);
    expect(hasDynamicParameters([parameter({ dynamic: true })])).toBe(true);
  });

  it("keeps a valid branch selection", () => {
    const result = reconcileParameterValues(
      [parameter({ name: "BRANCH", dynamic: true, choices: ["origin/dev", "origin/test"] })],
      { BRANCH: "origin/test" },
    );
    expect(result.BRANCH).toBe("origin/test");
  });

  it("uses the first enabled choice when a single-select value is stale", () => {
    const result = reconcileParameterValues(
      [parameter({
        name: "BRANCH",
        dynamic: true,
        choices: ["origin/old", "origin/dev"],
        disabledChoices: ["origin/old"],
        selectedValues: ["origin/old"],
      })],
      { BRANCH: "origin/removed" },
    );
    expect(result.BRANCH).toBe("origin/dev");
  });

  it("retains valid module selections and removes stale or disabled ones", () => {
    const result = reconcileParameterValues(
      [parameter({
        name: "BUILD_MODULES",
        dynamic: true,
        multiSelect: true,
        choices: ["iot-codec", "iot-log", "iot-connector"],
        disabledChoices: ["iot-log"],
      })],
      { BUILD_MODULES: "iot-codec,removed,iot-log", SKIP_TESTS: true },
    );
    expect(result.BUILD_MODULES).toBe("iot-codec");
    expect(result.SKIP_TESTS).toBe(true);
  });

  it("uses Jenkins selected values when no valid multi-select value remains", () => {
    const result = reconcileParameterValues(
      [parameter({
        name: "BUILD_MODULES",
        dynamic: true,
        multiSelect: true,
        choices: ["iot-codec", "iot-log"],
        selectedValues: ["iot-log"],
      })],
      { BUILD_MODULES: "请先选择分支" },
    );
    expect(result.BUILD_MODULES).toBe("iot-log");
  });

  it("leaves multi-select empty rather than build every module by default", () => {
    const result = reconcileParameterValues(
      [parameter({
        name: "BUILD_MODULES",
        dynamic: true,
        multiSelect: true,
        choices: ["iot-codec", "iot-log"],
      })],
      { BUILD_MODULES: "请先选择分支" },
    );
    expect(result.BUILD_MODULES).toBe("");
  });

  it("clears a dynamic value when its choices are empty", () => {
    const result = reconcileParameterValues(
      [parameter({ name: "BRANCH", dynamic: true, choices: [] })],
      { BRANCH: "origin/old" },
    );
    expect(result.BRANCH).toBe("");
  });

  it("normalizes an invalid static choice but preserves plain text", () => {
    const result = reconcileParameterValues(
      [
        parameter({ name: "REPOSITORY", type: "ChoiceParameterDefinition", choices: ["repo-a", "repo-b"] }),
        parameter({ name: "TEXT", type: "StringParameterDefinition" }),
      ],
      { REPOSITORY: "", TEXT: "custom" },
    );
    expect(result.REPOSITORY).toBe("repo-a");
    expect(result.TEXT).toBe("custom");
  });

  it("detects normalized values that require dependent options to refresh", () => {
    const parameters = [
      parameter({ name: "BRANCH", dynamic: true, referencedParameters: ["REPOSITORY"] }),
      parameter({ name: "BUILD_MODULES", dynamic: true, referencedParameters: ["REPOSITORY", "BRANCH"] }),
    ];
    const changed = changedReferencedParameters(
      parameters,
      { REPOSITORY: "", BRANCH: "请先选择代码仓库" },
      { REPOSITORY: "repo-a", BRANCH: "origin/master" },
    );
    expect(new Set(changed)).toEqual(new Set(["REPOSITORY", "BRANCH"]));
    expect(changedReferencedParameters(parameters, { REPOSITORY: "repo-a" }, { REPOSITORY: "repo-a" })).toEqual([]);
  });
});
