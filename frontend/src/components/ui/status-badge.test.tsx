import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusBadge } from "./status-badge";

describe("StatusBadge", () => {
  it("renders the label", () => {
    render(<StatusBadge label="Bestätigt" tone="green" />);
    expect(screen.getByText("Bestätigt")).toBeInTheDocument();
  });

  it("applies the tone colors via inline styles", () => {
    render(<StatusBadge label="Abgelehnt" tone="red" />);
    const label = screen.getByText("Abgelehnt");
    const pill = label.closest("span[class*='rounded-full']");
    expect(pill).not.toBeNull();
    expect((pill as HTMLElement).style.color).toBe("#B91C1C");
  });

  it("renders the label as the pill's only content, without a decorative dot (#2476)", () => {
    render(<StatusBadge label="Warteliste" tone="orange" />);
    const pill = screen.getByText("Warteliste");
    expect(pill.children).toHaveLength(0);
    expect(pill.textContent).toBe("Warteliste");
    // No spacer left behind where the dot used to sit.
    expect(pill.className).not.toMatch(/\bgap-/);
  });

  it("passes the title through to the pill element", () => {
    render(
      <StatusBadge label="Feiertag" tone="gray" title="Tag der Einheit" />,
    );
    expect(screen.getByText("Feiertag")).toHaveAttribute(
      "title",
      "Tag der Einheit",
    );
  });
});
