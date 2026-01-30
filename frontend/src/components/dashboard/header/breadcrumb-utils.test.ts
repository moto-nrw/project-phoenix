import { describe, expect, it } from "vitest";
import {
  getPageTitle,
  getSubPageLabel,
  getBreadcrumbLabel,
  getHistoryType,
  getPageTypeInfo,
} from "./breadcrumb-utils";
import type { PageTypeInfo } from "./breadcrumb-utils";

describe("breadcrumb-utils", () => {
  describe("getPageTitle", () => {
    describe("student pages", () => {
      it("should return 'Kindersuche' for /students/search", () => {
        expect(getPageTitle("/students/search")).toBe("Kindersuche");
      });

      it("should return 'Schüler Details' for student detail page", () => {
        expect(getPageTitle("/students/123")).toBe("Schüler Details");
      });

      it("should return 'Feedback Historie' for feedback history page", () => {
        expect(getPageTitle("/students/123/feedback_history")).toBe(
          "Feedback Historie",
        );
      });

      it("should return 'Mensa Historie' for mensa history page", () => {
        expect(getPageTitle("/students/123/mensa_history")).toBe(
          "Mensa Historie",
        );
      });

      it("should return 'Raum Historie' for room history page", () => {
        expect(getPageTitle("/students/123/room_history")).toBe(
          "Raum Historie",
        );
      });

      it("should return 'Schüler' for /students route", () => {
        expect(getPageTitle("/students")).toBe("Schüler");
      });
    });

    describe("room pages", () => {
      it("should return 'Raum Details' for room detail page", () => {
        expect(getPageTitle("/rooms/456")).toBe("Raum Details");
      });

      it("should return 'Räume' for /rooms route", () => {
        expect(getPageTitle("/rooms")).toBe("Räume");
      });
    });

    describe("database pages", () => {
      it("should return 'Aktivitäten' for database activities page", () => {
        expect(getPageTitle("/database/activities")).toBe("Aktivitäten");
      });

      it("should return 'Gruppen' for database groups page", () => {
        expect(getPageTitle("/database/groups")).toBe("Gruppen");
      });

      it("should return 'Kinder' for database students page", () => {
        expect(getPageTitle("/database/students")).toBe("Kinder");
      });

      it("should return 'Betreuer' for database teachers page", () => {
        expect(getPageTitle("/database/teachers")).toBe("Betreuer");
      });

      it("should return 'Räume' for database rooms page", () => {
        expect(getPageTitle("/database/rooms")).toBe("Räume");
      });

      it("should return 'Rollen' for database roles page", () => {
        expect(getPageTitle("/database/roles")).toBe("Rollen");
      });

      it("should return 'Geräte' for database devices page", () => {
        expect(getPageTitle("/database/devices")).toBe("Geräte");
      });

      it("should return 'Berechtigungen' for database permissions page", () => {
        expect(getPageTitle("/database/permissions")).toBe("Berechtigungen");
      });

      it("should return 'Datenbank' for unknown database page", () => {
        expect(getPageTitle("/database/unknown")).toBe("Datenbank");
      });

      it("should return 'Datenverwaltung' for /database route", () => {
        expect(getPageTitle("/database")).toBe("Datenverwaltung");
      });
    });

    describe("main routes", () => {
      it("should return 'Home' for /dashboard", () => {
        expect(getPageTitle("/dashboard")).toBe("Home");
      });

      it("should return 'Home' for root path", () => {
        expect(getPageTitle("/")).toBe("Home");
      });

      it("should return 'Meine Gruppe' for /ogs-groups", () => {
        expect(getPageTitle("/ogs-groups")).toBe("Meine Gruppe");
      });

      it("should return 'Aktuelle Aufsicht' for /active-supervisions", () => {
        expect(getPageTitle("/active-supervisions")).toBe("Aktuelle Aufsicht");
      });

      it("should return 'Mitarbeiter' for /staff", () => {
        expect(getPageTitle("/staff")).toBe("Mitarbeiter");
      });

      it("should return 'Aktivitäten' for /activities", () => {
        expect(getPageTitle("/activities")).toBe("Aktivitäten");
      });

      it("should return 'Statistiken' for /statistics", () => {
        expect(getPageTitle("/statistics")).toBe("Statistiken");
      });

      it("should return 'Vertretungen' for /substitutions", () => {
        expect(getPageTitle("/substitutions")).toBe("Vertretungen");
      });

      it("should return 'Einstellungen' for /settings", () => {
        expect(getPageTitle("/settings")).toBe("Einstellungen");
      });

      it("should return 'Einladungen' for /invitations", () => {
        expect(getPageTitle("/invitations")).toBe("Einladungen");
      });

      it("should return 'Borndal Feedback' for /borndal_feedback", () => {
        expect(getPageTitle("/borndal_feedback")).toBe("Borndal Feedback");
      });

      it("should return 'Home' for unknown route", () => {
        expect(getPageTitle("/unknown-route")).toBe("Home");
      });
    });
  });

  describe("getSubPageLabel", () => {
    it("should return 'CSV-Import' for csv-import segment", () => {
      expect(getSubPageLabel("/database/students/csv-import")).toBe(
        "CSV-Import",
      );
    });

    it("should return 'Erstellen' for create segment", () => {
      expect(getSubPageLabel("/database/groups/create")).toBe("Erstellen");
    });

    it("should return 'Bearbeiten' for edit segment", () => {
      expect(getSubPageLabel("/database/students/123/edit")).toBe("Bearbeiten");
    });

    it("should return 'Details' for details segment", () => {
      expect(getSubPageLabel("/students/123/details")).toBe("Details");
    });

    it("should return 'Berechtigungen' for permissions segment", () => {
      expect(getSubPageLabel("/database/roles/1/permissions")).toBe(
        "Berechtigungen",
      );
    });

    it("should capitalize first letter for unknown segment", () => {
      expect(getSubPageLabel("/database/groups/settings")).toBe("Settings");
    });

    it("should return 'Unbekannt' for empty pathname", () => {
      expect(getSubPageLabel("")).toBe("Unbekannt");
    });

    it("should return 'Unbekannt' for pathname with only slashes", () => {
      expect(getSubPageLabel("///")).toBe("Unbekannt");
    });

    it("should handle single segment path", () => {
      expect(getSubPageLabel("/create")).toBe("Erstellen");
    });
  });

  describe("getBreadcrumbLabel", () => {
    it("should return 'Meine Gruppe' for /ogs-groups referrer", () => {
      expect(getBreadcrumbLabel("/ogs-groups")).toBe("Meine Gruppe");
    });

    it("should return 'Meine Gruppe' for /ogs-groups sub-path referrer", () => {
      expect(getBreadcrumbLabel("/ogs-groups/123")).toBe("Meine Gruppe");
    });

    it("should return 'Aktuelle Aufsicht' for /active-supervisions referrer", () => {
      expect(getBreadcrumbLabel("/active-supervisions")).toBe(
        "Aktuelle Aufsicht",
      );
    });

    it("should return 'Aktuelle Aufsicht' for /active-supervisions sub-path referrer", () => {
      expect(getBreadcrumbLabel("/active-supervisions/456")).toBe(
        "Aktuelle Aufsicht",
      );
    });

    it("should return 'Kindersuche' for unknown referrer", () => {
      expect(getBreadcrumbLabel("/students")).toBe("Kindersuche");
    });

    it("should return 'Kindersuche' for empty referrer", () => {
      expect(getBreadcrumbLabel("")).toBe("Kindersuche");
    });

    it("should return 'Kindersuche' for dashboard referrer", () => {
      expect(getBreadcrumbLabel("/dashboard")).toBe("Kindersuche");
    });
  });

  describe("getHistoryType", () => {
    it("should return 'Feedback Historie' for feedback history path", () => {
      expect(getHistoryType("/students/123/feedback_history")).toBe(
        "Feedback Historie",
      );
    });

    it("should return 'Mensa Historie' for mensa history path", () => {
      expect(getHistoryType("/students/123/mensa_history")).toBe(
        "Mensa Historie",
      );
    });

    it("should return 'Raum Historie' for room history path", () => {
      expect(getHistoryType("/students/123/room_history")).toBe(
        "Raum Historie",
      );
    });

    it("should return empty string for non-history path", () => {
      expect(getHistoryType("/students/123")).toBe("");
    });

    it("should return empty string for empty path", () => {
      expect(getHistoryType("")).toBe("");
    });

    it("should return empty string for unrelated path", () => {
      expect(getHistoryType("/dashboard")).toBe("");
    });
  });

  describe("getPageTypeInfo", () => {
    describe("student detail page", () => {
      it("should identify student detail page", () => {
        const result = getPageTypeInfo("/students/123");
        expect(result.isStudentDetailPage).toBe(true);
        expect(result.isStudentHistoryPage).toBe(false);
      });

      it("should not identify /students as detail page", () => {
        const result = getPageTypeInfo("/students");
        expect(result.isStudentDetailPage).toBe(false);
      });

      it("should not identify /students/search as detail page", () => {
        const result = getPageTypeInfo("/students/search");
        expect(result.isStudentDetailPage).toBe(false);
      });

      it("should not identify history pages as detail page", () => {
        const result = getPageTypeInfo("/students/123/feedback_history");
        expect(result.isStudentDetailPage).toBe(false);
        expect(result.isStudentHistoryPage).toBe(true);
      });
    });

    describe("student history page", () => {
      it("should identify feedback history page", () => {
        const result = getPageTypeInfo("/students/123/feedback_history");
        expect(result.isStudentHistoryPage).toBe(true);
        expect(result.isStudentDetailPage).toBe(false);
      });

      it("should identify mensa history page", () => {
        const result = getPageTypeInfo("/students/123/mensa_history");
        expect(result.isStudentHistoryPage).toBe(true);
      });

      it("should identify room history page", () => {
        const result = getPageTypeInfo("/students/123/room_history");
        expect(result.isStudentHistoryPage).toBe(true);
      });

      it("should not identify non-history student page as history", () => {
        const result = getPageTypeInfo("/students/123");
        expect(result.isStudentHistoryPage).toBe(false);
      });
    });

    describe("room detail page", () => {
      it("should identify room detail page", () => {
        const result = getPageTypeInfo("/rooms/456");
        expect(result.isRoomDetailPage).toBe(true);
      });

      it("should not identify /rooms as detail page", () => {
        const result = getPageTypeInfo("/rooms");
        expect(result.isRoomDetailPage).toBe(false);
      });
    });

    describe("activity detail page", () => {
      it("should identify activity detail page", () => {
        const result = getPageTypeInfo("/activities/789");
        expect(result.isActivityDetailPage).toBe(true);
      });

      it("should not identify /activities as detail page", () => {
        const result = getPageTypeInfo("/activities");
        expect(result.isActivityDetailPage).toBe(false);
      });
    });

    describe("database pages", () => {
      it("should identify database sub-page", () => {
        const result = getPageTypeInfo("/database/students");
        expect(result.isDatabaseSubPage).toBe(true);
        expect(result.isDatabaseDeepPage).toBe(false);
      });

      it("should not identify /database as sub-page", () => {
        const result = getPageTypeInfo("/database");
        expect(result.isDatabaseSubPage).toBe(false);
      });

      it("should identify database deep page (4+ segments)", () => {
        const result = getPageTypeInfo("/database/students/123/edit");
        expect(result.isDatabaseDeepPage).toBe(true);
        expect(result.isDatabaseSubPage).toBe(true);
      });

      it("should not identify shallow path as deep page", () => {
        const result = getPageTypeInfo("/database/students");
        expect(result.isDatabaseDeepPage).toBe(false);
      });
    });

    describe("combined page types", () => {
      it("should return all false for root path", () => {
        const result = getPageTypeInfo("/");
        const expected: PageTypeInfo = {
          isStudentDetailPage: false,
          isStudentHistoryPage: false,
          isRoomDetailPage: false,
          isActivityDetailPage: false,
          isDatabaseSubPage: false,
          isDatabaseDeepPage: false,
        };
        expect(result).toEqual(expected);
      });

      it("should return all false for dashboard", () => {
        const result = getPageTypeInfo("/dashboard");
        expect(result.isStudentDetailPage).toBe(false);
        expect(result.isStudentHistoryPage).toBe(false);
        expect(result.isRoomDetailPage).toBe(false);
        expect(result.isActivityDetailPage).toBe(false);
        expect(result.isDatabaseSubPage).toBe(false);
        expect(result.isDatabaseDeepPage).toBe(false);
      });

      it("should correctly identify multiple database segments", () => {
        const result = getPageTypeInfo("/database/students/create");
        expect(result.isDatabaseSubPage).toBe(true);
        // Path has 4 segments: ["", "database", "students", "create"]
        expect(result.isDatabaseDeepPage).toBe(true);
      });
    });

    describe("edge cases", () => {
      it("should handle empty pathname", () => {
        const result = getPageTypeInfo("");
        expect(result.isStudentDetailPage).toBe(false);
        expect(result.isRoomDetailPage).toBe(false);
      });

      it("should handle pathname with trailing slash", () => {
        const result = getPageTypeInfo("/students/123/");
        expect(result.isStudentDetailPage).toBe(true);
      });

      it("should handle multiple slashes", () => {
        const result = getPageTypeInfo("//students//123//");
        // Multiple slashes break startsWith check, so not identified as student detail
        expect(result.isStudentDetailPage).toBe(false);
      });
    });
  });
});
