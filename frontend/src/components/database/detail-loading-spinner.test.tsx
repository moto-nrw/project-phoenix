import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { DetailLoadingSpinner } from "./detail-loading-spinner";

describe("DetailLoadingSpinner", () => {
  it("renders the localized label as a status so the panel never goes blank during fetches", () => {
    render(<DetailLoadingSpinner label="Rollendaten werden geladen..." />);
    expect(screen.getByRole("status")).toHaveTextContent(
      "Rollendaten werden geladen...",
    );
  });

  it("uses the brand-blue accent ring on the spinner element", () => {
    const { container } = render(<DetailLoadingSpinner label="…" />);
    // The spinner is the only animate-spin descendant; it MUST keep the blue
    // LOCATION_COLORS.OTHER_ROOM accent so the indicator stays distinguishable
    // from the surrounding gray ring.
    const spinner = container.querySelector(".animate-spin");
    expect(spinner).not.toBeNull();
    expect(spinner).toHaveStyle({
      borderTopColor: LOCATION_COLORS.OTHER_ROOM,
    });
  });
});
