/**
 * Tests for PageHeader Component
 * Tests rendering and functionality of mobile page header
 */
import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { PageHeader } from "./PageHeader";

describe("PageHeader", () => {
  const mockIcon = <svg data-testid="mock-icon" />;
  const mockActionButton = <button data-testid="action-button">Action</button>;

  it("renders title", () => {
    render(<PageHeader title="Test Title" />);
    expect(screen.getByText("Test Title")).toBeInTheDocument();
  });

  it("returns null when no title provided", () => {
    const { container } = render(<PageHeader title="" />);
    expect(container.firstChild).toBeNull();
  });

  it("renders badge with count", () => {
    render(<PageHeader title="Test" badge={{ count: 42, label: "Items" }} />);

    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("renders badge label", () => {
    render(<PageHeader title="Test" badge={{ count: 10, label: "Schüler" }} />);

    expect(screen.getByText("Schüler")).toBeInTheDocument();
  });

  it("renders badge icon", () => {
    render(<PageHeader title="Test" badge={{ count: 5, icon: mockIcon }} />);

    expect(screen.getByTestId("mock-icon")).toBeInTheDocument();
  });

  it("renders status indicator with green color", () => {
    const { container } = render(
      <PageHeader
        title="Test"
        statusIndicator={{ color: "green", tooltip: "Active" }}
      />,
    );

    const indicator = container.querySelector(".bg-green-500");
    expect(indicator).toBeInTheDocument();
    expect(indicator).toHaveClass("animate-pulse");
  });

  it("renders status indicator with yellow color", () => {
    const { container } = render(
      <PageHeader title="Test" statusIndicator={{ color: "yellow" }} />,
    );

    expect(container.querySelector(".bg-yellow-500")).toBeInTheDocument();
  });

  it("renders status indicator with red color", () => {
    const { container } = render(
      <PageHeader title="Test" statusIndicator={{ color: "red" }} />,
    );

    expect(container.querySelector(".bg-red-500")).toBeInTheDocument();
  });

  it("renders status indicator with gray color", () => {
    const { container } = render(
      <PageHeader title="Test" statusIndicator={{ color: "gray" }} />,
    );

    expect(container.querySelector(".bg-gray-400")).toBeInTheDocument();
  });

  it("applies tooltip to status indicator", () => {
    const { container } = render(
      <PageHeader
        title="Test"
        statusIndicator={{ color: "green", tooltip: "System Active" }}
      />,
    );

    const indicator = container.querySelector(".bg-green-500");
    expect(indicator).toHaveAttribute("title", "System Active");
  });

  it("renders action button when provided", () => {
    render(<PageHeader title="Test" actionButton={mockActionButton} />);

    expect(screen.getByTestId("action-button")).toBeInTheDocument();
  });

  it("prioritizes action button over badge and status", () => {
    render(
      <PageHeader
        title="Test"
        actionButton={mockActionButton}
        badge={{ count: 10 }}
        statusIndicator={{ color: "green" }}
      />,
    );

    expect(screen.getByTestId("action-button")).toBeInTheDocument();
    expect(screen.queryByText("10")).not.toBeInTheDocument();
  });

  it("shows badge and status when no action button", () => {
    const { container } = render(
      <PageHeader
        title="Test"
        badge={{ count: 10 }}
        statusIndicator={{ color: "green" }}
      />,
    );

    expect(screen.getByText("10")).toBeInTheDocument();
    expect(container.querySelector(".bg-green-500")).toBeInTheDocument();
  });

  it("applies custom className", () => {
    const { container } = render(
      <PageHeader title="Test" className="custom-class" />,
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("is hidden on desktop (md:hidden)", () => {
    const { container } = render(<PageHeader title="Test" />);
    expect(container.firstChild).toHaveClass("md:hidden");
  });
});
