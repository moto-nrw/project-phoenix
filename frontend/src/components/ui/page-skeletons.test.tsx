import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MobilePageHelpTargetContext } from "./mobile-page-help-target";
import {
  ChoiceTileListSkeleton,
  HistorySkeleton,
  SkeletonRegion,
  TenantPageHeaderSkeleton,
} from "./page-skeletons";

describe("SkeletonRegion", () => {
  it("shows its loading shape immediately without an invisible transition", () => {
    render(
      <SkeletonRegion label="Daten werden geladen">
        <div data-testid="shape" />
      </SkeletonRegion>,
    );

    expect(screen.getByTestId("shape").parentElement).not.toHaveClass(
      "moto-skeleton-defer",
    );
    expect(
      screen.getByRole("status", { name: "Daten werden geladen" }),
    ).toHaveAttribute("aria-busy", "true");
  });
});

// Steht die Portalhülle, meldet sich der Platz für die Seitenhilfe hier an.
const helpRegistration = {
  registerTarget: vi.fn(),
  unregisterTarget: vi.fn(),
};

describe("TenantPageHeaderSkeleton", () => {
  it("drops the status line row for a page without stats", () => {
    const { container: withStats } = render(<TenantPageHeaderSkeleton />);
    const { container: withoutStats } = render(
      <TenantPageHeaderSkeleton stats={false} />,
    );

    expect(withStats.querySelectorAll(".animate-pulse")).toHaveLength(2);
    expect(withoutStats.querySelectorAll(".animate-pulse")).toHaveLength(1);
  });

  it("reserves the info button next to the title when requested", () => {
    const { container } = render(
      <TenantPageHeaderSkeleton stats={false} info />,
    );

    expect(container.querySelectorAll(".animate-pulse")).toHaveLength(2);
    expect(container.querySelector(".h-8.w-8")).toBeInTheDocument();
  });

  // Unter `lg` steht die Hilfe als 44-px-Knopf im Kopf von `TenantPage`. Ohne
  // denselben Platz im Ladekopf wächst der Kopf beim Laden um diesen Knopf
  // (#6568). Das Elternportal behält seinen bisherigen Ladekopf (#6613).
  it("offers the page help its place in the head, as TenantPage does", () => {
    const { registerTarget } = helpRegistration;
    const { rerender } = render(
      <MobilePageHelpTargetContext.Provider value={helpRegistration}>
        <TenantPageHeaderSkeleton matches="parent-page" />
      </MobilePageHelpTargetContext.Provider>,
    );
    expect(screen.getByRole("banner")).toBeInTheDocument();
    expect(registerTarget).not.toHaveBeenCalled();

    rerender(
      <MobilePageHelpTargetContext.Provider value={helpRegistration}>
        <TenantPageHeaderSkeleton stats={false} info />
      </MobilePageHelpTargetContext.Provider>,
    );
    expect(registerTarget).toHaveBeenCalledOnce();
    expect(screen.getByRole("banner")).toContainElement(
      registerTarget.mock.calls[0]?.[0] as HTMLElement,
    );
  });
});

describe("HistorySkeleton", () => {
  it("mirrors a grouped history with separate detail cards", () => {
    const { container } = render(<HistorySkeleton />);

    expect(
      container.querySelectorAll(".moto-content-surface.rounded-xl"),
    ).toHaveLength(2);
    expect(container.querySelectorAll(".border-t")).toHaveLength(2);
  });
});

describe("ChoiceTileListSkeleton", () => {
  it("mirrors separate choice rows with an action slot", () => {
    const { container } = render(<ChoiceTileListSkeleton rows={2} avatar />);

    expect(container.querySelectorAll(".rounded-xl.border")).toHaveLength(2);
    expect(container.querySelectorAll(".h-8.w-16")).toHaveLength(2);
    expect(container.querySelectorAll(".h-8.w-8.rounded-full")).toHaveLength(2);
    expect(container.querySelector(".divide-y")).not.toBeInTheDocument();
  });
});
