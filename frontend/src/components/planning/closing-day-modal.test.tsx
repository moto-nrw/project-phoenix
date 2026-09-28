import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ClosingDay } from "~/lib/closing-day-helpers";

// The factory is hoisted above the imports, so the mock helper has to be
// pulled in inside it rather than at the top of the file.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const { mockCreate, mockUpdate, mockBulkCancel } = vi.hoisted(() => ({
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockBulkCancel: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { bulkCancel: mockBulkCancel },
}));

vi.mock("~/lib/closing-day-api", () => ({
  closingDayService: {
    create: mockCreate,
    update: mockUpdate,
  },
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    children,
    footer,
  }: {
    isOpen: boolean;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div>
        {children}
        {footer}
      </div>
    ) : null,
}));

import { ClosingDayModal } from "./closing-day-modal";

const existingClosingDay: ClosingDay = {
  id: "7",
  startDate: "2026-12-24",
  endDate: "2026-12-31",
  reason: "Weihnachtswoche",
};

function renderModal(initial: ClosingDay | null = null) {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  render(
    <ClosingDayModal
      isOpen
      onClose={onClose}
      onSaved={onSaved}
      initial={initial}
    />,
  );
  return { onClose, onSaved };
}

describe("ClosingDayModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreate.mockResolvedValue(existingClosingDay);
    mockUpdate.mockResolvedValue(existingClosingDay);
  });

  it("creates a closing day with a trimmed reason", async () => {
    const { onClose, onSaved } = renderModal();

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "  Pädagogischer Tag  " },
    });
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-09-14" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-14" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith({
        start_date: "2026-09-14",
        end_date: "2026-09-14",
        reason: "Pädagogischer Tag",
      }),
    );
    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  // #3594: appointments planned before the closure was entered stay in the
  // plan, so saving offers to cancel them, and only when there are some.
  it("offers to cancel the appointments still planned in the saved range", async () => {
    mockBulkCancel.mockResolvedValue({
      from: "2026-10-12",
      to: "2026-10-25",
      dryRun: true,
      count: 83,
      days: [],
    });
    const onOfferCancel = vi.fn();
    render(
      <ClosingDayModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initial={null}
        onOfferCancel={onOfferCancel}
      />,
    );

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "Herbstferien" },
    });
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-10-12" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-10-25" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(onOfferCancel).toHaveBeenCalledWith({
        startDate: "2026-10-12",
        endDate: "2026-10-25",
      }),
    );
    expect(mockBulkCancel).toHaveBeenCalledWith(
      "2026-10-12",
      "2026-10-25",
      true,
    );
  });

  it("offers nothing when no appointment is left or counting fails", async () => {
    const onOfferCancel = vi.fn();
    const { rerender } = render(
      <ClosingDayModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initial={existingClosingDay}
        onOfferCancel={onOfferCancel}
      />,
    );

    mockBulkCancel.mockResolvedValueOnce({
      from: "2026-12-24",
      to: "2026-12-31",
      dryRun: true,
      count: 0,
      days: [],
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mockBulkCancel).toHaveBeenCalledOnce());

    mockBulkCancel.mockRejectedValueOnce(new Error("Forbidden"));
    rerender(
      <ClosingDayModal
        isOpen
        onClose={vi.fn()}
        onSaved={vi.fn()}
        initial={existingClosingDay}
        onOfferCancel={onOfferCancel}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mockBulkCancel).toHaveBeenCalledTimes(2));

    expect(onOfferCancel).not.toHaveBeenCalled();
    expect(mockUpdate).toHaveBeenCalledTimes(2);
  });

  it("updates the selected closing day", async () => {
    const { onClose, onSaved } = renderModal(existingClosingDay);

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "Weihnachtsferien" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith("7", {
        start_date: "2026-12-24",
        end_date: "2026-12-31",
        reason: "Weihnachtsferien",
      }),
    );
    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("rejects a whitespace-only reason before calling the API", () => {
    renderModal();

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "   " },
    });
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-09-14" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-14" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bitte einen Grund angeben.",
    );
    expect(mockCreate).not.toHaveBeenCalled();
    expect(mockUpdate).not.toHaveBeenCalled();
  });
});
