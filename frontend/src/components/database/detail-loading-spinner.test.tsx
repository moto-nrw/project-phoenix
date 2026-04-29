import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DetailLoadingSpinner } from "./detail-loading-spinner";

describe("DetailLoadingSpinner", () => {
  it("renders the localized label so the panel never goes blank during fetches", () => {
    render(<DetailLoadingSpinner label="Rollendaten werden geladen..." />);
    expect(
      screen.getByText("Rollendaten werden geladen..."),
    ).toBeInTheDocument();
  });

  it("uses the brand-blue accent ring on the spinner element", () => {
    const { container } = render(<DetailLoadingSpinner label="…" />);
    // The spinner is the only animate-spin descendant; it MUST keep the
    // brand-blue top border so the indicator stays distinguishable from the
    // surrounding gray ring (#5080D8 from LOCATION_COLORS.OTHER_ROOM).
    const spinner = container.querySelector(".animate-spin");
    expect(spinner).not.toBeNull();
    expect(spinner?.className).toContain("border-t-[#5080D8]");
  });
});
