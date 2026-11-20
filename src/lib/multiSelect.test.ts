import { describe, expect, it } from "vitest";
import {
  filterParameterChoices,
  nextParameterChoiceIndex,
  selectFilteredParameterChoices,
} from "./multiSelect";

const choices = ["buss-mock", "common-component", "iot-codec", "iot-connector", "iot-log"];

describe("filterParameterChoices", () => {
  it("keeps choice order and matches names case-insensitively", () => {
    expect(filterParameterChoices(choices, " IOT-")).toEqual(["iot-codec", "iot-connector", "iot-log"]);
  });

  it("trims the query and returns every choice for an empty filter", () => {
    expect(filterParameterChoices(choices, "  log ")).toEqual(["iot-log"]);
    expect(filterParameterChoices(choices, "  ")).toEqual(choices);
  });

  it("returns an empty result when no choice matches", () => {
    expect(filterParameterChoices(choices, "missing")).toEqual([]);
  });
});

describe("selectFilteredParameterChoices", () => {
  it("adds visible choices without clearing selections outside the filter", () => {
    expect(selectFilteredParameterChoices(
      choices,
      ["iot-codec", "iot-log"],
      "buss-mock",
    )).toBe("buss-mock,iot-codec,iot-log");
  });

  it("does not select disabled choices and preserves unknown existing values", () => {
    expect(selectFilteredParameterChoices(
      choices,
      ["iot-codec", "iot-log"],
      "legacy",
      ["iot-log"],
    )).toBe("iot-codec,legacy");
  });
});

describe("nextParameterChoiceIndex", () => {
  it("starts at the first enabled option and skips disabled options", () => {
    expect(nextParameterChoiceIndex(choices, -1, 1, ["buss-mock"])).toBe(1);
    expect(nextParameterChoiceIndex(choices, 2, 1, ["iot-connector"])).toBe(4);
  });

  it("supports reverse navigation and wraps at the ends", () => {
    expect(nextParameterChoiceIndex(choices, 0, -1)).toBe(4);
    expect(nextParameterChoiceIndex(choices, 4, 1)).toBe(0);
  });

  it("returns -1 when every option is disabled", () => {
    expect(nextParameterChoiceIndex(choices, -1, 1, choices)).toBe(-1);
    expect(nextParameterChoiceIndex([], -1, 1)).toBe(-1);
  });
});
