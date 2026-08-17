import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { PageHeaderSkeleton } from "./page-header/PageHeaderSkeleton";
import { SkeletonRegion } from "./page-skeletons";

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
});
