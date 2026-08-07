import { describe, expect, it } from "vitest";

import { timetableSeriesErrorMessage } from "./scope-error-message";

// Issue #2187: saving a series scope against a capped predecessor segment of a
// split chain used to surface raw backend text. These pin the German mapping.

const FALLBACK = "Der Termin konnte nicht gespeichert werden.";

const TEMPLATE_GONE =
  "Der Regeltermin konnte nicht geladen werden. Bitte laden Sie die Seite neu und öffnen Sie den Termin erneut.";

function apiError(
  message: string,
  extra: { httpStatus?: number; code?: string },
): Error {
  return Object.assign(new Error(message), extra);
}

describe("timetableSeriesErrorMessage (#2187)", () => {
  it("maps the template_not_found code to a reload hint", () => {
    expect(
      timetableSeriesErrorMessage(
        apiError("template not found", { code: "template_not_found" }),
        FALLBACK,
      ),
    ).toBe(TEMPLATE_GONE);
  });

  it("maps a past effective date before the status ladder", () => {
    expect(
      timetableSeriesErrorMessage(
        apiError("effective_date must not be in the past", { httpStatus: 400 }),
        FALLBACK,
      ),
    ).toBe(
      "Der Stichtag liegt in der Vergangenheit. Bitte einen künftigen Termin der Serie wählen.",
    );
  });

  it.each([
    [401, "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."],
    [403, "Sie haben keine Berechtigung für diese Änderung."],
    [404, TEMPLATE_GONE],
    [
      409,
      "Der Regeltermin wurde zwischenzeitlich geändert. Bitte laden Sie die Seite neu.",
    ],
    [503, FALLBACK],
  ])("maps HTTP %i to its German message", (httpStatus, expected) => {
    expect(
      timetableSeriesErrorMessage(
        apiError("backend detail", { httpStatus }),
        FALLBACK,
      ),
    ).toBe(expected);
  });

  it("passes an informative backend message through", () => {
    expect(
      timetableSeriesErrorMessage(
        new Error("Endzeit muss nach der Startzeit liegen"),
        FALLBACK,
      ),
    ).toBe("Endzeit muss nach der Startzeit liegen");
  });

  it("falls back when the error carries no message", () => {
    expect(timetableSeriesErrorMessage("boom", FALLBACK)).toBe(FALLBACK);
    expect(timetableSeriesErrorMessage(new Error(""), FALLBACK)).toBe(FALLBACK);
  });
});
