import { describe, expect, it, vi } from "vitest";
import * as sessionCache from "./session-cache";
import {
  fetchStaffOnboarding,
  mapStaffOnboardingState,
} from "./staff-onboarding-api";

describe("fetchStaffOnboarding", () => {
  it("refuses a state without step lists instead of reopening every step", async () => {
    vi.spyOn(sessionCache, "sessionFetch").mockResolvedValue(
      new Response(JSON.stringify({ data: { dismissed: false } }), {
        status: 200,
      }),
    );

    await expect(fetchStaffOnboarding()).rejects.toMatchObject({
      status: 500,
      code: "general.server",
    });
  });

  it("has no checklist without a school account", async () => {
    vi.spyOn(sessionCache, "sessionFetch").mockResolvedValue(
      new Response(null, { status: 403 }),
    );

    await expect(fetchStaffOnboarding()).resolves.toBeNull();
  });
});

describe("mapStaffOnboardingState", () => {
  it("maps the backend status and drops unknown steps", () => {
    expect(
      mapStaffOnboardingState({
        dismissed: false,
        school_ready: true,
        done_steps: ["students", "retired"],
        skipped_steps: ["work_time"],
      }),
    ).toEqual({
      dismissed: false,
      schoolReady: true,
      doneSteps: ["students"],
      skippedSteps: ["work_time"],
    });
  });
});
