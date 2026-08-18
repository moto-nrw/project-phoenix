import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SchoolCheckinSelectionBar } from "./school-checkin-selection-bar";

// Sub-mode bar of the check-in mode (#2359). SegmentedControl renders plain
// buttons, so fireEvent.click works (unlike Radix Tabs).

function renderBar(
  overrides: Partial<
    React.ComponentProps<typeof SchoolCheckinSelectionBar>
  > = {},
) {
  const props = {
    selectionActive: false,
    onSelectionActiveChange: vi.fn(),
    selectedCount: 0,
    onClearSelection: vi.fn(),
    onBulkAction: vi.fn(),
    isRunning: false,
    ...overrides,
  };
  render(<SchoolCheckinSelectionBar {...props} />);
  return props;
}

describe("SchoolCheckinSelectionBar", () => {
  it("shows the immediate-mode hint by default and no bulk actions", () => {
    renderBar();
    expect(
      screen.getByText("Jeder Tipp meldet sofort an oder ab."),
    ).toBeInTheDocument();
    expect(screen.queryByText("Abmelden")).not.toBeInTheDocument();
  });

  it("switches to the selection sub-mode via the segmented control", () => {
    const props = renderBar();
    fireEvent.click(screen.getByRole("button", { name: "Auswahl" }));
    expect(props.onSelectionActiveChange).toHaveBeenCalledWith(true);
  });

  it("switches back to immediate mode", () => {
    const props = renderBar({ selectionActive: true });
    fireEvent.click(screen.getByRole("button", { name: "Sofort" }));
    expect(props.onSelectionActiveChange).toHaveBeenCalledWith(false);
  });

  it("disables the bulk actions while nothing is selected", () => {
    renderBar({ selectionActive: true, selectedCount: 0 });
    expect(screen.getByRole("button", { name: /Anmelden/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Abmelden/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Aufheben" })).toBeDisabled();
    expect(screen.getByText(/Tippe auf Kinder/)).toBeInTheDocument();
  });

  it("shows the count and fires the bulk actions", () => {
    const props = renderBar({ selectionActive: true, selectedCount: 3 });
    expect(screen.getByText("3 ausgewählt")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Abmelden/ }));
    expect(props.onBulkAction).toHaveBeenCalledWith("out");

    fireEvent.click(screen.getByRole("button", { name: /Anmelden/ }));
    expect(props.onBulkAction).toHaveBeenCalledWith("in");

    fireEvent.click(screen.getByRole("button", { name: "Aufheben" }));
    expect(props.onClearSelection).toHaveBeenCalled();
  });

  it("locks the actions while a bulk run is in flight", () => {
    renderBar({ selectionActive: true, selectedCount: 3, isRunning: true });
    expect(screen.getByRole("button", { name: /Anmelden/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Abmelden/ })).toBeDisabled();
  });

  it("locks the sub-mode toggle while a bulk run is in flight", () => {
    // Switching to "Sofort" clears the selection; mid-flight that would
    // discard the failed IDs kept marked for retry (review #2372).
    const props = renderBar({
      selectionActive: true,
      selectedCount: 3,
      isRunning: true,
    });
    const immediate = screen.getByRole("button", { name: "Sofort" });
    expect(immediate).toBeDisabled();
    fireEvent.click(immediate);
    expect(props.onSelectionActiveChange).not.toHaveBeenCalled();
  });
});
