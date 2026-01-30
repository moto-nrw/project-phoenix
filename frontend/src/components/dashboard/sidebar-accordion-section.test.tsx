import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { SidebarAccordionSection } from "./sidebar-accordion-section";

const defaultProps = {
  icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3",
  label: "Test Section",
  isExpanded: false,
  onToggle: vi.fn(),
  isActive: false,
  hasChildren: false,
};

describe("SidebarAccordionSection", () => {
  it("renders the label", () => {
    render(<SidebarAccordionSection {...defaultProps} />);
    expect(screen.getByText("Test Section")).toBeInTheDocument();
  });

  it("calls onToggle when clicked", () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    fireEvent.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("calls onToggle on Enter key", () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    fireEvent.keyDown(screen.getByRole("button"), { key: "Enter" });
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("calls onToggle on Space key", () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    fireEvent.keyDown(screen.getByRole("button"), { key: " " });
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("does not call onToggle on other keys", () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    fireEvent.keyDown(screen.getByRole("button"), { key: "Tab" });
    expect(onToggle).not.toHaveBeenCalled();
  });

  it("applies active styling when isActive is true", () => {
    render(<SidebarAccordionSection {...defaultProps} isActive={true} />);
    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-gray-100");
    expect(button.className).toContain("font-semibold");
  });

  it("applies inactive styling when isActive is false", () => {
    render(<SidebarAccordionSection {...defaultProps} isActive={false} />);
    const button = screen.getByRole("button");
    expect(button.className).toContain("text-gray-600");
    expect(button.className).toContain("font-medium");
  });

  it("sets aria-expanded based on isExpanded prop", () => {
    const { rerender } = render(
      <SidebarAccordionSection {...defaultProps} isExpanded={false} />,
    );
    expect(screen.getByRole("button")).toHaveAttribute(
      "aria-expanded",
      "false",
    );

    rerender(<SidebarAccordionSection {...defaultProps} isExpanded={true} />);
    expect(screen.getByRole("button")).toHaveAttribute("aria-expanded", "true");
  });

  it("shows loading skeleton when isLoading is true and expanded", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isExpanded={true}
        isLoading={true}
        hasChildren={false}
      />,
    );
    // Skeleton shimmer divs have animate-pulse class
    const pulseElements = document.querySelectorAll(".animate-pulse");
    expect(pulseElements.length).toBe(3);
  });

  it("shows empty text when hasChildren is false and not loading", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isExpanded={true}
        isLoading={false}
        hasChildren={false}
      />,
    );
    expect(screen.getByText("Keine Einträge")).toBeInTheDocument();
  });

  it("shows custom empty text", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isExpanded={true}
        isLoading={false}
        hasChildren={false}
        emptyText="Nichts gefunden"
      />,
    );
    expect(screen.getByText("Nichts gefunden")).toBeInTheDocument();
  });

  it("renders children when hasChildren is true and not loading", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isExpanded={true}
        isLoading={false}
        hasChildren={true}
      >
        <div data-testid="child-content">Child Content</div>
      </SidebarAccordionSection>,
    );
    expect(screen.getByTestId("child-content")).toBeInTheDocument();
  });

  it("applies activeColor when isActive is true", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isActive={true}
        activeColor="text-blue-500"
      />,
    );
    const svg = document.querySelector("svg");
    expect(svg?.getAttribute("class")).toContain("text-blue-500");
  });

  it("applies activeColor when isIconActive is true (overrides isActive)", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isActive={false}
        isIconActive={true}
        activeColor="text-green-500"
      />,
    );
    const svg = document.querySelector("svg");
    expect(svg?.getAttribute("class")).toContain("text-green-500");
  });

  it("does not apply activeColor when neither isActive nor isIconActive", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isActive={false}
        isIconActive={false}
        activeColor="text-red-500"
      />,
    );
    const svg = document.querySelector("svg");
    const classAttr = svg?.getAttribute("class") ?? "";
    expect(classAttr).not.toContain("text-red-500");
  });

  it("does not apply activeColor when activeColor is undefined", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isActive={true}
        activeColor={undefined}
      />,
    );
    // Should not crash and icon should not have any color class beyond base
    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});
