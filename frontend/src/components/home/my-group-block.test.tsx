import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { HomeGroupSnapshot } from "~/lib/hooks/use-home-group";
import type { OgsLiveWireStudent } from "~/lib/ogs-group-live-api";

const snapshot = vi.hoisted(() => ({
  current: {} as HomeGroupSnapshot,
}));

vi.mock("~/lib/hooks/use-home-group", () => ({
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
    away: [],
    nextPickup: "14:30",
    isLoading: false,
    error: undefined,
    ...overrides,
  };
}

describe("MyGroupBlock (#2180)", () => {
  beforeEach(() => {
    snapshot.current = withGroup();
  });

  it("zeigt Gruppe, Anwesenheit und nächste Abholung", () => {
    render(<MyGroupBlock />);

    expect(
      screen.getByText("Sternengruppe · 18 von 22 da · nächste Abholung 14:30"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Alle Kinder Ihrer Gruppe sind da"),
    ).toBeInTheDocument();
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

  it("kennzeichnet eine Gruppe in Vertretung", () => {
    snapshot.current = withGroup({
      group: {
        id: "5",
        name: "Bärengruppe",
        viaSubstitution: true,
      },
      nextPickup: null,
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
