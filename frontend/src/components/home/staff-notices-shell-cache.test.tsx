import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi, afterEach } from "vitest";

import {
  ShellCacheSeeder,
  shellOwnedEntries,
  shellSwrConfig,
} from "~/app/[tenant]/providers";
import { useStaffNoticesPending } from "~/lib/hooks/use-staff-notices-pending";
import type { ShellBootstrap } from "~/lib/shell-seed";
import {
  STAFF_NOTICES_REFRESH_EVENT,
  type StaffNotice,
} from "~/lib/staff-notices-api";
import { SWRConfig } from "~/lib/swr";
import { StaffNoticesBlock } from "./staff-notices-block";

// Badge der Seitenleiste und Startseiten-Baustein lesen einen Eintrag, den die
// App-Hülle aus dem Server-Seed anlegt. Beim Wechsel zur Startseite lief
// /api/staff-notices/today trotzdem erneut (#4989).

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
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ mode: "teacher" }),
}));
// SWRs gemeinsamer Cache wie in der App; jeder Test nimmt eine eigene
// Schule, damit kein Eintrag eines anderen Tests mitzählt.
const tenantState = vi.hoisted(() => ({ slug: "schule-0" }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => tenantState.slug,
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));
vi.mock("~/components/home/home-card", () => ({
  HOME_CARD_BODY: "",
  HomeCardIcon: () => null,
}));
vi.mock("~/components/home/home-card-rows", () => ({
  HomeCardLink: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  HomeMoreRow: () => null,
  useHomeCardRows: (items: readonly StaffNotice[]) => ({
    shown: items,
    hidden: 0,
  }),
}));
vi.mock("~/components/ui/section-card", () => ({
  SectionCard: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));
// Wie die echte Liste: nach der Kenntnisnahme den Eintrag neu laden und das
// Ereignis für die Zähler senden.
vi.mock("~/components/staff-notices/today-notice-list", () => ({
  TodayNoticeList: ({
    notices,
    onChanged,
  }: {
    notices: readonly StaffNotice[];
    onChanged: () => Promise<unknown>;
  }) => (
    <>
      {notices.map((n) => (
        <p key={n.id}>{n.title}</p>
      ))}
      <button
        type="button"
        onClick={() =>
          void onChanged().then(() =>
            window.dispatchEvent(new Event(STAFF_NOTICES_REFRESH_EVENT)),
          )
        }
      >
        Zur Kenntnis genommen
      </button>
    </>
  ),
}));

const PENDING: StaffNotice = {
  id: "1",
  title: "Elternabend am Donnerstag",
  body: "",
  priority: "important",
  audience: "all",
  valid_from: "2026-09-09",
  weekdays: [],
  week_pattern: 0,
  requires_acknowledgement: true,
  active: true,
};

const fetchMock = vi.fn<typeof fetch>();

function noticeRequests(): number {
  return fetchMock.mock.calls.filter(
    ([url]) => String(url) === "/api/staff-notices/today",
  ).length;
}

function Badge() {
  const { unreadCount } = useStaffNoticesPending();
  return <span data-testid="badge">{unreadCount}</span>;
}

function Shell({
  notices,
  onHome,
}: Readonly<{ notices: StaffNotice[]; onHome: boolean }>) {
  // Wie TenantProviders: Seed in fallback und cacheData, die Einträge der
  // Hülle vor dem ersten Leser im Cache.
  const shell: ShellBootstrap = {
    accountId: "7",
    counts: {},
    staffNoticesToday: notices,
  };
  return (
    <SWRConfig value={shellSwrConfig(shell, tenantState.slug)}>
      <ShellCacheSeeder entries={shellOwnedEntries(shell, tenantState.slug)} />
      <Badge />
      {onHome && <StaffNoticesBlock />}
    </SWRConfig>
  );
}

let nextSchool = 0;
beforeEach(() => {
  nextSchool += 1;
  tenantState.slug = `schule-${nextSchool}`;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async () =>
    Response.json({
      data: [{ ...PENDING, acknowledged_at: "2026-09-09T10:00:00Z" }],
    }),
  );
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("Tagesinformationen aus dem Eintrag der App-Hülle", () => {
  it("shows the shell's notices on the home page without loading them again", async () => {
    // Der Wechsel zur Startseite kommt später als SWRs Deduplizierungsfenster
    // (5 s); erst danach zählt eine erneute Abfrage als Request.
    vi.useFakeTimers();
    const { rerender } = render(<Shell notices={[PENDING]} onHome={false} />);
    expect(screen.getByTestId("badge")).toHaveTextContent("1");
    await act(() => vi.advanceTimersByTimeAsync(10_000));

    rerender(<Shell notices={[PENDING]} onHome />);
    await act(() => vi.advanceTimersByTimeAsync(1_000));

    expect(screen.getByText("Elternabend am Donnerstag")).toBeInTheDocument();
    expect(screen.getByTestId("badge")).toHaveTextContent("1");
    expect(noticeRequests()).toBe(0);
  });

  // Der erste Abruf nach dem Seed fragt den Server; ohne den Eintrag im
  // Cache hätte SWR den Seed zurückgegeben und das Badge stünde weiter.
  it("updates the badge with one request after an acknowledgement", async () => {
    render(<Shell notices={[PENDING]} onHome />);
    expect(screen.getByTestId("badge")).toHaveTextContent("1");

    fireEvent.click(
      screen.getByRole("button", { name: "Zur Kenntnis genommen" }),
    );

    await waitFor(() =>
      expect(screen.getByTestId("badge")).toHaveTextContent("0"),
    );
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(noticeRequests()).toBe(1);
  });
});
