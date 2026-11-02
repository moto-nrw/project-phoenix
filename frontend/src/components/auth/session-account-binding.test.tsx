import { act, render, screen, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The real SWR: the test follows a key from one account to the next.
vi.unmock("swr");

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));
vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
  getSession: vi.fn(),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

// Every message the tab sends to its SessionProvider (next-auth channel).
const providerWakeUps: unknown[] = [];
class RecordingChannel {
  constructor(readonly name: string) {}
  postMessage(message: unknown) {
    if (this.name === "next-auth") providerWakeUps.push(message);
  }
  close() {}
}

const fetchMock = vi.fn<typeof fetch>();
vi.stubGlobal("fetch", fetchMock);
vi.stubGlobal("BroadcastChannel", RecordingChannel);

// Loaded after the stubs: the binding installs the fetch observer on load.
const { mutate } = await import("~/lib/swr");
const { SessionAccountBinding } = await import("./session-account-binding");
const { SWRScopeBoundary } = await import("~/lib/swr/scope-boundary");
const { useSWRAuth } = await import("~/lib/swr/hooks");
const { resetSWRScope, resumeSWRSession } = await import("~/lib/swr/scope");
const { apiErrorFromResponse, onSessionTenantMismatch } =
  await import("~/lib/api-error");
const { bindSessionAccount, boundSessionAccount, SESSION_ACCOUNT_HEADER } =
  await import("~/lib/session-account");
const { primeSessionCache } = await import("~/lib/session-cache");

function signedIn(id: string) {
  return {
    data: { user: { id, tenantId: 1 }, expires: "2099-01-01" } as Session,
    status: "authenticated" as const,
  };
}

function sentAccounts(): (string | null)[] {
  return fetchMock.mock.calls.map(([, init]) =>
    new Headers(init?.headers).get(SESSION_ACCOUNT_HEADER),
  );
}

/** What the BFF answers: the data of the cookie's account, or the refusal. */
let cookieAccount = "A";
function bff(): Promise<Response> {
  const [, init] = fetchMock.mock.calls.at(-1)!;
  const expected = new Headers(init?.headers).get(SESSION_ACCOUNT_HEADER);
  if (expected !== null && expected !== cookieAccount) {
    return Promise.resolve(
      Response.json(
        { status: 409, code: "identity.session_account_mismatch" },
        { status: 409 },
      ),
    );
  }
  return Promise.resolve(
    Response.json({ list: `Kinder von ${cookieAccount}` }),
  );
}

function Students() {
  const { data, error } = useSWRAuth("students", async () => {
    const response = await fetch("/api/students");
    if (!response.ok) throw await apiErrorFromResponse(response, "refused");
    return ((await response.json()) as { list: string }).list;
  });
  return (
    <>
      <p>{data ?? "leer"}</p>
      {error ? <p role="status">abgelehnt</p> : null}
    </>
  );
}

function Portal() {
  return (
    <>
      <SessionAccountBinding />
      <SWRScopeBoundary expectedTenantId={1} />
      <Students />
    </>
  );
}

const refusals = vi.fn();
let unsubscribe: () => void = () => undefined;

beforeEach(() => {
  cookieAccount = "A";
  providerWakeUps.length = 0;
  fetchMock.mockReset();
  fetchMock.mockImplementation(bff);
  refusals.mockReset();
  unsubscribe = onSessionTenantMismatch(refusals);
  primeSessionCache(null);
  resetSWRScope({ revalidate: false });
  mockUseSession.mockReturnValue(signedIn("A"));
});

afterEach(() => {
  unsubscribe();
  resumeSWRSession();
  bindSessionAccount(null);
});

describe("another account signs in in another tab (#5183)", () => {
  it("refuses the old tab's requests and drops its scope once the provider names the new account", async () => {
    const view = render(<Portal />);
    expect(await screen.findByText("Kinder von A")).toBeInTheDocument();
    expect(sentAccounts()).toEqual(["A"]);

    // Tab B signs in as B: the shared cookie holds B, this tab still shows A.
    cookieAccount = "B";
    await act(async () => {
      await mutate(() => true);
    });

    // The read went out as A and was refused; B's data never reached A's page.
    expect(sentAccounts().at(-1)).toBe("A");
    expect(await screen.findByRole("status")).toHaveTextContent("abgelehnt");
    expect(screen.getByText("Kinder von A")).toBeInTheDocument();
    expect(screen.queryByText("Kinder von B")).not.toBeInTheDocument();
    // The refusal wakes the provider, like the school refusal.
    expect(refusals).toHaveBeenCalledWith("identity.session_account_mismatch");
    expect(providerWakeUps).toHaveLength(1);

    // The provider answers with B: the scope of A goes, B loads as B.
    mockUseSession.mockReturnValue(signedIn("B"));
    view.rerender(<Portal />);

    expect(await screen.findByText("Kinder von B")).toBeInTheDocument();
    expect(screen.queryByText("Kinder von A")).not.toBeInTheDocument();
    expect(sentAccounts().at(-1)).toBe("B");
  });

  it("lets the same account through after it signed in again", async () => {
    const view = render(<Portal />);
    expect(await screen.findByText("Kinder von A")).toBeInTheDocument();

    mockUseSession.mockReturnValue(signedIn("A"));
    view.rerender(<Portal />);
    await act(async () => {
      await mutate(() => true);
    });

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    expect(sentAccounts()).toEqual(["A", "A"]);
    expect(refusals).not.toHaveBeenCalled();
    expect(screen.getByText("Kinder von A")).toBeInTheDocument();
  });
});

describe("SessionAccountBinding", () => {
  it.each([
    ["signed out", { data: null, status: "unauthenticated" as const }],
    [
      "a session without access",
      {
        data: {
          user: { id: "A" },
          expires: "2099-01-01",
          error: "RefreshTokenError",
        } as Session,
        status: "authenticated" as const,
      },
    ],
  ])("names no account when %s", (_case, session) => {
    bindSessionAccount("A");
    mockUseSession.mockReturnValue(session);

    render(<SessionAccountBinding />);

    expect(boundSessionAccount()).toBeNull();
  });

  it("keeps the account while the provider's first lookup runs", () => {
    bindSessionAccount("A");
    mockUseSession.mockReturnValue({ data: undefined, status: "loading" });

    render(<SessionAccountBinding />);

    expect(boundSessionAccount()).toBe("A");
  });
});
