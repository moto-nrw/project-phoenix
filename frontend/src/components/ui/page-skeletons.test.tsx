import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

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
