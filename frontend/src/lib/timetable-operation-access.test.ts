import { describe, expect, it } from "vitest";

import { ApiError } from "./api-error";
import { isTimetableOperationForbidden } from "./timetable-operation-access";
import { TimetableOperationsApiError } from "./timetable-operations-api";

describe("isTimetableOperationForbidden", () => {
  it("recognises the backend's planning denial by its code", () => {
    expect(
      isTimetableOperationForbidden(
        new TimetableOperationsApiError(
          "timetable operation forbidden",
          403,
          "timetable.operation_not_planned",
        ),
      ),
    ).toBe(true);
  });

  it.each([
    ["another 403", new TimetableOperationsApiError("forbidden", 403)],
    [
      "the denial text without its code",
      new TimetableOperationsApiError("timetable operation forbidden", 403),
    ],
    [
      "an admin without a staff profile",
      new ApiError("timetable operation forbidden: no staff profile", 403, {
        code: "timetable.no_staff_profile",
      }),
    ],
    ["a plain error", new Error("timetable operation forbidden")],
    ["a non-error value", "timetable.operation_not_planned"],
  ])("ignores %s", (_label, err) => {
    expect(isTimetableOperationForbidden(err)).toBe(false);
  });
});
