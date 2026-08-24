import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const useSWRAuth = vi.hoisted(() => vi.fn());

vi.mock("~/lib/swr", () => ({ useSWRAuth }));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

import { PhaseExpiryWarnings } from "./phase-expiry-warnings";

describe("PhaseExpiryWarnings", () => {
  beforeEach(() => {
    useSWRAuth.mockReset();
  });

  it("fordert 30 Tage vorher zum Erstellen einer Anschlussphase auf", () => {
    useSWRAuth.mockReturnValue({
      data: [
        {
          source_phase_id: "3",
          source_phase_name: "1. Halbjahr",
          first_affected_date: "2027-02-01",
          affected_children: 204,
          unresolved_children: 204,
          state: "missing_successor",
          overdue: false,
        },
      ],
      error: undefined,
      isLoading: false,
    });

    render(<PhaseExpiryWarnings />);

    expect(screen.getByText("Anschlussphase fehlt")).toBeVisible();
    expect(
      screen.getByText(
        "Ab 1. Februar 2027 enden die Buchungen für 204 Kinder. Erstellen Sie jetzt eine Anschlussphase.",
      ),
    ).toBeVisible();
    const actions = screen.getAllByRole("link", {
      name: "Anschlussphase erstellen",
    });
    expect(actions).toHaveLength(2);
    expect(actions[0]!).toHaveAttribute(
      "href",
      "/enrollment-phases/3/rollover",
    );
    expect(actions[0]!).toHaveClass("lg:hidden");
    expect(actions[1]!).toHaveAttribute(
      "href",
      "/enrollment-phases?rollover=3",
    );
    expect(actions[1]!).toHaveClass("hidden", "lg:inline-flex");
    expect(screen.getByRole("status")).toHaveClass("bg-moto-orange-soft");
  });

  it("zeigt eine unvollständige Übernahme nach dem Ausfalldatum rot", () => {
    useSWRAuth.mockReturnValue({
      data: [
        {
          source_phase_id: "3",
          source_phase_name: "1. Halbjahr",
          successor_phase_id: "12",
          successor_phase_name: "2. Halbjahr",
          first_affected_date: "2027-02-01",
          affected_children: 204,
          unresolved_children: 17,
          state: "incomplete",
          overdue: true,
        },
      ],
      error: undefined,
      isLoading: false,
    });

    render(<PhaseExpiryWarnings />);

    expect(screen.getByText("Übernahme nicht abgeschlossen")).toBeVisible();
    expect(
      screen.getByText(
        "Seit dem 1. Februar 2027 fehlen Buchungen für 17 Kinder. Schließen Sie jetzt die Übernahme ab.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Anschlussphase öffnen" }),
    ).toHaveAttribute("href", "/admin/enrollments/phases/12");
    expect(screen.getByRole("alert")).toHaveClass("bg-moto-red-soft");
  });

  it("nennt vor dem Ausfalldatum die noch offenen Folgebuchungen", () => {
    useSWRAuth.mockReturnValue({
      data: [
        {
          source_phase_id: "3",
          source_phase_name: "1. Halbjahr",
          successor_phase_id: "12",
          successor_phase_name: "2. Halbjahr",
          first_affected_date: "2027-02-01",
          affected_children: 204,
          unresolved_children: 17,
          state: "incomplete",
          overdue: false,
        },
      ],
      error: undefined,
      isLoading: false,
    });

    render(<PhaseExpiryWarnings />);

    expect(screen.getByText("Übernahme noch offen")).toBeVisible();
    expect(
      screen.getByText(
        "Ab 1. Februar 2027 fehlen noch Buchungen für 17 Kinder. Schließen Sie jetzt die Übernahme ab.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("status")).toHaveClass("bg-moto-orange-soft");
  });

  it("meldet eine weiterhin fehlende Anschlussphase nach dem Ausfalldatum", () => {
    useSWRAuth.mockReturnValue({
      data: [
        {
          source_phase_id: "3",
          source_phase_name: "1. Halbjahr",
          first_affected_date: "2027-02-01",
          affected_children: 204,
          unresolved_children: 204,
          state: "missing_successor",
          overdue: true,
        },
      ],
      error: undefined,
      isLoading: false,
    });

    render(<PhaseExpiryWarnings />);

    expect(screen.getByText("Buchungen fehlen")).toBeVisible();
    expect(
      screen.getByText(
        "Seit dem 1. Februar 2027 fehlen Buchungen für 204 Kinder. Erstellen Sie jetzt eine Anschlussphase.",
      ),
    ).toBeVisible();
    expect(screen.getByRole("alert")).toHaveClass("bg-moto-red-soft");
  });

  it("zeigt ohne Warnungen keinen leeren Platzhalter", () => {
    useSWRAuth.mockReturnValue({
      data: [],
      error: undefined,
      isLoading: false,
    });

    const { container } = render(<PhaseExpiryWarnings />);

    expect(container).toBeEmptyDOMElement();
  });

  it("verschweigt einen fehlgeschlagenen Bericht nicht", () => {
    useSWRAuth.mockReturnValue({
      data: undefined,
      error: new Error("boom"),
      isLoading: false,
    });

    render(<PhaseExpiryWarnings />);

    expect(screen.getByText("Hinweise nicht geladen")).toBeVisible();
    expect(
      screen.getByText(
        "Die Hinweise zum Phasenende konnten nicht geladen werden. Laden Sie die Seite neu.",
      ),
    ).toBeVisible();
  });
});
