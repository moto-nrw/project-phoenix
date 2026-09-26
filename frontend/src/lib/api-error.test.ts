import { describe, expect, it } from "vitest";
import { ApiError, apiErrorFromBody, errorClassCode } from "./api-error";

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
