import { act, render, screen, waitFor } from "@testing-library/react";
import { StrictMode } from "react";
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

const sessionState = vi.hoisted(() => ({
  permissions: ["users:read"] as string[],
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", permissions: sessionState.permissions },
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
  sessionState.permissions = ["users:read"];
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

// Ein SSE-Ereignis lädt die geteilte Ressource einmal, nicht einmal je Leser
// (#5129). Auf der Startseite lesen Glocke, Baustein und die Seite selbst.
const READERS = ["bell", "home-block", "home-page"] as const;

function Readers({
  reminders,
  count,
}: Readonly<{ reminders: RemindersResult; count: number }>) {
  const shell: ShellBootstrap = { accountId: "7", counts: {}, reminders };
  return (
    <SWRConfig value={shellSwrConfig(shell, tenantState.slug)}>
      <ShellCacheSeeder entries={shellOwnedEntries(shell, tenantState.slug)} />
      {READERS.slice(0, count).map((name) => (
        <Reminders key={name} name={name} />
      ))}
    </SWRConfig>
  );
}

const ENABLED: RemindersResult = { ...SHELL_REMINDERS, enabled: true };

function remindersStale() {
  act(() => {
    window.dispatchEvent(new CustomEvent("phoenix:reminders-stale"));
  });
}

function deferred() {
  let resolve!: (value: RemindersResult) => void;
  const promise = new Promise<RemindersResult>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

async function expectAllReaders(count: number, readers: number) {
  for (const name of READERS.slice(0, readers)) {
    await waitFor(() =>
      expect(screen.getByTestId(name)).toHaveTextContent(String(count)),
    );
  }
}

describe("useReminders on phoenix:reminders-stale", () => {
  beforeEach(() => {
    // Die Antwort lässt die Erinnerungen eingeschaltet; sonst endet das
    // Zuhören nach dem ersten Abruf.
    fetchReminders.mockResolvedValue({ ...ENABLED, count: 2 });
  });

  it.each([1, 2, 3])(
    "loads once for one event with %i readers",
    async (readers) => {
      // Die Antwort bleibt offen, bis alle Leser das Ereignis gesehen haben:
      // so zählt jeder parallel gestartete Request.
      const answer = deferred();
      fetchReminders.mockReturnValueOnce(answer.promise);
      render(<Readers reminders={ENABLED} count={readers} />);
      await expectAllReaders(1, readers);

      remindersStale();
      await act(async () => {
        answer.resolve({ ...ENABLED, count: 2 });
        await answer.promise;
      });

      await expectAllReaders(2, readers);
      expect(fetchReminders).toHaveBeenCalledTimes(1);
    },
  );

  it("loads once again for a later event", async () => {
    render(<Readers reminders={ENABLED} count={3} />);
    remindersStale();
    await expectAllReaders(2, 3);

    fetchReminders.mockResolvedValue({ ...ENABLED, count: 3 });
    remindersStale();

    await expectAllReaders(3, 3);
    expect(fetchReminders).toHaveBeenCalledTimes(2);
  });

  it("keeps a change from an event during a running request", async () => {
    // Die erste Antwort ist älter als das zweite Ereignis und kommt zuletzt
    // an; sichtbar bleibt der Stand nach dem zweiten Ereignis.
    const first = deferred();
    const second = deferred();
    fetchReminders
      .mockReturnValueOnce(first.promise)
      .mockReturnValueOnce(second.promise);
    render(<Readers reminders={ENABLED} count={3} />);

    remindersStale();
    remindersStale();
    expect(fetchReminders).toHaveBeenCalledTimes(2);

    await act(async () => {
      second.resolve({ ...ENABLED, count: 3 });
      await second.promise;
    });
    await act(async () => {
      first.resolve({ ...ENABLED, count: 2 });
      await first.promise;
    });

    await expectAllReaders(3, 3);
    expect(fetchReminders).toHaveBeenCalledTimes(2);
  });

  it("loads once under StrictMode", async () => {
    render(
      <StrictMode>
        <Readers reminders={ENABLED} count={3} />
      </StrictMode>,
    );
    remindersStale();

    await expectAllReaders(2, 3);
    expect(fetchReminders).toHaveBeenCalledTimes(1);
  });

  it("still loads for the reader that stays mounted", async () => {
    const { rerender } = render(<Readers reminders={ENABLED} count={3} />);
    rerender(<Readers reminders={ENABLED} count={1} />);

    remindersStale();

    await expectAllReaders(2, 1);
    expect(fetchReminders).toHaveBeenCalledTimes(1);
  });

  it("does not load while reminders are switched off", async () => {
    render(<Readers reminders={SHELL_REMINDERS} count={3} />);
    await expectAllReaders(1, 3);

    remindersStale();
    await act(async () => {
      await Promise.resolve();
    });

    expect(fetchReminders).not.toHaveBeenCalled();
  });

  it("does not load without users:read", async () => {
    sessionState.permissions = [];
    render(<Readers reminders={ENABLED} count={3} />);
    // Anker: ohne Recht liest kein Leser den Eintrag der Hülle.
    await expectAllReaders(0, 3);

    remindersStale();
    await act(async () => {
      await Promise.resolve();
    });

    expect(fetchReminders).not.toHaveBeenCalled();
  });

  it("loads once for the new school and drops the old school's answer", async () => {
    const old = deferred();
    fetchReminders.mockReturnValueOnce(old.promise);
    const { rerender } = render(<Readers reminders={ENABLED} count={3} />);
    remindersStale();
    expect(fetchReminders).toHaveBeenCalledTimes(1);

    tenantState.slug = "andere-schule";
    rerender(<Readers reminders={ENABLED} count={3} />);
    fetchReminders.mockResolvedValue({ ...ENABLED, count: 4 });
    remindersStale();
    await expectAllReaders(4, 3);

    await act(async () => {
      old.resolve({ ...ENABLED, count: 2 });
      await old.promise;
    });

    await expectAllReaders(4, 3);
    expect(fetchReminders).toHaveBeenCalledTimes(2);
  });
});

describe("useReminders at next_change_at", () => {
  it("loads once at the boundary with 3 readers", async () => {
    // Testuhr: 12:00 in Berlin. Die Grenze liegt vor dem 60-s-Takt, damit
    // nur der Timer abfragt. Die Antwort bleibt offen, bis jeder Timer
    // gefeuert hätte.
    vi.useFakeTimers();
    const reminders = {
      ...ENABLED,
      next_change_at: "2026-09-09T12:00:30+02:00",
    };
    const answer = deferred();
    fetchReminders.mockReturnValueOnce(answer.promise);
    render(<Readers reminders={reminders} count={3} />);

    await act(() => vi.advanceTimersByTimeAsync(31_000));
    expect(fetchReminders).not.toHaveBeenCalled();

    await act(() => vi.advanceTimersByTimeAsync(2_000));
    await act(async () => {
      answer.resolve({ ...ENABLED, count: 2 });
      await answer.promise;
    });

    for (const name of READERS) {
      expect(screen.getByTestId(name)).toHaveTextContent("2");
    }
    expect(fetchReminders).toHaveBeenCalledTimes(1);
  });
});
