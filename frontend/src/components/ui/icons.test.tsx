import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { CheckIcon, SpinnerIcon } from "./icons";

describe("CheckIcon", () => {
  it("renders with default className", () => {
    const { container } = render(<CheckIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toHaveClass("h-5", "w-5");
  });

  it("renders with custom className", () => {
    const { container } = render(
      <CheckIcon className="h-4 w-4 text-green-500" />,
    );

    const svg = container.querySelector("svg");
    expect(svg).toHaveClass("h-4", "w-4", "text-green-500");
  });

  it("renders SVG path", () => {
    const { container } = render(<CheckIcon />);

    const path = container.querySelector("path");
    expect(path).toBeInTheDocument();
  });
});

describe("SpinnerIcon", () => {
  it("renders with default className and animate-spin", () => {
    const { container } = render(<SpinnerIcon />);

    const svg = container.querySelector("svg");
    expect(svg).toHaveClass("animate-spin", "h-4", "w-4");
  });

  it("renders with custom className and animate-spin", () => {
    const { container } = render(
      <SpinnerIcon className="h-8 w-8 text-blue-500" />,
    );

    const svg = container.querySelector("svg");
    expect(svg).toHaveClass("animate-spin", "h-8", "w-8", "text-blue-500");
  });

  it("renders circle and path elements", () => {
    const { container } = render(<SpinnerIcon />);

    const circle = container.querySelector("circle");
    const path = container.querySelector("path");

    expect(circle).toBeInTheDocument();
    expect(path).toBeInTheDocument();
  });
});
