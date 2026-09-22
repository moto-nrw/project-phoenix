import { describe, expect, it } from "vitest";
import {
  DEMO_SETUP_STEPS,
  demoEntryEvent,
  demoLinkFragment,
  demoSetupStep,
  isDemoEntryPath,
} from "./demo-access";

// A restart (#3470) travels through the fragment like a role switch does,
// and the entry into the fresh school reports it as such.
describe("demoEntryEvent", () => {
  it("reports an entry, a switch or a restart", () => {
    expect(demoEntryEvent({ token: "t" })).toBe("demo_entered");
    expect(demoEntryEvent({ token: "t", switched: true })).toBe(
      "demo_role_switched",
    );
    expect(demoEntryEvent({ token: "t", restarted: true })).toBe(
      "demo_restarted",
    );
  });

  it("carries the restart through the fragment", () => {
    expect(
      demoLinkFragment({ token: "t", role: "lead", restarted: true }),
    ).toBe("#token=t&role=lead&restarted=1");
    expect(demoLinkFragment({ token: "t" })).toBe("#token=t");
  });
});

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
