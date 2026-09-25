import { describe, expect, it } from "vitest";
import { ApiError } from "./api-error";
import { presentError } from "./error-presentation";

describe("presentError", () => {
  it("uses structured details, never the backend sentence", () => {
    const error = new ApiError("This is a backend diagnostic", 409, {
      code: "iot.room_capacity_exceeded",
      details: { room_name: "Sonnenraum" },
      instance: "req-17",
    });
    const result = presentError(error, "Der Raum");
    expect(result.kind).toBe("api");
    expect(result.message).toBe(
      "Im Raum Sonnenraum ist kein Platz mehr. Bitte wählen Sie einen anderen Raum.",
    );
    expect(result.message).not.toContain(error.message);
    expect(result.requestId).toBeUndefined();
    expect(result.retryable).toBe(false);
  });

  it("falls back to the German class text when a code is unknown", () => {
    const error = new ApiError("English diagnostic", 503, {
      code: "future.unknown",
      instance: "req-18",
    });
    const result = presentError(error, "die Gruppe", "en");
    expect(result.message).toContain("Die Gruppe");
    expect(result.message).not.toContain("English diagnostic");
    expect(result.requestId).toBe("req-18");
    expect(result.retryable).toBe(true);
  });

  it("uses the class text when a known code has no runtime override", () => {
    const error = new ApiError("backend", 400, {
      code: "care.announcement_ack_not_required",
    });
    expect(presentError(error, "Der Elternbrief").message).toBe(
      "Der Elternbrief konnte nicht übernommen werden. Bitte prüfen Sie Ihre Angaben.",
    );
  });

  it("distinguishes an API class error from an uncoded crash", () => {
    const api = presentError(new ApiError("backend", 400), "Das Kind");
    const crash = presentError(new TypeError("frontend crash"), "Das Kind");
    expect(api).toMatchObject({ kind: "api", errorClass: "input" });
    expect(crash).toMatchObject({ kind: "crash", retryable: false });
    expect(crash.message).not.toContain("frontend crash");
  });

  it("marks a 401 for login navigation instead of a toast", () => {
    const error = new ApiError("backend", 401, { code: "general.permission" });
    expect(presentError(error, "die Gruppe")).toMatchObject({
      kind: "api",
      requiresLogin: true,
      retryable: false,
      requestId: undefined,
    });
  });

  it("never interpolates values from the diagnostic sentence", () => {
    const error = new ApiError("room_name=Hinterzimmer", 409, {
      code: "iot.room_capacity_exceeded",
    });
    expect(presentError(error, "Der Raum").message).toBe(
      "Der Raum konnte nicht geändert werden. Bitte prüfen Sie den aktuellen Stand.",
    );
  });
});
