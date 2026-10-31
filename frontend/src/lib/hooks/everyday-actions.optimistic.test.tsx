import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";

vi.unmock("swr");
const mocks = vi.hoisted(() => ({
  session: vi.fn(),
  layout: vi.fn(),
  save: vi.fn(),
  reset: vi.fn(),
  policies: vi.fn(),
  current: vi.fn(),
  checkIn: vi.fn(),
  checkOut: vi.fn(),
  meals: vi.fn(),
  day: vi.fn(),
  clear: vi.fn(),
  week: vi.fn(),
  schedule: vi.fn(),
}));
vi.mock("next-auth/react", () => ({ useSession: mocks.session }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/home-layout-api", () => ({
  fetchHomeLayout: mocks.layout,
  saveHomeLayout: mocks.save,
  resetHomeLayout: mocks.reset,
  saveHomeBlockPolicies: mocks.policies,
}));
vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: {
    getCurrentSession: mocks.current,
    checkIn: mocks.checkIn,
    checkOut: mocks.checkOut,
  },
}));
vi.mock("~/lib/parent-api", () => ({
  getMealParticipation: mocks.meals,
  setMealParticipationDay: mocks.day,
  clearMealParticipationDay: mocks.clear,
  setMealParticipationWeek: mocks.week,
  replaceMealParticipationSchedule: mocks.schedule,
}));

import { ApiError } from "~/lib/api-error";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resetSWRScope, resumeSWRSession } from "~/lib/swr/scope";
import { timeTrackingKeys } from "~/lib/swr/portal-keys";
import type { WorkSession } from "~/lib/time-tracking-helpers";
import type { MealParticipation } from "~/lib/parent-api";
import { useHomeLayout } from "./use-home-layout";
import { useOptimisticStamp } from "./use-optimistic-stamp";
import { useMealParticipation } from "./use-meal-participation";

