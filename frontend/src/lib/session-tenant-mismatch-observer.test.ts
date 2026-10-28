import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { getSessionMock } = vi.hoisted(() => ({ getSessionMock: vi.fn() }));
vi.mock("next-auth/react", () => ({ getSession: getSessionMock }));

const { installSessionTenantMismatchObserver } =
  await import("./session-tenant-mismatch-observer");
const { onSessionTenantMismatch } = await import("./api-error");
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
