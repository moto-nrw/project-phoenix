/**
 * Tests for the head parts of a Träger or school page.
 *
 * Covers active/inactive labels, the "Seit" year derivation from createdAt
 * (including the invalid-date fallback path) and the stats list.
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";

import { EntityStats, EntityStatusLine } from "./entity-head";

describe("EntityStatusLine", () => {
  it("renders the address and the active label", () => {
    render(<EntityStatusLine subdomain="test-school" active />);

    expect(screen.getByText("test-school")).toBeInTheDocument();
    // DataTableStatusBadge defaults to "Aktiv" / "Inaktiv".
    expect(screen.getByText("Aktiv")).toBeInTheDocument();
  });

  it("renders the inactive label when active is false", () => {
    render(<EntityStatusLine active={false} />);

    expect(screen.getByText("Inaktiv")).toBeInTheDocument();
  });
});

describe("EntityStats", () => {
  it("derives the 'Seit' year from a valid createdAt", () => {
    render(<EntityStats createdAt="2025-06-15T10:00:00Z" />);

    expect(screen.getByText("Seit")).toBeInTheDocument();
    expect(screen.getByText("2025")).toBeInTheDocument();
  });

  it("does not render 'Seit' when createdAt is invalid (covers NaN branch)", () => {
    render(
      <EntityStats
        createdAt="not-a-date"
        stats={[{ label: "Schulen", value: "3" }]}
      />,
    );

    // Anker: die übrige Kennzahl ist gerendert.
    expect(screen.getByText("Schulen")).toBeInTheDocument();
    expect(screen.queryByText("Seit")).not.toBeInTheDocument();
  });

  it("renders custom stats next to the year", () => {
    render(
      <EntityStats
        createdAt="2024-01-01T00:00:00Z"
        stats={[{ label: "Schulen", value: "3" }]}
      />,
    );

    expect(screen.getByText("Schulen")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("2024")).toBeInTheDocument();
  });
});
