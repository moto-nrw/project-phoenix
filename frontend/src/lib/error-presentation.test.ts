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

  it("explains the remaining capacity for a partial bulk admission", () => {
    const error = new ApiError("capacity", 409, {
      code: "presence.room_capacity_exceeded",
      details: {
        room_name: "Turnhalle",
        current_occupancy: 29,
        max_capacity: 30,
        incoming_students: 2,
      },
    });

    expect(presentError(error, "die Anwesenheit").message).toBe(
      "Der Raum Turnhalle: 29 von 30 Plätzen sind belegt. Freie Plätze: 1. Es sollen 2 Kinder dazukommen.",
    );
  });

  it("names the browser's cookies, not the reader's entries, for an oversized request", () => {
    // #3883: Node rejects the request with 431 before any route answers, so
    // the error has no envelope and only the status says what happened.
    const result = presentError(
      new ApiError("Request Header Fields Too Large", 431),
      "Die Personalliste",
    );

    expect(result.message).toBe(
      "Die Personalliste konnte nicht bearbeitet werden. Bitte löschen Sie im Browser die Cookies dieser Seite. Melden Sie sich danach neu an.",
    );
    expect(result.retryable).toBe(false);
    expect(result.requestId).toBeUndefined();
  });

  it("falls back to the class text in the reader's language when a code is unknown", () => {
    const error = new ApiError("Backend diagnostic", 503, {
      code: "future.unknown",
      instance: "req-18",
    });
    const result = presentError(error, "the group", "en");
    // #2518: parents read the next step in their own language, also for a
    // code this frontend does not know yet.
    expect(result.message).toBe(
      "The group is unavailable right now. Please try again.",
    );
    expect(result.message).not.toContain("Backend diagnostic");
    expect(result.requestId).toBe("req-18");
    expect(result.retryable).toBe(true);
  });

  it.each([
    "identity.mfa_blocked",
    "identity.password_reset_rate_limited",
  ] as const)("makes %s retryable after its cooldown", (code) => {
    const error = new ApiError("rate limited", 429, {
      code,
      instance: "req-429",
    });

    expect(presentError(error, "die Anmeldung")).toMatchObject({
      errorClass: "unavailable",
      retryable: true,
      requestId: "req-429",
    });
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
