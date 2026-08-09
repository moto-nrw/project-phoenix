/**
 * Tests for EntityHeaderCard.
 *
 * Covers title/subtitle rendering, active/inactive labels, the "Seit" year
 * derivation from createdAt (including the invalid-date fallback path),
 * and stats list rendering.
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";

import { EntityHeaderCard } from "./entity-header-card";

describe("EntityHeaderCard", () => {
  it("renders title and the active label", () => {
    render(<EntityHeaderCard title="Test School" active />);

    expect(screen.getByText("Test School")).toBeInTheDocument();
    // DataTableStatusBadge defaults to "Aktiv" / "Inaktiv".
    expect(screen.getByText("Aktiv")).toBeInTheDocument();
  });

  it("renders the inactive label when active is false", () => {
    render(<EntityHeaderCard title="Inactive Org" active={false} />);

    expect(screen.getByText("Inaktiv")).toBeInTheDocument();
  });

  it("derives the 'Seit' year from a valid createdAt", () => {
    render(
      <EntityHeaderCard title="Org" active createdAt="2025-06-15T10:00:00Z" />,
    );

    expect(screen.getByText("Seit")).toBeInTheDocument();
    expect(screen.getByText("2025")).toBeInTheDocument();
  });

  it("does not render 'Seit' when createdAt is invalid (covers NaN branch)", () => {
    render(<EntityHeaderCard title="Org" active createdAt="not-a-date" />);

    expect(screen.queryByText("Seit")).not.toBeInTheDocument();
  });

  it("renders custom stats next to the year", () => {
    render(
      <EntityHeaderCard
        title="Org"
        active
        createdAt="2024-01-01T00:00:00Z"
        stats={[{ label: "Schulen", value: "3" }]}
      />,
    );

    expect(screen.getByText("Schulen")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("2024")).toBeInTheDocument();
  });

  it("renders the grey concept icon tile when concept is set", () => {
    const { container } = render(
      <EntityHeaderCard title="Org" active concept="organizations" />,
    );

    expect(
      container.querySelector('[data-moto-duotone-tone="petrol"]'),
    ).toBeInTheDocument();
  });

  it("renders no concept icon tile when concept is omitted", () => {
    const { container } = render(<EntityHeaderCard title="Org" active />);

    expect(
      container.querySelector("[data-moto-duotone-tone]"),
    ).not.toBeInTheDocument();
  });
});
