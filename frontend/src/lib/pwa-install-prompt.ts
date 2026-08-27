/**
 * Capture point for Chrome's `beforeinstallprompt` event.
 *
 * Chrome fires the event as soon as the PWA becomes installable, which is
 * usually before any single component has mounted. Registering the listener
 * inside a component therefore loses the event. This module registers at
 * import time and caches the event so a later-mounting component can still
 * offer a one-tap install.
 *
 * Safari never fires the event (Apple has not implemented it), so the iOS
 * path stays a manual "Zum Home-Bildschirm" instruction.
 */

import { RESERVED_SLUGS } from "~/lib/reserved-slugs";

/** The non-standard Chrome event; not in lib.dom.d.ts. Module-internal. */
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  readonly userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

export type InstallOutcome = "accepted" | "dismissed" | "unavailable";

const DISMISS_KEY = "moto-pwa-install-hint-dismissed";
const VISIT_COUNT_KEY = "moto-pwa-install-hint-visits";
const SESSION_COUNTED_KEY = "moto-pwa-install-hint-session-counted";

/**
 * web.dev "Patterns for promoting PWA installation": promote after a few
 * interactions, never on the first page load, or the promotion is missed and
 * reads as noise. One visit = one browser session.
 */
const MIN_VISITS = 2;

/** True in Android browsers where the custom install card can render. */
export function isAndroidDevice(nav: Navigator): boolean {
  return (
    /android/i.test(nav.userAgent) &&
    !/(?:;\s*wv\b|version\/4\.0)/i.test(nav.userAgent)
  );
}

/** True for Samsung Internet, whose install prompt currently creates an old WebAPK. */
export function isSamsungInternet(nav: Navigator): boolean {
  return isAndroidDevice(nav) && /SamsungBrowser\//i.test(nav.userAgent);
}

/** Opens the same page in Chrome, with the normal URL as an Android fallback. */
export function createChromeIntentUrl(url: URL): string {
  const scheme = url.protocol.replace(":", "");
  if (scheme !== "http" && scheme !== "https") {
    throw new Error("Chrome intent URLs require an HTTP or HTTPS URL.");
  }

  return [
    "intent://",
    url.host,
    url.pathname,
    url.search,
    url.hash,
    "#Intent;scheme=",
    scheme,
    ";package=com.android.chrome;S.browser_fallback_url=",
    encodeURIComponent(url.href),
    ";end",
  ].join("");
}

let deferredPrompt: BeforeInstallPromptEvent | null = null;
let installationCompleted = false;
const subscribers = new Set<() => void>();

const PUBLIC_TENANT_PATHS = [
  "/",
  "/display",
  "/enroll",
  "/invite",
  "/reset-password",
] as const;

const PUBLIC_PARENT_PATHS = [
  "/login",
  "/reset-password",
  "/email-confirm",
  "/accept-guardian-invite",
  "/enroll/status",
] as const;

function notify(): void {
  for (const subscriber of subscribers) subscriber();
}

/** True only for a direct, non-reserved tenant subdomain. */
function isTenantInstallHost(hostname: string, tenantDomain: string): boolean {
  const normalizedHostname = hostname.toLowerCase().replace(/\.$/, "");
  const normalizedTenantDomain = tenantDomain.toLowerCase().replace(/\.$/, "");
  if (!normalizedHostname.endsWith(`.${normalizedTenantDomain}`)) return false;

  const slug = normalizedHostname.slice(
    0,
    -(normalizedTenantDomain.length + 1),
  );
  return Boolean(slug && !slug.includes(".") && !RESERVED_SLUGS.has(slug));
}

function isCurrentTenantInstallHost(): boolean {
  const tenantDomain = process.env.NEXT_PUBLIC_TENANT_DOMAIN;
  if (!tenantDomain) {
    throw new Error("NEXT_PUBLIC_TENANT_DOMAIN is not set.");
  }
  return isTenantInstallHost(window.location.hostname, tenantDomain);
}

function isCurrentProtectedTenantPath(): boolean {
  const pathname = window.location.pathname;
  return !PUBLIC_TENANT_PATHS.some((publicPath) => {
    if (publicPath === "/") return pathname === publicPath;
    return pathname === publicPath || pathname.startsWith(`${publicPath}/`);
  });
}

function isCurrentParentInstallHost(): boolean {
  const configuredHost = process.env.NEXT_PUBLIC_PARENTS_HOSTNAME;
  if (!configuredHost) {
    throw new Error("NEXT_PUBLIC_PARENTS_HOSTNAME is not set.");
  }
  const hostname = new URL(`http://${configuredHost}`).hostname;
  return window.location.hostname.toLowerCase() === hostname.toLowerCase();
}

