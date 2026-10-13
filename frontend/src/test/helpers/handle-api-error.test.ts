import { describe, expect, it } from "vitest";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { mockHandleApiError } from "./handle-api-error";

describe("mockHandleApiError", () => {
  it("forwards the backend envelope with its domain code, details and field errors", async () => {
    const envelope = {
      type: "about:blank",
      title: "Conflict",
      status: 409,
      detail: "stale",
      code: "students.change_request_stale",
      details: { request_id: "7" },
      errors: [{ field: "version", reason: "stale" }],
    };

    const response = mockHandleApiError(
      new ApiResponseError(409, JSON.stringify(envelope), {
        contentType: "application/problem+json",
      }),
    );

    expect(response.status).toBe(409);
    expect(await response.json()).toEqual(envelope);
  });

  it("answers an error without a status with 500", async () => {
    const response = mockHandleApiError(new Error("API error (404): text"));

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      status: 500,
      code: "general.server",
    });
  });
});
