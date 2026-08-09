import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DatabaseListItem } from "./database-list-item";

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
});
