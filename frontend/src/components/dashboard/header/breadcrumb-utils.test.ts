import { describe, expect, it } from "vitest";
import {
  getPageTitle,
  getSectionBreadcrumb,
  getSubPageLabel,
  getBreadcrumbLabel,
  getHistoryType,
  getPageTypeInfo,
} from "./breadcrumb-utils";
import type { PageTypeInfo } from "./breadcrumb-utils";

describe("breadcrumb-utils", () => {
  describe("getPageTitle", () => {
    describe("student pages", () => {
      it("should return 'Kinder' for /students/search", () => {
        expect(getPageTitle("/students/search")).toBe("Kinder");
      });

      it("should return 'Kinder Details' for student detail page", () => {
        expect(getPageTitle("/students/123")).toBe("Kinder Details");
      });

      it("should return 'Feedback Historie' for feedback history page", () => {
        expect(getPageTitle("/students/123/feedback-history")).toBe(
          "Feedback Historie",
        );
      });

      it("should return 'Anwesenheitsprotokoll' for room history page", () => {
        expect(getPageTitle("/students/123/room-history")).toBe(
          "Anwesenheitsprotokoll",
        );
      });
    });

    describe("room pages", () => {
      // Eine Detailregel für /rooms/{id} gibt es nicht mehr: die Route ist
      // eine Server-Weiterleitung auf /rooms?room={id} und rendert nie einen
      // Header.
      it("should return 'Räume' for /rooms route", () => {
        expect(getPageTitle("/rooms")).toBe("Räume");
      });
    });

    describe("register pages", () => {
      // Die Register sind Reiter an ihrer Fläche: der Titel ist der
      // Reitername, die Fläche steht als erste Stufe in der Brotkrume.
      it("titles the four record registers 'Stammdaten'", () => {
        expect(getPageTitle("/database/students")).toBe("Stammdaten");
        expect(getPageTitle("/database/personal")).toBe("Stammdaten");
        expect(getPageTitle("/database/rooms")).toBe("Stammdaten");
        expect(getPageTitle("/database/activities")).toBe("Stammdaten");
      });

      it("titles the configuration registers by their tab name", () => {
        expect(getPageTitle("/database/groups")).toBe("Gruppen");
        expect(getPageTitle("/database/roles")).toBe("Rollen");
        expect(getPageTitle("/database/devices")).toBe("Geräte");
        expect(getPageTitle("/database/permissions")).toBe("Berechtigungen");
        expect(getPageTitle("/database/exports")).toBe("Exporte");
        expect(getPageTitle("/database/grade-transitions")).toBe(
          "Jahrgangswechsel",
        );
      });

      it("should return 'Stammdaten' for unknown database page", () => {
        // Unveränderte Regel: ein /database/*-Pfad ohne Katalogeintrag fällt
        // auf den Sektionsnamen zurück, nie auf den Home-Fallback.
        expect(getPageTitle("/database/unknown")).toBe("Stammdaten");
      });

      it("should return 'Stammdaten' for /database route", () => {
        expect(getPageTitle("/database")).toBe("Stammdaten");
      });
    });

    describe("main routes", () => {
      it("should return 'Start' for /dashboard", () => {
        expect(getPageTitle("/dashboard")).toBe("Start");
      });

      it("should return 'Home' for root path", () => {
        expect(getPageTitle("/")).toBe("Home");
      });

      it("should return 'Meine Gruppen' for /ogs-groups", () => {
        expect(getPageTitle("/ogs-groups")).toBe("Meine Gruppen");
      });

      it("should return 'Aufsicht heute' for /active-supervisions", () => {
        expect(getPageTitle("/active-supervisions")).toBe("Aufsicht heute");
      });

      it("should return 'Mitarbeitende' for /staff", () => {
        expect(getPageTitle("/staff")).toBe("Mitarbeitende");
      });

      it("should return 'Dienstplan' for /staff/dienstplan", () => {
        expect(getPageTitle("/staff/dienstplan")).toBe("Dienstplan");
      });

      it("should return 'Aktivitäten' for /activities", () => {
        expect(getPageTitle("/activities")).toBe("Aktivitäten");
      });

      it("should return 'Vertretungszugriff' for /substitutions", () => {
        expect(getPageTitle("/substitutions")).toBe("Vertretungszugriff");
      });

      it("titles the three planning areas and their redirect frames", () => {
        // Planung-Redesign (docs/planung-redesign/docs/03 Abschnitt 5): die
        // Bereiche sind flache Hauptrouten; die Redirect-Frames behalten
        // Titel, damit während des Client-Redirects nichts Falsches aufblitzt.
        expect(getPageTitle("/betreuungsplan")).toBe("Betreuungsplan");
        expect(getPageTitle("/dienstplan")).toBe("Dienstplan");
        expect(getPageTitle("/vertretung")).toBe("Vertretung");
        expect(getPageTitle("/planung")).toBe("Planung");
        expect(getPageTitle("/timetables")).toBe("Betreuungsplan");
        expect(getPageTitle("/vertretungsplan")).toBe("Vertretung");
        expect(getPageTitle("/staff/dienstplan")).toBe("Dienstplan");
      });

      it("should return 'Zeiträume' for /calendar-periods", () => {
        expect(getPageTitle("/calendar-periods")).toBe("Zeiträume");
      });

      it("should return 'Tageslisten' for /lists", () => {
        expect(getPageTitle("/lists")).toBe("Tageslisten");
      });

      it("should return 'Abrechnung' for /payroll", () => {
        // Abrechnung hängt jetzt am Planung-Katalog statt an einem eigenen
        // flachen Eintrag.
        expect(getPageTitle("/payroll")).toBe("Abrechnung");
      });

      it("should title the flat routes that previously fell through to 'Home'", () => {
        expect(getPageTitle("/calendar")).toBe("Kalender");
        expect(getPageTitle("/day-log")).toBe("Tagesbericht");
        expect(getPageTitle("/info-displays")).toBe("Info-Displays");
      });

      it("should return 'Einstellungen' for /settings", () => {
        expect(getPageTitle("/settings")).toBe("Einstellungen");
      });

      it("should return 'Notfall' for /emergency", () => {
        expect(getPageTitle("/emergency")).toBe("Notfall");
      });

      it("should return enrollment sub-page titles", () => {
        expect(getPageTitle("/admin/enrollments")).toBe("Anmeldungen");
        expect(getPageTitle("/admin/enrollments/phases/phase-1")).toBe(
          "Anmeldephase",
        );
        expect(getPageTitle("/admin/enrollments/request-1")).toBe("Anmeldung");
        expect(getPageTitle("/enrollment-phases")).toBe("Anmeldephasen");
        expect(getPageTitle("/care-offerings")).toBe("Angebote");
        expect(getPageTitle("/enrollment-form")).toBe("Anmeldeformulare");
      });

      it("should return titles for staff admin request pages", () => {
        expect(getPageTitle("/admin/guardian-approvals")).toBe(
          "Neue Elternkonten",
        );
        // Alt-Route der Freigabeansicht: nur noch ein Redirect-Frame auf
        // /anfragen (#2429); der Titel verhindert den "Home"-Blitzer.
        expect(getPageTitle("/admin/change-requests")).toBe("Anfragen");
        expect(getPageTitle("/anfragen")).toBe("Anfragen");
      });

      it("should return titles for recent staff navigation entries", () => {
        expect(getPageTitle("/messages")).toBe("Nachrichten");
        expect(getPageTitle("/messages/thread-1")).toBe("Nachrichten");
        expect(getPageTitle("/parent-announcements")).toBe(
          "Mitteilungen und Umfragen",
        );
        expect(getPageTitle("/meal-plan")).toBe("Essensplan");
      });

      it("should return titles for operator navigation entries", () => {
        expect(getPageTitle("/operator/organizations")).toBe("Träger");
        expect(getPageTitle("/operator/schools")).toBe("Schulen");
        expect(getPageTitle("/operator/accounts")).toBe("Konten");
        expect(getPageTitle("/operator/devices")).toBe("Geräte");
        expect(getPageTitle("/operator/persons")).toBe("Personen");
        expect(getPageTitle("/operator/unregistered-tags")).toBe(
          "Unbekannte RFID",
        );
        expect(getPageTitle("/operator/operators")).toBe("Operatoren");
        expect(getPageTitle("/operator/announcements")).toBe("Ankündigungen");
      });

      it("should return German fallback titles for parent navigation entries", () => {
        expect(getPageTitle("/parents/messages")).toBe("Nachrichten");
        expect(getPageTitle("/parents/news")).toBe("Neuigkeiten");
        expect(getPageTitle("/parents/meal-plan")).toBe("Essensplan");
      });

      it("should return 'Home' for unknown route", () => {
        expect(getPageTitle("/unknown-route")).toBe("Home");
      });
    });
  });

  describe("getSubPageLabel", () => {
    it("should return 'Importieren' for import segment", () => {
      expect(getSubPageLabel("/database/students/import")).toBe("Importieren");
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
    it("should return 'Meine Gruppen' for /ogs-groups referrer", () => {
      expect(getBreadcrumbLabel("/ogs-groups")).toBe("Meine Gruppen");
    });

    it("should return 'Meine Gruppen' for /ogs-groups sub-path referrer", () => {
      expect(getBreadcrumbLabel("/ogs-groups/123")).toBe("Meine Gruppen");
    });

    it("should return 'Aufsicht heute' for /active-supervisions referrer", () => {
      expect(getBreadcrumbLabel("/active-supervisions")).toBe("Aufsicht heute");
    });

    it("should return 'Aufsicht heute' for /active-supervisions sub-path referrer", () => {
      expect(getBreadcrumbLabel("/active-supervisions/456")).toBe(
        "Aufsicht heute",
      );
    });

    it("should return 'Räume' for /rooms/{id} referrer (drill-in from room detail)", () => {
      expect(getBreadcrumbLabel("/rooms/42")).toBe("Räume");
    });

    it("should return 'Räume' for /rooms?room={id} referrer (drill-in from modal)", () => {
      // Modal flow at /rooms?room={id} (#1374), same room context as the
      // legacy subpage drill-in, must produce the same breadcrumb label.
      expect(getBreadcrumbLabel("/rooms?room=42")).toBe("Räume");
    });

    it("should return 'Kinder' for unknown referrer", () => {
      expect(getBreadcrumbLabel("/students")).toBe("Kinder");
    });

    it("should return 'Kinder' for empty referrer", () => {
      expect(getBreadcrumbLabel("")).toBe("Kinder");
    });

    it("should return 'Kinder' for dashboard referrer", () => {
      expect(getBreadcrumbLabel("/dashboard")).toBe("Kinder");
    });
  });

  describe("getHistoryType", () => {
    it("should return 'Feedback Historie' for feedback history path", () => {
      expect(getHistoryType("/students/123/feedback-history")).toBe(
        "Feedback Historie",
      );
    });

    it("should return 'Anwesenheitsprotokoll' for room history path", () => {
      expect(getHistoryType("/students/123/room-history")).toBe(
        "Anwesenheitsprotokoll",
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

  describe("getSectionBreadcrumb", () => {
    describe("register pages", () => {
      // Die Register hängen als Reiter an ihrer Fläche: erste Stufe ist die
      // Sammlung, zweite der Reitername.
      it("should identify a register page", () => {
        const result = getSectionBreadcrumb("/database/students");
        expect(result).not.toBeNull();
        expect(result?.sectionLabel).toBe("Kinder");
        expect(result?.sectionHref).toBe("/students/search");
        expect(result?.pageLabel).toBe("Stammdaten");
        // Zweistufig: keine dritte Ebene, deshalb auch kein Link auf der
        // Mittelstufe.
        expect(result?.pageHref).toBeUndefined();
        expect(result?.deepLabel).toBeUndefined();
      });

      it("should not identify /database as sub-page", () => {
        expect(getSectionBreadcrumb("/database")).toBeNull();
      });

      it("should identify a deep register page (4+ segments)", () => {
        const result = getSectionBreadcrumb("/database/students/123/edit");
        expect(result?.sectionLabel).toBe("Kinder");
        expect(result?.pageLabel).toBe("Stammdaten");
        expect(result?.pageHref).toBe("/database/students");
        expect(result?.deepLabel).toBe("Bearbeiten");
      });

      it("should correctly identify multiple register segments", () => {
        const result = getSectionBreadcrumb("/database/students/create");
        expect(result?.pageLabel).toBe("Stammdaten");
        expect(result?.pageHref).toBe("/database/students");
        expect(result?.deepLabel).toBe("Erstellen");
      });

      it("should build the three-level breadcrumb for /database/students/import", () => {
        expect(getSectionBreadcrumb("/database/students/import")).toEqual({
          sectionLabel: "Kinder",
          sectionHref: "/students/search",
          pageLabel: "Stammdaten",
          pageHref: "/database/students",
          deepLabel: "Importieren",
        });
      });
    });

    describe("planning pages", () => {
      it("should resolve a planning page without a section link", () => {
        const result = getSectionBreadcrumb("/betreuungsplan");
        expect(result?.sectionLabel).toBe("Planung");
        expect(result?.pageLabel).toBe("Betreuungsplan");
        // Planung hat keine Hub-Seite: ohne href rendert die Sektion als
        // Text statt als Link ins Leere.
        expect(result?.sectionHref).toBeUndefined();
        expect(result?.pageHref).toBeUndefined();
        expect(result?.deepLabel).toBeUndefined();
      });

      it("should resolve every planning entry", () => {
        expect(getSectionBreadcrumb("/dienstplan")?.pageLabel).toBe(
          "Dienstplan",
        );
        expect(getSectionBreadcrumb("/vertretung")?.pageLabel).toBe(
          "Vertretung",
        );
        expect(getSectionBreadcrumb("/lists")?.pageLabel).toBe("Tageslisten");
        expect(getSectionBreadcrumb("/calendar-periods")?.pageLabel).toBe(
          "Zeiträume",
        );
      });

      it("should resolve /payroll to Auswertung › Abrechnung", () => {
        const result = getSectionBreadcrumb("/payroll");
        expect(result?.sectionLabel).toBe("Auswertung");
        expect(result?.pageLabel).toBe("Abrechnung");
        expect(result?.sectionHref).toBeUndefined();
      });

      it("should map legacy planning paths to their current entry", () => {
        expect(getSectionBreadcrumb("/timetables")?.pageLabel).toBe(
          "Betreuungsplan",
        );
        expect(getSectionBreadcrumb("/vertretungsplan")?.pageLabel).toBe(
          "Vertretung",
        );
        expect(getSectionBreadcrumb("/staff/dienstplan")?.pageLabel).toBe(
          "Dienstplan",
        );
        expect(getSectionBreadcrumb("/staff/dienstplan")?.sectionLabel).toBe(
          "Planung",
        );
      });

      it("should keep sub-paths of a planning page on that page", () => {
        expect(getSectionBreadcrumb("/betreuungsplan/2026-07-30")).toEqual({
          sectionLabel: "Planung",
          pageLabel: "Betreuungsplan",
        });
      });
    });

    describe("parent pages", () => {
      it("should resolve /messages to Eltern › Nachrichten", () => {
        expect(getSectionBreadcrumb("/messages")).toEqual({
          sectionLabel: "Eltern",
          sectionHref: "/eltern",
          pageLabel: "Nachrichten",
        });
      });

      it("should keep a message thread on the Nachrichten entry", () => {
        const result = getSectionBreadcrumb("/messages/42");
        expect(result?.sectionLabel).toBe("Eltern");
        expect(result?.pageLabel).toBe("Nachrichten");
      });

      it("should resolve the remaining parent entries", () => {
        expect(getSectionBreadcrumb("/admin/guardian-approvals")).toEqual({
          sectionLabel: "Eltern",
          sectionHref: "/eltern",
          pageLabel: "Neue Elternkonten",
          pageHref: undefined,
          deepLabel: undefined,
        });
        // /admin/change-requests ist kein Eltern-Katalogeintrag mehr — nur
        // noch ein Redirect auf das Top-Level-Modul /anfragen (#2429).
        expect(getSectionBreadcrumb("/admin/change-requests")).toBeNull();
        expect(getSectionBreadcrumb("/parent-announcements")?.pageLabel).toBe(
          "Mitteilungen und Umfragen",
        );
        expect(getSectionBreadcrumb("/meal-plan")?.pageLabel).toBe(
          "Essensplan",
        );
      });

      it("should return null for the /eltern hub itself", () => {
        // Der Hub zeigt nur seinen eigenen Sektionsnamen, keine Breadcrumb
        // auf sich selbst.
        expect(getSectionBreadcrumb("/eltern")).toBeNull();
      });
    });

    describe("pages outside a section", () => {
      it("should return null for flat routes", () => {
        expect(getSectionBreadcrumb("/dashboard")).toBeNull();
        expect(getSectionBreadcrumb("/planung")).toBeNull();
        expect(getSectionBreadcrumb("/students/123")).toBeNull();
        expect(getSectionBreadcrumb("/unknown-route")).toBeNull();
        expect(getSectionBreadcrumb("")).toBeNull();
      });

      it("should not pull the parents portal into the Eltern section", () => {
        expect(getSectionBreadcrumb("/parents/messages")).toBeNull();
        expect(getSectionBreadcrumb("/parents/meal-plan")).toBeNull();
      });
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
        const result = getPageTypeInfo("/students/123/feedback-history");
        expect(result.isStudentDetailPage).toBe(false);
        expect(result.isStudentHistoryPage).toBe(true);
      });
    });

    describe("student history page", () => {
      it("should identify feedback history page", () => {
        const result = getPageTypeInfo("/students/123/feedback-history");
        expect(result.isStudentHistoryPage).toBe(true);
        expect(result.isStudentDetailPage).toBe(false);
      });

      it("should identify room history page", () => {
        const result = getPageTypeInfo("/students/123/room-history");
        expect(result.isStudentHistoryPage).toBe(true);
      });

      it("should not identify non-history student page as history", () => {
        const result = getPageTypeInfo("/students/123");
        expect(result.isStudentHistoryPage).toBe(false);
      });
    });

    describe("combined page types", () => {
      it("should return all false for root path", () => {
        const result = getPageTypeInfo("/");
        const expected: PageTypeInfo = {
          isStudentDetailPage: false,
          isStudentHistoryPage: false,
          isStaffDetailPage: false,
          isEnrollmentPage: false,
        };
        expect(result).toEqual(expected);
      });

      it("should return all false for dashboard", () => {
        const result = getPageTypeInfo("/dashboard");
        expect(result.isStudentDetailPage).toBe(false);
        expect(result.isStudentHistoryPage).toBe(false);
        expect(result.isStaffDetailPage).toBe(false);
        expect(result.isEnrollmentPage).toBe(false);
      });

      it("should identify enrollment pages", () => {
        expect(getPageTypeInfo("/admin/enrollments").isEnrollmentPage).toBe(
          true,
        );
        expect(getPageTypeInfo("/enrollment-phases").isEnrollmentPage).toBe(
          true,
        );
        expect(getPageTypeInfo("/care-offerings").isEnrollmentPage).toBe(true);
        expect(getPageTypeInfo("/enrollment-form").isEnrollmentPage).toBe(true);
      });
    });

    describe("staff pages", () => {
      it("does not treat Dienstplan as a staff detail page", () => {
        const info = getPageTypeInfo("/staff/dienstplan");
        expect(info.isStaffDetailPage).toBe(false);
      });
    });

    describe("edge cases", () => {
      it("should handle empty pathname", () => {
        const result = getPageTypeInfo("");
        expect(result.isStudentDetailPage).toBe(false);
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
