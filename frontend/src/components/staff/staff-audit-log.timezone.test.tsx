import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

import { ToastProvider } from "~/contexts/ToastContext";
import type { AuditLogEvent } from "~/lib/staff-audit-log-api";
import { setTestClock } from "~/test/clock";

import { StaffAuditLog } from "./staff-audit-log";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
afterEach(() => vi.unstubAllEnvs());

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

const getAuditLog = vi.hoisted(() => vi.fn());

vi.mock("~/lib/staff-audit-log-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/staff-audit-log-api")>();
  return { ...actual, staffAuditLogService: { getAuditLog } };
});

describe("StaffAuditLog in a browser outside Germany (#6293)", () => {
  it("shows when an entry happened in Berlin time", async () => {
    vi.stubEnv("TZ", "UTC");
    setTestClock("2026-11-09T12:00:00+01:00");
    // Monday 26.10.2026 at 00:30 Berlin, still Sunday 23:30 in UTC.
    const occurredAt = "2026-10-25T23:30:00Z";
    // The zone change took effect, or the case below proves nothing.
    expect(new Date(occurredAt).getDate()).toBe(25);

    const entry: AuditLogEvent = {
      occurredAt,
      source: "month_reopen",
      entryId: "1",
      staffId: "42",
      staffName: "Anna Muster",
      actorStaffId: "7",
      actorName: "Leitung",
      actorIsSystem: false,
      actorIsSelf: false,
      reason: "",
      detail: { year: 2026, month: 10 },
    };
    getAuditLog.mockResolvedValue({
      events: [entry],
      nextCursor: null,
      retentionCutoff: "2024-01-01",
    });

    render(<StaffAuditLog staffOptions={[]} />, { wrapper: ToastProvider });

    // The table renders each row for desktop and phone.
    expect(
      (await screen.findAllByText("26.10.2026, 00:30")).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText("25.10.2026, 23:30")).not.toBeInTheDocument();
  });
});
