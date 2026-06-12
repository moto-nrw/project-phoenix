import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { mockToastSuccess, mockToastError, mockCreate, mockUpdate, mockDelete } =
  vi.hoisted(() => ({
    mockToastSuccess: vi.fn(),
    mockToastError: vi.fn(),
    mockCreate: vi.fn(),
    mockUpdate: vi.fn(),
    mockDelete: vi.fn(),
  }));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    create: mockCreate,
    update: mockUpdate,
    delete: mockDelete,
  },
}));

import { CalendarPeriodModal } from "./calendar-period-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";

const period: CalendarPeriod = {
  id: "5",
  tenantId: "1",
  name: "Schuljahr 2026/2027",
  periodType: "school_year",
  startDate: "2026-08-01",
  endDate: "2027-07-31",
  weekCycleLength: 2,
  weekCycleAnchor: "2026-08-03",
  isActive: true,
  createdAt: "2026-05-01T00:00:00Z",
  updatedAt: "2026-05-01T00:00:00Z",
};

describe("CalendarPeriodModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // create/update return { period, warnings } since the period-warning
    // contract landed; the modal destructures .period and ignores warnings.
    mockCreate.mockResolvedValue({ period, warnings: [] });
    mockUpdate.mockResolvedValue({ period, warnings: [] });
    mockDelete.mockResolvedValue(undefined);
  });

  it("creates a period using defaults and validates date/cycle rules", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        createDefaults={{
          name: "Schuljahr 2026/2027",
          startDate: "2026-08-01",
          endDate: "2027-07-31",
          weekCycleLength: "1",
        }}
      />,
    );

    fireEvent.change(screen.getByLabelText("Enddatum*"), {
      target: { value: "2026-07-31" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Enddatum muss nach dem Startdatum liegen.",
    );

    fireEvent.change(screen.getByLabelText("Enddatum*"), {
      target: { value: "2027-07-31" },
    });
    fireEvent.change(screen.getByLabelText("Wiederholung in Wochen"), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bei einer Wiederholung über mehrere Wochen ist das Startdatum der Wiederholung erforderlich.",
    );

    fireEvent.change(screen.getByLabelText("Startdatum der Wiederholung*"), {
      target: { value: "2026-08-03" },
    });
    fireEvent.click(screen.getByLabelText(/Zeitraum im Plan verwenden/));
    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith({
        name: "Schuljahr 2026/2027",
        period_type: "school_year",
        start_date: "2026-08-01",
        end_date: "2027-07-31",
        week_cycle_length: 2,
        is_active: false,
        week_cycle_anchor: "2026-08-03",
      }),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Planungszeitraum "Schuljahr 2026/2027" angelegt',
    );
    expect(onSaved).toHaveBeenCalledWith(period);
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("updates and deletes an existing period with confirmation", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const onDeleted = vi.fn();
    const { rerender } = render(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        onDeleted={onDeleted}
        initial={period}
      />,
    );

    fireEvent.change(screen.getByLabelText("Bezeichnung*"), {
      target: { value: "Neues Schuljahr" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "5",
        expect.objectContaining({ name: "Neues Schuljahr" }),
      ),
    );
    expect(onSaved).toHaveBeenCalledWith(period);

    rerender(
      <CalendarPeriodModal
        isOpen
        onClose={onClose}
        onSaved={onSaved}
        onDeleted={onDeleted}
        initial={period}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    expect(screen.getByText(/Löschen klappt nur/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Löschen abbrechen" }));
    expect(screen.queryByText(/Löschen klappt nur/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Löschen bestätigen" }));
    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith("5"));
    expect(onDeleted).toHaveBeenCalledWith(period);
  });

  it("surfaces API failures", async () => {
    mockCreate.mockRejectedValueOnce(new Error("Periode ueberlappt"));
    render(
      <CalendarPeriodModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        createDefaults={{
          name: "Schuljahr",
          startDate: "2026-08-01",
          endDate: "2027-07-31",
        }}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Anlegen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Periode ueberlappt",
    );
    expect(mockToastError).toHaveBeenCalledWith("Periode ueberlappt");
  });
});
