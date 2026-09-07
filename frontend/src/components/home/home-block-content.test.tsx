import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { DashboardAnalytics } from "~/lib/dashboard-helpers";

vi.mock("~/lib/dashboard-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/dashboard-helpers")>();
  return {
    ...actual,
    formatRecentActivityTime: vi.fn(() => "14:05"),
    getActivityStatusColor: vi.fn(() => "bg-moto-green"),
    getGroupStatusColor: vi.fn(() => "bg-moto-green"),
  };
});

// Die vier Bausteine mit eigener Datenquelle haben eigene Tests; hier geht es
// um die Kacheln und Listen, die aus den Betriebszahlen der Seite leben.
vi.mock("~/components/home/day-flow-block", () => ({
  DayFlowBlock: () => <div data-testid="day-flow-block" />,
}));
vi.mock("~/components/home/open-requests-block", () => ({
  OpenRequestsBlock: () => <div data-testid="open-requests-block" />,
}));
vi.mock("~/components/home/staff-notices-block", () => ({
  StaffNoticesBlock: () => <div data-testid="staff-notices-block" />,
}));
vi.mock("~/components/time-tracking/betreuungsplan-heute-card", () => ({
  BetreuungsplanHeuteCard: ({ title }: { title?: string }) => (
    <div data-testid="my-day-block">{title}</div>
  ),
}));

import { HomeBlockContent, type HomeBlockData } from "./home-block-content";

const analytics: DashboardAnalytics = {
  studentsPresent: 150,
  studentsInRooms: 120,
  studentsInTransit: 20,
  studentsOnPlayground: 10,
  studentsSick: 4,
  studentsExcused: 3,
  studentsHome: 33,
  activeOGSGroups: 8,
  activeActivities: 5,
  capacityUtilization: 0.75,
  supervisorsToday: 10,
  recentActivity: [
    {
      type: "checkin",
      groupName: "Gruppe 1",
      roomName: "Raum 101",
      timestamp: "2026-09-07T12:05:00Z",
      count: 5,
    },
  ],
  currentActivities: [
    {
      id: "101",
      name: "Schach",
      category: "Sport",
      participants: 8,
      maxCapacity: 10,
      status: "active",
    },
  ],
  activeGroupsSummary: [
    {
      type: "ogs",
      name: "OGS Gruppe A",
      location: "Raum 101",
      studentCount: 15,
      status: "active",
    },
  ],
} as unknown as DashboardAnalytics;

function data(overrides: Partial<HomeBlockData> = {}): HomeBlockData {
  return {
    analytics,
    analyticsLoading: false,
    birthdays: undefined,
    birthdaysLoading: false,
    tenantPath: (path: string) => `/test-tenant${path}`,
    ...overrides,
  };
}

describe("HomeBlockContent — Kennzahlen", () => {
  it("zeigt Beschriftung und Wert einer Kachel", () => {
    render(
      <HomeBlockContent blockKey="tile.students_present" data={data()} />,
    );

    expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();
    expect(screen.getByText("150")).toBeInTheDocument();
  });

  it("führt von einer Kachel in die passend gefilterte Kinderliste", () => {
    render(<HomeBlockContent blockKey="tile.students_sick" data={data()} />);

    expect(screen.getByRole("link")).toHaveAttribute(
      "href",
      "/test-tenant/students/search?status=krank",
    );
  });

  it("zeigt die Auslastung als Prozentwert", () => {
    render(
      <HomeBlockContent blockKey="tile.capacity_utilization" data={data()} />,
    );

    expect(screen.getByText("75%")).toBeInTheDocument();
  });

  it("zeigt eine Null, solange die Zahlen noch geladen werden", () => {
    render(
      <HomeBlockContent
        blockKey="tile.students_present"
        data={data({ analytics: undefined, analyticsLoading: true })}
      />,
    );

    expect(screen.getByText("Kinder anwesend")).toBeInTheDocument();
  });
});

