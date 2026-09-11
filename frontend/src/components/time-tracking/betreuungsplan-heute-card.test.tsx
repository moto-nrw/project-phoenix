import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OwnAssignment } from "~/lib/shift-helpers";

const swr = vi.hoisted(() => ({
  data: undefined as OwnAssignment[] | undefined,
  error: undefined as Error | undefined,
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => swr,
}));
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-09-07",
}));
vi.mock("~/lib/shift-api", () => ({
  ownShiftService: { getOwnAssignments: vi.fn() },
}));
// Feste Uhrzeit: die Karte zeigt ab dem laufenden Einsatz, und ein Test, der
// an der Wanduhr hängt, wird am Nachmittag rot.
vi.mock("~/components/home/home-card-rows", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/components/home/home-card-rows")
  >()),
  useBerlinClock: () => "07:00",
}));

import { BetreuungsplanHeuteCard } from "./betreuungsplan-heute-card";

function assignment(overrides: Partial<OwnAssignment> = {}): OwnAssignment {
  return {
    instanceId: "1",
    date: "2026-09-07",
    startTime: "13:00",
    endTime: "14:00",
    title: "Lernzeit",
    groupName: "Mondgruppe",
    roomName: "OGS-Raum 1",
    cancelled: false,
    isAbsent: false,
    isSubstitute: false,
    understaffedAck: false,
    ...overrides,
  } as OwnAssignment;
}

