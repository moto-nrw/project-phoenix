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
          timetable_enabled: true,
          parent_app_used: false,
        },
        steps: [
          { key: "basics", applies: true, done: true, skipped: false },
          { key: "rooms", applies: false, done: false, skipped: false },
        ],
      }),
    ).toEqual({
      completed: false,
      dismissed: true,
      basics: {
        presenceMode: "binary",
        groupMode: "open_care",
        timetableEnabled: true,
        parentAppUsed: false,
      },
      steps: [
        { key: "basics", applies: true, done: true, skipped: false },
        { key: "rooms", applies: false, done: false, skipped: false },
      ],
    });
  });

  it("keeps an unanswered parent app question open and drops unknown steps", () => {
    const state = mapSchoolSetupState({
      basics: { parent_app_used: null },
      steps: [{ key: "someday", applies: true }],
    });
    expect(state.basics.parentAppUsed).toBeNull();
    expect(state.basics.presenceMode).toBe("detailed");
    expect(state.steps).toEqual([]);
  });
});
