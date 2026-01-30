/**
 * Tests for BadgeDisplay Components
 * Tests rendering of badge components with counts, labels, and icons
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { BadgeDisplay, BadgeDisplayCompact } from "./BadgeDisplay";

describe("BadgeDisplay", () => {
  const mockIcon = <svg data-testid="test-icon" />;

  it("renders count", () => {
    render(<BadgeDisplay count={42} />);
    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("renders string count", () => {
    render(<BadgeDisplay count="15/20" />);
    expect(screen.getByText("15/20")).toBeInTheDocument();
  });

  it("renders label when showLabel is true", () => {
    render(<BadgeDisplay count={10} label="Schüler" showLabel={true} />);
    expect(screen.getByText("Schüler")).toBeInTheDocument();
  });

  it("does not render label when showLabel is false", () => {
    render(<BadgeDisplay count={10} label="Schüler" showLabel={false} />);
    expect(screen.queryByText("Schüler")).not.toBeInTheDocument();
  });

  it("renders icon when provided", () => {
    render(<BadgeDisplay count={5} icon={mockIcon} />);
    expect(screen.getByTestId("test-icon")).toBeInTheDocument();
  });

  it("does not render icon when not provided", () => {
    render(<BadgeDisplay count={5} />);
    expect(screen.queryByTestId("test-icon")).not.toBeInTheDocument();
  });

  it("hides label on mobile by default", () => {
    const { container } = render(
      <BadgeDisplay count={10} label="Schüler" showLabel={true} />,
    );
    const label = container.querySelector(".hidden.md\\:inline");
    expect(label).toBeInTheDocument();
  });

  it("applies small size classes", () => {
    const { container } = render(<BadgeDisplay count={5} size="sm" />);
    expect(container.firstChild).toHaveClass("px-2", "py-1.5", "gap-1.5");
  });

  it("applies medium size classes by default", () => {
    const { container } = render(<BadgeDisplay count={5} />);
    expect(container.firstChild).toHaveClass("px-3", "py-1.5", "gap-2");
  });
});

describe("BadgeDisplayCompact", () => {
  const mockIcon = <svg data-testid="compact-icon" />;

  it("renders count", () => {
    render(<BadgeDisplayCompact count={25} />);
    expect(screen.getByText("25")).toBeInTheDocument();
  });

  it("renders icon when provided", () => {
    render(<BadgeDisplayCompact count={8} icon={mockIcon} />);
    expect(screen.getByTestId("compact-icon")).toBeInTheDocument();
  });

  it("does not render label (compact version)", () => {
    render(<BadgeDisplayCompact count={10} />);
    // Compact version should only show count, no label
    const container = document.body;
    expect(container.textContent).toBe("10");
  });

  it("applies compact size classes", () => {
    const { container } = render(<BadgeDisplayCompact count={3} />);
    expect(container.firstChild).toHaveClass("px-2", "py-1.5", "gap-1.5");
  });
});
