// #6594: Eine späte Antwort auf „Meldung zurücknehmen“ schließt nur ihre
// eigene Rückfrage, nicht die inzwischen für ein anderes Kind geöffnete.
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { SchoolDayChild } from "~/lib/school-class-day-api";
import { deferred } from "~/test/helpers/form-modal-pending";

import {
  useSchoolAbsenceReports,
  type AbsenceReportTarget,
} from "./use-school-absence-reports";

const target = (studentId: string): AbsenceReportTarget => ({
  schoolClass: "2a",
  child: { student_id: studentId } as unknown as SchoolDayChild,
});

describe("useSchoolAbsenceReports withdraw", () => {
  it("lässt eine von außen geschlossene und neu geöffnete Rückfrage offen", async () => {
    const answer = deferred();
    const withdraw = vi.fn(() => answer.promise);
    const onChanged = vi.fn(() => Promise.resolve(true));
    const { result } = renderHook(() =>
      useSchoolAbsenceReports({
        dateISO: "2026-09-09",
        report: vi.fn(),
        withdraw,
        onChanged,
      }),
    );
    const mia = target("1");
    const ben = target("2");

    act(() => result.current.askWithdraw(mia));
    let running!: Promise<void>;
    act(() => {
      running = result.current.confirmWithdraw();
    });
    expect(result.current.withdrawing).toBe(true);

    // Von außen geschlossen (Berechtigung fiel weg), dann für Ben geöffnet.
    act(() => result.current.closeWithdraw());
    act(() => result.current.askWithdraw(ben));

    await act(async () => {
      answer.resolve();
      await running;
    });

    expect(onChanged).toHaveBeenCalledOnce();
    expect(result.current.withdrawTarget).toBe(ben);
  });

  it("schließt die eigene Rückfrage nach dem Zurücknehmen", async () => {
    const withdraw = vi.fn(() => Promise.resolve());
    const { result } = renderHook(() =>
      useSchoolAbsenceReports({
        dateISO: "2026-09-09",
        report: vi.fn(),
        withdraw,
        onChanged: () => Promise.resolve(true),
      }),
    );
    const mia = target("1");
    act(() => result.current.askWithdraw(mia));
    // Anker: die Rückfrage ist offen.
    expect(result.current.withdrawTarget).toBe(mia);

    await act(async () => {
      await result.current.confirmWithdraw();
    });

    expect(withdraw).toHaveBeenCalledWith("2a", "2026-09-09", "1");
    expect(result.current.withdrawTarget).toBeNull();
  });
});
