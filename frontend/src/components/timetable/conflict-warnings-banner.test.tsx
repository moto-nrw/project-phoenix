import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ConflictWarningsBanner } from "./conflict-warnings-banner";

describe("ConflictWarningsBanner", () => {
  it("renders nothing when there are no conflicts", () => {
    const { container } = render(<ConflictWarningsBanner conflictCount={0} />);

    expect(container).toBeEmptyDOMElement();
  });

  it("uses singular and plural labels", () => {
    const { rerender } = render(<ConflictWarningsBanner conflictCount={1} />);

    expect(screen.getByRole("status")).toHaveTextContent("1 Konflikt");

    rerender(<ConflictWarningsBanner conflictCount={3} />);
    expect(screen.getByRole("status")).toHaveTextContent("3 Konflikte");
  });
});
