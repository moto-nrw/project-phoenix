import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Loading } from "./loading";

describe("Loading", () => {
  it("renders with default message", () => {
    render(<Loading />);

    expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
  });

  it("renders with custom message", () => {
    render(<Loading message="Loading data..." />);

    expect(screen.getByLabelText("Loading data...")).toBeInTheDocument();
  });

  it("renders screen reader only message", () => {
    render(<Loading message="Please wait" />);

    expect(screen.getByText("Please wait")).toBeInTheDocument();
    expect(screen.getByText("Please wait")).toHaveClass("sr-only");
  });

  it("renders with fullPage styles by default", () => {
    render(<Loading />);

    const output = screen.getByLabelText("Lädt...");
    expect(output.className).toContain("fixed");
    expect(output.className).toContain("inset-0");
    expect(output.className).toContain("z-50");
  });

  it("renders without fullPage styles when fullPage is false", () => {
    render(<Loading fullPage={false} />);

    const output = screen.getByLabelText("Lädt...");
    expect(output.className).not.toContain("fixed");
    expect(output.className).toContain("flex");
    expect(output.className).toContain("pt-24");
  });

  it("renders skeleton elements", () => {
    const { container } = render(<Loading />);

    // Check that skeleton elements are rendered
    const skeletons = container.querySelectorAll(".animate-pulse");
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it("has proper aria-live attribute for accessibility", () => {
    render(<Loading />);

    const output = screen.getByLabelText("Lädt...");
    expect(output).toHaveAttribute("aria-live", "polite");
  });
});
