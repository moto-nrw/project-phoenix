import { ApiError, apiErrorFromBody, transportFetch } from "./api-error";
/**
 * Web Push client (#2003): service worker registration, permission +
 * subscription lifecycle, and the proxy-route calls that persist the
 * subscription on the backend.
 *
 * Portal awareness: the staff app talks to /api/notifications/push/*, the
 * parents portal to /api/parent/me/push/*. Both portals are separate origins,
 * so their service workers and push endpoints never collide.
 */

export type PushPortal = "tenant" | "parent" | "school";

/** Identifies the expected backend response when VAPID is not configured. */
export function isPushConfigurationMissing(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

function basePath(portal: PushPortal): string {
  switch (portal) {
    case "parent":
      return "/api/parent/me/push";
    case "school":
      return "/api/school/notifications/push";
    default:
      return "/api/notifications/push";
  }
}

/** True when this browser can register for Web Push at all. */
export function isPushSupported(): boolean {
  return (
    typeof window !== "undefined" &&
    "serviceWorker" in navigator &&
    "PushManager" in window &&
    "Notification" in window
  );
}

/** True when the app runs installed (home screen / standalone mode). */
export function isStandaloneApp(): boolean {
  if (typeof window === "undefined") return false;
  if (window.matchMedia("(display-mode: standalone)").matches) return true;
  // iOS Safari exposes the legacy navigator.standalone flag.
  return (
    "standalone" in window.navigator &&
    (window.navigator as { standalone?: boolean }).standalone === true
  );
}

/** True on iOS/iPadOS until the app is running from the home screen. */
export function needsIOSInstall(): boolean {
  if (typeof window === "undefined") return false;
  const ua = window.navigator.userAgent;
  const isAppleMobile =
    /iPad|iPhone|iPod/.test(ua) ||
    // iPadOS 13+ reports as macOS but has touch support.
    (ua.includes("Macintosh") && window.navigator.maxTouchPoints > 1);
  return isAppleMobile && !isStandaloneApp();
}

async function requestJson<T>(url: string, init?: RequestInit): Promise<T> {
  // A request that never reaches the API is general.unavailable (#2517).
  const response = await transportFetch(url, {
    credentials: "include",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (response.status === 204) return undefined as T;

  const text = await response.text();
  let data: unknown = null;
  try {
    data = text ? (JSON.parse(text) as unknown) : null;
  } catch {
    data = null;
  }
  if (!response.ok) {
    throw apiErrorFromBody(
      `Push-Anfrage fehlgeschlagen (${response.status}).`,
      response.status,
      data,
    );
  }
  return data as T;
}

interface PublicKeyResponse {
  data?: { public_key?: string };
  public_key?: string;
}

/** Fetches and validates the server's uncompressed P-256 VAPID public key. */
async function getPushPublicKey(
  portal: PushPortal,
): Promise<Uint8Array<ArrayBuffer>> {
  const response = await requestJson<PublicKeyResponse>(
    `${basePath(portal)}/public-key`,
  );
  const key = response.data?.public_key ?? response.public_key;
  if (!key) {
    throw new ApiError(
      "Web Push ist auf diesem Server nicht eingerichtet.",
      404,
    );
  }
  try {
    const decoded = urlBase64ToUint8Array(key);
    if (decoded.length !== 65 || decoded[0] !== 4) {
      throw new Error("invalid P-256 public key");
    }
    return decoded;
  } catch {
    throw new ApiError("Der Web-Push-Schlüssel des Servers ist ungültig.", 502);
  }
}

export async function verifyPushConfiguration(
  portal: PushPortal,
): Promise<void> {
  await getPushPublicKey(portal);
}

/** Registers /sw.js (idempotent) and returns the registration. */
async function registerPushServiceWorker(): Promise<ServiceWorkerRegistration> {
  const registration = await navigator.serviceWorker.register("/sw.js");
  await navigator.serviceWorker.ready;
  return registration;
}

/** Returns the current browser push subscription, if any. */
export async function getExistingSubscription(): Promise<PushSubscription | null> {
  if (!isPushSupported()) return null;
  const registration = await navigator.serviceWorker.getRegistration();
  if (!registration) return null;
  return registration.pushManager.getSubscription();
}

/**
 * Subscribes this device: requests permission (must be called from a user
 * gesture), creates the browser subscription, and persists it server-side.
 */
export async function subscribePush(portal: PushPortal): Promise<void> {
  // The settings UI preflights server configuration before enabling this
  // action. Invoke permission directly in the click path so Safari preserves
  // the transient user activation while the key is fetched again.
  const permissionPromise = Notification.requestPermission();
  const publicKeyPromise = getPushPublicKey(portal);
  const [permission, publicKey] = await Promise.all([
    permissionPromise,
    publicKeyPromise,
  ]);
  if (permission !== "granted") {
    throw new ApiError("Benachrichtigungen wurden nicht erlaubt.", 403);
  }

  const registration = await registerPushServiceWorker();
  const existing = await registration.pushManager.getSubscription();
  const subscription =
    existing && applicationServerKeyMatches(existing, publicKey)
      ? existing
      : await replaceBrowserSubscription(registration, existing, publicKey);

  await syncSubscription(portal, subscription);
}

async function replaceBrowserSubscription(
  registration: ServiceWorkerRegistration,
  existing: PushSubscription | null,
  publicKey: Uint8Array<ArrayBuffer>,
): Promise<PushSubscription> {
  if (existing) {
    const removed = await existing.unsubscribe();
    if (!removed) {
      throw new ApiError(
        "Die bestehende Push-Registrierung konnte nicht erneuert werden.",
        409,
      );
    }
  }
  return registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: publicKey,
  });
}

function applicationServerKeyMatches(
  subscription: PushSubscription,
  expected: Uint8Array<ArrayBuffer>,
): boolean {
  const current = subscription.options.applicationServerKey;
  if (!current) return false;
  const actual = new Uint8Array(current);
  if (actual.length !== expected.length) return false;
  return actual.every((value, index) => value === expected[index]);
}

/** Persists an existing browser subscription server-side (e.g. re-sync). */
async function syncSubscription(
  portal: PushPortal,
  subscription: PushSubscription,
): Promise<void> {
  await requestJson<void>(`${basePath(portal)}/subscriptions`, {
    method: "POST",
    body: JSON.stringify(subscription.toJSON()),
  });
}

/**
 * Rebinds an existing browser subscription to the authenticated account.
 * It never requests permission or creates a first subscription.
 */
export async function syncExistingPushSubscription(
  portal: PushPortal,
): Promise<PushSubscription | null> {
  if (!isPushSupported()) return null;
  const publicKey = await getPushPublicKey(portal);
  const registration = await registerPushServiceWorker();
  const existing = await registration.pushManager.getSubscription();
  if (!existing) return null;

  const subscription = applicationServerKeyMatches(existing, publicKey)
    ? existing
    : await replaceBrowserSubscription(registration, existing, publicKey);
  await syncSubscription(portal, subscription);
  return subscription;
}

/**
 * Unsubscribes this device: removes the server-side registration and the
 * browser subscription. Best effort — errors surface to the caller.
 */
export async function unsubscribePush(
  portal: PushPortal,
  signal?: AbortSignal,
): Promise<void> {
  signal?.throwIfAborted();
  const subscription = await getExistingSubscription();
  signal?.throwIfAborted();
  if (!subscription) return;
  try {
    await requestJson<void>(
      `${basePath(portal)}/subscriptions?endpoint=${encodeURIComponent(subscription.endpoint)}`,
      { method: "DELETE", ...(signal ? { signal } : {}) },
    );
  } finally {
    // Neither a late lookup nor a late response may start device cleanup
    // after this logout attempt has ended.
    if (!signal?.aborted) await subscription.unsubscribe();
  }
}

/**
 * Best-effort unsubscribe for logout flows: never throws, so a push cleanup
 * failure can never block logging out.
 */
export async function unsubscribePushSilently(
  portal: PushPortal,
  deadline?: { signal: AbortSignal; timeoutMs: number },
): Promise<void> {
  if (deadline?.signal.aborted) return;
  const controller = deadline ? new AbortController() : undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  let abort: (() => void) | undefined;
  try {
    if (!deadline || !controller) {
      await unsubscribePush(portal);
      return;
    }
    const ended = new Promise<void>((resolve) => {
      abort = () => {
        controller.abort();
        resolve();
      };
      deadline.signal.addEventListener("abort", abort, { once: true });
      timer = setTimeout(abort, deadline.timeoutMs);
    });
    // Browser Push APIs cannot be aborted. Stop waiting for them too, while
    // unsubscribePush's signal guards prevent their late continuations.
    await Promise.race([unsubscribePush(portal, controller.signal), ended]);
  } catch {
    // A server row left behind after browser cleanup is pruned on 404/410.
  } finally {
    clearTimeout(timer);
    if (deadline && abort) deadline.signal.removeEventListener("abort", abort);
  }
}

/** Converts a URL-safe base64 VAPID key to the byte array subscribe() needs. */
function urlBase64ToUint8Array(base64String: string): Uint8Array<ArrayBuffer> {
  const padding = "=".repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding)
    .replaceAll("-", "+")
    .replaceAll("_", "/");
  const rawData = window.atob(base64);
  const outputArray = new Uint8Array(new ArrayBuffer(rawData.length));
  for (let i = 0; i < rawData.length; i += 1) {
    outputArray[i] = rawData.codePointAt(i) as number;
  }
  return outputArray;
}
