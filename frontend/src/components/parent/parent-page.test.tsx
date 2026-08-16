import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { ParentPageSkeleton } from "./parent-page";

describe("ParentPageSkeleton", () => {
  it("reserves the header and section internals instead of using solid blocks", () => {
    render(<ParentPageSkeleton rows={2} />);

    expect(screen.getByTestId("parent-page-header-skeleton")).toHaveClass(
      "moto-content-surface",
    );
    expect(screen.getAllByTestId("parent-page-section-skeleton")).toHaveLength(
      2,
    );
    expect(
      screen.getAllByTestId("parent-page-section-row-skeleton"),
    ).toHaveLength(6);
  });
});
