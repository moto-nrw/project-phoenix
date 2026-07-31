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

/** The non-standard Chrome event; not in lib.dom.d.ts. Module-internal. */
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  readonly userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

export type InstallOutcome = "accepted" | "dismissed" | "unavailable";

let deferredPrompt: BeforeInstallPromptEvent | null = null;
const subscribers = new Set<() => void>();

function notify(): void {
  for (const subscriber of subscribers) subscriber();
}

if (typeof window !== "undefined") {
  window.addEventListener("beforeinstallprompt", (event) => {
    // Suppress Chrome's own mini-infobar so our in-flow card is the single
    // install surface instead of two competing prompts.
    event.preventDefault();
    deferredPrompt = event as BeforeInstallPromptEvent;
    notify();
  });
  window.addEventListener("appinstalled", () => {
    deferredPrompt = null;
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
  notify();
}
