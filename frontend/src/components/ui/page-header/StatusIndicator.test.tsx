/**
 * Tests for StatusIndicator Component
 * Tests rendering of status indicator dot with colors
 */
import { render } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { StatusIndicator } from "./StatusIndicator";

describe("StatusIndicator", () => {
  it("renders with green color and pulse animation", () => {
    const { container } = render(<StatusIndicator color="green" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("bg-green-500", "animate-pulse");
  });

  it("renders with yellow color", () => {
    const { container } = render(<StatusIndicator color="yellow" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("bg-yellow-500");
    expect(indicator).not.toHaveClass("animate-pulse");
  });

  it("renders with red color", () => {
    const { container } = render(<StatusIndicator color="red" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("bg-red-500");
    expect(indicator).not.toHaveClass("animate-pulse");
  });

  it("renders with gray color", () => {
    const { container } = render(<StatusIndicator color="gray" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("bg-gray-400");
    expect(indicator).not.toHaveClass("animate-pulse");
  });

  it("applies tooltip when provided", () => {
    const { container } = render(
      <StatusIndicator color="green" tooltip="System is active" />,
    );
    const indicator = container.firstChild;

    expect(indicator).toHaveAttribute("title", "System is active");
  });

  it("does not have tooltip when not provided", () => {
    const { container } = render(<StatusIndicator color="green" />);
    const indicator = container.firstChild;

    expect(indicator).not.toHaveAttribute("title");
  });

  it("applies small size by default", () => {
    const { container } = render(<StatusIndicator color="green" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("h-2.5", "w-2.5");
  });

  it("applies small size when explicitly set", () => {
    const { container } = render(<StatusIndicator color="green" size="sm" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("h-2.5", "w-2.5");
  });

  it("applies medium size when specified", () => {
    const { container } = render(<StatusIndicator color="green" size="md" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("h-3", "w-3");
  });

  it("has rounded-full class", () => {
    const { container } = render(<StatusIndicator color="green" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("rounded-full");
  });

  it("has flex-shrink-0 class", () => {
    const { container } = render(<StatusIndicator color="green" />);
    const indicator = container.firstChild;

    expect(indicator).toHaveClass("flex-shrink-0");
  });
});
