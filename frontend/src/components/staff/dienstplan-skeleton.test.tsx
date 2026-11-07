import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RESOURCE_GRID_METRICS } from "~/components/ui/planning/resource-grid";

import { DienstplanGridSkeleton } from "./dienstplan-skeleton";

describe("DienstplanGridSkeleton", () => {
  it("applies the shared minimum height and full width to each day cell", () => {
    const { container } = render(<DienstplanGridSkeleton />);

    const cells = container.querySelectorAll("tbody td > div");
    expect(cells).toHaveLength(30);
    for (const cell of cells) {
      expect(cell).toHaveClass(RESOURCE_GRID_METRICS.cellMinHeightClass.days);
      expect(cell).toHaveClass("w-full");
    }
  });
});
