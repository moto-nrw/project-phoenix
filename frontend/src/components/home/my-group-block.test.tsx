import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { HomeGroupSnapshot, HomePickup } from "~/lib/hooks/use-home-group";
import type { OgsLiveWireStudent } from "~/lib/ogs-group-live-api";

const snapshot = vi.hoisted(() => ({
  current: {} as HomeGroupSnapshot,
}));

vi.mock("~/lib/hooks/use-home-group", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/hooks/use-home-group")>()),
  useHomeGroup: () => snapshot.current,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { MyGroupBlock } from "./my-group-block";

function student(
  overrides: Partial<OgsLiveWireStudent> = {},
): OgsLiveWireStudent {
  return {
    id: "1",
    first_name: "Mia",
    last_name: "Berger",
    school_class: "2a",
    current_location: "HOME",
    sick: false,
    excused: false,
    class_trip: false,
    ...overrides,
  };
}

function pickup(overrides: Partial<HomePickup> = {}): HomePickup {
  return {
    student: student({
      id: "7",
      first_name: "Emma",
      last_name: "Meyer",
      current_location: "Anwesend - OGS-Raum 1",
    }),
    time: "14:30",
    note: "Musikunterricht danach",
    isException: false,
    ...overrides,
  };
}

function withGroup(
  overrides: Partial<HomeGroupSnapshot> = {},
): HomeGroupSnapshot {
  return {
    group: {
      id: "5",
      name: "Sternengruppe",
      roomName: "OGS-Raum 1",
      viaSubstitution: false,
    },
    present: 18,
    total: 22,
    elsewhere: 0,
    away: [],
    missing: [],
    pickups: [],
    nextPickup: null,
    isLoading: false,
    error: undefined,
    ...overrides,
  };
}

describe("MyGroupBlock (#2180)", () => {
  beforeEach(() => {
    snapshot.current = withGroup();
  });

  // Das ist die Karte für die Kraft am Tisch: wer wann abgeholt wird und was
  // die Eltern dazu gesagt haben, steht vor allem anderen.
  it("zeigt Gruppe, Anwesenheit und die nächsten Abholungen mit Notiz", () => {
    snapshot.current = withGroup({
      pickups: [
        pickup(),
        pickup({
          student: student({ id: "8", first_name: "Noah", last_name: "Klein" }),
          time: "15:30",
          note: undefined,
          isException: true,
        }),
      ],
      nextPickup: "14:30",
    });

    render(<MyGroupBlock />);

    expect(
      screen.getByText("Sternengruppe · 18 von 22 da"),
    ).toBeInTheDocument();
    expect(screen.getByText("Emma Meyer")).toBeInTheDocument();
    expect(screen.getByText("14:30")).toBeInTheDocument();
    expect(screen.getByText("Musikunterricht danach")).toBeInTheDocument();
    expect(screen.getByText("Ausnahme")).toBeInTheDocument();
    expect(
      screen.getByRole("link", {
        name: "Emma Meyer, Abholung 14:30: Gruppe öffnen",
      }),
    ).toHaveAttribute("href", "/test-tenant/ogs-groups");
  });

  it("nennt, wer fehlt, mit dem Grund", () => {
    snapshot.current = withGroup({
      away: [
        student({ id: "1", sick: true }),
        student({
          id: "2",
          first_name: "Paul",
          last_name: "Wolf",
          excused: true,
        }),
      ],
      present: 20,
    });

    render(<MyGroupBlock />);

    expect(screen.getByText("Mia Berger")).toBeInTheDocument();
    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.getByText("Entschuldigt")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Mia Berger: Gruppe öffnen" }),
    ).toHaveAttribute("href", "/test-tenant/ogs-groups");
  });

  // Die Reihenfolge ist die Dringlichkeit: wer längst da sein sollte, dann
  // was auf einen zukommt, zuletzt, was schon entschieden ist.
  it("stellt Fehlende vor Abholungen vor Abwesende", () => {
    const felix = student({
      id: "3",
      first_name: "Felix",
      last_name: "Lang",
      arrival_time: "09:15",
      arrival_notes: "Arzttermin, kommt danach",
    });
    snapshot.current = withGroup({
      pickups: [pickup()],
      missing: [
        { student: felix, expected: "09:15", note: "Arzttermin, kommt danach" },
      ],
      away: [student({ id: "1", sick: true }), felix],
    });

    render(<MyGroupBlock />);

    const links = screen.getAllByRole("link", { name: /Gruppe öffnen/ });
    expect(links).toHaveLength(3);
    expect(links[0]).toHaveAccessibleName(
      "Felix Lang, fehlt seit 09:15: Gruppe öffnen",
    );
    expect(links[1]).toHaveAccessibleName(/Emma Meyer, Abholung/);
    expect(links[2]).toHaveAccessibleName("Mia Berger: Gruppe öffnen");
    expect(screen.getByText("Fehlt noch")).toBeInTheDocument();
    expect(screen.getByText("Arzttermin, kommt danach")).toBeInTheDocument();
  });

  // Vor der Ankunftszeit ist ein Kind nicht „zuhause", es wird erwartet.
  it("nennt die Ankunftszeit, solange das Kind noch erwartet wird", () => {
    snapshot.current = withGroup({
      away: [student({ id: "1", arrival_time: "09:15" })],
    });

    render(<MyGroupBlock />);

    expect(screen.getByText("Kommt 09:15")).toBeInTheDocument();
  });

  it("zählt, wer gerade nicht im Gruppenraum ist", () => {
    snapshot.current = withGroup({ elsewhere: 3 });

    render(<MyGroupBlock />);

    expect(
      screen.getByText(
        "Sternengruppe · 18 von 22 da · 3 außerhalb des Gruppenraums",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Alle da, keine Abholung mehr heute"),
    ).toBeInTheDocument();
  });

  it("kennzeichnet eine Gruppe in Vertretung", () => {
    snapshot.current = withGroup({
      group: {
        id: "5",
        name: "Bärengruppe",
        viaSubstitution: true,
      },
    });

    render(<MyGroupBlock />);

    expect(
      screen.getByText("Bärengruppe (Vertretung) · 18 von 22 da"),
    ).toBeInTheDocument();
  });

  it("sagt es, wenn heute keine Gruppe zugeteilt ist", () => {
    snapshot.current = withGroup({ group: null });

    render(<MyGroupBlock />);

    expect(
      screen.getByText("Ihnen ist heute keine Gruppe zugeteilt"),
    ).toBeInTheDocument();
  });

  it("unterscheidet einen Ladefehler von einer leeren Gruppe", () => {
    snapshot.current = withGroup({ group: null, error: new Error("boom") });

    render(<MyGroupBlock />);

    expect(screen.getByText(/konnte nicht geladen werden/)).toBeInTheDocument();
  });
});
