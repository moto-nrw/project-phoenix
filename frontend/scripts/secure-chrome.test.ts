import { describe, expect, it } from "vitest";

import {
  assertPatchedChrome,
  isPatchedChrome,
  MIN_CHROME_VERSION,
  secureChromeUse,
  secureLaunchOptions,
} from "./secure-chrome";

describe("secure Chrome (#5612)", () => {
  it.each([
    ["156.0.8078.4", false], // bundled with Playwright 1.64.1
    ["156.0.8078.47", false], // preinstalled on GitHub runners
    ["151.0.7922.169", false], // preinstalled on Blacksmith runners
    ["156.0.8078.72", false],
    [MIN_CHROME_VERSION, true],
    ["156.0.8078.79", true],
    ["156.0.8079.0", true],
    ["158.0.8127.2", true],
  ])("treats %s as patched: %s", (version, patched) => {
    expect(isPatchedChrome(version)).toBe(patched);
  });

  it("rejects an outdated Chrome with an actionable message", () => {
    expect(() => assertPatchedChrome("156.0.8078.4")).toThrow(
      /156\.0\.8078\.4 ist älter als 156\.0\.8078\.73/,
    );
    expect(() => assertPatchedChrome("156.0.8078.80")).not.toThrow();
  });

  it("fails closed on an unknown version format", () => {
    expect(() => isPatchedChrome("HeadlessChrome/156")).toThrow(
      /Unbekanntes Chrome-Versionsformat/,
    );
  });

  it("forces system Chrome with sandbox and keeps existing options", () => {
    expect(
      secureLaunchOptions({
        args: ["--x"],
        chromiumSandbox: false,
        channel: "chromium",
      }),
    ).toEqual({ args: ["--x"], channel: "chrome", chromiumSandbox: true });
    expect(secureChromeUse()).toEqual({
      channel: "chrome",
      launchOptions: { channel: "chrome", chromiumSandbox: true },
    });
  });
});
