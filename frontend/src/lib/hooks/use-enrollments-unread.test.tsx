import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { announceEnrollmentReadChange } from "~/lib/enrollment-unread-api";
import { useEnrollmentsUnread } from "./use-enrollments-unread";

// Seitenleiste, mobile Navigation und der Startseiten-Baustein „Offene
// Anfragen“ zeigen denselben Zähler. Beim ersten Aufruf liefen dafür drei
// gleiche Requests (#4989).

// Echtes SWR: gezählt wird, was die gemeinsame Deduplizierung übrig lässt.
vi.unmock("swr");

const sessionState = vi.hoisted(() => ({
  user: { id: "100", permissions: ["config:manage"] as string[] },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: sessionState.user, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ mode: "teacher" }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "schule-a",
  useModuleBooked: () => true,
}));

const UNREAD_URL = "/api/enrollment/admin/requests/unread-count";
const fetchMock = vi.fn<typeof fetch>();

function answer(count: number) {
  fetchMock.mockImplementation(async () =>
    Response.json({ data: { unread_count: count } }),
  );
}

function unreadRequests(): number {
  return fetchMock.mock.calls.filter(([url]) => String(url) === UNREAD_URL)
    .length;
}

function Badge({ name }: Readonly<{ name: string }>) {
  const { unreadCount } = useEnrollmentsUnread();
  return <span data-testid={name}>{unreadCount}</span>;
}

/** Die drei Stellen, die den Zähler auf der Startseite gleichzeitig zeigen. */
function Shell() {
  return (
    <>
      <Badge name="sidebar" />
      <Badge name="mobile-nav" />
      <Badge name="home-block" />
    </>
  );
}

// SWRs gemeinsamer Cache wie in der App: announceEnrollmentReadChange
// erreicht ihn über das globale mutate. Jeder Test nimmt ein eigenes Konto,
// damit kein Eintrag eines anderen Tests mitzählt.
let nextAccount = 100;
function signIn(permissions: string[] = ["config:manage"]) {
  nextAccount += 1;
  sessionState.user = { id: String(nextAccount), permissions };
}

function renderShell() {
  return render(<Shell />);
}

async function expectAllShow(count: string) {
  await waitFor(() =>
    expect(screen.getByTestId("home-block")).toHaveTextContent(count),
  );
  expect(screen.getByTestId("sidebar")).toHaveTextContent(count);
  expect(screen.getByTestId("mobile-nav")).toHaveTextContent(count);
}

beforeEach(() => {
  signIn();
  fetchMock.mockReset();
  answer(4);
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useEnrollmentsUnread", () => {
  it("loads the count once for every badge that shows it", async () => {
    renderShell();

    await expectAllShow("4");
    expect(unreadRequests()).toBe(1);
  });

  it("counts again once after a read change", async () => {
    renderShell();
    await expectAllShow("4");

    answer(3);
    act(() => announceEnrollmentReadChange());

    await expectAllShow("3");
    // Kurz warten: ein verzögerter Abruf je Badge käme sonst erst danach.
    await new Promise((resolve) => setTimeout(resolve, 400));
    expect(unreadRequests()).toBe(2);
  });

  it("never shows the count of the account that was signed in before", async () => {
    const { rerender } = renderShell();
    await expectAllShow("4");

    signIn();
    fetchMock.mockImplementation(() => new Promise<Response>(() => {}));
    rerender(<Shell />);

    await expectAllShow("0");
    expect(unreadRequests()).toBe(2);
  });

  it("asks nothing without config:manage", async () => {
    signIn(["users:read"]);
    renderShell();

    // Anker: die Badges sind gerendert und zeigen 0.
    await expectAllShow("0");
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(unreadRequests()).toBe(0);
  });
});
