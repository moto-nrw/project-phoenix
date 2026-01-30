import { describe, it, expect } from "vitest";
import { getDeleteErrorMessage } from "./activity-management-modal";

describe("getDeleteErrorMessage", () => {
  it("returns default message for non-Error objects", () => {
    expect(getDeleteErrorMessage(null)).toBe(
      "Fehler beim Löschen der Aktivität",
    );
    expect(getDeleteErrorMessage(undefined)).toBe(
      "Fehler beim Löschen der Aktivität",
    );
    expect(getDeleteErrorMessage("string error")).toBe(
      "Fehler beim Löschen der Aktivität",
    );
    expect(getDeleteErrorMessage(123)).toBe(
      "Fehler beim Löschen der Aktivität",
    );
  });

  it("returns students enrolled message when error mentions students", () => {
    const error = new Error("Cannot delete: students enrolled in activity");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Diese Aktivität kann nicht gelöscht werden, da noch Schüler eingeschrieben sind. Bitte entfernen Sie zuerst alle Schüler aus der Aktivität.",
    );
  });

  it("returns ownership error message for 403 with ownership context", () => {
    const error = new Error("403 you can only modify your own activities");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Sie können diese Aktivität nicht löschen, da Sie sie nicht erstellt haben und kein Betreuer sind.",
    );
  });

  it("returns ownership error message for 403 with supervise context", () => {
    const error = new Error("403 activities you created or supervise");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Sie können diese Aktivität nicht löschen, da Sie sie nicht erstellt haben und kein Betreuer sind.",
    );
  });

  it("returns session expired message for 401 error", () => {
    const error = new Error("Request failed with status 401");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.",
    );
  });

  it("returns generic permission denied message for other 403 errors", () => {
    const error = new Error("Forbidden 403");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe(
      "Sie haben keine Berechtigung, diese Aktivität zu löschen.",
    );
  });

  it("returns original error message for other errors", () => {
    const error = new Error("Network timeout");
    const result = getDeleteErrorMessage(error);
    expect(result).toBe("Network timeout");
  });
});
