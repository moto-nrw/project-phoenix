import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Skeleton, SkeletonRows } from "./skeleton";

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

  it.each([
    ["round", "rounded-full"],
    ["control", "rounded-lg"],
    ["inner", "rounded-xl"],
    ["card", "rounded-2xl"],
    ["none", "rounded-none"],
  ] as const)("takes its radius from shape %s", (shape, radius) => {
    render(<Skeleton data-testid="skeleton" shape={shape} />);

    expect(screen.getByTestId("skeleton")).toHaveClass(radius);
  });

  it("renders the requested number of rows, hidden from assistive tech", () => {
    const { container } = render(<SkeletonRows rows={2} />);

    const list = container.firstElementChild;
    expect(list).toHaveAttribute("aria-hidden", "true");
    expect(list?.children).toHaveLength(2);
  });
});
