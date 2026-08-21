import { describe, expect, it } from "vitest";

import { getApiErrorMessage, readableApiMessage } from "./api-error-message";

describe("getApiErrorMessage", () => {
  const action = "erstellen";
  const entityType = "Aktivitäten";
  const defaultMessage = "Ein Fehler ist aufgetreten";

  it("returns default message for non-Error objects", () => {
    expect(getApiErrorMessage(null, action, entityType, defaultMessage)).toBe(
      defaultMessage,
    );
    expect(
      getApiErrorMessage(undefined, action, entityType, defaultMessage),
    ).toBe(defaultMessage);
    expect(
      getApiErrorMessage("string error", action, entityType, defaultMessage),
    ).toBe(defaultMessage);
    expect(getApiErrorMessage(123, action, entityType, defaultMessage)).toBe(
      defaultMessage,
    );
  });

  it("returns authentication message for unauthenticated errors", () => {
    const result = getApiErrorMessage(
      new Error("user is not authenticated"),
      action,
      entityType,
      defaultMessage,
    );

    expect(result).toBe(
      "Sie müssen angemeldet sein, um Aktivitäten zu erstellen.",
    );
  });

  it("returns session expired message for 401 errors", () => {
    const result = getApiErrorMessage(
      new Error("Request failed with status 401"),
      action,
      entityType,
      defaultMessage,
    );

    expect(result).toBe(
      "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
    );
  });

  it("returns permission denied message for generic 403 errors", () => {
    const result = getApiErrorMessage(
      new Error("Forbidden 403"),
      action,
      entityType,
      defaultMessage,
    );

    expect(result).toBe(
      "Sie haben keine Berechtigung, diese Aktivitäten zu erstellen.",
    );
  });

  it("returns ownership error message for 403 ownership context", () => {
    const result = getApiErrorMessage(
      new Error("403 you can only modify your own activities"),
      "bearbeiten",
      "Aktivitäten",
      defaultMessage,
    );

    expect(result).toBe(
      "Sie können diese Aktivitäten nicht bearbeiten, da Sie sie nicht erstellt haben und kein Betreuer sind.",
    );
  });

  it("returns invalid input message for 400 errors", () => {
    const result = getApiErrorMessage(
      new Error("Bad request 400"),
      action,
      entityType,
      defaultMessage,
    );

    expect(result).toBe(
      "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.",
    );
  });

  it("returns original error message for other errors", () => {
    const result = getApiErrorMessage(
      new Error("Network timeout"),
      action,
      entityType,
      defaultMessage,
    );

    expect(result).toBe("Network timeout");
  });

  it("uses dynamic action and entity type in authentication messages", () => {
    const result = getApiErrorMessage(
      new Error("user is not authenticated"),
      "bearbeiten",
      "Gruppen",
      defaultMessage,
    );

    expect(result).toBe(
      "Sie müssen angemeldet sein, um Gruppen zu bearbeiten.",
    );
  });
});

describe("readableApiMessage", () => {
  it("drops the status prefix the API client adds", () => {
    expect(
      readableApiMessage(
        new Error(
          "HTTP 409: eine verwendete Abwesenheitsart kann nicht umbenannt werden",
        ),
      ),
    ).toBe("Eine verwendete Abwesenheitsart kann nicht umbenannt werden.");
  });

  it("keeps a sentence that already ends properly", () => {
    expect(readableApiMessage(new Error("HTTP 400: Name fehlt!"))).toBe(
      "Name fehlt!",
    );
  });

  it("returns null when nothing readable is left", () => {
    expect(readableApiMessage(new Error("HTTP 500: "))).toBeNull();
    expect(readableApiMessage("not an error")).toBeNull();
  });
});
