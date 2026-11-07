import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TileCard } from "./tile-card";

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a data-client-navigation="true" {...props}>
      {children}
    </a>
  ),
}));

describe("TileCard nativeAnchor (#5594)", () => {
  it("navigiert ohne nativeAnchor über die Client-Navigation", () => {
    render(<TileCard href="/anmeldung/1">Schuljahr</TileCard>);

    const link = screen.getByRole("link", { name: "Schuljahr" });
    expect(link).toHaveAttribute("href", "/anmeldung/1");
    expect(link).toHaveAttribute("data-client-navigation", "true");
  });

  it("rendert mit nativeAnchor ein schlichtes <a> für einen Dokumentwechsel", () => {
    render(
      <TileCard href="/anmeldung/1" nativeAnchor className="rounded-xl">
        Schuljahr
      </TileCard>,
    );

    const link = screen.getByRole("link", { name: "Schuljahr" });
    expect(link).toHaveAttribute("href", "/anmeldung/1");
    expect(link).not.toHaveAttribute("data-client-navigation");
    expect(link).toHaveClass("moto-content-surface", "rounded-xl");
    expect(link).not.toHaveClass("rounded-2xl");
  });

  it("bleibt ohne Ziel bei deaktivierter Kachel eine Schaltfläche", () => {
    render(
      <TileCard href="/anmeldung/1" nativeAnchor disabled>
        Schuljahr
      </TileCard>,
    );

    expect(screen.getByRole("button", { name: "Schuljahr" })).toBeDisabled();
  });
});
