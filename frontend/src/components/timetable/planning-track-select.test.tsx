import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PlanningTrackSelect } from "./planning-track-select";

const tracks = [
  { id: "1", name: "Jahrgang 1", color: "#5080D8", sortOrder: 0 },
  { id: "2", name: "Jahrgang 2", color: "#83CD2D", sortOrder: 1 },
  {
    id: "3",
    name: "Archiv",
    color: "#F78C10",
    sortOrder: 2,
    archivedAt: "2026-08-03T12:00:00Z",
  },
];

function renderSelect(value = "") {
  const onChange = vi.fn();
  render(
    <PlanningTrackSelect value={value} tracks={tracks} onChange={onChange} />,
  );
  return { onChange };
}

// Seit #3114 ist das Feld eine reine Auswahl: Anlegen, Umbenennen,
// Umsortieren und Archivieren liegen unter „Datenverwaltung →
// Planungsspuren", nicht mehr in diesem Popover.
describe("PlanningTrackSelect", () => {
  it("selects a track and closes", () => {
    const { onChange } = renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.click(screen.getByRole("option", { name: /Jahrgang 2/ }));

    expect(onChange).toHaveBeenCalledWith("2");
    expect(
      screen.getByRole("combobox", { name: "Planungsspur" }),
    ).toHaveAttribute("aria-expanded", "false");
  });

  it("clears the choice", () => {
    const { onChange } = renderSelect("2");

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.click(screen.getByRole("option", { name: "Keine Planungsspur" }));

    expect(onChange).toHaveBeenCalledWith("");
  });

  it("filters by the typed text", () => {
    renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.change(screen.getByPlaceholderText("Planungsspur suchen …"), {
      target: { value: "jahrgang 2" },
    });

    expect(screen.getByRole("option", { name: /Jahrgang 2/ })).toBeTruthy();
    expect(
      screen.queryByRole("option", { name: /Jahrgang 1/ }),
    ).not.toBeInTheDocument();
  });

  it("offers no way to create a track from the field", () => {
    renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    fireEvent.change(screen.getByPlaceholderText("Planungsspur suchen …"), {
      target: { value: "Nord" },
    });

    expect(
      screen.getByText("Keine Planungsspur gefunden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /anlegen/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /verwalten/i }),
    ).not.toBeInTheDocument();
  });

  it("hides an archived track unless it is the current value", () => {
    const { onChange } = renderSelect();

    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    expect(
      screen.queryByRole("option", { name: /Archiv/ }),
    ).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps showing the archived track a Termin still points at", () => {
    renderSelect("3");

    expect(
      screen.getByRole("combobox", { name: "Planungsspur" }),
    ).toHaveTextContent("Archiv (archiviert)");
    fireEvent.click(screen.getByRole("combobox", { name: "Planungsspur" }));
    expect(
      screen.getByRole("option", { name: /Archiv \(archiviert\)/ }),
    ).toBeInTheDocument();
  });

  it("closes only the popover on Escape and restores trigger focus", () => {
    renderSelect();
    const trigger = screen.getByRole("combobox", { name: "Planungsspur" });

    fireEvent.click(trigger);
    fireEvent.keyDown(screen.getByPlaceholderText("Planungsspur suchen …"), {
      key: "Escape",
    });

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
    expect(
      screen.queryByPlaceholderText("Planungsspur suchen …"),
    ).not.toBeInTheDocument();
  });
});
