import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { DateRange } from "react-day-picker";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { BulkCancelResult } from "~/lib/timetable-types";

const { mockBulkCancel, mockToastSuccess, mockRefreshPlan } = vi.hoisted(
  () => ({
    mockBulkCancel: vi.fn(),
    mockToastSuccess: vi.fn(),
    mockRefreshPlan: vi.fn(),
  }),
);

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { bulkCancel: mockBulkCancel },
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutateMatching: () => mockRefreshPlan,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: vi.fn() }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

// The kit picker is covered by its own tests; here it only has to report a
// new range the way the real one does.
vi.mock("~/components/ui/date-range-picker", () => ({
  DateRangePicker: ({
    onChange,
  }: {
    onChange: (range: DateRange | undefined) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        onChange({
          from: new Date(2026, 9, 12),
          to: new Date(2026, 9, 16),
        })
      }
    >
      Mock Zeitraum 12.–16. Okt.
    </button>
  ),
}));

import { BulkCancelAppointmentsModal } from "./bulk-cancel-appointments-modal";

function preview(overrides: Partial<BulkCancelResult>): BulkCancelResult {
  return {
    from: "2026-10-12",
    to: "2026-10-25",
    dryRun: true,
    count: 0,
    days: [],
    kept: 0,
    keptSeries: [],
    ...overrides,
  };
}

const HOLIDAY_SERIES = [
  { name: "Ferienbetreuung", count: 5 },
  { name: "Ferienspiele", count: 1 },
];

function renderModal() {
  return render(
    <BulkCancelAppointmentsModal
      isOpen
      initialFrom="2026-10-12"
      initialTo="2026-10-25"
      onClose={vi.fn()}
    />,
  );
}

describe("BulkCancelAppointmentsModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockRefreshPlan.mockResolvedValue(undefined);
  });

  it("nennt die Serien, die im Plan bleiben, mit Namen und Anzahl", async () => {
    mockBulkCancel.mockResolvedValue(
      preview({ count: 23, kept: 6, keptSeries: HOLIDAY_SERIES }),
    );

    renderModal();

    expect(
      await screen.findByText(
        "Serien, die auch an Schließtagen stattfinden, bleiben: Ferienbetreuung (5 Termine), Ferienspiele (1 Termin).",
      ),
    ).toBeInTheDocument();
    const checkbox = screen.getByRole("checkbox", {
      name: "Auch diese Serien absagen",
    });
    expect(checkbox).not.toBeChecked();
    expect(mockBulkCancel).toHaveBeenCalledWith(
      "2026-10-12",
      "2026-10-25",
      true,
      false,
    );
  });

  it("zeigt keinen Widerspruch, wenn nur noch Serien an Schließtagen übrig sind", async () => {
    mockBulkCancel.mockImplementation(
      (_from: string, _to: string, dryRun: boolean, include: boolean) =>
        Promise.resolve(
          include
            ? preview({ dryRun, count: 6 })
            : preview({
                dryRun,
                count: 0,
                kept: 6,
                keptSeries: HOLIDAY_SERIES,
              }),
        ),
    );

    renderModal();

    expect(
      await screen.findByText(
        "Im Zeitraum 12.10.2026 – 25.10.2026 sind nur noch Serien geplant, die auch an Schließtagen stattfinden: Ferienbetreuung (5 Termine), Ferienspiele (1 Termin). Zum Absagen setzen Sie unten das Häkchen.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/sind keine Termine mehr geplant/),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Termine absagen" }),
    ).toBeDisabled();

    // Das Häkchen zählt neu, jetzt mit den Serien.
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: "Auch diese Serien absagen",
      }),
    );
    expect(
      await screen.findByText(
        "Im Zeitraum 12.10.2026 – 25.10.2026 werden 6 Termine abgesagt. Eltern bekommen keine Nachricht.",
      ),
    ).toBeInTheDocument();
    expect(mockBulkCancel).toHaveBeenCalledWith(
      "2026-10-12",
      "2026-10-25",
      true,
      true,
    );
    // Nach dem Neuzählen bleibt das Häkchen sichtbar und abwählbar.
    expect(
      screen.getByRole("checkbox", {
        name: "Auch diese Serien absagen",
      }),
    ).toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: "Termine absagen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig absagen" }));

    await waitFor(() =>
      expect(mockBulkCancel).toHaveBeenCalledWith(
        "2026-10-12",
        "2026-10-25",
        false,
        true,
      ),
    );
    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith("6 Termine abgesagt"),
    );
  });

  it("zählt neu, wenn der Zeitraum geändert wird", async () => {
    mockBulkCancel.mockImplementation((from: string, to: string) =>
      Promise.resolve(
        preview({ from, to, count: to === "2026-10-16" ? 12 : 40 }),
      ),
    );

    renderModal();

    expect(
      await screen.findByText(/werden 40 Termine abgesagt/),
    ).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Zeitraum" })).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Mock Zeitraum 12.–16. Okt." }),
    );

    expect(
      await screen.findByText(
        "Im Zeitraum 12.10.2026 – 16.10.2026 werden 12 Termine abgesagt. Eltern bekommen keine Nachricht.",
      ),
    ).toBeInTheDocument();
    expect(mockBulkCancel).toHaveBeenLastCalledWith(
      "2026-10-12",
      "2026-10-16",
      true,
      false,
    );
  });

  it("schreibt einen einzelnen Tag als ein Datum", async () => {
    mockBulkCancel.mockResolvedValue(preview({ count: 1 }));

    render(
      <BulkCancelAppointmentsModal
        isOpen
        initialFrom="2026-10-12"
        initialTo="2026-10-12"
        onClose={vi.fn()}
      />,
    );

    expect(
      await screen.findByText(
        "Am 12.10.2026 wird 1 Termin abgesagt. Eltern bekommen keine Nachricht.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", {
        name: "Auch diese Serien absagen",
      }),
    ).not.toBeInTheDocument();
  });
});
