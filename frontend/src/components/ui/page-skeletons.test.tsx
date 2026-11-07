import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

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
