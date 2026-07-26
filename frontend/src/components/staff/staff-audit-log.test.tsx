import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AuditLogEvent, AuditLogPage } from "~/lib/staff-audit-log-api";
import { StaffAuditLog } from "./staff-audit-log";

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
});
