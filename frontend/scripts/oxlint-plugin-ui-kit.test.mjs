import { describe, expect, it } from "vitest";

import uiKitPlugin from "./oxlint-plugin-ui-kit.mjs";

const rule = uiKitPlugin.rules["no-generic-brand-colors"];

function lintLiteral(value, filename = "src/components/example.tsx") {
  const reports = [];
  const visitors = rule.create({
    filename,
    physicalFilename: filename,
    report(finding) {
      reports.push(finding);
    },
  });

  visitors.Literal?.({ type: "Literal", value });
  return reports;
}

describe("ui-kit/no-generic-brand-colors", () => {
  it("rejects the first generic brand color without a baseline", () => {
    expect(lintLiteral("bg-red-50")).toHaveLength(1);
  });

  it("reports every generic brand color in a class string", () => {
    expect(lintLiteral("bg-red-50 hover:text-blue-600")).toHaveLength(2);
  });

  it("rejects directional generic border colors", () => {
    expect(lintLiteral("border-t-green-600 border-s-blue-500")).toHaveLength(2);
  });

  it("accepts semantic moto color tokens", () => {
    expect(lintLiteral("bg-moto-red-soft text-moto-red-strong")).toHaveLength(
      0,
    );
  });

  it("keeps test fixtures exempt", () => {
    expect(lintLiteral("bg-red-50", "src/example.test.tsx")).toHaveLength(0);
  });
});

describe("ui-kit/green-fill-white-text", () => {
  const greenRule = uiKitPlugin.rules["green-fill-white-text"];

  function lintGreen(value, filename = "src/components/example.tsx") {
    const reports = [];
    const visitors = greenRule.create({
      filename,
      physicalFilename: filename,
      report(finding) {
        reports.push(finding);
      },
    });
    visitors.Literal?.({ type: "Literal", value });
    return reports.map((report) => report.messageId);
  }

  it("rejects dark text on an opaque green fill", () => {
    expect(lintGreen("bg-moto-green text-gray-950")).toEqual(["darkText"]);
    expect(lintGreen("bg-moto-green-fill text-gray-900")).toEqual(["darkText"]);
    expect(
      lintGreen("[&>button]:!bg-moto-green [&>button]:!text-black"),
    ).toEqual(["darkText"]);
  });

  it("rejects white text on the light brand greens", () => {
    expect(lintGreen("bg-moto-green text-white")).toEqual([
      "whiteOnLightGreen",
    ]);
    expect(lintGreen("bg-moto-green-hover text-white")).toEqual([
      "whiteOnLightGreen",
    ]);
  });

  it("accepts white text on the fill steps", () => {
    expect(
      lintGreen(
        "bg-moto-green-fill text-white hover:bg-moto-green-fill-hover active:bg-moto-green-fill-active",
      ),
    ).toEqual([]);
  });

  it("leaves tinted green surfaces with dark text alone", () => {
    expect(lintGreen("bg-moto-green/10 text-gray-900")).toEqual([]);
    expect(lintGreen("bg-moto-green-soft text-gray-950")).toEqual([]);
  });

  it("keeps test fixtures exempt", () => {
    expect(
      lintGreen("bg-moto-green text-gray-950", "src/example.test.tsx"),
    ).toEqual([]);
  });
});
