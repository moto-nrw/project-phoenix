import { describe, expect, it } from "vitest";
import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromText,
  errorClassCode,
} from "./api-error";

describe("ApiError", () => {
  it("keeps a 409 code and problem fields as structured values", () => {
    const error = apiErrorFromBody("Conflict", 409, {
      code: "students.child_quota_reached",
      details: { occupied_places: 50 },
      errors: [{ field: "first_name", reason: "required" }],
      instance: "request-409",
    });

    expect(error).toBeInstanceOf(ApiError);
    expect(error.code).toBe("students.child_quota_reached");
    expect(error.status).toBe(409);
    expect(error.details).toEqual({ occupied_places: 50 });
    expect(error.errors).toEqual([{ field: "first_name", reason: "required" }]);
    expect(error.requestId).toBe("request-409");
  });

  it.each([
    [400, "general.input"],
    [403, "general.permission"],
    [409, "general.business_rejection"],
    [429, "general.unavailable"],
    [503, "general.unavailable"],
    [500, "general.server"],
  ])("uses the backend class code for uncoded HTTP %i", (status, code) => {
    expect(errorClassCode(status)).toBe(code);
    expect(apiErrorFromBody("Legacy error", status, {}).code).toBe(code);
  });
});

describe("apiErrorFromText", () => {
  it("keeps the caller's message and reads the envelope from the raw body", () => {
    const error = apiErrorFromText(
      "Failed to update person: raw",
      400,
      JSON.stringify({
        code: "general.input",
        errors: [{ field: "first_name", reason: "is required" }],
        instance: "req-7",
      }),
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error.message).toBe("Failed to update person: raw");
    expect(error.code).toBe("general.input");
    expect(error.errors).toEqual([
      { field: "first_name", reason: "is required" },
    ]);
    expect(error.requestId).toBe("req-7");
  });

  it("falls back to the status class when the body is not JSON", () => {
    const error = apiErrorFromText("Bad gateway", 502, "<html>");

    expect(error.code).toBe("general.unavailable");
    expect(error.errors).toBeUndefined();
  });
});
