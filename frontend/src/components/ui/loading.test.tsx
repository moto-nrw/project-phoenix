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

  it("renders a visible status message", () => {
    render(<Loading message="Please wait" />);

    expect(screen.getByText("Please wait")).toBeInTheDocument();
    expect(screen.getByText("Please wait")).not.toHaveClass("sr-only");
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
    expect(output.className).toContain("min-h-40");
  });

  it("uses a neutral progress indicator when the future layout is unknown", () => {
    const { container } = render(<Loading />);

    expect(container.querySelector(".animate-pulse")).not.toBeInTheDocument();
    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("has proper aria-live attribute for accessibility", () => {
    render(<Loading />);

    const output = screen.getByLabelText("Lädt...");
    expect(output).toHaveAttribute("aria-live", "polite");
    expect(output).toHaveAttribute("aria-busy", "true");
  });
});
