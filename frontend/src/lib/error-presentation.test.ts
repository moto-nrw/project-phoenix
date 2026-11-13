import { describe, expect, it } from "vitest";
import { ApiError } from "./api-error";
import { errorCodeMessage, presentError } from "./error-presentation";

describe("presentError", () => {
  it.each(["de", "en", "pl", "ru", "sq", "tr", "uk"] as const)(
    "formats the public holiday day limit without booking totals in %s",
    (locale) => {
      const error = new ApiError("internal day limit diagnostic", 400, {
        code: "enrollment.holiday_child_day_limit_public",
        details: { limit: 3, child: 1 },
      });
      const result = presentError(error, "die Anmeldung", locale);
      expect(result.message).toContain("3");
      expect(result.message).not.toMatch(/[{}#]|plural|diagnostic/);
      expect(result.retryable).toBe(false);
    },
  );

  it.each(["de", "en", "pl", "ru", "sq", "tr", "uk"] as const)(
    "formats the holiday day limit and other registrations in %s",
    (locale) => {
      for (const elsewhere of [0, 1, 3]) {
        const error = new ApiError("internal day limit diagnostic", 409, {
          code: "enrollment.holiday_child_day_limit",
          details: { limit: 5, days: 8, elsewhere, child: "child-17" },
        });
        const result = presentError(error, "die Anmeldung", locale);
        expect(result.message).toContain("5");
        expect(result.message).toContain("8");
        expect(result.message).toContain(String(elsewhere));
        expect(result.message).not.toMatch(/[{}#]|plural|child-17|diagnostic/);
        expect(result.retryable).toBe(false);
      }
    },
  );

  it.each([
    [
      "display.manage_forbidden",
      "Sie dürfen Info-Displays ansehen, aber nicht ändern. Bitte fragen Sie Ihre Schulleitung.",
    ],
    [
      "meals.participation_manage_denied",
      "Sie dürfen die Essensanmeldung ansehen, aber nicht ändern. Bitte wenden Sie sich an die OGS.",
    ],
  ] as const)(
    "explains read-only access for %s without retry",
    (code, message) => {
      const error = new ApiError("internal permission diagnostic", 403, {
        code,
      });
      expect(presentError(error, "Änderung")).toMatchObject({
        kind: "api",
        errorClass: "permission",
        message,
        retryable: false,
        requiresLogin: false,
      });
      for (const locale of ["en", "pl", "ru", "sq", "tr", "uk"] as const) {
        const translated = presentError(error, "Änderung", locale);
        expect(translated.retryable).toBe(false);
        expect(translated.message).not.toBe(message);
        expect(translated.message).not.toContain(
          "internal permission diagnostic",
        );
      }
    },
  );

  it.each(["de", "en", "pl", "ru", "sq", "tr", "uk"] as const)(
    "names the rejected upload in %s without exposing backend diagnostics",
    (locale) => {
      const error = new ApiError("unsupported magic bytes", 400, {
        code: "enrollment.upload_content_mismatch",
        details: { filename: "Bescheid.pdf" },
      });
      const result = presentError(error, "Nachweis", locale);
      expect(result.kind).toBe("api");
      expect(result.message).toContain("Bescheid.pdf");
      expect(result.message).not.toContain("unsupported magic bytes");
      expect(result.message).not.toContain("{filename}");
      expect(result.retryable).toBe(false);
    },
  );

  it("explains a disguised proof and the next step in plain German", () => {
    const error = new ApiError("unsupported magic bytes", 400, {
      code: "enrollment.upload_content_mismatch",
      details: { filename: "Bescheid.pdf" },
    });
    expect(presentError(error, "Nachweis").message).toBe(
      "Der Inhalt von „Bescheid.pdf“ passt nicht zur Dateiart. Bitte wählen Sie eine andere Datei. Umbenennen reicht nicht.",
    );
  });

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

  // #6871: a Träger-Büro asks its own Leitung, every other portal its school.
  describe("contact of a missing right", () => {
    const refusal = new ApiError("Forbidden", 403);
    const traegerText =
      "Für die Personen des Träger-Büros fehlt Ihnen die Berechtigung. Bitte fragen Sie die Leitung Ihres Träger-Büros.";

    it("sends a Träger-Büro to its Leitung, not to a school", () => {
      expect(
        presentError(refusal, "die Personen des Träger-Büros", "de", "traeger"),
      ).toMatchObject({ errorClass: "permission", message: traegerText });
    });

    it("keeps the school for every other portal", () => {
      expect(
        presentError(refusal, "die Personen des Träger-Büros"),
      ).toMatchObject({
        errorClass: "permission",
        message:
          "Für die Personen des Träger-Büros fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      });
    });

    it("leaves the own text of a code and other classes unchanged", () => {
      const ownText = new ApiError("forbidden", 403, {
        code: "traeger.no_membership",
      });
      expect(presentError(ownText, "Ihr Konto", "de", "traeger").message).toBe(
        presentError(ownText, "Ihr Konto").message,
      );
      const input = new ApiError("bad", 400);
      expect(presentError(input, "Ihr Name", "de", "traeger").message).toBe(
        presentError(input, "Ihr Name").message,
      );
    });

    it("names the Leitung in every language", () => {
      for (const locale of ["en", "pl", "ru", "sq", "tr", "uk"] as const) {
        expect(presentError(refusal, "X", locale, "traeger").message).not.toBe(
          presentError(refusal, "X", locale).message,
        );
      }
    });
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

describe("errorCodeMessage", () => {
  it("names a school's refusal in the words of the catalog (#4704)", () => {
    expect(
      errorCodeMessage("enrollment.traeger_copy_rejected", "die Anmeldephase"),
    ).toBe(
      "Die Schule kann diese Angaben so nicht übernehmen. Bitte prüfen Sie die Angaben oder die Einstellungen der Schule.",
    );
  });

  it("falls back to the class text for a code this build does not know", () => {
    expect(
      errorCodeMessage("enrollment.unknown_future_code", "die Anmeldephase"),
    ).toBe(presentError(new ApiError("x", 400), "die Anmeldephase").message);
  });
});