describe("HomeBlockContent — Listen", () => {
  it("zeigt die letzten Bewegungen mit Gruppe, Raum und Zeit", () => {
    render(
      <HomeBlockContent blockKey="section.recent_activity" data={data()} />,
    );

    expect(screen.getByText("Letzte Bewegungen")).toBeInTheDocument();
    expect(screen.getByText("Gruppe 1")).toBeInTheDocument();
    expect(screen.getByText("Raum 101")).toBeInTheDocument();
    expect(screen.getByText("5 Kinder")).toBeInTheDocument();
  });

  it("sagt es, wenn es keine Bewegungen gibt", () => {
    render(
      <HomeBlockContent
        blockKey="section.recent_activity"
        data={data({ analytics: { ...analytics, recentActivity: [] } })}
      />,
    );

    expect(screen.getByText("Keine aktuellen Bewegungen")).toBeInTheDocument();
  });

  it("zeigt laufende Aktivitäten mit Kategorie und Belegung", () => {
    render(
      <HomeBlockContent blockKey="section.current_activities" data={data()} />,
    );

    expect(screen.getByText("Schach")).toBeInTheDocument();
    expect(screen.getByText("Sport • 8/10 Teilnehmer")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Laufende Aktivitäten/ }),
    ).toHaveAttribute("href", "/test-tenant/activities");
  });

  it("sagt es, wenn keine Aktivität läuft", () => {
    render(
      <HomeBlockContent
        blockKey="section.current_activities"
        data={data({ analytics: { ...analytics, currentActivities: [] } })}
      />,
    );

    expect(screen.getByText("Keine laufenden Aktivitäten")).toBeInTheDocument();
  });

  it("zeigt aktive Gruppen mit Ort und Kinderzahl", () => {
    render(<HomeBlockContent blockKey="section.active_groups" data={data()} />);

    expect(screen.getByText("OGS Gruppe A")).toBeInTheDocument();
    expect(screen.getByText("Raum 101 • 15 Kinder")).toBeInTheDocument();
  });

  it("sagt es, wenn keine Gruppe aktiv ist", () => {
    render(
      <HomeBlockContent
        blockKey="section.active_groups"
        data={data({ analytics: { ...analytics, activeGroupsSummary: [] } })}
      />,
    );

    expect(screen.getByText("Keine aktiven Gruppen")).toBeInTheDocument();
  });

  it("zeigt die Geburtstage der Kinder", () => {
    render(
      <HomeBlockContent
        blockKey="section.birthdays"
        data={data({
          birthdays: {
            enabled: true,
            celebrations: [
              {
                kind: "student",
                id: "1",
                name: "Henri Fuchs",
                groupName: "Mondgruppe",
                schoolClass: "3a",
                date: "2026-09-05",
                age: 9,
                isToday: false,
              },
            ],
          } as unknown as HomeBlockData["birthdays"],
        })}
      />,
    );

    expect(screen.getByText("Geburtstage")).toBeInTheDocument();
    expect(screen.getByText("Henri Fuchs")).toBeInTheDocument();
  });
});

describe("HomeBlockContent — eigene Quellen", () => {
  it("nennt den Baustein der eigenen Einsätze 'Mein Tag'", () => {
    render(<HomeBlockContent blockKey="section.my_day" data={data()} />);

    expect(screen.getByTestId("my-day-block")).toHaveTextContent("Mein Tag");
  });

  it("rendert die drei Bausteine mit eigener Abfrage", () => {
    const { rerender } = render(
      <HomeBlockContent blockKey="section.staff_notices" data={data()} />,
    );
    expect(screen.getByTestId("staff-notices-block")).toBeInTheDocument();

    rerender(<HomeBlockContent blockKey="section.day_flow" data={data()} />);
    expect(screen.getByTestId("day-flow-block")).toBeInTheDocument();

    rerender(
      <HomeBlockContent blockKey="section.open_requests" data={data()} />,
    );
    expect(screen.getByTestId("open-requests-block")).toBeInTheDocument();
  });
});
