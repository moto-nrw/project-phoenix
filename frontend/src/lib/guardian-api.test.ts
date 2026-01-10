import { describe, it, expect } from "vitest";
import { translateApiError, errorTranslations } from "./guardian-api";

describe("translateApiError", () => {
  it("translates 'invalid email format' to German", () => {
    expect(translateApiError("invalid email format")).toBe(
      "Ungültiges E-Mail-Format",
    );
  });

  it("translates error message case-insensitively", () => {
    expect(translateApiError("Invalid Email Format")).toBe(
      "Ungültiges E-Mail-Format",
    );
    expect(translateApiError("INVALID EMAIL FORMAT")).toBe(
      "Ungültiges E-Mail-Format",
    );
  });

  it("translates 'email already exists' to German", () => {
    expect(translateApiError("email already exists")).toBe(
      "Diese E-Mail-Adresse wird bereits verwendet",
    );
  });

  it("translates 'guardian not found' to German", () => {
    expect(translateApiError("guardian not found")).toBe(
      "Erziehungsberechtigte/r nicht gefunden",
    );
  });

  it("translates 'student not found' to German", () => {
    expect(translateApiError("student not found")).toBe(
      "Schüler/in nicht gefunden",
    );
  });

  it("translates 'relationship already exists' to German", () => {
    expect(translateApiError("relationship already exists")).toBe(
      "Diese Verknüpfung existiert bereits",
    );
  });

  it("translates 'validation failed' to German", () => {
    expect(translateApiError("validation failed")).toBe(
      "Validierung fehlgeschlagen",
    );
  });

  it("translates 'unauthorized' to German", () => {
    expect(translateApiError("unauthorized")).toBe("Keine Berechtigung");
  });

  it("translates 'forbidden' to German", () => {
    expect(translateApiError("forbidden")).toBe("Zugriff verweigert");
  });

  it("handles error patterns contained in longer messages", () => {
    expect(translateApiError("API error: invalid email format detected")).toBe(
      "Ungültiges E-Mail-Format",
    );
    expect(translateApiError("Error 400: email already exists in database")).toBe(
      "Diese E-Mail-Adresse wird bereits verwendet",
    );
  });

  it("returns generic German message for unknown errors", () => {
    expect(translateApiError("some unknown error")).toBe(
      "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
    );
    expect(translateApiError("connection timeout")).toBe(
      "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
    );
  });

  it("returns generic message for empty string", () => {
    expect(translateApiError("")).toBe(
      "Ein Fehler ist aufgetreten. Bitte versuchen Sie es erneut.",
    );
  });
});

describe("errorTranslations", () => {
  it("contains all expected error patterns", () => {
    const expectedPatterns = [
      "invalid email format",
      "email already exists",
      "guardian not found",
      "student not found",
      "relationship already exists",
      "validation failed",
      "unauthorized",
      "forbidden",
    ];

    for (const pattern of expectedPatterns) {
      expect(errorTranslations).toHaveProperty(pattern);
    }
  });

  it("all translations are non-empty strings", () => {
    for (const translation of Object.values(errorTranslations)) {
      expect(translation).toBeTruthy();
      expect(typeof translation).toBe("string");
      expect(translation.length).toBeGreaterThan(0);
    }
  });

  it("has exactly 8 error translations", () => {
    expect(Object.keys(errorTranslations).length).toBe(8);
  });
});
