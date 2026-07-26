/**
 * Web Push client (#2003): service worker registration, permission +
 * subscription lifecycle, and the proxy-route calls that persist the
 * subscription on the backend.
 *
 * Portal awareness: the staff app talks to /api/notifications/push/*, the
 * parents portal to /api/parent/me/push/*. Both portals are separate origins,
 * so their service workers and push endpoints never collide.
 */

export type PushPortal = "tenant" | "parent";

class PushApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "PushApiError";
  }
}

function basePath(portal: PushPortal): string {
  return portal === "parent"
    ? "/api/parent/me/push"
    : "/api/notifications/push";
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
function isStandalone(): boolean {
  if (typeof window === "undefined") return false;
  if (window.matchMedia("(display-mode: standalone)").matches) return true;
  // iOS Safari exposes the legacy navigator.standalone flag.
  return (
    "standalone" in window.navigator &&
    (window.navigator as { standalone?: boolean }).standalone === true
  );
}

/**
 * True on iOS/iPadOS Safari where Web Push only exists after the app was
 * added to the home screen: push is unsupported AND we're on an Apple touch
 * device AND not running standalone.
 */
export function needsIOSInstall(): boolean {
  if (typeof window === "undefined") return false;
  const ua = window.navigator.userAgent;
  const isAppleMobile =
    /iPad|iPhone|iPod/.test(ua) ||
    // iPadOS 13+ reports as macOS but has touch support.
    (ua.includes("Macintosh") && window.navigator.maxTouchPoints > 1);
  return isAppleMobile && !isStandalone() && !isPushSupported();
}

async function requestJson<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
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
    const message =
      data && typeof data === "object" && "error" in data
        ? String((data as { error: unknown }).error)
        : `Push-Anfrage fehlgeschlagen (${response.status}).`;
    throw new PushApiError(response.status, message);
  }
  return data as T;
}

interface PublicKeyResponse {
  data?: { public_key?: string };
  public_key?: string;
}

/** Fetches the server's VAPID public key; throws 404 when push is not configured. */
async function getPushPublicKey(portal: PushPortal): Promise<string> {
  const response = await requestJson<PublicKeyResponse>(
    `${basePath(portal)}/public-key`,
  );
  const key = response.data?.public_key ?? response.public_key;
  if (!key) {
    throw new PushApiError(
      404,
      "Web Push ist auf diesem Server nicht eingerichtet.",
    );
  }
  return key;
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
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    throw new PushApiError(403, "Benachrichtigungen wurden nicht erlaubt.");
  }

  const publicKey = await getPushPublicKey(portal);
  const registration = await registerPushServiceWorker();
  const subscription =
    (await registration.pushManager.getSubscription()) ??
    (await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(publicKey),
    }));

  await syncSubscription(portal, subscription);
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
 * Unsubscribes this device: removes the server-side registration and the
 * browser subscription. Best effort — errors surface to the caller.
 */
export async function unsubscribePush(portal: PushPortal): Promise<void> {
  const subscription = await getExistingSubscription();
  if (!subscription) return;
  await requestJson<void>(
    `${basePath(portal)}/subscriptions?endpoint=${encodeURIComponent(subscription.endpoint)}`,
    { method: "DELETE" },
  );
  await subscription.unsubscribe();
}

/**
 * Best-effort unsubscribe for logout flows: never throws, so a push cleanup
 * failure can never block logging out.
 */
export async function unsubscribePushSilently(
  portal: PushPortal,
): Promise<void> {
  try {
    await unsubscribePush(portal);
  } catch {
    // Dead subscriptions are pruned server-side on the next 410 anyway.
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
