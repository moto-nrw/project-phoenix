import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { groupsConcept } from "~/lib/concepts/groups";
import { SidebarAccordionSection } from "./sidebar-accordion-section";

const defaultProps = {
  concept: groupsConcept,
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

  // Die Kopfzeile ist ein nativer Knopf: Enter und Leertaste lösen ihn ohne
  // eigenen Tasten-Handler genau einmal aus (#6580).
  it("calls onToggle on Enter key", async () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    screen.getByRole("button").focus();
    await userEvent.keyboard("{Enter}");
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("calls onToggle on Space key", async () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    screen.getByRole("button").focus();
    await userEvent.keyboard(" ");
    expect(onToggle).toHaveBeenCalledOnce();
  });

  it("does not call onToggle on other keys", async () => {
    const onToggle = vi.fn();
    render(<SidebarAccordionSection {...defaultProps} onToggle={onToggle} />);
    screen.getByRole("button").focus();
    await userEvent.keyboard("{Tab}");
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

  // Das Icon zeigt den Aktivzustand farbig im Ton seines Konzepts; vorher
  // hing das an `activeColor` eines Pfad-Icons, das kein Aufrufer mehr
  // nutzte (#6580).
  function iconTone(): string | null {
    // Anker: das Icon des Bereichs ist gerendert.
    const icon = document.querySelector("[data-nav-scene]");
    expect(icon).toBeInTheDocument();
    return icon?.getAttribute("data-moto-duotone-tone") ?? null;
  }

  it("colors the icon when isActive is true", () => {
    render(<SidebarAccordionSection {...defaultProps} isActive={true} />);
    expect(iconTone()).toBe(groupsConcept.tone);
  });

  it("colors the icon when isIconActive is true (overrides isActive)", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isActive={false}
        isIconActive={true}
      />,
    );
    expect(iconTone()).toBe(groupsConcept.tone);
  });

  it("does not color the icon when neither isActive nor isIconActive", () => {
    render(
      <SidebarAccordionSection
        {...defaultProps}
        isActive={true}
        isIconActive={false}
      />,
    );
    expect(iconTone()).toBeNull();
  });
});
