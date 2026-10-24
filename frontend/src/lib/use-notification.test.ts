import { describe, it, expect } from "vitest";
import { getDbOperationMessage } from "./use-notification";

describe("getDbOperationMessage", () => {
  describe("create operation", () => {
    it("should return create message without identifier", () => {
      const message = getDbOperationMessage("create", "Gruppe");
      expect(message).toBe("Gruppe wurde erfolgreich erstellt");
    });

    it("should return create message with identifier", () => {
      const message = getDbOperationMessage("create", "Gruppe", "Klasse 5a");
      expect(message).toBe('Gruppe "Klasse 5a" wurde erfolgreich erstellt');
    });
  });

  describe("update operation", () => {
    it("should return update message without identifier", () => {
      const message = getDbOperationMessage("update", "Raum");
      expect(message).toBe("Raum wurde erfolgreich aktualisiert");
    });

    it("should return update message with identifier", () => {
      const message = getDbOperationMessage("update", "Raum", "Raum 101");
      expect(message).toBe('Raum "Raum 101" wurde erfolgreich aktualisiert');
    });
  });

  describe("delete operation", () => {
    it("should return delete message without identifier", () => {
      const message = getDbOperationMessage("delete", "Student");
      expect(message).toBe("Student wurde erfolgreich gelöscht");
    });

    it("should return delete message with identifier", () => {
      const message = getDbOperationMessage(
        "delete",
        "Student",
        "Max Mustermann",
      );
      expect(message).toBe(
        'Student "Max Mustermann" wurde erfolgreich gelöscht',
      );
    });
  });

  describe("edge cases", () => {
    it("should handle empty identifier as no identifier", () => {
      const message = getDbOperationMessage("create", "Gruppe", "");
      expect(message).toBe("Gruppe wurde erfolgreich erstellt");
    });

    it("should handle entity names with special characters", () => {
      const message = getDbOperationMessage("update", "Gruppe/Klasse", "5a");
      expect(message).toBe('Gruppe/Klasse "5a" wurde erfolgreich aktualisiert');
    });

    it("should handle identifiers with quotes", () => {
      const message = getDbOperationMessage("delete", "Raum", 'Raum "Spezial"');
      expect(message).toBe('Raum "Raum "Spezial"" wurde erfolgreich gelöscht');
    });
  });
});
