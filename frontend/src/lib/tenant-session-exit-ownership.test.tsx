import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { randomBytes } from "node:crypto";
import { reloginAtTenant } from "~/components/tenant/tenant-relogin";
import { isTenantSessionEnding } from "./tenant-session-end";

vi.mock("~/lib/swr", () => ({ mutate: vi.fn().mockResolvedValue(undefined) }));
vi.mock("~/lib/push-api", () => ({ unsubscribePushSilently: vi.fn() }));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TENANT_DOMAIN: "mfa.localhost" },
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
});
