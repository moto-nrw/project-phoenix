import { afterEach, describe, expect, it, vi } from "vitest";
import { ApiError, errorClassCode, unavailableApiError } from "~/lib/api-error";
import { ERROR_CODE_CLASSES } from "~/lib/error-codes.generated";
import {
  ERROR_DISPLAYED,
  errorLogFields,
  logDisplayedError,
  statusErrorClass,
} from "~/lib/error-log";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("errorLogFields", () => {
  it("keeps code, class, status and request ID of an API error", () => {
    const error = new ApiError("Der Raum ist voll.", 409, {
      code: "presence.room_capacity_exceeded",
      instance: "req-4711",
    });

    expect(errorLogFields(error)).toEqual({
      error_class: "business_rejection",
      error_code: "presence.room_capacity_exceeded",
      status: 409,
      request_id: "req-4711",
      error_name: "ApiError",
    });
  });

  it("classes a request that never reached the API as unavailable", () => {
    expect(errorLogFields(new TypeError("Failed to fetch"))).toEqual({
      error_class: "unavailable",
      error_name: "TypeError",
    });
    // The 503 of a transport failure is not an answer and is not logged.
    expect(
      errorLogFields(unavailableApiError(new TypeError("Load failed"))),
    ).toEqual({
      error_class: "unavailable",
      error_code: "general.unavailable",
      error_name: "TypeError",
    });
  });

  it("classes any other exception as a crash", () => {
    expect(errorLogFields(new TypeError("x is undefined"))).toEqual({
      error_class: "crash",
      error_name: "TypeError",
    });
    expect(errorLogFields("connection refused")).toEqual({
      error_class: "crash",
    });
  });

  it("drops a code or name that is not an identifier", () => {
    const error = new ApiError("x", 500, { code: "Anna Schmidt fehlt" });
    error.name = "Anna Schmidt";

    expect(errorLogFields(error)).toEqual({
      error_class: "server",
      status: 500,
    });
  });

  it("derives the class of every status like the registry's class codes", () => {
    for (const status of [400, 401, 403, 404, 409, 413, 422, 429, 500, 503]) {
      expect(statusErrorClass(status)).toBe(
        ERROR_CODE_CLASSES[errorClassCode(status)],
      );
    }
  });
});

describe("logDisplayedError", () => {
  it("writes no name from any field of the failure", () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    const error = new ApiError("Anna Schmidt ist schon eingecheckt", 409, {
      code: "presence.room_capacity_exceeded",
      detail: "student Anna Schmidt already checked in",
      details: { student_name: "Anna Schmidt" },
      errors: [{ field: "first_name", reason: "Anna Schmidt" }],
      instance: "req-1",
    });

    logDisplayedError("toast", {
      ...errorLogFields(error),
      fields: ["first_name"],
    });

    expect(warn).toHaveBeenCalledWith(ERROR_DISPLAYED, {
      surface: "toast",
      error_class: "business_rejection",
      error_code: "presence.room_capacity_exceeded",
      status: 409,
      request_id: "req-1",
      error_name: "ApiError",
      fields: ["first_name"],
    });
    expect(JSON.stringify(warn.mock.calls)).not.toContain("Anna");
  });
});
