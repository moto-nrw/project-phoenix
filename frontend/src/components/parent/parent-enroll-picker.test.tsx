import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type React from "react";

const mocks = vi.hoisted(() => ({
  listEnrollableSchools: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href?.toString()} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("~/lib/parent-api", () => ({
  listEnrollableSchools: mocks.listEnrollableSchools,
}));

import { ParentEnrollPicker } from "./parent-enroll-picker";
import type { EnrollablePhase } from "~/lib/parent-api";

function phase(overrides: Partial<EnrollablePhase> = {}): EnrollablePhase {
  return {
    school_id: "1",
    school_name: "OGS Nord",
    // Slug and subdomain differ on purpose: links must use the subdomain, the
    // identifier resolveTenant() and the parent enrollment endpoints accept.
    school_slug: "nord",
    school_subdomain: "ogs-nord",
    phase_id: "10",
    phase_name: "Schuljahr 2026/27",
    phase_kind: "school_year",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    enrollment_close_at: undefined,
    already_linked: true,
    audience: "open",
    ...overrides,
  };
}

describe("ParentEnrollPicker", () => {
  beforeEach(() => {
    mocks.listEnrollableSchools.mockReset();
  });

  it("reserves school headers and phase rows while loading", () => {
    mocks.listEnrollableSchools.mockReturnValue(new Promise(() => {}));

    render(<ParentEnrollPicker />);

    const skeleton = screen.getByTestId("parent-enroll-skeleton");
    expect(skeleton.querySelectorAll(".rounded-2xl.border")).toHaveLength(2);
    expect(skeleton.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
      10,
    );
  });

  it("renders linked/other groups, phase rows, links, kind + audience labels", async () => {
    mocks.listEnrollableSchools.mockResolvedValueOnce([
      phase(),
      phase({
        phase_id: "11",
        phase_name: "Ferien Sommer 2027",
        phase_kind: "holiday",
        audience: "new_students",
        enrollment_close_at: "2027-05-31T21:59:00Z",
      }),
      phase({
        school_id: "2",
        school_name: "OGS Süd",
        school_slug: "sued",
        school_subdomain: "ogs-sued",
        phase_id: "20",
        phase_name: "Sonderphase",
        phase_kind: "custom",
        already_linked: false,
        audience: "linked_parents",
      }),
    ]);

    render(<ParentEnrollPicker />);

    expect(
      await screen.findByRole("heading", { name: "Neue Anmeldung", level: 1 }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Anmeldung für die OGS-Betreuung"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Wählen Sie eine Schule und eine Anmeldephase, um Ihr Kind anzumelden.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zurück zum Elternportal" }),
    ).not.toBeInTheDocument();

    // Section labels
    expect(screen.getByText("Ihre Schulen")).toBeInTheDocument();
    expect(screen.getByText("Weitere Schulen")).toBeInTheDocument();

    // School headings
    expect(
      screen.getByRole("heading", { name: "OGS Nord" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "OGS Süd" }),
    ).toBeInTheDocument();

    // Phase links point at /parents/anmeldung/{subdomain}/{phaseId} — the slug
    // would not resolve for a school whose slug differs from its subdomain.
    expect(
      screen.getByRole("link", { name: "Anmeldung öffnen: Schuljahr 2026/27" }),
    ).toHaveAttribute("href", "/parents/anmeldung/ogs-nord/10");
    expect(
      screen.getByRole("link", { name: "Anmeldung öffnen: Sonderphase" }),
    ).toHaveAttribute("href", "/parents/anmeldung/ogs-sued/20");

    // Kind labels
    expect(screen.getByText("Schuljahr")).toBeInTheDocument();
    expect(screen.getByText("Ferienbetreuung")).toBeInTheDocument();
    expect(screen.getByText("Sonstiges")).toBeInTheDocument();

    // Audience badges (only for non-open phases)
    expect(screen.getByText("Nur neue Kinder")).toBeInTheDocument();
    expect(screen.getByText("Nur für verknüpfte Eltern")).toBeInTheDocument();

    // Close date shown for the phase that has one
    expect(screen.getByText(/Anmeldung bis/)).toBeInTheDocument();
  });

  it("shows the closing time, not just the date, for a non-midnight deadline", async () => {
    mocks.listEnrollableSchools.mockResolvedValueOnce([
      // 07:00 UTC = 09:00 Europe/Berlin (CEST). A date-only render would imply
      // availability for the whole day; the picker must surface the time.
      phase({ enrollment_close_at: "2027-05-31T07:00:00Z" }),
    ]);

    render(<ParentEnrollPicker />);

    expect(
      await screen.findByText(/Anmeldung bis .*09:00/),
    ).toBeInTheDocument();
  });

  it("shows the empty state when no phases are available", async () => {
    mocks.listEnrollableSchools.mockResolvedValueOnce([]);

    render(<ParentEnrollPicker />);

    expect(
      await screen.findByText("Derzeit sind keine Anmeldephasen verfügbar."),
    ).toBeInTheDocument();
  });

  it("shows an error state when loading fails", async () => {
    mocks.listEnrollableSchools.mockRejectedValueOnce(new Error("offline"));

    render(<ParentEnrollPicker />);

    expect(
      await screen.findByText(
        "Die Anmeldephasen konnten nicht geladen werden.",
      ),
    ).toBeInTheDocument();
  });
});
