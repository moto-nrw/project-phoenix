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
    onFinish: vi.fn(),
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

  it.each([false, true])(
    "names the active mode visibly (selectionActive=%s)",
    (selectionActive) => {
      renderBar({ selectionActive });
      expect(
        screen.getByText("An- und Abmelde-Modus aktiv"),
      ).toBeInTheDocument();
    },
  );

  it.each([false, true])(
    "renders Fertig as the dark primary action (selectionActive=%s)",
    (selectionActive) => {
      renderBar({ selectionActive });
      const finish = screen.getByRole("button", { name: "Fertig" });
      expect(finish).toHaveClass("bg-gray-900", "text-white");
      expect(finish).not.toHaveClass("rounded-full");
    },
  );

  it("switches to the selection sub-mode via the segmented control", () => {
    const props = renderBar();
    fireEvent.click(screen.getByRole("button", { name: "Mehrere" }));
    expect(props.onSelectionActiveChange).toHaveBeenCalledWith(true);
  });

  it("switches back to immediate mode", () => {
    const props = renderBar({ selectionActive: true });
    fireEvent.click(screen.getByRole("button", { name: "Direkt" }));
    expect(props.onSelectionActiveChange).toHaveBeenCalledWith(false);
  });

  it("ends the active mode from the compact toolbar", () => {
    const props = renderBar();
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));
    expect(props.onFinish).toHaveBeenCalledOnce();
  });

  it("disables the bulk actions while nothing is selected", () => {
    renderBar({ selectionActive: true, selectedCount: 0 });
    expect(screen.getByRole("button", { name: /Anmelden/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Abmelden/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Aufheben" })).toBeDisabled();
    expect(
      screen.getByRole("region", { name: "An- und Abmelde-Modus" }),
    ).toHaveClass("hidden", "md:block");
  });

  it("shows the count and fires the bulk actions", () => {
    const props = renderBar({ selectionActive: true, selectedCount: 3 });
    expect(screen.getByText("3 ausgewählt")).toBeInTheDocument();

    expect(screen.getByRole("button", { name: /Anmelden/ })).toHaveClass(
      "bg-moto-green",
      "text-white",
      "rounded-lg",
    );
    expect(screen.getByRole("button", { name: /Abmelden/ })).toHaveClass(
      "bg-moto-red",
      "text-white",
      "rounded-lg",
    );

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
    // Switching to "Direkt" clears the selection; mid-flight that would
    // discard the failed IDs kept marked for retry (review #2372).
    const props = renderBar({
      selectionActive: true,
      selectedCount: 3,
      isRunning: true,
    });
    const immediate = screen.getByRole("button", { name: "Direkt" });
    expect(immediate).toBeDisabled();
    fireEvent.click(immediate);
    expect(props.onSelectionActiveChange).not.toHaveBeenCalled();
  });

  it("shows a spinner only on the running action", () => {
    renderBar({
      selectionActive: true,
      selectedCount: 3,
      isRunning: true,
      runningAction: "out",
    });

    expect(
      screen.getByRole("button", { name: /Abmelden/ }).querySelector("svg"),
    ).toHaveClass("animate-spin");
    expect(
      screen.getByRole("button", { name: /Anmelden/ }).querySelector("svg"),
    ).not.toHaveClass("animate-spin");
  });

  describe("Alle auswählen (#4762)", () => {
    const selectAll = {
      count: 12,
      leftOutCount: 3,
      allSelected: false,
      onSelectAll: vi.fn(),
    };

    it("marks the children on screen and names the left-out ones", () => {
      const onSelectAll = vi.fn();
      renderBar({
        selectionActive: true,
        selectAll: { ...selectAll, onSelectAll },
      });

      fireEvent.click(
        screen.getByRole("button", { name: "Alle 12 auswählen" }),
      );
      expect(onSelectAll).toHaveBeenCalledOnce();
      expect(
        screen.getByText(
          "3 ausgelassen: krank, entschuldigt, auf Klassenfahrt oder „Kommt heute nicht“",
        ),
      ).toBeInTheDocument();
    });

    it("drops the left-out line when nobody is left out", () => {
      renderBar({
        selectionActive: true,
        selectAll: { ...selectAll, leftOutCount: 0 },
      });
      expect(
        screen.getByRole("button", { name: "Alle 12 auswählen" }),
      ).toBeEnabled();
      expect(screen.queryByText(/ausgelassen/)).not.toBeInTheDocument();
    });

    it.each([
      ["every child is marked already", { allSelected: true }, false],
      ["no child can be marked", { count: 0 }, false],
      ["a batch is running", {}, true],
    ])("is disabled when %s", (_, override, isRunning) => {
      renderBar({
        selectionActive: true,
        isRunning,
        selectAll: { ...selectAll, ...override },
      });
      expect(
        screen.getByRole("button", { name: /^Alle \d+ auswählen$/ }),
      ).toBeDisabled();
    });

    it("is not offered while every tap acts immediately", () => {
      renderBar({ selectionActive: false, selectAll });
      expect(
        screen.getByText("Jeder Tipp meldet sofort an oder ab."),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: /auswählen/ }),
      ).not.toBeInTheDocument();
    });
  });
});
