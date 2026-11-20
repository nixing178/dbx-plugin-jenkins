import { describe, expect, it } from "vitest";
import type { ParameterDefinition } from "../types";
import { hasDynamicParameters, reconcileParameterValues } from "./dynamicParams";

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

  it("does not change static parameter values", () => {
    const result = reconcileParameterValues(
      [parameter({ name: "ENV", choices: ["dev", "prod"] })],
      { ENV: "custom" },
    );
    expect(result.ENV).toBe("custom");
  });
});
