import { describe, expect, it, vi } from "vitest";

import {
  readEnrollmentError,
  translateEnrollmentErrorMessage,
} from "./enrollment-error-messages";

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 400,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("enrollment-error-messages", () => {
  it("translates the phase name validation error to German", () => {
    expect(
      translateEnrollmentErrorMessage(
        "phase validation: phase name is required",
      ),
    ).toBe("Bitte gib einen Namen für die Anmeldephase ein.");
  });

  it("prioritizes stable error codes over raw backend text", () => {
    expect(
      translateEnrollmentErrorMessage(
        "raw backend text",
        "enrollment.care_offering_full",
      ),
    ).toContain("bereits voll");
  });

  it("maps an incompatible Regeltermin period to a German explanation", () => {
    expect(
      translateEnrollmentErrorMessage(
        "care offering phase must be within the linked timetable template period",
        "enrollment.care_offering_template_period_mismatch",
      ),
    ).toBe(
      "Der Planungszeitraum des gewählten Regeltermins muss den gesamten Betreuungszeitraum der Anmeldephase abdecken. Wähle einen passenden Regeltermin oder entferne die Verknüpfung.",
    );
  });

  it("does not expose unknown English backend text to the UI", async () => {
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
    expect(error.rawMessage).toBe("weird backend error");
    expect(logger.error).toHaveBeenCalledWith(
      "test_event",
      expect.objectContaining({
        message: "Phase konnte nicht gespeichert werden (HTTP 500)",
        rawMessage: "weird backend error",
      }),
    );
  });

  it("maps the off-list pickup time code to a German message", () => {
    expect(
      translateEnrollmentErrorMessage(
        'invalid submission: child 0 field "pickup_times": weekday "mon" time "15:00" is not an allowed pickup time',
        "enrollment.pickup_time_not_allowed",
      ),
    ).toContain("vorgegebenen Liste");
  });

  it("maps the late-invite invalid code to a German message", () => {
    expect(
      translateEnrollmentErrorMessage(
        "late invite is invalid",
        "enrollment.late_invite_invalid",
      ),
    ).toContain("Nachzügler-Link");
  });

  it("keeps already-German backend messages", () => {
    expect(
      translateEnrollmentErrorMessage("Bitte prüfe die eingegebenen Daten."),
    ).toBe("Bitte prüfe die eingegebenen Daten.");
  });
});