describe("BetreuungsplanHeuteCard", () => {
  beforeEach(() => {
    swr.data = undefined;
    swr.error = undefined;
  });

  // Zeiterfassung: zweizeilig, Raum unter der Aufgabe — die gewohnte Ansicht.
  it("zeigt Zeit, Aufgabe, Gruppe und Raum eines Einsatzes", () => {
    swr.data = [assignment()];

    render(<BetreuungsplanHeuteCard />);

    expect(screen.getByText("13:00–14:00")).toBeInTheDocument();
    expect(screen.getByText(/Lernzeit/)).toBeInTheDocument();
    expect(screen.getByText("· Mondgruppe")).toBeInTheDocument();
    expect(screen.getByText("OGS-Raum 1")).toBeInTheDocument();
  });

  // Startseite: eine Zeile je Einsatz, Raum als Zusatz dahinter.
  it("fasst den Einsatz mit dense in eine Zeile", () => {
    swr.data = [assignment()];

    render(<BetreuungsplanHeuteCard dense />);

    expect(screen.getByText("· OGS-Raum 1 · Mondgruppe")).toBeInTheDocument();
  });

  // Die Karte hat auf der Startseite eine feste Höhe. Sie scrollt nicht: eine
  // angeschnittene Zeile am Kartenrand liest sich, als liefe der Baustein aus
  // seiner Karte heraus. Was nicht hineinpasst, wird gezählt und verlinkt.
  it("zeigt bei zu vielen Einsätzen den Rest als eine Zeile", () => {
    swr.data = [
      assignment({ instanceId: "1", startTime: "08:00", title: "Frühdienst" }),
      assignment({ instanceId: "2", startTime: "11:00", title: "Mittagessen" }),
      assignment({ instanceId: "3", startTime: "13:00", title: "Lernzeit" }),
      assignment({ instanceId: "4", startTime: "15:00", title: "Freispiel" }),
    ];

    render(
      <BetreuungsplanHeuteCard maxRows={3} href="/test-tenant/tagesplan" />,
    );

    expect(screen.getByText("Frühdienst")).toBeInTheDocument();
    expect(screen.getByText("Mittagessen")).toBeInTheDocument();
    expect(screen.queryByText("Lernzeit")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Noch 2 Einsätze im Tagesplan" }),
    ).toHaveAttribute("href", "/test-tenant/tagesplan");
  });

  it("zeigt alle Einsätze, wenn sie in die Karte passen", () => {
    swr.data = [
      assignment({ instanceId: "1", title: "Frühdienst" }),
      assignment({ instanceId: "2", title: "Mittagessen" }),
      assignment({ instanceId: "3", title: "Lernzeit" }),
    ];

    render(
      <BetreuungsplanHeuteCard maxRows={3} href="/test-tenant/tagesplan" />,
    );

    expect(screen.getByText("Lernzeit")).toBeInTheDocument();
    expect(screen.queryByText(/Noch \d+ Einsä?tz/)).not.toBeInTheDocument();
  });

  it("zeigt nach Tagesende die letzten Einsätze", () => {
    swr.data = [
      assignment({
        instanceId: "1",
        startTime: "05:00",
        endTime: "05:15",
        title: "Frühdienst",
      }),
      assignment({
        instanceId: "2",
        startTime: "05:15",
        endTime: "05:30",
        title: "Mittagessen",
      }),
      assignment({
        instanceId: "3",
        startTime: "05:30",
        endTime: "05:45",
        title: "Lernzeit",
      }),
      assignment({
        instanceId: "4",
        startTime: "05:45",
        endTime: "06:00",
        title: "Freispiel",
      }),
    ];

    render(<BetreuungsplanHeuteCard maxRows={3} />);

    expect(screen.queryByText("Frühdienst")).not.toBeInTheDocument();
    expect(screen.queryByText("Mittagessen")).not.toBeInTheDocument();
    expect(screen.getByText("Lernzeit")).toBeInTheDocument();
    expect(screen.getByText("Freispiel")).toBeInTheDocument();
  });

  // Auf der Startseite trägt jede Zeile ihren Zustand relativ zur Uhr (07:00):
  // „Läuft" für den Moment, die Zeit bis zum Beginn für das Kommende.
  it("zeigt auf der Startseite den Zustand jeder Zeile", () => {
    swr.data = [
      assignment({ instanceId: "1", startTime: "06:30", endTime: "07:30" }),
      assignment({
        instanceId: "2",
        startTime: "07:45",
        endTime: "08:30",
        title: "Frühstück",
      }),
    ];

    render(<BetreuungsplanHeuteCard maxRows={3} />);

    expect(screen.getByText("Läuft")).toBeInTheDocument();
    expect(screen.getByText("in 45 Min")).toBeInTheDocument();
  });

  // Auf der Zeiterfassung steht der Tag als Liste; dort wäre „vorbei" an
  // jeder zweiten Zeile nur Rauschen.
  it("zeigt auf der Zeiterfassung keinen Zustand", () => {
    swr.data = [
      assignment({ instanceId: "1", startTime: "06:30", endTime: "07:30" }),
    ];

    render(<BetreuungsplanHeuteCard />);

    expect(screen.queryByText("Läuft")).not.toBeInTheDocument();
  });

  // Ohne Höhenvorgabe (Zeiterfassung) steht der ganze Tag da.
  it("kappt ohne maxRows nichts", () => {
    swr.data = [
      assignment({ instanceId: "1", title: "Frühdienst" }),
      assignment({ instanceId: "2", title: "Mittagessen" }),
      assignment({ instanceId: "3", title: "Lernzeit" }),
      assignment({ instanceId: "4", title: "Freispiel" }),
    ];

    render(<BetreuungsplanHeuteCard />);

    expect(screen.getByText("Freispiel")).toBeInTheDocument();
  });

  // Auf der Startseite hat die Karte keinen Kopflink; die Zeile ist der Weg
  // weiter. Ohne Ziel bleibt sie eine Anzeige.
  it("macht die Zeile nur mit Ziel anklickbar", () => {
    swr.data = [assignment()];

    const { rerender } = render(<BetreuungsplanHeuteCard />);
    expect(screen.queryAllByRole("link")).toHaveLength(0);

    rerender(<BetreuungsplanHeuteCard href="/test-tenant/tagesplan" />);
    expect(
      screen.getByRole("link", { name: "Lernzeit: im Tagesplan öffnen" }),
    ).toHaveAttribute("href", "/test-tenant/tagesplan");
  });

  // Auf der Zeiterfassung verschwindet die leere Karte, auf der Startseite ist
  // sie ein gewählter Baustein und muss antworten.
  it("zeigt ohne Einsätze nur mit showEmpty eine Antwort", () => {
    swr.data = [];

    const { container, rerender } = render(<BetreuungsplanHeuteCard />);
    expect(container).toBeEmptyDOMElement();

    rerender(<BetreuungsplanHeuteCard showEmpty />);
    expect(
      screen.getByText("Heute ist für Sie nichts geplant"),
    ).toBeInTheDocument();
  });

  it("unterscheidet einen Ladefehler von einem leeren Tag", () => {
    swr.error = new Error("boom");

    render(<BetreuungsplanHeuteCard showEmpty />);

    expect(
      screen.getByText(/konnten nicht geladen werden/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Heute ist für Sie nichts geplant"),
    ).not.toBeInTheDocument();
  });
});