const opened: WorkSession = {
  id: "session-1",
  staffId: "1",
  date: "2026-09-09",
  status: "present",
  checkInTime: "2026-09-09T08:00:00+02:00",
  checkOutTime: null,
  breakMinutes: 0,
  notes: "",
  autoCheckedOut: false,
  createdBy: "1",
  updatedBy: null,
  createdAt: "2026-09-09T08:00:00+02:00",
  updatedAt: "2026-09-09T08:00:00+02:00",
};
const layout = {
  blocks: [],
  overrides: { news: false },
  policies: {},
  canManagePolicies: false,
};
const meals: MealParticipation = {
  weekdays: [1],
  cutoff_time: "10:00",
  model: "ongoing_single",
  schedule_changeable: true,
  sick_cancels_after_deadline: false,
  days: [
    {
      date: "2026-09-09",
      participating: false,
      regular_participating: false,
      source: "none",
      changeable: true,
      can_register: true,
      can_cancel: true,
      register_until: "2026-09-09T13:00:00+02:00",
      cancel_until: "2026-09-09T13:00:00+02:00",
    },
  ],
};
function signedIn(id = "A") {
  return {
    data: {
      user: { id, tenantId: 1 },
      expires: "2099-01-01",
    } satisfies Session,
    status: "authenticated",
  };
}
function Wrapper({ children }: { children: ReactNode }) {
  return (
    <SWRConfig
      value={{
        provider: () => new Map(),
        dedupingInterval: 0,
        shouldRetryOnError: false,
      }}
    >
      <SWRScopeBoundary expectedTenantId={1} />
      {children}
    </SWRConfig>
  );
}
function deferred() {
  let resolve!: (value: never) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<never>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return {
    promise,
    resolve: (value: unknown) => resolve(value as never),
    reject,
  };
}
function useIn() {
  const reader = useSWRAuth<WorkSession | null>(
    timeTrackingKeys.current(),
    mocks.current,
  );
  const action = useOptimisticStamp(reader.data, reader.mutate, "2026-09-09");
  return {
    data:
      action.data === undefined
        ? undefined
        : action.data?.checkOutTime === null,
    run: () => action.stamp("present"),
    ready: reader.data !== undefined,
  };
}
function useOut() {
  const reader = useSWRAuth<WorkSession | null>(
    timeTrackingKeys.current(),
    mocks.current,
  );
  const action = useOptimisticStamp(reader.data, reader.mutate, "2026-09-09");
  return {
    data:
      action.data === undefined
        ? undefined
        : action.data?.checkOutTime !== null,
    run: () => action.stamp(null),
    ready: reader.data !== undefined,
  };
}
function useSave() {
  const hook = useHomeLayout();
  return {
    data: hook.isReady ? hook.state.overrides.news : undefined,
    run: () => hook.save({ news: true }, []),
    ready: hook.isReady,
  };
}
function useReset() {
  const hook = useHomeLayout();
  return {
    data: hook.isReady
      ? Object.keys(hook.state.overrides).length === 0
      : undefined,
    run: hook.reset,
    ready: hook.isReady,
  };
}
function useDay() {
  const hook = useMealParticipation("1", "child-1", "2026-09-07", "2026-10-02");
  return {
    data: hook.data?.days[0]?.participating,
    run: () => hook.saveDay("2026-09-09", true),
    ready: !!hook.data,
  };
}
function useWeek() {
  const hook = useMealParticipation("1", "child-1", "2026-09-07", "2026-10-02");
  return {
    data: hook.data?.days[0]?.participating,
    run: () => hook.saveWeek(["2026-09-09"], "2026-09-07", true),
    ready: !!hook.data,
  };
}
function useSchedule() {
  const hook = useMealParticipation("1", "child-1", "2026-09-07", "2026-10-02");
  return {
    data: hook.data?.weekdays.includes(3),
    run: () => hook.saveSchedule([3]),
    ready: !!hook.data,
  };
}
function useCancelDay() {
  const hook = useMealParticipation("1", "child-1", "2026-09-07", "2026-10-02");
  return {
    data: hook.data ? !hook.data.days[0]?.participating : undefined,
    run: () => hook.saveDay("2026-09-09", false),
    ready: !!hook.data,
  };
}
function useResetDay() {
  const hook = useMealParticipation("1", "child-1", "2026-09-07", "2026-10-02");
  return {
    data: hook.data ? !hook.data.days[0]?.participating : undefined,
    run: () => hook.saveDay("2026-09-09", null),
    ready: !!hook.data,
  };
}
function useCancelWeek() {
  const hook = useMealParticipation("1", "child-1", "2026-09-07", "2026-10-02");
  return {
    data: hook.data ? !hook.data.days[0]?.participating : undefined,
    run: () => hook.saveWeek(["2026-09-09"], "2026-09-07", false),
    ready: !!hook.data,
  };
}
function usePolicies() {
  const hook = useHomeLayout();
  return {
    data: hook.isReady ? hook.state.policies.news === "required" : undefined,
    run: () => hook.savePolicies({ news: "required" }),
    ready: hook.isReady,
  };
}
// refused: the route's refusal code (everyday-action-refusals.ts); the
// layout has none, its read is open to every signed-in account.
// business: a failure that only rolls back. recheck: the reader that loads
// again after it, because reading may be refused too (#5423).
const cases = [
  {
    name: "Einstempeln",
    useCase: useIn,
    request: mocks.checkIn,
    reply: opened,
    refused: "general.permission",
    business: [409, "workforce.already_checked_in"],
    recheck: undefined,
  },
  {
    name: "Ausstempeln",
    useCase: useOut,
    request: mocks.checkOut,
    reply: { ...opened, checkOutTime: "2026-09-09T12:00:00+02:00" },
    refused: "general.permission",
    business: [404, "workforce.no_active_session"],
    recheck: undefined,
  },
  {
    name: "Layout speichern",
    useCase: useSave,
    request: mocks.save,
    reply: undefined,
    refused: undefined,
    business: [400, "general.input"],
    recheck: undefined,
  },
  {
    name: "Layout zurücksetzen",
    useCase: useReset,
    request: mocks.reset,
    reply: undefined,
    refused: undefined,
    business: [400, "general.input"],
    recheck: undefined,
  },
  {
    name: "Essenstag anmelden",
    useCase: useDay,
    request: mocks.day,
    reply: undefined,
    refused: "care.child_not_linked",
    business: [403, "care.guardian_permission_denied"],
    recheck: mocks.meals,
  },
  {
    name: "Essenswoche anmelden",
    useCase: useWeek,
    request: mocks.week,
    reply: { changed: ["2026-09-09"], locked: [] },
    refused: "care.child_not_linked",
    business: [403, "care.guardian_permission_denied"],
    recheck: mocks.meals,
  },
  {
    name: "Essenswochenplan",
    useCase: useSchedule,
    request: mocks.schedule,
    reply: { effective_from: "2026-09-09" },
    refused: "care.child_not_linked",
    business: [403, "care.guardian_permission_denied"],
    recheck: mocks.meals,
  },
  {
    name: "Essenstag abmelden",
    useCase: useCancelDay,
    request: mocks.day,
    reply: undefined,
    refused: "care.child_not_linked",
    business: [403, "care.guardian_permission_denied"],
    recheck: mocks.meals,
  },
  {
    name: "Essenstag zurücksetzen",
    useCase: useResetDay,
    request: mocks.clear,
    reply: undefined,
    refused: "care.child_not_linked",
    business: [403, "care.guardian_permission_denied"],
    recheck: mocks.meals,
  },
  {
    name: "Essenswoche abmelden",
    useCase: useCancelWeek,
    request: mocks.week,
    reply: { changed: ["2026-09-09"], locked: [] },
    refused: "care.child_not_linked",
    business: [403, "care.guardian_permission_denied"],
    recheck: mocks.meals,
  },
  {
    name: "Layout-Vorgaben",
    useCase: usePolicies,
    request: mocks.policies,
    reply: undefined,
    refused: undefined,
    business: [403, "general.permission"],
    recheck: undefined,
  },
] as const;

