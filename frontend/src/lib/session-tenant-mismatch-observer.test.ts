import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { getSessionMock } = vi.hoisted(() => ({ getSessionMock: vi.fn() }));
vi.mock("next-auth/react", () => ({ getSession: getSessionMock }));

const { installSessionTenantMismatchObserver } =
  await import("./session-tenant-mismatch-observer");
const { onSessionTenantMismatch } = await import("./api-error");
const { bindSessionAccount, SESSION_ACCOUNT_HEADER } =
  await import("./session-account");
const { authFetch, fetchWithRetry } = await import("./api-helpers");
const { getCachedSession, primeSessionCache } = await import("./session-cache");
const { listAvailableTenants } = await import("./tenant-api");
const { timeTrackingService } = await import("./time-tracking-api");

/**
 * #5130, review of #5147: every fetch path of the tab reports that the BFF
 * refused the session, also the clients that read only `response.ok` and
 * never build an ApiError from the body.
 */

const refusal = () =>
  Response.json(
    { status: 409, code: "identity.session_tenant_mismatch" },
    { status: 409 },
  );

const fetchMock = vi.fn<typeof fetch>();
let restoreObserver: () => void = () => undefined;
let unsubscribe: () => void = () => undefined;
const listener = vi.fn();

beforeEach(() => {
  fetchMock.mockReset();
  listener.mockReset();
  getSessionMock.mockReset();
  getSessionMock.mockResolvedValue({
    user: { id: "7", tenantId: 1 },
    expires: "2099-01-01T00:00:00.000Z",
  });
  // The provider's answer re-arms the session cache's one report per
  // mismatch; between tests nobody else answers.
  primeSessionCache(null);
  vi.stubGlobal("fetch", fetchMock);
  restoreObserver = installSessionTenantMismatchObserver();
  unsubscribe = onSessionTenantMismatch(listener);
});

afterEach(() => {
  unsubscribe();
  restoreObserver();
  vi.unstubAllGlobals();
});

describe("the session tenant refusal reaches TenantGuard from every client", () => {
  it.each([
    ["a plain fetch that reads only ok", () => fetch("/api/students")],
    ["a sessionFetch client that answers []", () => listAvailableTenants()],
    ["authFetch", () => authFetch("/api/students")],
    ["fetchWithRetry", () => fetchWithRetry("/api/students")],
    ["the time tracking client", () => timeTrackingService.checkIn("present")],
  ])("%s", async (_client, send) => {
    fetchMock.mockResolvedValue(refusal());

    await send().catch(() => undefined);

    expect(listener).toHaveBeenCalled();
  });

  it("leaves the caller the body to show the refusal's own text", async () => {
    fetchMock.mockResolvedValue(refusal());

    await expect(timeTrackingService.checkIn("present")).rejects.toMatchObject({
      status: 409,
      code: "identity.session_tenant_mismatch",
    });
  });

  it("drops the cached session of a client that never reads the body", async () => {
    fetchMock.mockResolvedValue(Response.json({ data: [] }));
    await listAvailableTenants();
    expect(getSessionMock).toHaveBeenCalledTimes(1);

    fetchMock.mockResolvedValue(refusal());
    await listAvailableTenants();
    await getCachedSession();

    expect(getSessionMock).toHaveBeenCalledTimes(2);
  });

  it.each([
    ["another conflict", "/api/students", "care.announcement_stale"],
    [
      "a foreign origin",
      "https://example.org/api/students",
      "identity.session_tenant_mismatch",
    ],
  ])("ignores %s", async (_case, url, code) => {
    fetchMock.mockResolvedValue(
      Response.json({ status: 409, code }, { status: 409 }),
    );

    const response = await fetch(url);

    expect(response.status).toBe(409);
    expect(listener).not.toHaveBeenCalled();
  });

  it("keeps a single observer when installed twice", () => {
    expect(installSessionTenantMismatchObserver()).toBe(restoreObserver);
  });
});

/**
 * #5183: every request to the tab's own BFF names the account the tab holds
 * data for; the BFF's refusal of another account reaches the same path as
 * the school refusal.
 */
describe("the tab's account on every request to its own BFF", () => {
  const accountRefusal = () =>
    Response.json(
      { status: 409, code: "identity.session_account_mismatch" },
      { status: 409 },
    );

  function sentAccount(): string | null {
    const [input, init] = fetchMock.mock.calls.at(-1)!;
    const headers = new Headers(
      init?.headers ?? (input instanceof Request ? input.headers : undefined),
    );
    return headers.get(SESSION_ACCOUNT_HEADER);
  }

  afterEach(() => {
    bindSessionAccount(null);
  });

  it.each([
    ["a plain fetch", () => fetch("/api/students")],
    [
      "a fetch with its own headers",
      () => fetch("/api/students", { headers: { "X-Other": "1" } }),
    ],
    ["a Request object", () => fetch(new Request(`${location.origin}/api/x`))],
    ["a sessionFetch client", () => listAvailableTenants()],
    ["authFetch", () => authFetch("/api/students")],
  ])("names the bound account on %s", async (_client, send) => {
    bindSessionAccount("7");
    fetchMock.mockResolvedValue(Response.json({ data: [] }));

    await send().catch(() => undefined);

    expect(sentAccount()).toBe("7");
  });

  it("keeps the caller's other headers", async () => {
    bindSessionAccount("7");
    fetchMock.mockResolvedValue(Response.json({ data: [] }));

    await fetch("/api/students", { headers: { "X-Other": "1" } });

    const init = fetchMock.mock.calls.at(-1)![1];
    expect(new Headers(init?.headers).get("X-Other")).toBe("1");
  });

  it("names no account while the tab shows none", async () => {
    fetchMock.mockResolvedValue(Response.json({ data: [] }));

    await fetch("/api/students");

    expect(sentAccount()).toBeNull();
  });

  it.each([
    ["another origin", "https://example.org/api/students"],
    ["a page of this origin", "/de/hilfe"],
  ])("sends no account to %s", async (_case, url) => {
    bindSessionAccount("7");
    fetchMock.mockResolvedValue(Response.json({}));

    await fetch(url);

    expect(sentAccount()).toBeNull();
  });

  it.each([
    ["a plain fetch that reads only ok", () => fetch("/api/students")],
    ["a sessionFetch client", () => listAvailableTenants()],
    ["the time tracking client", () => timeTrackingService.checkIn("present")],
  ])(
    "reports the refusal of another account from %s",
    async (_client, send) => {
      bindSessionAccount("7");
      fetchMock.mockResolvedValue(accountRefusal());

      await send().catch(() => undefined);

      expect(listener).toHaveBeenCalledWith(
        "identity.session_account_mismatch",
      );
    },
  );

  it("drops the cached session so the provider reads the new account", async () => {
    bindSessionAccount("7");
    fetchMock.mockResolvedValue(Response.json({ data: [] }));
    await listAvailableTenants();
    expect(getSessionMock).toHaveBeenCalledTimes(1);

    fetchMock.mockResolvedValue(accountRefusal());
    await listAvailableTenants();
    await getCachedSession();

    expect(getSessionMock).toHaveBeenCalledTimes(2);
  });
});
