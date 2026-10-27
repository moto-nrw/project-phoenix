import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { REMINDERS_SWR_KEY, type RemindersResult } from "~/lib/reminders-api";
import { SWRConfig } from "~/lib/swr";
import { useReminders } from "./use-reminders";

// Die Glocke der Hülle hält die Erinnerungen aktuell (Server-Seed, Abfrage im
// Takt, SSE, Fokus). Der Startseiten-Baustein liest dieselben Daten; beim
// Wechsel zur Startseite lief /api/reminders trotzdem erneut (#4989).

// Echtes SWR: gezählt wird, was die gemeinsame Deduplizierung übrig lässt.
vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", permissions: ["users:read"] },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "schule-a",
}));
const { fetchReminders } = vi.hoisted(() => ({ fetchReminders: vi.fn() }));
vi.mock("~/lib/reminders-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/reminders-api")>()),
  fetchReminders,
}));

const SHELL_REMINDERS: RemindersResult = {
  reminders: [
    {
      type: "pickup_upcoming",
      title: "Hannah",
      due_time: "15:30",
      minutes_away: 20,
    },
  ],
  count: 1,
  enabled: false,
};

function Reminders({ name }: Readonly<{ name: string }>) {
  const { count } = useReminders();
  return <span data-testid={name}>{count}</span>;
}

function Shell({ onHome }: Readonly<{ onHome: boolean }>) {
  return (
    <>
      <Reminders name="bell" />
      {onHome && <Reminders name="home-block" />}
    </>
  );
}

/** Wie TenantProviders: der Server-Seed liegt unter dem Tenant-Key. */
function seeded(data: RemindersResult | null) {
  const entries = data ? { [`schule-a:${REMINDERS_SWR_KEY}`]: data } : {};
  return {
    provider: () => new Map(),
    fallback: entries,
    cacheData: entries,
  };
}

beforeEach(() => {
  fetchReminders.mockReset();
  fetchReminders.mockResolvedValue({ ...SHELL_REMINDERS, count: 2 });
});

describe("useReminders with the shell cache", () => {
  it("does not load again when the home block opens next to the bell", async () => {
    // Der Wechsel zur Startseite kommt später als SWRs Deduplizierungsfenster
    // (5 s); erst danach zählt eine erneute Abfrage als Request.
    vi.useFakeTimers();
    const config = seeded(SHELL_REMINDERS);
    const { rerender } = render(
      <SWRConfig value={config}>
        <Shell onHome={false} />
      </SWRConfig>,
    );
    expect(screen.getByTestId("bell")).toHaveTextContent("1");
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    rerender(
      <SWRConfig value={config}>
        <Shell onHome />
      </SWRConfig>,
    );
    await act(() => vi.advanceTimersByTimeAsync(1_000));

    expect(screen.getByTestId("home-block")).toHaveTextContent("1");
    expect(screen.getByTestId("bell")).toHaveTextContent("1");
    expect(fetchReminders).not.toHaveBeenCalled();
  });

  it("loads once when the shell holds no reminders", async () => {
    render(
      <SWRConfig value={seeded(null)}>
        <Shell onHome />
      </SWRConfig>,
    );

    await waitFor(() =>
      expect(screen.getByTestId("home-block")).toHaveTextContent("2"),
    );
    expect(screen.getByTestId("bell")).toHaveTextContent("2");
    expect(fetchReminders).toHaveBeenCalledTimes(1);
  });
});