beforeEach(() => {
  vi.clearAllMocks();
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
  mocks.session.mockReturnValue(signedIn());
  mocks.layout.mockResolvedValue(layout);
  mocks.current.mockResolvedValue(null);
  mocks.meals.mockResolvedValue(meals);
});
afterEach(() => {
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
});

it("keeps confirmed policies separate from optimistic updates and rollback", async () => {
  const pending = deferred();
  mocks.policies.mockReturnValue(pending.promise);
  const { result, unmount } = renderHook(() => useHomeLayout(), {
    wrapper: Wrapper,
  });
  await waitFor(() => expect(result.current.isReady).toBe(true));
  const confirmed = result.current.confirmedPolicies;
  expect(confirmed).toBe(layout.policies);
  let task!: Promise<void>;
  act(() => {
    task = result.current
      .savePolicies({ news: "required" })
      .catch(() => undefined);
  });
  expect(result.current.state.policies).toEqual({ news: "required" });
  expect(result.current.confirmedPolicies).toBe(confirmed);
  await act(async () => {
    pending.reject(new ApiError("diag", 500));
    await task;
  });
  expect(result.current.state.policies).toEqual(layout.policies);
  expect(result.current.confirmedPolicies).toBe(confirmed);
  unmount();
});

it("clears the current session after checkout and permits another check-in", async () => {
  mocks.current.mockResolvedValue(opened);
  const closed = { ...opened, checkOutTime: new Date().toISOString() };
  mocks.checkOut.mockResolvedValue(closed);
  mocks.checkIn.mockResolvedValue({ ...opened, id: "session-2" });
  const { result, unmount } = renderHook(
    () => {
      const reader = useSWRAuth<WorkSession | null>(
        timeTrackingKeys.current(),
        mocks.current,
      );
      return {
        reader,
        action: useOptimisticStamp(reader.data, reader.mutate, "2026-09-09"),
      };
    },
    { wrapper: Wrapper },
  );
  await waitFor(() => expect(result.current.reader.data).toEqual(opened));
  await act(async () => {
    const outcome = await result.current.action.stamp(null);
    expect(outcome).toEqual({ status: "done", result: closed });
  });
  expect(result.current.reader.data).toBeNull();
  expect(result.current.action.data).toBeNull();
  await act(async () => {
    await result.current.action.stamp("present");
  });
  expect(result.current.reader.data?.id).toBe("session-2");
  expect(mocks.checkIn).toHaveBeenCalledOnce();
  expect(mocks.current).toHaveBeenCalledOnce();
  unmount();
});

