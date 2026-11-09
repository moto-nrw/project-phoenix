import { afterEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";

import type { WorkSessionEdit } from "~/lib/time-tracking-helpers";
import { setTestClock } from "~/test/clock";

import { EditHistoryAccordion } from "./edit-history-accordion";

// Runs in `school-date-dom`: only forked processes apply a runtime TZ change.
afterEach(() => vi.unstubAllEnvs());

describe("EditHistoryAccordion in a browser outside Germany (#6293)", () => {
  it("shows when and what was changed in Berlin time", () => {
    vi.stubEnv("TZ", "UTC");
    setTestClock("2026-11-09T12:00:00+01:00");
    // Saved on Monday 26.10.2026 at 00:30 Berlin, still Sunday 23:30 in UTC.
    const savedAt = "2026-10-25T23:30:00Z";
    // The zone change took effect, or the case below proves nothing.
    expect(new Date(savedAt).getDate()).toBe(25);

    const edit: WorkSessionEdit = {
      id: "1",
      sessionId: "42",
      staffId: "7",
      editedBy: "3",
      fieldName: "check_in_time",
      // 08:00 → 07:30 Berlin summer time on 20.10.2026
      oldValue: "2026-10-20T06:00:00Z",
      newValue: "2026-10-20T05:30:00Z",
      notes: "Zeitkorrektur",
      createdAt: savedAt,
      editorName: "Anna Müller",
      isSelfEdit: false,
    };
    render(<EditHistoryAccordion edits={[edit]} isLoading={false} />);

    // The table renders each row for desktop and phone.
    expect(screen.getAllByText("26.10.2026, 00:30").length).toBeGreaterThan(0);
    expect(screen.getAllByText("08:00").length).toBeGreaterThan(0);
    expect(screen.getAllByText("07:30").length).toBeGreaterThan(0);
    expect(screen.queryByText("25.10.2026, 23:30")).not.toBeInTheDocument();
  });
});
