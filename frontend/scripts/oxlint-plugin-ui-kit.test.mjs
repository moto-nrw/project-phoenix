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

  it("accepts semantic moto color tokens", () => {
    expect(lintLiteral("bg-moto-red-soft text-moto-red-strong")).toHaveLength(
      0,
    );
  });

  it("keeps test fixtures exempt", () => {
    expect(lintLiteral("bg-red-50", "src/example.test.tsx")).toHaveLength(0);
  });
});