it.each([
  ["care.guardian_permission_denied", true],
  ["care.child_not_linked", true],
  ["general.permission", true],
  ["general.unavailable", false],
  ["identity.session_account_mismatch", false],
] as const)(
  "handles a revalidation rejected with %s after a refused write",
  async (code, refused) => {
    const { result, unmount } = renderHook(
      () => ({
        current: useMealParticipation(
          "1",
          "child-1",
          "2026-09-07",
          "2026-10-02",
        ),
        otherRange: useMealParticipation(
          "1",
          "child-1",
          "2026-09-14",
          "2026-10-02",
        ),
        otherChild: useMealParticipation(
          "1",
          "child-2",
          "2026-09-07",
          "2026-10-02",
        ),
      }),
      { wrapper: Wrapper },
    );
    await waitFor(() => {
      expect(result.current.current.data).toEqual(meals);
      expect(result.current.otherRange.data).toEqual(meals);
      expect(result.current.otherChild.data).toEqual(meals);
    });
    const error = new ApiError("diag", 403, { code });
    mocks.day.mockRejectedValue(
      new ApiError("diag", 403, { code: "care.guardian_permission_denied" }),
    );
    mocks.meals.mockImplementation(() =>
      Promise.reject(new ApiError("diag", 403, { code })),
    );
    await act(async () => {
      await result.current.current.saveDay("2026-09-09", true);
    });
    await waitFor(() => expect(result.current.current.error).toEqual(error));
    if (refused) {
      await waitFor(() => {
        expect(result.current.current.data).toBeUndefined();
        expect(result.current.otherRange.data).toBeUndefined();
      });
      await act(async () => {
        expect(
          await result.current.current.saveDay("2026-09-09", true),
        ).toEqual({ status: "discarded" });
      });
      expect(mocks.day).toHaveBeenCalledOnce();
    } else {
      expect(result.current.current.data).toEqual(meals);
      expect(result.current.otherRange.data).toEqual(meals);
    }
    expect(result.current.otherChild.data).toEqual(meals);
    unmount();
  },
);

