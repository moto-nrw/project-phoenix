import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PageHeaderSkeleton } from "./page-header/PageHeaderSkeleton";
import { SkeletonRegion, TenantPageHeaderSkeleton } from "./page-skeletons";

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

describe("PageHeaderSkeleton", () => {
  it("keeps the mobile-only title separate from search and actions", () => {
    const { container } = render(<PageHeaderSkeleton actions={1} />);
    const [title, mobileAction, search, desktopAction] =
      container.querySelectorAll(".animate-pulse");

    expect(title?.parentElement).toHaveClass("md:hidden");
    expect(title?.parentElement).toBe(
      mobileAction?.parentElement?.parentElement,
    );
    expect(desktopAction?.parentElement).toHaveClass("hidden", "lg:flex");
    expect(search?.parentElement).toBe(
      desktopAction?.parentElement?.parentElement,
    );
  });

  it("mirrors PageHeader exactly when only a title is shown", () => {
    const { container } = render(<PageHeaderSkeleton search={false} />);
    const wrapper = container.firstElementChild;
    const title = container.querySelector(".animate-pulse");

    // PageHeader renders a mobile-only mb-6 wrapper around a 32px
    // text-2xl title row — no empty search row, no gap.
    expect(wrapper).toHaveClass("mb-6", "md:hidden");
    expect(wrapper?.childElementCount).toBe(1);
    expect(title).toHaveClass("h-8");
  });
});
