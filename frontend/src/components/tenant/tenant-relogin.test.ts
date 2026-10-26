import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";

const { signOut, mutate, clearSessionCache, unsubscribePushSilently } =
  vi.hoisted(() => ({
    signOut: vi.fn(),
    mutate: vi.fn(),
    clearSessionCache: vi.fn(),
    unsubscribePushSilently: vi.fn(),
  }));
vi.mock("next-auth/react", () => ({ signOut }));
vi.mock("~/lib/swr", () => ({ mutate }));
vi.mock("~/lib/session-cache", () => ({
  clearSessionCache,
  DELIBERATE_LOGOUT_KEY: "deliberateLogout",
}));
vi.mock("~/lib/push-api", () => ({ unsubscribePushSilently }));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TENANT_DOMAIN: "mfa.localhost" },
}));

import { reloginAtTenant } from "./tenant-relogin";

describe("target-school reauthentication", () => {
  const location = window.location;
  beforeEach(() => {
    vi.clearAllMocks();
    signOut.mockImplementation(
      async ({ callbackUrl }: { callbackUrl: string }) => {
        window.location.href = callbackUrl;
      },
    );
    mutate.mockResolvedValue(undefined);
    Object.defineProperty(window, "location", {
      configurable: true,
      value: {
        protocol: "http:",
        port: "13845",
        href: "http://school-a.mfa.localhost:13845/home",
      },
    });
  });
  afterEach(() => {
    vi.restoreAllMocks();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: location,
    });
  });

  it("waits for backend family revocation and domain-cookie deletion before navigating, including double clicks", async () => {
    let revoke!: (response: Response) => void;
    let deleteCookie!: () => void;
    const fetch = vi.spyOn(globalThis, "fetch").mockReturnValue(
      new Promise((resolve) => {
        revoke = resolve;
      }),
    );
    signOut.mockReturnValue(
      new Promise<void>((resolve) => {
        deleteCookie = () => {
          window.location.href = "http://school-b.mfa.localhost:13845/";
          resolve();
        };
      }),
    );
    const first = reloginAtTenant("school-b");
    const second = reloginAtTenant("school-b");
    await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(fetch).toHaveBeenCalledWith("/api/auth/logout", {
      method: "POST",
      signal: expect.any(AbortSignal),
    });
    expect(window.location.href).toBe(
      "http://school-a.mfa.localhost:13845/home",
    );
    expect(signOut).not.toHaveBeenCalled();
    revoke(new Response("{}"));
    await vi.waitFor(() =>
      expect(signOut).toHaveBeenCalledWith({
        callbackUrl: "http://school-b.mfa.localhost:13845/",
      }),
    );
    expect(window.location.href).toBe(
      "http://school-a.mfa.localhost:13845/home",
    );
    deleteCookie();
    await Promise.all([first, second]);
    expect(window.location.href).toBe("http://school-b.mfa.localhost:13845/");
    expect(clearSessionCache).toHaveBeenCalled();
  });

  it.each([403, 503])(
    "does not delete cookies or navigate when revocation answers %s",
    async (status) => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        new Response("{}", { status }),
      );
      await expect(reloginAtTenant("school-b")).rejects.toMatchObject({
        status,
      });
      expect(window.location.href).toBe(
        "http://school-a.mfa.localhost:13845/home",
      );
      expect(signOut).not.toHaveBeenCalled();
    },
  );

  it("does not navigate when the backend cannot be reached", async () => {
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new TypeError("offline"));
    await expect(reloginAtTenant("school-b")).rejects.toThrow("offline");
    expect(window.location.href).toBe(
      "http://school-a.mfa.localhost:13845/home",
    );
    expect(signOut).not.toHaveBeenCalled();
  });

  it("allows a retry when revocation succeeded but cookie deletion failed", async () => {
    vi.spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response("{}"))
      .mockResolvedValueOnce(new Response("{}"))
      .mockResolvedValueOnce(new Response("{}", { status: 401 }));
    signOut.mockRejectedValueOnce(new Error("cookie deletion failed"));
    await expect(reloginAtTenant("school-b")).rejects.toThrow(
      "cookie deletion failed",
    );
    expect(window.location.href).toBe(
      "http://school-a.mfa.localhost:13845/home",
    );
    await reloginAtTenant("school-b");
    expect(window.location.href).toBe("http://school-b.mfa.localhost:13845/");
  });

  it("removes the source account's personal entries before the cookie goes (#5000)", async () => {
    localStorage.setItem("sidebar-last-room-name:1:7", "Raum Sonne");
    localStorage.setItem("messages_unread_count:school-a:7", '{"count":3}');
    localStorage.setItem("sidebar-collapsed", "true");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}"));
    let leftAtSignOut: (string | null)[] = [];
    signOut.mockImplementation(
      async ({ callbackUrl }: { callbackUrl: string }) => {
        leftAtSignOut = [
          localStorage.getItem("sidebar-last-room-name:1:7"),
          localStorage.getItem("messages_unread_count:school-a:7"),
        ];
        window.location.href = callbackUrl;
      },
    );

    await reloginAtTenant("school-b");

    expect(window.location.href).toBe("http://school-b.mfa.localhost:13845/");
    expect(leftAtSignOut).toEqual([null, null]);
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
    localStorage.clear();
  });

  it("still deletes a revoked cookie when SWR cleanup fails", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("{}"));
    mutate.mockRejectedValueOnce(new Error("cache cleanup failed"));
    await reloginAtTenant("school-b");
    expect(signOut).toHaveBeenCalledWith({
      callbackUrl: "http://school-b.mfa.localhost:13845/",
    });
    expect(window.location.href).toBe("http://school-b.mfa.localhost:13845/");
  });
});