for (const scenario of cases) {
  describe(scenario.name, () => {
    async function setup() {
      if (scenario.useCase === useOut) mocks.current.mockResolvedValue(opened);
      if (
        [useCancelDay, useResetDay, useCancelWeek].includes(
          scenario.useCase as typeof useCancelDay,
        )
      )
        mocks.meals.mockResolvedValue({
          ...meals,
          days: meals.days.map((day) => ({
            ...day,
            participating: true,
            source: "override",
          })),
        });
      const pending = deferred();
      scenario.request.mockReturnValue(pending.promise);
      const hook = renderHook(() => scenario.useCase(), { wrapper: Wrapper });
      await waitFor(() => expect(hook.result.current.ready).toBe(true));
      return { ...hook, pending };
    }
    it("zeigt den Zwischenstand im Klick-Takt und übernimmt den Erfolg", async () => {
      const { result, pending } = await setup();
      let task!: Promise<unknown>;
      const start = performance.now();
      act(() => {
        task = result.current.run();
      });
      expect(performance.now() - start).toBeLessThan(100);
      expect(result.current.data).toBe(true);
      // Revalidation returns the server's new, confirmed state.
      mocks.layout.mockResolvedValue({
        ...layout,
        overrides: scenario.useCase === useReset ? {} : { news: true },
        policies: { news: "required" },
      });
      mocks.meals.mockResolvedValue({
        ...meals,
        weekdays: [3],
        days: meals.days.map((day) => ({
          ...day,
          participating: ![useCancelDay, useResetDay, useCancelWeek].includes(
            scenario.useCase as typeof useCancelDay,
          ),
          source: "override",
        })),
      });
      await act(async () => {
        pending.resolve(scenario.reply);
        await task;
      });
      expect(result.current.data).toBe(true);
    });
    it("nimmt bei einem Netzwerkfehler genau die eigene Änderung zurück", async () => {
      const { result, pending } = await setup();
      const before = result.current.data;
      let task!: Promise<unknown>;
      act(() => {
        task = result.current.run().catch((error: unknown) => error);
      });
      expect(result.current.data).toBe(true);
      await act(async () => {
        pending.reject(new TypeError("offline"));
        await task;
      });
      expect(result.current.data).toBe(before);
    });
    it.each([
      "identity.session_account_mismatch",
      "general.unavailable",
    ] as const)("nimmt %s nur zurück", async (code) => {
      const { result, pending } = await setup();
      const before = result.current.data;
      let task!: Promise<unknown>;
      act(() => {
        task = result.current.run().catch((error: unknown) => error);
      });
      expect(result.current.data).toBe(true);
      await act(async () => {
        pending.reject(new ApiError("diag", 403, { code }));
        await task;
      });
      expect(result.current.ready).toBe(true);
      expect(result.current.data).toBe(before);
    });
    if (scenario.refused !== undefined) {
      const refused = scenario.refused;
      it("verwirft die Ressource bei ihrem echten Ablehnungscode", async () => {
        const { result, pending } = await setup();
        let task!: Promise<unknown>;
        act(() => {
          task = result.current.run().catch((error: unknown) => error);
        });
        expect(result.current.data).toBe(true);
        await act(async () => {
          pending.reject(new ApiError("refused", 403, { code: refused }));
          await task;
        });
        expect(result.current.ready).toBe(false);
      });
    }
    it("nimmt einen fachlichen Zustand nur zurück und behält die Daten", async () => {
      const { result, pending } = await setup();
      const before = result.current.data;
      const reads = scenario.recheck?.mock.calls.length ?? 0;
      let task!: Promise<unknown>;
      act(() => {
        task = result.current.run().catch((error: unknown) => error);
      });
      expect(result.current.data).toBe(true);
      const [status, code] = scenario.business;
      await act(async () => {
        pending.reject(new ApiError("state", status, { code }));
        await task;
      });
      expect(result.current.ready).toBe(true);
      expect(result.current.data).toBe(before);
      if (scenario.recheck) {
        await waitFor(() =>
          expect(scenario.recheck!.mock.calls.length).toBe(reads + 1),
        );
      }
    });
    it.each(["Erfolg", "Fehler"])(
      "zeigt nach Kontowechsel weder Antwort noch Rollback: %s",
      async (end) => {
        const { result, rerender, pending } = await setup();
        let task!: Promise<unknown>;
        act(() => {
          task = result.current.run().catch((error: unknown) => error);
        });
        expect(result.current.data).toBe(true);
        act(() => {
          mocks.session.mockReturnValue(signedIn("B"));
          rerender();
        });
        await waitFor(() => expect(result.current.ready).toBe(true));
        const next = result.current.data;
        await act(async () => {
          if (end === "Erfolg") pending.resolve(scenario.reply);
          else pending.reject(new TypeError("offline"));
          await task;
        });
        expect(result.current.data).toBe(next);
        expect(result.current.data).toBe(false);
      },
    );
    it("startet aus einem zurückbehaltenen Render nach Scope-Wechsel nichts", async () => {
      const { result, rerender } = await setup();
      const retained = result.current.run;
      act(() => {
        mocks.session.mockReturnValue(signedIn("B"));
        rerender();
      });
      await act(async () => {
        await retained();
      });
      expect(scenario.request).not.toHaveBeenCalled();
    });
    it("startet aus einem Render vor einem Generationswechsel bei gleichem Scope nichts", async () => {
      const { result } = await setup();
      const retained = result.current.run;
      act(() => resetSWRScope({ revalidate: true }));
      await act(async () => {
        await retained();
      });
      expect(result.current.data).not.toBe(true);
      expect(scenario.request).not.toHaveBeenCalled();
    });
    it("lässt einen Doppelklick nur einmal schreiben", async () => {
      const { result, pending } = await setup();
      const retained = result.current.run;
      let first!: Promise<unknown>;
      act(() => {
        first = retained();
        void retained();
      });
      expect(result.current.data).toBe(true);
      expect(scenario.request).toHaveBeenCalledTimes(1);
      await act(async () => {
        pending.resolve(scenario.reply);
        await first;
      });
    });
    it("behält nach Unmount und spätem Fehler keinen Zwischenstand", async () => {
      const { result, pending, unmount } = await setup();
      let task!: Promise<unknown>;
      act(() => {
        task = result.current.run().catch((error: unknown) => error);
      });
      expect(result.current.data).toBe(true);
      unmount();
      await act(async () => {
        pending.reject(new TypeError("offline"));
        await task;
      });
      const next = renderHook(() => scenario.useCase(), { wrapper: Wrapper });
      await waitFor(() => expect(next.result.current.ready).toBe(true));
      expect(next.result.current.data).toBe(false);
    });
  });
}
