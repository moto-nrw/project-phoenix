import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { SidebarSubItem } from "./sidebar-sub-item";

// Mock next/link
vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

describe("SidebarSubItem", () => {
  it("renders label and links to href", () => {
    render(<SidebarSubItem href="/rooms/1" label="Room A" isActive={false} />);
    const link = screen.getByRole("link", { name: /Room A/ });
    expect(link).toHaveAttribute("href", "/rooms/1");
  });

  it("applies active styling when isActive is true", () => {
    render(<SidebarSubItem href="/rooms/1" label="Room A" isActive={true} />);
    const link = screen.getByRole("link");
    expect(link.className).toContain("bg-gray-100");
    expect(link.className).toContain("font-semibold");
  });

  it("applies inactive styling when isActive is false", () => {
    render(<SidebarSubItem href="/rooms/1" label="Room A" isActive={false} />);
    const link = screen.getByRole("link");
    expect(link.className).toContain("text-gray-500");
    expect(link.className).toContain("font-medium");
  });

  it("shows count badge when count is provided", () => {
    render(
      <SidebarSubItem
        href="/rooms/1"
        label="Room A"
        isActive={false}
        count={5}
      />,
    );
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("shows count badge when count is 0", () => {
    render(
      <SidebarSubItem
        href="/rooms/1"
        label="Room A"
        isActive={false}
        count={0}
      />,
    );
    expect(screen.getByText("0")).toBeInTheDocument();
  });

  it("does not show count badge when count is undefined", () => {
    render(<SidebarSubItem href="/rooms/1" label="Room A" isActive={false} />);
    // Only the label span should be present, no count span
    const link = screen.getByRole("link");
    const spans = link.querySelectorAll("span");
    expect(spans).toHaveLength(1); // Only the label span
  });
});
