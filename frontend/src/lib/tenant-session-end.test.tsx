import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TeacherShellProvider, useShellAuth } from "./shell-auth-context";
import { reloginAtTenant } from "~/components/tenant/tenant-relogin";

const preview = vi.hoisted(() => ({ active: false, update: vi.fn() }));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7", roles: [], isPreview: preview.active } },
    status: "authenticated",
    update: preview.update,
  }),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({ show: vi.fn() }),
}));
vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({ profile: null }),
}));
vi.mock("~/lib/swr", () => ({ mutate: vi.fn() }));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TENANT_DOMAIN: "localhost" },
}));

const originalLocation = window.location;
const originalServiceWorker = Object.getOwnPropertyDescriptor(
  navigator,
  "serviceWorker",
);
const getSubscription = vi.fn();
const getRegistration = vi.fn();
const unsubscribe = vi.fn();
const fetchMock = vi.fn();
const cookieDeletes = () =>
  fetchMock.mock.calls.filter(([url]) => url === "/api/auth/signout");
const revocations = () =>
  fetchMock.mock.calls.filter(([url]) => url === "/api/auth/logout");
const device = () => ({
  endpoint: "https://push.example.invalid/device",
  unsubscribe,
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function csrfResponse() {
  return Response.json({ csrfToken: crypto.randomUUID() });
}
function ok() {
  return new Response(null, { status: 204 });
}
function restoreFromBFCache() {
  const event = new Event("pageshow");
  Object.defineProperty(event, "persisted", { value: true });
  window.dispatchEvent(event);
}

function startMenuLogout() {
  const { result } = renderHook(() => useShellAuth(), {
    wrapper: ({ children }: { children: ReactNode }) => (
      <TeacherShellProvider>{children}</TeacherShellProvider>
    ),
  });
  return result.current.logout();
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.resetAllMocks();
  preview.active = false;
  preview.update.mockResolvedValue({ user: {} });
  Object.defineProperty(window, "location", {
    configurable: true,
    value: {
      protocol: "http:",
      port: "",
      href: "http://school-a.localhost/home",
    },
  });
  Object.defineProperty(navigator, "serviceWorker", {
    configurable: true,
    value: { getRegistration },
  });
  vi.stubGlobal("PushManager", class {});
  vi.stubGlobal("Notification", {});
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockImplementation(async (url) =>
    url === "/api/auth/csrf"
      ? csrfResponse()
      : url === "/api/auth/signout"
        ? Response.json({})
        : ok(),
  );
  getRegistration.mockResolvedValue({ pushManager: { getSubscription } });
  getSubscription.mockImplementation(async () => device());
  unsubscribe.mockResolvedValue(true);
});

afterEach(() => {
  window.dispatchEvent(new Event("pagehide"));
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  if (originalServiceWorker)
    Object.defineProperty(navigator, "serviceWorker", originalServiceWorker);
  else Reflect.deleteProperty(navigator, "serviceWorker");
  Object.defineProperty(window, "location", {
    configurable: true,
    value: originalLocation,
  });
});

describe.each([
  ["menu", startMenuLogout, "/"],
  ["relogin", () => reloginAtTenant("school-b"), "http://school-b.localhost/"],
] as const)("tenant %s logout", (_name, start, destination) => {
  it.each(["registration", "subscription", "DELETE", "unsubscribe"])(
    "never waits for hanging push %s to delete the cookie",
    async (stage) => {
      const cookie = deferred<Response>();
      const hanging = new Promise(() => undefined);
      if (stage === "registration") getRegistration.mockReturnValue(hanging);
      if (stage === "subscription") getSubscription.mockReturnValue(hanging);
      if (stage === "unsubscribe") unsubscribe.mockReturnValue(hanging);
      fetchMock.mockImplementation(async (url, init) => {
        if (url === "/api/auth/csrf") return csrfResponse();
        if (url === "/api/auth/signout") return cookie.promise;
        return stage === "DELETE" && init.method === "DELETE" ? hanging : ok();
      });
      const logout = start();
      await vi.advanceTimersByTimeAsync(0);
      expect(revocations()).toHaveLength(1);
      expect(cookieDeletes()).toHaveLength(1);
      cookie.resolve(Response.json({}));
      await logout;
      expect(window.location.href).toBe(destination);
      const deletion = fetchMock.mock.calls.find(
        ([, init]) => init?.method === "DELETE",
      );
      if (stage === "DELETE") expect(deletion?.[1].signal.aborted).toBe(true);
      expect(vi.getTimerCount()).toBe(0);
    },
  );

  it("uses one five-second cleanup budget while cookie deletion is pending", async () => {
    const cookie = deferred<Response>();
    const lookup = deferred<unknown>();
    getSubscription.mockReturnValue(lookup.promise);
    fetchMock.mockImplementation(async (url) =>
      url === "/api/auth/csrf"
        ? csrfResponse()
        : url === "/api/auth/signout"
          ? cookie.promise
          : ok(),
    );
    const logout = start();
    await vi.advanceTimersByTimeAsync(4000);
    lookup.resolve(device());
    unsubscribe.mockReturnValue(new Promise(() => undefined));
    await vi.advanceTimersByTimeAsync(999);
    expect(unsubscribe).toHaveBeenCalledOnce();
    expect(vi.getTimerCount()).toBe(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(vi.getTimerCount()).toBe(0);
    cookie.resolve(Response.json({}));
    await logout;
    expect(window.location.href).toBe(destination);
  });

  it.each(["success", "lookup error", "DELETE error", "unsubscribe error"])(
    "finishes on cleanup %s",
    async (outcome) => {
      if (outcome === "lookup error")
        getSubscription.mockRejectedValue(new Error("lookup failed"));
      if (outcome === "DELETE error")
        fetchMock.mockImplementation(async (url, init) => {
          if (init?.method === "DELETE") throw new Error("offline");
          return url === "/api/auth/csrf"
            ? csrfResponse()
            : url === "/api/auth/signout"
              ? Response.json({})
              : ok();
        });
      if (outcome === "unsubscribe error")
        unsubscribe.mockRejectedValue(new Error("removal failed"));
      await start();
      expect(cookieDeletes()).toHaveLength(1);
      expect(window.location.href).toBe(destination);
      expect(vi.getTimerCount()).toBe(0);
    },
  );

  it.each([
    "lookup",
    "DELETE",
    "CSRF",
    "cookie response",
    "revocation",
    "pagehide",
    "owner change",
  ])(
    "invalidates A on bfcache restore during %s without releasing B's slot",
    async (stage) => {
      const lookup = deferred<unknown>();
      const oldResponse = deferred<Response>();
      const cookie = deferred<Response>();
      const firstCsrf = deferred<Response>();
      let csrfRequests = 0;
      let signouts = 0;
      getSubscription.mockReturnValueOnce(
        stage === "lookup" ? lookup.promise : Promise.resolve(device()),
      );
      fetchMock.mockImplementation(async (url, init) => {
        if (
          url === "/api/auth/logout" &&
          stage === "revocation" &&
          revocations().length === 1
        )
          return oldResponse.promise;
        if (init?.method === "DELETE" && stage === "DELETE")
          return oldResponse.promise;
        if (url === "/api/auth/csrf")
          return ++csrfRequests === 1 && stage !== "revocation"
            ? firstCsrf.promise
            : csrfResponse();
        if (url === "/api/auth/signout")
          return ++signouts === 1 && stage === "cookie response"
            ? oldResponse.promise
            : cookie.promise;
        return ok();
      });
      const first = start();
      await vi.advanceTimersByTimeAsync(0);
      if (stage === "cookie response") {
        firstCsrf.resolve(csrfResponse());
        await vi.advanceTimersByTimeAsync(0);
      }
      expect(revocations()).toHaveLength(1);
      const removedBeforeReset = unsubscribe.mock.calls.length;
      if (stage === "pagehide") window.dispatchEvent(new Event("pagehide"));
      else if (stage === "owner change")
        window.dispatchEvent(
          new StorageEvent("storage", {
            key: "moto:personal-storage-owner",
            newValue: "2:8",
            storageArea: localStorage,
          }),
        );
      else restoreFromBFCache();
      getSubscription.mockReset().mockResolvedValue(null);
      const second = start();
      await vi.advanceTimersByTimeAsync(0);
      lookup.resolve(device());
      firstCsrf.resolve(csrfResponse());
      oldResponse.resolve(ok());
      await first;
      await vi.advanceTimersByTimeAsync(5000);
      expect(start()).toBe(second);
      expect(revocations()).toHaveLength(2);
      // A may already have sent its POST; it must never originate a new one
      // after reset, nor navigate when that old response arrives.
      expect(cookieDeletes()).toHaveLength(stage === "cookie response" ? 2 : 1);
      expect(window.location.href).toBe("http://school-a.localhost/home");
      expect(unsubscribe).toHaveBeenCalledTimes(removedBeforeReset);
      cookie.resolve(Response.json({}));
      await second;
      expect(window.location.href).toBe(destination);
      expect(vi.getTimerCount()).toBe(0);
    },
  );

  it("uses the Auth.js CSRF contract and the current attempt's callback", async () => {
    const csrfToken = crypto.randomUUID();
    fetchMock.mockImplementation(async (url) =>
      url === "/api/auth/csrf"
        ? Response.json({ csrfToken })
        : url === "/api/auth/signout"
          ? Response.json({})
          : ok(),
    );
    await start();
    const [, request] = cookieDeletes()[0]!;
    expect(request.method).toBe("POST");
    expect(request.headers).toEqual({
      "Content-Type": "application/x-www-form-urlencoded",
      "X-Auth-Return-Redirect": "1",
    });
    expect(request.body.get("csrfToken")).toBe(csrfToken);
    expect(request.body.get("callbackUrl")).toBe(destination);
    expect(window.location.href).toBe(destination);
  });

  it("allows retry after a failed cookie deletion without late cleanup effects", async () => {
    const lookup = deferred<unknown>();
    getSubscription.mockReturnValueOnce(lookup.promise);
    fetchMock.mockImplementation(async (url) => {
      if (url === "/api/auth/csrf") return csrfResponse();
      if (url === "/api/auth/signout")
        throw new TypeError("cookie deletion failed");
      return ok();
    });
    await expect(start()).rejects.toThrow("cookie deletion failed");
    expect(window.location.href).toBe("http://school-a.localhost/home");
    getSubscription.mockResolvedValue(null);
    fetchMock.mockImplementation(async (url) =>
      url === "/api/auth/csrf"
        ? csrfResponse()
        : url === "/api/auth/signout"
          ? Response.json({})
          : ok(),
    );
    await start();
    lookup.resolve(device());
    await vi.advanceTimersByTimeAsync(0);
    expect(cookieDeletes()).toHaveLength(2);
    expect(unsubscribe).not.toHaveBeenCalled();
    expect(window.location.href).toBe(destination);
  });
});

it.each([true, false])(
  "coalesces menu and relogin clicks (menu first: %s)",
  async (menuFirst) => {
    const first = menuFirst ? startMenuLogout() : reloginAtTenant("school-b");
    const second = menuFirst ? reloginAtTenant("school-b") : startMenuLogout();
    expect(first).toBe(second);
    await Promise.all([first, second]);
    expect(revocations()).toHaveLength(1);
    expect(cookieDeletes()).toHaveLength(1);
  },
);

it("does not restore a preview or revoke a session after a cancelled audit resumes", async () => {
  preview.active = true;
  const audit = deferred<Response>();
  fetchMock.mockImplementation(async (url) =>
    url === "/api/auth/staff-preview/end"
      ? audit.promise
      : url === "/api/auth/csrf"
        ? csrfResponse()
        : url === "/api/auth/signout"
          ? Response.json({})
          : ok(),
  );
  const first = startMenuLogout();
  await vi.advanceTimersByTimeAsync(0);
  expect(fetchMock).toHaveBeenCalledWith(
    "/api/auth/staff-preview/end",
    expect.anything(),
  );
  restoreFromBFCache();
  preview.active = false;
  await startMenuLogout();
  audit.resolve(ok());
  await first;
  expect(revocations()).toHaveLength(1);
  expect(cookieDeletes()).toHaveLength(1);
  expect(preview.update).not.toHaveBeenCalled();
  expect(vi.getTimerCount()).toBe(0);
});
