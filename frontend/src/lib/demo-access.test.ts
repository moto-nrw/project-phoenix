import { describe, expect, it } from "vitest";
import {
  DEMO_SETUP_STEPS,
  demoSetupStep,
  isDemoEntryPath,
} from "./demo-access";

describe("isDemoEntryPath", () => {
  it("matches the entry page on a subdomain and in path mode", () => {
    expect(isDemoEntryPath("/demo", "messe-demo")).toBe(true);
    expect(isDemoEntryPath("/demo/", undefined)).toBe(true);
    expect(isDemoEntryPath("/messe-demo/demo", "messe-demo")).toBe(true);
  });

  it("matches nothing else", () => {
    expect(isDemoEntryPath(null, "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/", "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/demo/settings", "messe-demo")).toBe(false);
    expect(isDemoEntryPath("/other/demo", "messe-demo")).toBe(false);
  });
});

describe("demoSetupStep", () => {
  it("moves through the setup lines while the school is prepared", () => {
    expect(demoSetupStep(0)).toBe(0);
    expect(demoSetupStep(2)).toBe(0);
    expect(demoSetupStep(3)).toBe(1);
    expect(demoSetupStep(6)).toBe(2);
  });

  it("stays on the last line until the school is ready", () => {
    expect(demoSetupStep(1000)).toBe(DEMO_SETUP_STEPS.length - 1);
  });
});
