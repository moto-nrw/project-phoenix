import { describe, expect, it, vi } from "vitest";
import * as sessionCache from "./session-cache";
import {
  fetchSchoolSetup,
  mapSchoolSetupState,
  SchoolSetupError,
} from "./school-setup-api";

describe("fetchSchoolSetup", () => {
  it("refuses a state without steps instead of showing 0 of 0", async () => {
    vi.spyOn(sessionCache, "sessionFetch").mockResolvedValue(
      new Response(JSON.stringify({ data: { status: "success", data: {} } }), {
        status: 200,
      }),
    );

    await expect(fetchSchoolSetup()).rejects.toBeInstanceOf(SchoolSetupError);
  });
});

describe("mapSchoolSetupState", () => {
  it("maps the backend status to the wizard state", () => {
    expect(
      mapSchoolSetupState({
        completed: false,
        dismissed: true,
        basics: {
          presence_mode: "binary",
          group_mode: "open_care",
        },
        steps: [
          { key: "team", applies: true, done: true, skipped: false },
          { key: "rooms", applies: false, done: false, skipped: false },
        ],
      }),
    ).toEqual({
      completed: false,
      dismissed: true,
      basics: {
        presenceMode: "binary",
        groupMode: "open_care",
      },
      steps: [
        { key: "team", applies: true, done: true, skipped: false },
        { key: "rooms", applies: false, done: false, skipped: false },
      ],
    });
  });

  it("falls back to the registry defaults and drops unknown steps", () => {
    const state = mapSchoolSetupState({
      basics: {},
      // „basics“ war der gestrichene erste Schritt; ein alter Stand zeigt ihn nicht.
      steps: [
        { key: "someday", applies: true },
        { key: "basics", applies: true },
      ],
    });
    expect(state.basics).toEqual({
      presenceMode: "detailed",
      groupMode: "fixed_groups",
    });
    expect(state.steps).toEqual([]);
  });
});
