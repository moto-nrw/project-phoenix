import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DatabaseListItem } from "./database-list-item";

vi.mock("~/components/ui/navigation-link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

describe("DatabaseListItem", () => {
  it("renders title and subtitle and triggers select on click", () => {
    const onSelect = vi.fn();
    render(
      <DatabaseListItem
        title="Mathe AG"
        subtitle="Sport"
        isSelected={false}
        onSelect={onSelect}
      />,
    );

    expect(screen.getByText("Mathe AG")).toBeInTheDocument();
    expect(screen.getByText("Sport")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalled();
  });

  it("marks itself as current when selected", () => {
    render(
      <DatabaseListItem
        title="Raum 101"
        subtitle="Hauptgebäude"
        isSelected
        onSelect={() => undefined}
      />,
    );

    expect(screen.getByRole("button")).toHaveAttribute("aria-current", "true");
  });

  it("renders an optional trailing accessory before the chevron", () => {
    render(
      <DatabaseListItem
        title="Raum 101"
        subtitle="Belegt"
        isSelected={false}
        onSelect={() => undefined}
        trailingAccessory={<span data-testid="warning">!</span>}
      />,
    );

    expect(screen.getByTestId("warning")).toBeInTheDocument();
  });

  it("uses a native checkbox and does not open the detail in selection mode", () => {
    const onSelect = vi.fn();
    const onToggle = vi.fn();
    render(
      <DatabaseListItem
        title="Kind Eins"
        subtitle="3a"
        isSelected={false}
        onSelect={onSelect}
        selectionMode
        isChecked
        onToggleSelection={onToggle}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox"));

    expect(onToggle).toHaveBeenCalledOnce();
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders a link to the object route when href is set", () => {
    render(
      <DatabaseListItem
        title="Mia Fischer"
        subtitle="3a"
        isSelected={false}
        href="/students/7?from=%2Fdatabase%2Fstudents"
      />,
    );

    const link = screen.getByRole("link", { name: /Mia Fischer/ });
    expect(link).toHaveAttribute(
      "href",
      "/students/7?from=%2Fdatabase%2Fstudents",
    );
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("keeps the checkbox row in selection mode even with an href", () => {
    const onToggle = vi.fn();
    render(
      <DatabaseListItem
        title="Mia Fischer"
        subtitle="3a"
        isSelected={false}
        href="/students/7"
        selectionMode
        isChecked={false}
        onToggleSelection={onToggle}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox"));

    expect(onToggle).toHaveBeenCalledOnce();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
