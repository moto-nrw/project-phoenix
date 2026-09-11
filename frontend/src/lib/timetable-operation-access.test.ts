import { describe, expect, it } from "vitest";

import { isTimetableOperationForbidden } from "./timetable-operation-access";
import { TimetableOperationsApiError } from "./timetable-operations-api";

describe("isTimetableOperationForbidden", () => {
  it("recognises the backend's planning denial", () => {
    expect(
      isTimetableOperationForbidden(
        new TimetableOperationsApiError("timetable operation forbidden", 403),
      ),
    ).toBe(true);
  });

  it.each([
    ["another 403", new TimetableOperationsApiError("forbidden", 403)],
    [
      "the same text with another status",
      new TimetableOperationsApiError("timetable operation forbidden", 409),
    ],
    [
      "an admin without a staff profile",
      new TimetableOperationsApiError(
        "timetable operation forbidden: no staff profile",
        403,
      ),
    ],
    ["a plain error", new Error("timetable operation forbidden")],
    ["a non-error value", "timetable operation forbidden"],
  ])("ignores %s", (_label, err) => {
    expect(isTimetableOperationForbidden(err)).toBe(false);
  });
});
