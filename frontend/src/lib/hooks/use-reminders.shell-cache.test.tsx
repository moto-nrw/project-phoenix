import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  ShellCacheSeeder,
  shellOwnedEntries,
  shellSwrConfig,
} from "~/app/[tenant]/providers";
import type { RemindersResult } from "~/lib/reminders-api";
import type { ShellBootstrap } from "~/lib/shell-seed";
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
// SWRs gemeinsamer Cache wie in der App; jeder Test nimmt eine eigene
// Schule, damit kein Eintrag eines anderen Tests mitzählt.
const tenantState = vi.hoisted(() => ({ slug: "schule-0" }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => tenantState.slug,
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

function Shell({
  reminders,
  onHome,
}: Readonly<{ reminders: RemindersResult | null; onHome: boolean }>) {
  // Wie TenantProviders: Seed in fallback und cacheData, die Einträge der
  // Hülle vor dem ersten Leser im Cache.
  const shell: ShellBootstrap = {
    accountId: "7",
    counts: {},
    ...(reminders ? { reminders } : {}),
  };
  return (
    <SWRConfig value={shellSwrConfig(shell, tenantState.slug)}>
      <ShellCacheSeeder entries={shellOwnedEntries(shell, tenantState.slug)} />
      <Reminders name="bell" />
      {onHome && <Reminders name="home-block" />}
    </SWRConfig>
  );
}

let nextSchool = 0;
beforeEach(() => {
  nextSchool += 1;
  tenantState.slug = `schule-${nextSchool}`;
  fetchReminders.mockReset();
  fetchReminders.mockResolvedValue({ ...SHELL_REMINDERS, count: 2 });
});

describe("useReminders with the shell cache", () => {
  it("does not load again when the home block opens next to the bell", async () => {
    // Der Wechsel zur Startseite kommt später als SWRs Deduplizierungsfenster
    // (5 s); erst danach zählt eine erneute Abfrage als Request.
    vi.useFakeTimers();
    const { rerender } = render(
      <Shell reminders={SHELL_REMINDERS} onHome={false} />,
    );
    expect(screen.getByTestId("bell")).toHaveTextContent("1");
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    rerender(<Shell reminders={SHELL_REMINDERS} onHome />);
    await act(() => vi.advanceTimersByTimeAsync(1_000));

    expect(screen.getByTestId("home-block")).toHaveTextContent("1");
    expect(screen.getByTestId("bell")).toHaveTextContent("1");
    expect(fetchReminders).not.toHaveBeenCalled();
  });

  it("asks the server on the first refresh after the seed", async () => {
    render(
      <Shell
        reminders={{ ...SHELL_REMINDERS, enabled: true }}
        onHome={false}
      />,
    );
    expect(screen.getByTestId("bell")).toHaveTextContent("1");

    act(() => {
      window.dispatchEvent(new CustomEvent("phoenix:reminders-stale"));
    });

    await waitFor(() =>
      expect(screen.getByTestId("bell")).toHaveTextContent("2"),
    );
    expect(fetchReminders).toHaveBeenCalledTimes(1);
  });

  it("loads once when the shell holds no reminders", async () => {
    render(<Shell reminders={null} onHome />);

    await waitFor(() =>
      expect(screen.getByTestId("home-block")).toHaveTextContent("2"),
    );
    expect(screen.getByTestId("bell")).toHaveTextContent("2");
    expect(fetchReminders).toHaveBeenCalledTimes(1);
  });
});
