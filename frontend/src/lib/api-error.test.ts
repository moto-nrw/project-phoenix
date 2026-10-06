import { describe, expect, it, vi } from "vitest";
import {
  ApiError,
  apiErrorFromBody,
  apiErrorFromResponse,
  apiErrorFromText,
  errorClassCode,
  transportFetch,
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

describe("apiErrorFromResponse", () => {
  it("reads code, fields and request ID from the response body", async () => {
    const error = await apiErrorFromResponse(
      new Response(
        JSON.stringify({
          status: "error",
          error: "note too long",
          code: "general.input",
          errors: [{ field: "content", reason: "too long" }],
          instance: "req-9",
        }),
        { status: 400 },
      ),
      "Notiz konnte nicht gespeichert werden.",
    );

    expect(error).toBeInstanceOf(ApiError);
    expect(error.message).toBe("Notiz konnte nicht gespeichert werden.");
    expect(error.status).toBe(400);
    expect(error.code).toBe("general.input");
    expect(error.errors).toEqual([{ field: "content", reason: "too long" }]);
    expect(error.requestId).toBe("req-9");
  });

  it("falls back to the status class when the body cannot be read", async () => {
    const response = new Response("gateway down", { status: 502 });
    await response.text();

    const error = await apiErrorFromResponse(response, "Laden fehlgeschlagen");

    expect(error.status).toBe(502);
    expect(error.code).toBe("general.unavailable");
  });
});

describe("transportFetch", () => {
  it("preserves an aborted request", async () => {
    const abortError = new DOMException("Request aborted", "AbortError");
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValueOnce(abortError);
    try {
      await expect(
        transportFetch("/api/timetable/shift-coverage"),
      ).rejects.toBe(abortError);
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("preserves an AbortError that is not an Error instance", async () => {
    const abortError = { name: "AbortError", message: "Request aborted" };
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValueOnce(abortError);
    try {
      await expect(
        transportFetch("/api/timetable/shift-coverage"),
      ).rejects.toBe(abortError);
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("turns a request that never reached the API into general.unavailable", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValueOnce(new TypeError("Failed to fetch"));
    try {
      const error = await transportFetch("/api/timetable/instances").catch(
        (err: unknown) => err,
      );
      expect(error).toBeInstanceOf(ApiError);
      expect((error as ApiError).code).toBe("general.unavailable");
      expect((error as ApiError).status).toBe(503);
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("hands every HTTP answer through, including errors", async () => {
    const response = new Response("{}", { status: 409 });
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(response);
    try {
      await expect(transportFetch("/api/x")).resolves.toBe(response);
    } finally {
      fetchSpy.mockRestore();
    }
  });
});
