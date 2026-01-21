import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Skeleton } from "./skeleton";

describe("Skeleton", () => {
  it("renders a div element", () => {
    render(<Skeleton data-testid="skeleton" />);

    expect(screen.getByTestId("skeleton")).toBeInTheDocument();
    expect(screen.getByTestId("skeleton").tagName).toBe("DIV");
  });

  it("applies animate-pulse class", () => {
    render(<Skeleton data-testid="skeleton" />);

    expect(screen.getByTestId("skeleton")).toHaveClass("animate-pulse");
  });

  it("applies bg-gray-200 class", () => {
    render(<Skeleton data-testid="skeleton" />);

    expect(screen.getByTestId("skeleton")).toHaveClass("bg-gray-200");
  });

  it("applies rounded-md class by default", () => {
    render(<Skeleton data-testid="skeleton" />);

    expect(screen.getByTestId("skeleton")).toHaveClass("rounded-md");
  });

  it("merges custom className", () => {
    render(<Skeleton data-testid="skeleton" className="h-4 w-full" />);

    const skeleton = screen.getByTestId("skeleton");
    expect(skeleton).toHaveClass("animate-pulse");
    expect(skeleton).toHaveClass("h-4");
    expect(skeleton).toHaveClass("w-full");
  });

  it("passes additional props", () => {
    render(<Skeleton data-testid="skeleton" aria-label="Loading content" />);

    expect(screen.getByTestId("skeleton")).toHaveAttribute(
      "aria-label",
      "Loading content",
    );
  });

  it("can override rounded class with custom className", () => {
    render(<Skeleton data-testid="skeleton" className="rounded-full" />);

    const skeleton = screen.getByTestId("skeleton");
    expect(skeleton).toHaveClass("rounded-full");
  });
});
