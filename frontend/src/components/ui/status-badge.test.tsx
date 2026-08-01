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
    expect((pill as HTMLElement).style.color).toBe("#9F1F1E");
  });

  it("renders a decorative dot hidden from assistive tech", () => {
    const { container } = render(
      <StatusBadge label="Warteliste" tone="orange" />,
    );
    const dot = container.querySelector("span[aria-hidden='true']");
    expect(dot).not.toBeNull();
  });
});
