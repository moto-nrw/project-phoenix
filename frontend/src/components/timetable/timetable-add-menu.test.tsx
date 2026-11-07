import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TimetableAddMenu } from "./timetable-add-menu";

describe("TimetableAddMenu", () => {
  it("works with the keyboard and starts the chosen entry", () => {
    const onAddInstance = vi.fn();
    const onAddSeries = vi.fn();
    render(
      <TimetableAddMenu
        onAddInstance={onAddInstance}
        onAddSeries={onAddSeries}
      />,
    );
    const trigger = screen.getByRole("button", { name: "Neu anlegen" });
    // Enter und Leertaste lösen einen Klick ohne Zeiger aus (detail 0).
    fireEvent.click(trigger, { detail: 0 });

    const single = screen.getByRole("menuitem", { name: "Einmaliger Termin" });
    const series = screen.getByRole("menuitem", {
      name: "Regelmäßiger Termin",
    });
    expect(single).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("menu"), { key: "ArrowDown" });
    expect(series).toHaveFocus();
    fireEvent.keyDown(series, { key: "Enter" });

    expect(onAddSeries).toHaveBeenCalledOnce();
    expect(onAddInstance).not.toHaveBeenCalled();
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
    expect(trigger).toHaveFocus();
  });

  it("cannot be opened while disabled", () => {
    render(
      <TimetableAddMenu
        onAddInstance={vi.fn()}
        onAddSeries={vi.fn()}
        disabled
      />,
    );

    expect(screen.getByRole("button", { name: "Neu anlegen" })).toBeDisabled();
  });
});
