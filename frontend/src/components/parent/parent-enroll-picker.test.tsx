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
    school_slug: "nord",
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

    // Phase links point at /parents/enroll/{slug}/{phaseId}
    expect(
      screen.getByRole("link", { name: "Anmeldung öffnen: Schuljahr 2026/27" }),
    ).toHaveAttribute("href", "/parents/enroll/nord/10");
    expect(
      screen.getByRole("link", { name: "Anmeldung öffnen: Sonderphase" }),
    ).toHaveAttribute("href", "/parents/enroll/sued/20");

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
