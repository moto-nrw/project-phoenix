import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import {
  getHelpContent,
  SPECIFIC_PAGE_HELP,
  NAVIGATION_HELP,
} from "./help-content";

describe("help-content", () => {
  describe("SPECIFIC_PAGE_HELP", () => {
    it("should have student-detail content", () => {
      const content = SPECIFIC_PAGE_HELP["student-detail"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Schülerdetails")).toBeTruthy();
      expect(container.textContent?.includes("Persönliche Daten")).toBeTruthy();
    });

    it("should have feedback_history content", () => {
      const content = SPECIFIC_PAGE_HELP.feedback_history;
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Feedback Historie")).toBeTruthy();
      expect(
        container.textContent?.includes("Pädagogische Beobachtungen"),
      ).toBeTruthy();
    });

    it("should have mensa_history content", () => {
      const content = SPECIFIC_PAGE_HELP.mensa_history;
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Mensa Historie")).toBeTruthy();
      expect(container.textContent?.includes("Teilnahmehistorie")).toBeTruthy();
    });

    it("should have room_history content", () => {
      const content = SPECIFIC_PAGE_HELP.room_history;
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Raum Historie")).toBeTruthy();
      expect(container.textContent?.includes("Raumwechsel")).toBeTruthy();
    });

    it("should have room-detail content", () => {
      const content = SPECIFIC_PAGE_HELP["room-detail"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Raumdetailansicht")).toBeTruthy();
      expect(container.textContent?.includes("Aktuelle Belegung")).toBeTruthy();
    });

    it("should have database-students content", () => {
      const content = SPECIFIC_PAGE_HELP["database-students"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(
        container.textContent?.includes("Schüler Verwaltung"),
      ).toBeTruthy();
      expect(
        container.textContent?.includes("Verfügbare Operationen"),
      ).toBeTruthy();
    });

    it("should have database-teachers content", () => {
      const content = SPECIFIC_PAGE_HELP["database-teachers"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(
        container.textContent?.includes("Betreuer Verwaltung"),
      ).toBeTruthy();
    });

    it("should have database-rooms content", () => {
      const content = SPECIFIC_PAGE_HELP["database-rooms"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Räume Verwaltung")).toBeTruthy();
    });

    it("should have database-activities content", () => {
      const content = SPECIFIC_PAGE_HELP["database-activities"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(
        container.textContent?.includes("Aktivitäten Verwaltung"),
      ).toBeTruthy();
    });

    it("should have database-groups content", () => {
      const content = SPECIFIC_PAGE_HELP["database-groups"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(
        container.textContent?.includes("Gruppen Verwaltung"),
      ).toBeTruthy();
    });

    it("should have database-roles content", () => {
      const content = SPECIFIC_PAGE_HELP["database-roles"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Rollen Verwaltung")).toBeTruthy();
    });

    it("should have database-devices content", () => {
      const content = SPECIFIC_PAGE_HELP["database-devices"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Geräte Verwaltung")).toBeTruthy();
    });

    it("should have database-permissions content", () => {
      const content = SPECIFIC_PAGE_HELP["database-permissions"];
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(
        container.textContent?.includes("Berechtigungen Verwaltung"),
      ).toBeTruthy();
    });

    it("should have invitations content", () => {
      const content = SPECIFIC_PAGE_HELP.invitations;
      expect(content).toBeDefined();

      const { container } = render(<div>{content}</div>);
      expect(container.textContent?.includes("Einladungen")).toBeTruthy();
      expect(container.textContent?.includes("Nutzer einladen")).toBeTruthy();
    });
  });

  describe("NAVIGATION_HELP", () => {
    it("should have dashboard help", () => {
      const help = NAVIGATION_HELP["/dashboard"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Dashboard Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(
        container.textContent?.includes("Dashboard Übersicht"),
      ).toBeTruthy();
      expect(
        container.textContent?.includes("Anwesenheitsübersicht"),
      ).toBeTruthy();
    });

    it("should have ogs-groups help", () => {
      const help = NAVIGATION_HELP["/ogs-groups"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("OGS-Gruppenansicht Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(
        container.textContent?.includes("OGS Gruppenübersicht"),
      ).toBeTruthy();
      expect(container.textContent?.includes("Gruppenraum:")).toBeTruthy();
    });

    it("should have active-supervisions help", () => {
      const help = NAVIGATION_HELP["/active-supervisions"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Aktuelle Aufsicht Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(
        container.textContent?.includes("Aktuelle Aufsicht Übersicht"),
      ).toBeTruthy();
    });

    it("should have students help", () => {
      const help = NAVIGATION_HELP["/students"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Schülersuche Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(container.textContent?.includes("Schülersuche")).toBeTruthy();
    });

    it("should have rooms help", () => {
      const help = NAVIGATION_HELP["/rooms"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Raumverwaltung Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(
        container.textContent?.includes("Raumverwaltung & Belegung"),
      ).toBeTruthy();
    });

    it("should have activities help", () => {
      const help = NAVIGATION_HELP["/activities"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Aktivitäten Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(container.textContent?.includes("Aktivitäten")).toBeTruthy();
    });

    it("should have statistics help", () => {
      const help = NAVIGATION_HELP["/statistics"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Statistiken Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(
        container.textContent?.includes("Statistiken & Auswertungen"),
      ).toBeTruthy();
    });

    it("should have substitutions help", () => {
      const help = NAVIGATION_HELP["/substitutions"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Vertretungen Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(
        container.textContent?.includes("Vertretungsmanagement"),
      ).toBeTruthy();
    });

    it("should have database help", () => {
      const help = NAVIGATION_HELP["/database"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Datenverwaltung Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(container.textContent?.includes("Datenverwaltung")).toBeTruthy();
      expect(
        container.textContent?.includes("Verfügbare Datenebenen (8)"),
      ).toBeTruthy();
    });

    it("should have staff help", () => {
      const help = NAVIGATION_HELP["/staff"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Mitarbeiter Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(container.textContent?.includes("Personalübersicht")).toBeTruthy();
    });

    it("should have settings help", () => {
      const help = NAVIGATION_HELP["/settings"];
      expect(help).toBeDefined();
      expect(help?.title).toBe("Einstellungen Hilfe");

      const { container } = render(<div>{help?.content}</div>);
      expect(container.textContent?.includes("Einstellungen")).toBeTruthy();
      expect(container.textContent?.includes("Profil")).toBeTruthy();
    });
  });

  describe("getHelpContent", () => {
    it("should return dashboard help for /dashboard", () => {
      const result = getHelpContent("/dashboard");
      expect(result.title).toBe("Dashboard Hilfe");
    });

    it("should return student detail help for /students/123", () => {
      const result = getHelpContent("/students/123");
      expect(result.title).toBe("Schülerdetails Hilfe");
    });

    it("should return student search help for /students/search", () => {
      const result = getHelpContent("/students/search");
      expect(result.title).toBe("Schülersuche Hilfe");
    });

    it("should return feedback history help for /students/123/feedback_history", () => {
      const result = getHelpContent("/students/123/feedback_history");
      expect(result.title).toBe("Feedback Historie Hilfe");
    });

    it("should return mensa history help for /students/123/mensa_history", () => {
      const result = getHelpContent("/students/123/mensa_history");
      expect(result.title).toBe("Mensa Historie Hilfe");
    });

    it("should return room history help for /students/123/room_history", () => {
      const result = getHelpContent("/students/123/room_history");
      expect(result.title).toBe("Raum Historie Hilfe");
    });

    it("should return room detail help for /rooms/456", () => {
      const result = getHelpContent("/rooms/456");
      expect(result.title).toBe("Raumdetail Hilfe");
    });

    it("should return rooms help for /rooms", () => {
      const result = getHelpContent("/rooms");
      expect(result.title).toBe("Raumverwaltung Hilfe");
    });

    it("should return database students help for /database/students", () => {
      const result = getHelpContent("/database/students");
      expect(result.title).toBe("Schüler Verwaltung Hilfe");
    });

    it("should return database teachers help for /database/teachers/new", () => {
      const result = getHelpContent("/database/teachers/new");
      expect(result.title).toBe("Betreuer Verwaltung Hilfe");
    });

    it("should return database rooms help for /database/rooms/edit/1", () => {
      const result = getHelpContent("/database/rooms/edit/1");
      expect(result.title).toBe("Räume Verwaltung Hilfe");
    });

    it("should return database activities help for /database/activities", () => {
      const result = getHelpContent("/database/activities");
      expect(result.title).toBe("Aktivitäten Verwaltung Hilfe");
    });

    it("should return database groups help for /database/groups", () => {
      const result = getHelpContent("/database/groups");
      expect(result.title).toBe("Gruppen Verwaltung Hilfe");
    });

    it("should return database roles help for /database/roles", () => {
      const result = getHelpContent("/database/roles");
      expect(result.title).toBe("Rollen Verwaltung Hilfe");
    });

    it("should return database devices help for /database/devices", () => {
      const result = getHelpContent("/database/devices");
      expect(result.title).toBe("Geräte Verwaltung Hilfe");
    });

    it("should return database permissions help for /database/permissions", () => {
      const result = getHelpContent("/database/permissions");
      expect(result.title).toBe("Berechtigungen Verwaltung Hilfe");
    });

    it("should return invitations help for /invitations", () => {
      const result = getHelpContent("/invitations");
      expect(result.title).toBe("Einladungen Hilfe");
    });

    it("should return ogs-groups help for /ogs-groups", () => {
      const result = getHelpContent("/ogs-groups");
      expect(result.title).toBe("OGS-Gruppenansicht Hilfe");
    });

    it("should return active-supervisions help for /active-supervisions", () => {
      const result = getHelpContent("/active-supervisions");
      expect(result.title).toBe("Aktuelle Aufsicht Hilfe");
    });

    it("should return activities help for /activities", () => {
      const result = getHelpContent("/activities");
      expect(result.title).toBe("Aktivitäten Hilfe");
    });

    it("should return statistics help for /statistics", () => {
      const result = getHelpContent("/statistics");
      expect(result.title).toBe("Statistiken Hilfe");
    });

    it("should return substitutions help for /substitutions", () => {
      const result = getHelpContent("/substitutions");
      expect(result.title).toBe("Vertretungen Hilfe");
    });

    it("should return database help for /database", () => {
      const result = getHelpContent("/database");
      expect(result.title).toBe("Datenverwaltung Hilfe");
    });

    it("should return staff help for /staff", () => {
      const result = getHelpContent("/staff");
      expect(result.title).toBe("Mitarbeiter Hilfe");
    });

    it("should return settings help for /settings", () => {
      const result = getHelpContent("/settings");
      expect(result.title).toBe("Einstellungen Hilfe");
    });

    it("should return default help for unknown route", () => {
      const result = getHelpContent("/unknown/path");
      expect(result.title).toBe("Allgemeine Hilfe");

      const { container } = render(<div>{result.content}</div>);
      expect(container.textContent?.includes("moto")).toBeTruthy();
      expect(
        container.textContent?.includes("kontextbezogene Hilfe"),
      ).toBeTruthy();
    });

    it("should handle root path with default help", () => {
      const result = getHelpContent("/");
      expect(result.title).toBe("Allgemeine Hilfe");
    });
  });

  describe("component rendering", () => {
    it("should render InfoListItem with title and description", () => {
      const content = SPECIFIC_PAGE_HELP["student-detail"];
      render(<div>{content}</div>);

      expect(screen.getByText("Persönliche Daten")).toBeDefined();
      expect(screen.getByText(/Name, Klasse, Geburtsdatum/)).toBeDefined();
    });

    it("should render BulletItem in student detail", () => {
      const content = SPECIFIC_PAGE_HELP["student-detail"];
      render(<div>{content}</div>);

      expect(screen.getByText("Raumbewegungen dokumentieren")).toBeDefined();
    });

    it("should render StatusBulletItem in OGS groups help", () => {
      const help = NAVIGATION_HELP["/ogs-groups"];
      render(<div>{help?.content}</div>);

      expect(screen.getByText(/Gruppenraum:/)).toBeDefined();
      expect(screen.getByText(/Fremder Raum:/)).toBeDefined();
      expect(screen.getByText(/Unterwegs:/)).toBeDefined();
    });

    it("should render CrudOperationsList in database sections", () => {
      const content = SPECIFIC_PAGE_HELP["database-students"];
      render(<div>{content}</div>);

      expect(screen.getByText("Verfügbare Operationen")).toBeDefined();
      expect(screen.getByText(/Anlegen:/)).toBeDefined();
      expect(screen.getByText(/Bearbeiten:/)).toBeDefined();
      expect(screen.getByText(/Anzeigen:/)).toBeDefined();
      expect(screen.getByText(/Löschen:/)).toBeDefined();
    });

    it("should render DatabaseSectionHelp component correctly", () => {
      const content = SPECIFIC_PAGE_HELP["database-teachers"];
      render(<div>{content}</div>);

      expect(screen.getByText("Betreuer Verwaltung")).toBeDefined();
      expect(screen.getByText(/Verwalte alle Betreuerdaten/)).toBeDefined();
    });
  });
});
