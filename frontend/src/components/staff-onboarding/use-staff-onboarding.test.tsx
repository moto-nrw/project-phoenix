import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import type { Session } from "next-auth";
import type { StaffOnboardingState } from "~/lib/staff-onboarding-api";

vi.unmock("swr");
const mocks = vi.hoisted(() => ({ fetch: vi.fn() }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "74", tenantId: 1, roles: ["user"], permissions: [] },
      expires: "2099-01-01",
    } satisfies Session,
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-one",
}));
vi.mock("~/lib/staff-onboarding-api", () => ({
  fetchStaffOnboarding: mocks.fetch,
}));

import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resetSWRScope, resumeSWRSession } from "~/lib/swr/scope";
import { useStaffOnboarding } from "./use-staff-onboarding";

function Wrapper({ children }: { children: ReactNode }) {
  return (
    <SWRConfig value={{ provider: () => new Map(), shouldRetryOnError: false }}>
      <SWRScopeBoundary expectedTenantId={1} />
      {children}
    </SWRConfig>
  );
}

const open: StaffOnboardingState = {
  dismissed: false,
  schoolReady: true,
  doneSteps: [],
  skippedSteps: [],
};
const done: StaffOnboardingState = { ...open, doneSteps: ["work_time"] };

function deferred() {
  let resolve!: (value: StaffOnboardingState) => void;
  const promise = new Promise<StaffOnboardingState>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
}

/**
 * Lädt den Stand neu wie beim Zurückkehren in den Tab: nach dem
 * Dedupe-Fenster der SWR-Konfiguration ein Fokus-Ereignis.
 */
async function refocus(expectedCalls: number) {
  await act(async () => {
    vi.advanceTimersByTime(6_000);
    window.dispatchEvent(new Event("focus"));
  });
  await waitFor(() => expect(mocks.fetch).toHaveBeenCalledTimes(expectedCalls));
}

async function loaded() {
  mocks.fetch.mockResolvedValueOnce(open);
  const view = renderHook(() => useStaffOnboarding(), { wrapper: Wrapper });
  await waitFor(() => expect(view.result.current.state).toEqual(open));
  return view;
}

// Invarianten zum Abschluss einer Tour (#6339): Der bestätigte Stand einer
// Speicherung gilt, bis ein Lesevorgang einen Stand liefert, der nach ihr
// begann. Eine ältere Leseantwort überschreibt ihn nie, egal in welcher
// Reihenfolge die Antworten eintreffen.
describe("useStaffOnboarding", () => {
  // Fake-Timer lassen das Dedupe-Fenster ohne echte Sekunden ablaufen;
  // Testing Library rückt sie in waitFor über `jest` vor.
  beforeAll(() => {
    vi.useFakeTimers();
    vi.stubGlobal("jest", {
      advanceTimersByTime: (ms: number) => vi.advanceTimersByTime(ms),
    });
  });
  afterAll(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });
  beforeEach(() => {
    vi.clearAllMocks();
    resumeSWRSession();
    resetSWRScope({ revalidate: false });
  });
  afterEach(() => {
    act(() => {
      resumeSWRSession();
      resetSWRScope({ revalidate: false });
    });
  });

  it("keeps a confirmed save when an older read answers after it", async () => {
    const view = await loaded();
    const read = deferred();
    mocks.fetch.mockReturnValueOnce(read.promise);
    await refocus(2);

    await act(async () => {
      await view.result.current.replace(done);
    });
    expect(view.result.current.state).toEqual(done);

    // Die Leseantwort begann vor der Bestätigung und trägt den alten Stand.
    await act(async () => {
      read.resolve(open);
      await read.promise;
    });
    expect(view.result.current.state).toEqual(done);
  });

  it("keeps a confirmed save when an older read answers before it", async () => {
    const view = await loaded();
    const read = deferred();
    mocks.fetch.mockReturnValueOnce(read.promise);
    await refocus(2);

    await act(async () => {
      read.resolve(open);
      await read.promise;
    });
    await act(async () => {
      await view.result.current.replace(done);
    });

    expect(view.result.current.state).toEqual(done);
  });

  it("shows a read that started after the save", async () => {
    const view = await loaded();
    await act(async () => {
      await view.result.current.replace(done);
    });

    // Danach geändert, etwa in einem anderen Tab: Der neue Lesevorgang gilt.
    const later = { ...done, skippedSteps: ["calendar" as const] };
    mocks.fetch.mockResolvedValueOnce(later);
    await refocus(2);

    await waitFor(() => expect(view.result.current.state).toEqual(later));
  });
});
