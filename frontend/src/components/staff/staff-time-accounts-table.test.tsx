import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  saldoPresets,
  StaffTimeAccountsTable,
} from "./staff-time-accounts-table";

const baseProps = {
  rows: [],
  year: 2026,
  month: 7,
  isLoading: false,
  error: null,
  onRowClick: vi.fn(),
  saldoPreset: "all" as const,
  customSaldoHours: "10",
  onPrevMonth: vi.fn(),
  onNextMonth: vi.fn(),
  canGoNextMonth: true,
  monthClose: null,
  monthIsOver: true,
  onCloseMonth: vi.fn(),
};

describe("StaffTimeAccountsTable", () => {
  it("setzt eine eigene Grenze beim Auswählen eines Presets vollständig zurück", () => {
    const onSaldoPresetChange = vi.fn();
    const onCustomSaldoHoursChange = vi.fn();
    const onShowCustomSaldoChange = vi.fn();

    render(
      <StaffTimeAccountsTable
        {...baseProps}
        showCustomSaldo
        onSaldoPresetChange={onSaldoPresetChange}
        onCustomSaldoHoursChange={onCustomSaldoHoursChange}
        onShowCustomSaldoChange={onShowCustomSaldoChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Plusstunden" }));

    expect(onCustomSaldoHoursChange).toHaveBeenCalledWith("");
    expect(onShowCustomSaldoChange).toHaveBeenCalledWith(false);
    expect(onSaldoPresetChange).toHaveBeenCalledWith("plus");
  });

  it("entfernt eine aktive eigene Grenze beim Ausblenden", () => {
    const onCustomSaldoHoursChange = vi.fn();
    const onShowCustomSaldoChange = vi.fn();

    render(
      <StaffTimeAccountsTable
        {...baseProps}
        showCustomSaldo
        onSaldoPresetChange={vi.fn()}
        onCustomSaldoHoursChange={onCustomSaldoHoursChange}
        onShowCustomSaldoChange={onShowCustomSaldoChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Grenze ausblenden" }));

    expect(onCustomSaldoHoursChange).toHaveBeenCalledWith("");
    expect(onShowCustomSaldoChange).toHaveBeenCalledWith(false);
  });

  it("entfernt das Preset beim Öffnen einer eigenen Grenze", () => {
    const onSaldoPresetChange = vi.fn();
    const onCustomSaldoHoursChange = vi.fn();
    const onShowCustomSaldoChange = vi.fn();

    render(
      <StaffTimeAccountsTable
        {...baseProps}
        saldoPreset="minus"
        customSaldoHours=""
        showCustomSaldo={false}
        onSaldoPresetChange={onSaldoPresetChange}
        onCustomSaldoHoursChange={onCustomSaldoHoursChange}
        onShowCustomSaldoChange={onShowCustomSaldoChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Eigene Grenze" }));

    expect(onCustomSaldoHoursChange).toHaveBeenCalledWith("");
    expect(onSaldoPresetChange).toHaveBeenCalledWith("all");
    expect(onShowCustomSaldoChange).toHaveBeenCalledWith(true);
  });

  it("schließt genau +20:00 aus dem Preset über +20 Std. aus", () => {
    expect(saldoPresets.find((preset) => preset.id === "plus20")?.min).toBe(
      1201,
    );
  });
});
