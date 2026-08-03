import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AuditLogEvent, AuditLogPage } from "~/lib/staff-audit-log-api";
import { isoDatePickerMock } from "~/test/mocks/date-picker";
import { StaffAuditLog } from "./staff-audit-log";

// The Von/Bis filters are kit date pickers, not native inputs. These tests are
// about the audit log's own behaviour (stale-response handling, name
// resolution), so the stub keeps them setting a date with fireEvent.change.
vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...isoDatePickerMock(),
}));

const getAuditLog = vi.hoisted(() => vi.fn());

vi.mock("~/lib/staff-audit-log-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/staff-audit-log-api")>();
  return {
    ...actual,
    staffAuditLogService: { getAuditLog },
  };
});

function event(
  month: number,
  staffId: string | null = "42",
  staffName = "Anna Muster",
): AuditLogEvent {
  return {
    occurredAt: `2026-${String(month).padStart(2, "0")}-01T09:00:00+02:00`,
    source: "month_reopen",
    entryId: String(month),
    staffId,
    staffName,
    actorStaffId: "7",
    actorName: "Leitung",
    actorIsSystem: false,
    actorIsSelf: false,
    reason: "",
    detail: { year: 2026, month },
  };
}

function page(
  events: AuditLogEvent[],
  nextCursor: string | null,
): AuditLogPage {
  return {
    events,
    nextCursor,
    retentionCutoff: "2024-01-01",
  };
}

describe("StaffAuditLog", () => {
  beforeEach(() => {
    getAuditLog.mockReset();
  });
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("ignoriert eine alte Weitere-Einträge-Antwort nach einem Filterwechsel", async () => {
    let resolveLoadMore!: (value: AuditLogPage) => void;
    const loadMore = new Promise<AuditLogPage>((resolve) => {
      resolveLoadMore = resolve;
    });
    getAuditLog
      .mockResolvedValueOnce(page([event(6)], "alte-seite"))
      .mockReturnValueOnce(loadMore)
      .mockResolvedValueOnce(page([event(7)], null));

    render(<StaffAuditLog staffOptions={[]} />);

    expect(
      await screen.findByText(/Monat 06\/2026 wieder geöffnet/),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Einträge laden" }),
    );
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-07-01" },
    });

    expect(
      await screen.findByText(/Monat 07\/2026 wieder geöffnet/),
    ).toBeInTheDocument();

    await act(async () => {
      resolveLoadMore(page([event(8)], null));
      await loadMore;
    });

    expect(
      screen.queryByText(/Monat 08\/2026 wieder geöffnet/),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/Monat 07\/2026 wieder geöffnet/),
    ).toBeInTheDocument();
  });

  it("unterscheidet schulweite von nicht mehr auflösbaren Mitarbeitenden", async () => {
    getAuditLog.mockResolvedValueOnce(
      page([event(5, null, ""), event(6, "99", "")], null),
    );

    render(<StaffAuditLog staffOptions={[]} />);

    await waitFor(() => {
      expect(
        within(screen.getByRole("table")).getByText("Alle Mitarbeitenden"),
      ).toBeInTheDocument();
      expect(
        within(screen.getByRole("table")).getByText(
          "Unbekannte/ehemalige Person",
        ),
      ).toBeInTheDocument();
    });
  });

  it("beschreibt Urlaubs-Übernahmen und deren Löschung", async () => {
    const opening: AuditLogEvent = {
      ...event(8),
      source: "vacation_opening",
      entryId: "opening-1",
      detail: {
        opening_id: 3,
        year: 2026,
        effective_date: "2026-08-01",
        taken_before_days: 4,
        entered_remaining_days: 12.5,
      },
    };
    const deleted: AuditLogEvent = {
      ...event(9),
      source: "deletion",
      entryId: "deletion-1",
      detail: {
        deleted_source: "vacation_opening",
        source_id: 3,
        payload: { year: 2026, entered_remaining_days: 12.5 },
      },
    };
    getAuditLog.mockResolvedValueOnce(page([opening, deleted], null));

    render(<StaffAuditLog staffOptions={[]} />);

    expect(
      await screen.findByText(
        "Urlaubs-Übernahme 2026: 12,5 Tage Rest zum 01.08.2026",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Urlaubs-Übernahme gelöscht: 2026 (12,5 Tage Rest)"),
    ).toBeInTheDocument();
  });

  it("rendert Ereignisse mit gleichem Umschlag über eindeutige Entry-IDs", async () => {
    const first = event(6);
    const second = { ...event(6), entryId: "different-entry" };
    getAuditLog.mockResolvedValueOnce(page([first, second], null));
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => {});

    render(<StaffAuditLog staffOptions={[]} />);

    await waitFor(() => {
      expect(
        screen.getAllByText(/Monat 06\/2026 wieder geöffnet/),
      ).toHaveLength(2);
    });
    expect(consoleError).not.toHaveBeenCalled();
  });
});
