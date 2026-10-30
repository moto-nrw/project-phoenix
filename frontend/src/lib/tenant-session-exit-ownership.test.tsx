import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { randomBytes } from "node:crypto";
import { act, renderHook } from "@testing-library/react";
import { reloginAtTenant } from "~/components/tenant/tenant-relogin";
import { isTenantSessionEnding } from "./tenant-session-end";
import { useNavigationGuard } from "./hooks/use-navigation-guard";
import { TeacherShellProvider, useShellAuth } from "./shell-auth-context";

vi.mock("~/lib/swr", () => ({ mutate: vi.fn().mockResolvedValue(undefined) }));
vi.mock("~/lib/push-api", () => ({ unsubscribePushSilently: vi.fn() }));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TENANT_DOMAIN: "mfa.localhost" },
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1", roles: [] } },
    status: "authenticated",
    update: vi.fn(),
  }),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({ show: vi.fn() }),
}));
vi.mock("~/lib/profile-context", () => ({
  useProfile: () => ({ profile: null }),
}));

describe("tenant exit ownership through real cookie deletion", () => {
  const location = window.location;
  let releaseCsrf: (response: Response) => void;
  let csrfStarted: Promise<void>;
  let csrfSignal: AbortSignal;
  let csrfToken: string;
  beforeEach(() => {
    csrfToken = randomBytes(24).toString("hex");
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        protocol: "http:",
        port: "13845",
        href: "http://school-a.mfa.localhost:13845/profile",
      },
    });
    let started: () => void;
    csrfStarted = new Promise<void>((resolve) => {
      started = resolve;
    });
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      if (input === "/api/auth/logout") return new Response("{}");
      if (input === "/api/auth/csrf") {
        if (!(init?.signal instanceof AbortSignal))
          throw new Error("missing cancellation signal");
        csrfSignal = init.signal;
        started();
        return new Promise<Response>((resolve) => {
          releaseCsrf = resolve;
        });
      }
      if (input === "/api/auth/signout")
        return Response.json({ url: "http://school-b.mfa.localhost:13845/" });
      throw new Error("unexpected fixture request");
    });
  });
  afterEach(() => {
    window.dispatchEvent(new Event("pagehide"));
    vi.restoreAllMocks();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: location,
    });
  });

  it("ignores another tab echoing cleared storage while CSRF is held", async () => {
    const flight = reloginAtTenant("school-b");
    await csrfStarted;
    try {
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: "moto:personal-storage-owner",
          newValue: null,
          storageArea: localStorage,
        }),
      );
      expect(isTenantSessionEnding()).toBe(true);
      expect(csrfSignal.aborted).toBe(false);
      releaseCsrf(Response.json({ csrfToken }));
      await vi.waitFor(() =>
        expect(window.location.href).toBe(
          "http://school-b.mfa.localhost:13845/",
        ),
      );
    } finally {
      releaseCsrf(Response.json({ csrfToken }));
      window.dispatchEvent(new Event("pagehide"));
      await flight;
    }
  });

  it("keeps ownership after assigning the destination until the old document leaves", async () => {
    const flight = reloginAtTenant("school-b");
    await csrfStarted;
    releaseCsrf(Response.json({ csrfToken }));
    try {
      await flight;
      expect(window.location.href).toBe("http://school-b.mfa.localhost:13845/");
      // Push cleanup must stop now, without releasing redirect ownership.
      expect(csrfSignal.aborted).toBe(true);
      expect(isTenantSessionEnding()).toBe(true);
      expect(reloginAtTenant("school-c")).toBe(flight);
      window.dispatchEvent(new Event("pagehide"));
      await flight;
      expect(isTenantSessionEnding()).toBe(false);
    } finally {
      window.dispatchEvent(new Event("pagehide"));
      await flight;
    }
  });

  it("still cancels a stale exit when another tab binds a new owner", async () => {
    const flight = reloginAtTenant("school-b");
    await csrfStarted;
    window.dispatchEvent(
      new StorageEvent("storage", {
        key: "moto:personal-storage-owner",
        newValue: "tenant:other-account",
        storageArea: localStorage,
      }),
    );
    expect(csrfSignal.aborted).toBe(true);
    releaseCsrf(Response.json({ csrfToken }));
    await flight;
    expect(isTenantSessionEnding()).toBe(false);
    expect(window.location.href).toBe(
      "http://school-a.mfa.localhost:13845/profile",
    );
  });

  function dirtyPage() {
    vi.spyOn(window.history, "pushState").mockImplementation(() => undefined);
    vi.spyOn(window.history, "go").mockImplementation(() => undefined);
    let href = window.location.href;
    let nativePrompts = 0;
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        protocol: "http:",
        port: "13845",
        get href() {
          return href;
        },
        set href(destination: string) {
          const unload = new Event("beforeunload", { cancelable: true });
          window.dispatchEvent(unload);
          if (unload.defaultPrevented) nativePrompts += 1;
          else href = destination;
        },
      },
    });
    const fetch = vi
      .mocked(globalThis.fetch)
      .mockImplementation(async (input) => {
        if (input === "/api/auth/csrf") return Response.json({ csrfToken });
        if (input === "/api/auth/signout")
          return Response.json({ url: "http://school-b.mfa.localhost:13845/" });
        if (input === "/api/auth/logout") return new Response("{}");
        throw new Error("unexpected fixture request");
      });
    const page = renderHook(() => useNavigationGuard(true));
    return {
      page,
      fetch,
      get href() {
        return href;
      },
      get nativePrompts() {
        return nativePrompts;
      },
    };
  }

  it.each(["menu", "relogin"])(
    "leaves a cancelled dirty-page %s exit unrevoked and permits a confirmed retry",
    async (entry) => {
      const dirty = dirtyPage();
      const { page, fetch } = dirty;
      const menu =
        entry === "menu"
          ? renderHook(() => useShellAuth(), { wrapper: TeacherShellProvider })
          : null;
      const start = () =>
        menu ? menu.result.current.logout() : reloginAtTenant("school-b");
      const destination =
        entry === "menu" ? "/" : "http://school-b.mfa.localhost:13845/";
      let flight!: Promise<void>;
      try {
        await act(async () => {
          flight = start();
        });
        // Reach either the old native cancellation or the guarded confirmation.
        await vi.waitFor(() =>
          expect(
            page.result.current.pendingHref !== null || dirty.nativePrompts > 0,
          ).toBe(true),
        );
        if (page.result.current.pendingHref !== null)
          act(() => page.result.current.cancelNavigation());
        await flight;
        expect(isTenantSessionEnding()).toBe(false);
        expect(fetch).toHaveBeenCalledTimes(0);
        expect(dirty.href).toBe("http://school-a.mfa.localhost:13845/profile");

        let retry!: Promise<void>;
        await act(async () => {
          retry = start();
        });
        expect(retry).not.toBe(flight);
        expect(page.result.current.pendingHref).toBe(destination);
        expect(isTenantSessionEnding()).toBe(false);
        await act(async () => {
          page.result.current.confirmNavigation();
          window.dispatchEvent(new PopStateEvent("popstate"));
        });
        await retry;
        expect(fetch).toHaveBeenCalledTimes(3);
        expect(dirty.nativePrompts).toBe(0);
        expect(dirty.href).toBe(destination);
        expect(isTenantSessionEnding()).toBe(true);
        window.dispatchEvent(new Event("pagehide"));
        expect(isTenantSessionEnding()).toBe(false);
        const stillDirty = new Event("beforeunload", { cancelable: true });
        window.dispatchEvent(stillDirty);
        expect(stillDirty.defaultPrevented).toBe(true);
      } finally {
        page.unmount();
        menu?.unmount();
        window.dispatchEvent(new Event("pagehide"));
        await flight;
      }
    },
  );

  it("restores dirty-page protection when a confirmed logout fails", async () => {
    const { page, fetch } = dirtyPage();
    fetch.mockResolvedValue(new Response("{}", { status: 503 }));
    let flight!: Promise<void>;
    try {
      await act(async () => {
        flight = reloginAtTenant("school-b");
      });
      expect(page.result.current.pendingHref).toBe(
        "http://school-b.mfa.localhost:13845/",
      );
      const failure = expect(flight).rejects.toMatchObject({ status: 503 });
      await act(async () => {
        page.result.current.confirmNavigation();
        window.dispatchEvent(new PopStateEvent("popstate"));
      });
      await failure;
      expect(fetch).toHaveBeenCalledTimes(1);
      expect(isTenantSessionEnding()).toBe(false);
      const unload = new Event("beforeunload", { cancelable: true });
      window.dispatchEvent(unload);
      expect(unload.defaultPrevented).toBe(true);
    } finally {
      page.unmount();
      window.dispatchEvent(new Event("pagehide"));
    }
  });

  it("cancels an unconfirmed exit when its dirty-page guard unmounts", async () => {
    const { page, fetch } = dirtyPage();
    let flight!: Promise<void>;
    await act(async () => {
      flight = reloginAtTenant("school-b");
    });
    expect(page.result.current.pendingHref).toBe(
      "http://school-b.mfa.localhost:13845/",
    );
    page.unmount();
    await flight;
    expect(fetch).toHaveBeenCalledTimes(0);
    expect(isTenantSessionEnding()).toBe(false);
  });
});
