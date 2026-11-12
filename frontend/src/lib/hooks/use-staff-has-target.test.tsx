import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { staffMonthSummaryService } from "~/lib/staff-api";
import { SWRConfig } from "~/lib/swr/testing";
import type { MonthSummary } from "~/lib/time-tracking-helpers";
import { useStaffHasTarget } from "./use-staff-has-target";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "target-test", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/staff-api", () => ({
  staffMonthSummaryService: { getMonthSummary: vi.fn() },
}));

describe("useStaffHasTarget", () => {
  it.each([true, false])(
    "verwirft den vorherigen Sollstatus %s beim Monatswechsel auch bei Ladefehlern",
    async (hasTarget) => {
      const cache = new Map();
      function wrapper({ children }: { readonly children: ReactNode }) {
        return (
          <SWRConfig
            value={{ provider: () => cache, shouldRetryOnError: false }}
          >
            {children}
          </SWRConfig>
        );
      }
      let rejectNext!: (error: Error) => void;
      const nextMonth = new Promise<MonthSummary>((_resolve, reject) => {
        rejectNext = reject;
      });
      vi.mocked(staffMonthSummaryService.getMonthSummary)
        .mockReset()
        .mockResolvedValueOnce({ hasTarget } as MonthSummary)
        .mockReturnValueOnce(nextMonth);
      const view = renderHook(
        ({ date }) => useStaffHasTarget("42", date, true),
        { initialProps: { date: "2026-09-30" }, wrapper },
      );
      await waitFor(() => expect(view.result.current).toBe(hasTarget));

      view.rerender({ date: "2026-10-01" });
      expect(view.result.current).toBeNull();
      expect(staffMonthSummaryService.getMonthSummary).toHaveBeenLastCalledWith(
        "42",
        2026,
        10,
      );

      const failure = new Error("month unavailable");
      await act(async () => rejectNext(failure));
      await waitFor(() =>
        expect(
          [...cache.values()].some((entry) => entry.error === failure),
        ).toBe(true),
      );
      expect(view.result.current).toBeNull();
    },
  );
});
