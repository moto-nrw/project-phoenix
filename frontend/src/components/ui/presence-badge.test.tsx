import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { PresenceBadge } from "./presence-badge";
import { LOCATION_COLORS, getLocationBadgeTone } from "~/lib/location-helper";

// Helper to fetch the rendered pill so assertions can interrogate both
// the label (visible text) and the inline color style. The render tree has
// a container <div> wrapping the pill + optional overlays; the pill carries
// a data-presence-state attribute we use as a stable selector.
function getPill(): HTMLElement {
  // eslint-disable-next-line testing-library/no-node-access
  const pill = document.querySelector("[data-presence-state]");
  if (!pill) throw new Error("PresenceBadge pill not found in render output");
  return pill as HTMLElement;
}

// Different DOM test environments serialise inline colors differently —
// jsdom collapses hex to `rgb(...)`, happy-dom preserves the original hex.
// Accept both forms so the same test works under either runner.
function bgMatchesBrandColor(pill: HTMLElement, hex: string): boolean {
  const raw = pill.style.backgroundColor.replace(/\s/g, "").toLowerCase();
  const hexLower = hex.toLowerCase();
  return raw === hexLower || raw === hexToRgbCompact(hex);
}

describe("PresenceBadge", () => {
  it("renders Anwesend in brand green when checked in", () => {
    render(<PresenceBadge student={{ current_location: "Anwesend" }} />);
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    const pill = getPill();
    expect(pill.getAttribute("data-presence-state")).toBe("anwesend");
    expect(
      bgMatchesBrandColor(
        pill,
        getLocationBadgeTone(LOCATION_COLORS.GROUP_ROOM).backgroundColor,
      ),
    ).toBe(true);
  });

  it("renders Schulhof in brand orange when on yard", () => {
    render(<PresenceBadge student={{ current_location: "Schulhof" }} />);
    expect(screen.getByText("Schulhof")).toBeInTheDocument();
    const pill = getPill();
    expect(pill.getAttribute("data-presence-state")).toBe("schulhof");
    expect(
      bgMatchesBrandColor(
        pill,
        getLocationBadgeTone(LOCATION_COLORS.SCHOOLYARD).backgroundColor,
      ),
    ).toBe(true);
  });

  it("renders Abwesend in brand red when checked out", () => {
    render(<PresenceBadge student={{ current_location: "Zuhause" }} />);
    expect(screen.getByText("Zuhause")).toBeInTheDocument();
    const pill = getPill();
    expect(pill.getAttribute("data-presence-state")).toBe("abwesend");
  });

  it("renders Abwesend when current_location is empty", () => {
    render(<PresenceBadge student={{ current_location: null }} />);
    expect(screen.getByText("Zuhause")).toBeInTheDocument();
    expect(getPill().getAttribute("data-presence-state")).toBe("abwesend");
  });

  it("replaces the Abwesend label with Krank for sick students at home", () => {
    render(
      <PresenceBadge student={{ current_location: "Zuhause", sick: true }} />,
    );
    expect(screen.getByText("Krank")).toBeInTheDocument();
    // "Zuhause" should not appear — sick replaces the entire label.
    expect(screen.queryByText("Zuhause")).not.toBeInTheDocument();
  });

  it("renders a Krank overlay when sick student is present", () => {
    render(
      <PresenceBadge student={{ current_location: "Anwesend", sick: true }} />,
    );
    // The main pill still reads "Anwesend" and there's an additional sick marker.
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    expect(screen.getByText("Krank")).toBeInTheDocument();
    // eslint-disable-next-line testing-library/no-node-access
    expect(document.querySelector("[data-sick-indicator]")).not.toBeNull();
  });

  it("replaces the Abwesend label with Entschuldigt for excused students at home", () => {
    render(
      <PresenceBadge
        student={{ current_location: "Zuhause", excused: true }}
      />,
    );
    expect(screen.getByText("Entschuldigt")).toBeInTheDocument();
  });

  it("collapses detailed-mode 'Anwesend - Room 101' to plain Anwesend", () => {
    // Defense in depth: PresenceBadge should never leak room names, even if
    // it's accidentally rendered on a student with detailed-mode data.
    render(
      <PresenceBadge student={{ current_location: "Anwesend - Room 101" }} />,
    );
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    expect(screen.queryByText(/Room 101/)).not.toBeInTheDocument();
  });

  it("honors showLocationSince when location_since is provided", () => {
    render(
      <PresenceBadge
        student={{
          current_location: "Anwesend",
          location_since: "2026-04-21T14:30:00Z",
        }}
        showLocationSince
      />,
    );
    // The exact time depends on the test runner's timezone — just assert
    // that the "seit" prefix is present.
    expect(screen.getByText(/^seit /)).toBeInTheDocument();
  });

  it("renders the simple variant without a pulse dot", () => {
    const { container } = render(
      <PresenceBadge
        student={{ current_location: "Anwesend" }}
        variant="simple"
      />,
    );
    // Simple variant has no dot/glow — just the label pill.
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    // eslint-disable-next-line testing-library/no-node-access
    expect(container.querySelector(".animate-pulse")).toBeNull();
  });

  it("renders the modern indicator without animation or glow", () => {
    const { container } = render(
      <PresenceBadge student={{ current_location: "Anwesend" }} />,
    );

    expect(container.querySelector(".animate-pulse")).toBeNull();
    expect(getPill().style.boxShadow).toBe("");
  });

  it("renders the small size with the sm class shape", () => {
    const { container } = render(
      <PresenceBadge student={{ current_location: "Anwesend" }} size="sm" />,
    );
    // eslint-disable-next-line testing-library/no-node-access
    const pill = container.querySelector("[data-presence-state]");
    expect(pill?.className).toMatch(/text-\[11px\]/);
  });

  it("renders the large size with the lg class shape", () => {
    const { container } = render(
      <PresenceBadge student={{ current_location: "Anwesend" }} size="lg" />,
    );
    // eslint-disable-next-line testing-library/no-node-access
    const pill = container.querySelector("[data-presence-state]");
    expect(pill?.className).toMatch(/text-sm/);
  });

  it("prefers sick-since timestamp over location_since when both are set", () => {
    render(
      <PresenceBadge
        student={{
          current_location: "Zuhause",
          sick: true,
          sick_since: "2026-04-21T10:00:00Z",
          location_since: "2026-04-21T06:00:00Z",
        }}
        showLocationSince
      />,
    );
    // Replace mode: "Krank" label, and the "seit" line is derived from
    // sick_since (not the earlier location_since).
    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.getByText(/^seit /)).toBeInTheDocument();
  });

  it("prefers excused-since timestamp over location_since when both are set", () => {
    render(
      <PresenceBadge
        student={{
          current_location: "Zuhause",
          excused: true,
          excused_since: "2026-04-21T11:00:00Z",
          location_since: "2026-04-21T06:00:00Z",
        }}
        showLocationSince
      />,
    );
    expect(screen.getByText("Entschuldigt")).toBeInTheDocument();
    expect(screen.getByText(/^seit /)).toBeInTheDocument();
  });

  it("renders an Entschuldigt overlay when excused student is present", () => {
    render(
      <PresenceBadge
        student={{ current_location: "Anwesend", excused: true }}
      />,
    );
    expect(screen.getByText("Anwesend")).toBeInTheDocument();
    expect(screen.getByText("Entschuldigt")).toBeInTheDocument();
    // eslint-disable-next-line testing-library/no-node-access
    expect(document.querySelector("[data-excused-indicator]")).not.toBeNull();
  });

  it("prioritises sick over excused when both flags are set (defensive)", () => {
    // Backend enforces mutual exclusion, but if a race sets both the badge
    // must still render a single replace label — sick wins, excused is
    // suppressed.
    render(
      <PresenceBadge
        student={{ current_location: "Zuhause", sick: true, excused: true }}
      />,
    );
    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.queryByText("Entschuldigt")).not.toBeInTheDocument();
  });
});

/**
 * Normalize a #RRGGBB hex string to the `rgb(r,g,b)` form jsdom serialises
 * into inline styles, collapsed to no-whitespace for easier comparison.
 */
function hexToRgbCompact(hex: string): string {
  const clean = hex.replace("#", "");
  const r = parseInt(clean.slice(0, 2), 16);
  const g = parseInt(clean.slice(2, 4), 16);
  const b = parseInt(clean.slice(4, 6), 16);
  return `rgb(${r},${g},${b})`;
}
