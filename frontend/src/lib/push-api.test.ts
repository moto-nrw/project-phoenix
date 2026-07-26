import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  getExistingSubscription,
  isPushConfigurationMissing,
  isPushSupported,
  needsIOSInstall,
  subscribePush,
  syncExistingPushSubscription,
  unsubscribePush,
  unsubscribePushSilently,
} from "./push-api";

const vapidPublicKey =
  "BAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE";
const vapidPublicKeyBytes = new Uint8Array(65).fill(1);
vapidPublicKeyBytes[0] = 4;

const originalServiceWorker = Object.getOwnPropertyDescriptor(
  navigator,
  "serviceWorker",
);
const originalUserAgent = Object.getOwnPropertyDescriptor(
  navigator,
  "userAgent",
);
const originalMaxTouchPoints = Object.getOwnPropertyDescriptor(
  navigator,
  "maxTouchPoints",
);
const originalStandalone = Object.getOwnPropertyDescriptor(
  navigator,
  "standalone",
);

function restoreNavigatorProperty(
  name: string,
  descriptor: PropertyDescriptor | undefined,
) {
  if (descriptor) {
    Object.defineProperty(navigator, name, descriptor);
    return;
  }
  Reflect.deleteProperty(navigator, name);
}

function response(body: unknown, status = 200): Response {
  return new Response(body === null ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("push-api", () => {
  const endpoint = "https://push.example.org/device?a=1&b=2";
  let fetchMock: ReturnType<typeof vi.fn>;
  let requestPermission: ReturnType<typeof vi.fn>;
  let getSubscription: ReturnType<typeof vi.fn>;
  let subscribeBrowser: ReturnType<typeof vi.fn>;
  let unsubscribeBrowser: ReturnType<typeof vi.fn>;
  let register: ReturnType<typeof vi.fn>;
  let getRegistration: ReturnType<typeof vi.fn>;
  let subscription: PushSubscription;
  let registration: ServiceWorkerRegistration;

  beforeEach(() => {
    fetchMock = vi.fn();
    requestPermission = vi.fn().mockResolvedValue("granted");
    getSubscription = vi.fn();
    subscribeBrowser = vi.fn();
    unsubscribeBrowser = vi.fn().mockResolvedValue(true);
    register = vi.fn();
    getRegistration = vi.fn();

    subscription = {
      endpoint,
      options: {
        applicationServerKey: vapidPublicKeyBytes.buffer,
        userVisibleOnly: true,
      },
      toJSON: vi.fn(() => ({
        endpoint,
        keys: { p256dh: "p256dh-key", auth: "auth-key" },
      })),
      unsubscribe: unsubscribeBrowser,
    } as unknown as PushSubscription;
    registration = {
      pushManager: {
        getSubscription,
        subscribe: subscribeBrowser,
      },
    } as unknown as ServiceWorkerRegistration;

    register.mockResolvedValue(registration);
    getRegistration.mockResolvedValue(registration);
    Object.defineProperty(navigator, "serviceWorker", {
      configurable: true,
      value: {
        register,
        getRegistration,
        ready: Promise.resolve(registration),
      },
    });
    vi.stubGlobal("PushManager", class PushManager {});
    vi.stubGlobal("Notification", {
      permission: "default",
      requestPermission,
    });
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: false })),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    restoreNavigatorProperty("serviceWorker", originalServiceWorker);
    restoreNavigatorProperty("userAgent", originalUserAgent);
    restoreNavigatorProperty("maxTouchPoints", originalMaxTouchPoints);
    restoreNavigatorProperty("standalone", originalStandalone);
  });

  it("creates and persists a tenant subscription", async () => {
    getSubscription.mockResolvedValue(null);
    subscribeBrowser.mockResolvedValue(subscription);
    fetchMock
      .mockResolvedValueOnce(response({ data: { public_key: vapidPublicKey } }))
      .mockResolvedValueOnce(response(null, 204));

    await subscribePush("tenant");

    expect(register).toHaveBeenCalledWith("/sw.js");
    expect(subscribeBrowser).toHaveBeenCalledWith({
      userVisibleOnly: true,
      applicationServerKey: vapidPublicKeyBytes,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/notifications/push/subscriptions",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(subscription.toJSON()),
      }),
    );
  });

  it("reuses an existing parent subscription", async () => {
    getSubscription.mockResolvedValue(subscription);
    fetchMock
      .mockResolvedValueOnce(response({ public_key: vapidPublicKey }))
      .mockResolvedValueOnce(response(null, 204));

    await subscribePush("parent");

    expect(subscribeBrowser).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/parent/me/push/public-key",
      expect.any(Object),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/parent/me/push/subscriptions",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("validates server configuration before requesting denied permission", async () => {
    requestPermission.mockResolvedValue("denied");
    fetchMock.mockResolvedValueOnce(response({ public_key: vapidPublicKey }));

    await expect(subscribePush("tenant")).rejects.toMatchObject({
      name: "PushApiError",
      status: 403,
      message: "Benachrichtigungen wurden nicht erlaubt.",
    });
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(requestPermission).toHaveBeenCalledOnce();
  });

  it("reports missing keys and malformed server errors", async () => {
    fetchMock.mockResolvedValueOnce(response({ data: {} }));
    try {
      await subscribePush("tenant");
      expect.unreachable("missing VAPID configuration must reject");
    } catch (error) {
      expect(error).toMatchObject({
        status: 404,
        message: "Web Push ist auf diesem Server nicht eingerichtet.",
      });
      expect(isPushConfigurationMissing(error)).toBe(true);
    }

    fetchMock.mockResolvedValueOnce(new Response("not-json", { status: 503 }));
    await expect(subscribePush("tenant")).rejects.toMatchObject({
      status: 503,
      message: "Push-Anfrage fehlgeschlagen (503).",
    });
    expect(isPushConfigurationMissing(new Error("unrelated"))).toBe(false);
  });

  it("uses the server error message when available", async () => {
    fetchMock.mockResolvedValueOnce(
      response({ error: "push is disabled" }, 409),
    );

    await expect(subscribePush("tenant")).rejects.toMatchObject({
      status: 409,
      message: "push is disabled",
    });
  });

  it("rejects a malformed VAPID key before requesting permission", async () => {
    fetchMock.mockResolvedValueOnce(response({ public_key: "YWJj" }));

    await expect(subscribePush("tenant")).rejects.toMatchObject({
      status: 502,
      message: "Der Web-Push-Schlüssel des Servers ist ungültig.",
    });
    expect(requestPermission).not.toHaveBeenCalled();
  });

  it("replaces a subscription created with an old VAPID key", async () => {
    const rotatedEndpoint = `${endpoint}-rotated`;
    const rotated = {
      ...subscription,
      endpoint: rotatedEndpoint,
      toJSON: vi.fn(() => ({
        endpoint: rotatedEndpoint,
        keys: { p256dh: "rotated-p256dh", auth: "rotated-auth" },
      })),
    } as PushSubscription;
    subscription = {
      ...subscription,
      options: {
        applicationServerKey: new Uint8Array(65).buffer,
        userVisibleOnly: true,
      },
    } as PushSubscription;
    getSubscription.mockResolvedValue(subscription);
    subscribeBrowser.mockResolvedValue(rotated);
    fetchMock
      .mockResolvedValueOnce(response({ public_key: vapidPublicKey }))
      .mockResolvedValueOnce(response(null, 204));

    await subscribePush("tenant");

    expect(unsubscribeBrowser).toHaveBeenCalledOnce();
    expect(subscribeBrowser).toHaveBeenCalledWith({
      userVisibleOnly: true,
      applicationServerKey: vapidPublicKeyBytes,
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/notifications/push/subscriptions",
      expect.objectContaining({
        body: JSON.stringify(rotated.toJSON()),
      }),
    );
  });

  it("rebinds an existing subscription without requesting permission", async () => {
    getSubscription.mockResolvedValue(subscription);
    fetchMock
      .mockResolvedValueOnce(response({ public_key: vapidPublicKey }))
      .mockResolvedValueOnce(response(null, 204));

    await expect(syncExistingPushSubscription("parent")).resolves.toBe(
      subscription,
    );

    expect(requestPermission).not.toHaveBeenCalled();
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/parent/me/push/subscriptions",
      expect.objectContaining({ method: "POST" }),
    );
  });

  it("checks VAPID configuration when no browser subscription exists", async () => {
    getSubscription.mockResolvedValue(null);
    fetchMock.mockResolvedValueOnce(
      response({ error: "web push is not configured" }, 404),
    );

    await expect(syncExistingPushSubscription("tenant")).rejects.toMatchObject({
      status: 404,
    });
    expect(register).not.toHaveBeenCalled();
  });

  it("returns existing subscriptions and handles missing registrations", async () => {
    getSubscription.mockResolvedValue(subscription);
    await expect(getExistingSubscription()).resolves.toBe(subscription);

    getRegistration.mockResolvedValue(null);
    await expect(getExistingSubscription()).resolves.toBeNull();
  });

  it("removes the server and browser subscriptions", async () => {
    getSubscription.mockResolvedValue(subscription);
    fetchMock.mockResolvedValueOnce(response(null, 204));

    await unsubscribePush("tenant");

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/notifications/push/subscriptions?endpoint=${encodeURIComponent(endpoint)}`,
      expect.objectContaining({ method: "DELETE" }),
    );
    expect(unsubscribeBrowser).toHaveBeenCalledOnce();
  });

  it("keeps logout working when cleanup fails", async () => {
    getSubscription.mockResolvedValue(subscription);
    fetchMock.mockRejectedValue(new Error("offline"));

    await expect(unsubscribePushSilently("parent")).resolves.toBeUndefined();
    expect(unsubscribeBrowser).not.toHaveBeenCalled();
  });

  it("detects iOS installation requirements", () => {
    Object.defineProperty(navigator, "userAgent", {
      configurable: true,
      value: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)",
    });
    Reflect.deleteProperty(window, "PushManager");

    expect(needsIOSInstall()).toBe(true);

    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({ matches: true })),
    );
    expect(needsIOSInstall()).toBe(false);
  });

  it("detects iPadOS desktop user agents and supported browsers", () => {
    Object.defineProperty(navigator, "userAgent", {
      configurable: true,
      value: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15)",
    });
    Object.defineProperty(navigator, "maxTouchPoints", {
      configurable: true,
      value: 5,
    });
    Reflect.deleteProperty(window, "PushManager");
    expect(needsIOSInstall()).toBe(true);

    vi.stubGlobal("PushManager", class PushManager {});
    expect(isPushSupported()).toBe(true);
    expect(needsIOSInstall()).toBe(false);
  });
});