function isCurrentProtectedParentPath(): boolean {
  const pathname = window.location.pathname.replace(/^\/parents(?=\/|$)/, "");
  return !PUBLIC_PARENT_PATHS.some(
    (publicPath) =>
      pathname === publicPath || pathname.startsWith(`${publicPath}/`),
  );
}

function isCurrentParentSettingsPath(): boolean {
  return (
    window.location.pathname.replace(/^\/parents(?=\/|$)/, "") === "/settings"
  );
}

// Never suppress Chrome unless the replacement card can actually render, or the
// visitor is left with no install affordance at all.
function canCaptureInstallPrompt(): boolean {
  if (isCurrentTenantInstallHost()) {
    return isCurrentProtectedTenantPath() && isInstallHintEligible(window);
  }
  if (isCurrentParentInstallHost()) {
    return isCurrentProtectedParentPath() && isInstallHintEligible(window);
  }
  return false;
}

function isCurrentInstallHost(): boolean {
  return isCurrentTenantInstallHost() || isCurrentParentInstallHost();
}

// Samsung Internet has a replacement only in the tenant-wide hint and the
// parent settings section. On every other route leave its native prompt alone.
function canSuppressSamsungInstallPrompt(): boolean {
  if (isCurrentTenantInstallHost()) return canCaptureInstallPrompt();
  return isCurrentParentInstallHost() && isCurrentParentSettingsPath();
}

/**
 * Counts this browser session as one visit, exactly once, and returns the
 * running total. The sessionStorage flag makes the call idempotent, so a
 * StrictMode double-effect or a remount does not inflate the count.
 */
export function recordVisit(win: Window): number {
  try {
    const stored = Number(win.localStorage.getItem(VISIT_COUNT_KEY) ?? "0");
    const previous = Number.isFinite(stored) && stored > 0 ? stored : 0;
    if (win.sessionStorage.getItem(SESSION_COUNTED_KEY) === "1") {
      return previous;
    }
    const next = previous + 1;
    win.localStorage.setItem(VISIT_COUNT_KEY, String(next));
    win.sessionStorage.setItem(SESSION_COUNTED_KEY, "1");
    return next;
  } catch {
    // Private mode can block storage. Treat the visit as sufficient rather
    // than hiding the hint forever on browsers that deny storage.
    return MIN_VISITS;
  }
}

/** True when the custom card may replace Chrome's native install promotion. */
export function isInstallHintEligible(win: Window): boolean {
  try {
    if (win.localStorage.getItem(DISMISS_KEY) === "1") return false;
  } catch {
    // Private mode can block storage access; show the hint anyway.
  }
  return recordVisit(win) >= MIN_VISITS;
}

/** Persists that the custom install card should no longer render. */
export function dismissInstallHint(win: Window): void {
  try {
    win.localStorage.setItem(DISMISS_KEY, "1");
  } catch {
    // The published component state still hides the hint for this page.
  }
}

if (typeof window !== "undefined") {
  window.addEventListener("beforeinstallprompt", (event) => {
    if (!isAndroidDevice(window.navigator)) {
      return;
    }
    if (isSamsungInternet(window.navigator)) {
      if (canSuppressSamsungInstallPrompt()) event.preventDefault();
      return;
    }
    if (!canCaptureInstallPrompt()) return;

    event.preventDefault();
    deferredPrompt = event as BeforeInstallPromptEvent;
    notify();
  });
  window.addEventListener("appinstalled", () => {
    if (!isCurrentInstallHost() || !isAndroidDevice(window.navigator)) {
      return;
    }
    deferredPrompt = null;
    installationCompleted = true;
    dismissInstallHint(window);
    notify();
  });
}

/** React `useSyncExternalStore` subscribe. Returns the unsubscribe callback. */
export function subscribeInstallPrompt(onChange: () => void): () => void {
  subscribers.add(onChange);
  return () => {
    subscribers.delete(onChange);
  };
}

/** True once Chrome has offered an installable event we can replay. */
export function canPromptInstall(): boolean {
  return deferredPrompt !== null;
}

/** True after this browser tab reports a completed app installation. */
export function isInstallationCompleted(): boolean {
  return installationCompleted;
}

/**
 * Replays the captured event. `prompt()` may only be called once per event,
 * so the reference is dropped immediately; Chrome fires a fresh
 * `beforeinstallprompt` if the user declines and stays eligible.
 */
export async function triggerInstallPrompt(): Promise<InstallOutcome> {
  const event = deferredPrompt;
  if (!event) return "unavailable";

  deferredPrompt = null;
  notify();

  await event.prompt();
  const { outcome } = await event.userChoice;
  return outcome;
}

/** Test seam: drops any captured event so cases start from a clean slate. */
export function resetInstallPromptForTests(): void {
  deferredPrompt = null;
  installationCompleted = false;
  notify();
}
