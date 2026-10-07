import { describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import { readEnrollmentError } from "./enrollment-api-error";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 400,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("readEnrollmentError", () => {
  it("keeps code, details, field errors and request ID for the shared error path", async () => {
    const logger = { error: vi.fn(), warn: vi.fn() };
    const error = await readEnrollmentError(
      jsonResponse(
        {
          status: "error",
          error: "child quota reached: 50 of 50 places occupied",
          code: "students.child_quota_reached",
          details: { booked_places: 50, occupied_places: 50 },
          errors: [{ field: "name", reason: "required" }],
          instance: "req-123",
        },
        { status: 409 },
      ),
      "Entscheidung konnte nicht gespeichert werden",
      logger,
      "test_event",
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(409);
    expect(error.code).toBe("students.child_quota_reached");
    expect(error.details).toEqual({ booked_places: 50, occupied_places: 50 });
    expect(error.errors).toEqual([{ field: "name", reason: "required" }]);
    expect(error.requestId).toBe("req-123");
  });

  it("never carries the backend sentence in the message", async () => {
    const logger = { error: vi.fn(), warn: vi.fn() };
    const error = await readEnrollmentError(
      jsonResponse({ error: "weird backend error" }, { status: 500 }),
      "Phase konnte nicht gespeichert werden",
      logger,
      "test_event",
    );

    expect(error.message).toBe(
      "Phase konnte nicht gespeichert werden (HTTP 500)",
    );
    expect(error.message).not.toContain("weird backend error");
    expect(error.code).toBe("general.server");
  });

  it("logs the backend sentence as diagnosis, server errors as error", async () => {
    const logger = { error: vi.fn(), warn: vi.fn() };
    await readEnrollmentError(
      jsonResponse(
        { error: "weird backend error", instance: "req-9" },
        { status: 500 },
      ),
      "Phase konnte nicht gespeichert werden",
      logger,
      "test_event",
    );

    expect(logger.error).toHaveBeenCalledWith(
      "test_event",
      expect.objectContaining({
        status: 500,
        code: "general.server",
        requestId: "req-9",
        rawMessage: "weird backend error",
      }),
    );
    expect(logger.warn).not.toHaveBeenCalled();
  });

  it("logs expected rejections as warnings", async () => {
    const logger = { error: vi.fn(), warn: vi.fn() };
    await readEnrollmentError(
      jsonResponse(
        {
          error: "phase name already exists",
          code: "enrollment.phase_name_exists",
        },
        { status: 409 },
      ),
      "Phase konnte nicht gespeichert werden",
      logger,
      "test_event",
    );

    expect(logger.warn).toHaveBeenCalledWith(
      "test_event",
      expect.objectContaining({ code: "enrollment.phase_name_exists" }),
    );
    expect(logger.error).not.toHaveBeenCalled();
  });

  it("classifies a body that is not JSON by its status", async () => {
    const logger = { error: vi.fn() };
    const error = await readEnrollmentError(
      new Response("Bad Gateway", { status: 502 }),
      "Anmeldung konnte nicht gesendet werden",
      logger,
      "test_event",
    );

    expect(error.code).toBe("general.unavailable");
    expect(logger.error).toHaveBeenCalledWith(
      "test_event",
      expect.objectContaining({ rawMessage: "Bad Gateway" }),
    );
  });
});
