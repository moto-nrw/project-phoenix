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
  monthCloseError: null,
  onRetryMonthClose: vi.fn(),
  monthIsOver: true,
  onCloseMonth: vi.fn(),
};

describe("StaffTimeAccountsTable", () => {
  it("zeigt einen Ladefehler vor Ort statt einer leeren Liste", () => {
    const retry = vi.fn();
    render(
      <StaffTimeAccountsTable
        {...baseProps}
        error={{
          message: "Die Zeitkonten sind gerade nicht erreichbar.",
          attempt: 1,
          retry: { label: "Wiederholen", onClick: retry },
        }}
        onSaldoPresetChange={vi.fn()}
        onCustomSaldoHoursChange={vi.fn()}
        showCustomSaldo={false}
        onShowCustomSaldoChange={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Die Zeitkonten sind gerade nicht erreichbar."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine Zeitkonten gefunden"),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalled();
  });

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

  it("zeigt das Abschlussdatum in der Berliner Zeitzone", () => {
    const originalTimeZone = process.env.TZ;
    process.env.TZ = "America/Los_Angeles";

    try {
      render(
        <StaffTimeAccountsTable
          {...baseProps}
          monthClose={{
            closed: true,
            closedAt: "2026-09-01T00:30:00Z",
            closedCount: 12,
          }}
          onSaldoPresetChange={vi.fn()}
          onCustomSaldoHoursChange={vi.fn()}
          onShowCustomSaldoChange={vi.fn()}
          showCustomSaldo={false}
        />,
      );

      expect(
        screen.getByText("Abgeschlossen am 01.09.2026"),
      ).toBeInTheDocument();
    } finally {
      process.env.TZ = originalTimeZone;
    }
  });
});

// #4804: je Person die Zahl der Tage mit zu kurzer Pause oder mehr als
// 10 Std. Arbeit, und ein Filter auf genau diese Personen.
describe("StaffTimeAccountsTable Hinweise", () => {
  const account = {
    employmentType: "",
    sollMinutes: 0,
    istMinutes: 0,
    balanceMinutes: 0,
    remainingVacationDays: 0,
  };
  const rows = [
    {
      ...account,
      staffId: "1",
      firstName: "Anna",
      lastName: "Albers",
      name: "Anna Albers",
      laborHintDays: 3,
    },
    {
      ...account,
      staffId: "2",
      firstName: "Ben",
      lastName: "Brandt",
      name: "Ben Brandt",
      laborHintDays: 0,
    },
  ];

  function renderAccounts() {
    render(
      <StaffTimeAccountsTable
        {...baseProps}
        rows={rows}
        onSaldoPresetChange={vi.fn()}
        onCustomSaldoHoursChange={vi.fn()}
        showCustomSaldo={false}
        onShowCustomSaldoChange={vi.fn()}
      />,
    );
  }

  it("zeigt je Person die Zahl der markierten Tage", () => {
    renderAccounts();

    expect(screen.getByText("3 Tage")).toBeInTheDocument();
    expect(screen.getByText("Brandt, Ben")).toBeInTheDocument();
  });

  it("filtert auf Personen mit Hinweisen", () => {
    renderAccounts();

    const filter = screen.getByRole("button", { name: "Mit Hinweisen" });
    fireEvent.click(filter);

    expect(filter).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("Albers, Anna")).toBeInTheDocument();
    expect(screen.queryByText("Brandt, Ben")).not.toBeInTheDocument();

    fireEvent.click(filter);
    expect(screen.getByText("Brandt, Ben")).toBeInTheDocument();
  });
});
