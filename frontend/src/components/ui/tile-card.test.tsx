import { render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes } from "react";
import { describe, expect, it, vi } from "vitest";

import { TileCard } from "./tile-card";

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({
    children,
    ...props
  }: AnchorHTMLAttributes<HTMLAnchorElement>) => (
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
      <TileCard href="/anmeldung/1" nativeAnchor>
        Schuljahr
      </TileCard>,
    );

    const link = screen.getByRole("link", { name: "Schuljahr" });
    expect(link).toHaveAttribute("href", "/anmeldung/1");
    expect(link).not.toHaveAttribute("data-client-navigation");
    expect(link).toHaveClass("moto-content-surface", "rounded-2xl");
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

describe("TileCard newTab (#5594)", () => {
  it("öffnet das Ziel in einem neuen Tab und sagt es dem Screenreader", () => {
    render(
      <TileCard href="/anmeldung/preview?schemaId=7" newTab>
        Vorschau öffnen
      </TileCard>,
    );

    const link = screen.getByRole("link", {
      name: "Vorschau öffnen (öffnet in neuem Tab)",
    });
    expect(link).toHaveAttribute("href", "/anmeldung/preview?schemaId=7");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(link).not.toHaveAttribute("data-client-navigation");
    expect(link).toHaveClass("moto-content-surface", "rounded-2xl");
  });

  it("hängt den Hinweis auch an ein eigenes ariaLabel", () => {
    render(
      <TileCard href="/vorschau" newTab ariaLabel="Vorschau öffnen">
        <span aria-hidden="true">↗</span>
      </TileCard>,
    );

    expect(
      screen.getByRole("link", {
        name: "Vorschau öffnen (öffnet in neuem Tab)",
      }),
    ).toHaveAttribute("target", "_blank");
  });

  it("bleibt deaktiviert eine Schaltfläche ohne neuen Tab", () => {
    render(
      <TileCard href="/vorschau" newTab disabled>
        Vorschau öffnen
      </TileCard>,
    );

    expect(
      screen.getByRole("button", { name: "Vorschau öffnen" }),
    ).toBeDisabled();
    expect(screen.queryByRole("link")).toBeNull();
  });
});
