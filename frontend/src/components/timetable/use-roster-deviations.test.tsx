import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { SWRConfig } from "~/lib/swr/testing";
import { describe, expect, it, vi } from "vitest";

import type { RosterDeviation } from "~/lib/roster-deviation";
import { timetableService } from "~/lib/timetable-api";
import { useRosterDeviations } from "./use-roster-deviations";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: { id: "1" } }, status: "authenticated" }),
}));
vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getRosterDeviations: vi.fn() },
}));

function wrapper({ children }: { children: ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map(), errorRetryCount: 0 }}>
      {children}
    </SWRConfig>
  );
}

const deviation: RosterDeviation = {
  templateId: "7",
  offerings: [{ id: "3", name: "OGS Ganztag" }],
  gradeLevels: [],
  schoolClasses: ["2a"],
  unbookedCount: 1,
  missingCount: 0,
  candidates: [],
  combinations: [],
  ambiguous: false,
};

describe("useRosterDeviations deep links", () => {
  it("revalidates a cached empty result before settling", async () => {
    let resolve!: (value: RosterDeviation[]) => void;
    vi.mocked(timetableService.getRosterDeviations).mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    const cache = new Map([
      ["test-tenant:roster-deviations-5-connect-7", { data: [] }],
    ]);
    const { result } = renderHook(
      () => useRosterDeviations("5", true, undefined, "7"),
      {
        wrapper: ({ children }) => (
          <SWRConfig value={{ provider: () => cache }}>{children}</SWRConfig>
        ),
      },
    );
    expect(result.current.settled).toBe(false);
    await waitFor(() =>
      expect(timetableService.getRosterDeviations).toHaveBeenCalledWith("5"),
    );
    await act(async () => {
      resolve([deviation]);
    });
    await waitFor(() => expect(result.current.settled).toBe(true));
    expect(result.current.deviations.get("7")).toEqual(deviation);
  });

  it("does not settle a failed read and supports retry", async () => {
    vi.mocked(timetableService.getRosterDeviations)
      .mockRejectedValueOnce(new Error("unavailable"))
      .mockResolvedValueOnce([deviation]);
    const { result } = renderHook(
      () => useRosterDeviations("5", true, undefined, "7"),
      { wrapper },
    );
    await waitFor(() => expect(result.current.error).toBeInstanceOf(Error));
    expect(result.current.settled).toBe(false);
    await act(async () => {
      await result.current.retry();
    });
    await waitFor(() => expect(result.current.settled).toBe(true));
    expect(result.current.deviations.get("7")).toEqual(deviation);
  });
});
