"use client";

import { useCallback, useSyncExternalStore } from "react";

const STORAGE_KEY = "sidebar-collapsed";
// Same-document writes don't fire the cross-tab "storage" event, so toggles
// broadcast this custom event to wake up every mounted consumer.
const LOCAL_CHANGE_EVENT = "sidebar-collapsed-change";
// Unterhalb von 1280px (Tailwind xl) startet die Leiste eingeklappt, solange
// der Nutzer nie selbst umgeschaltet hat (#2825). Unter 1024px ist die
// Seitenleiste ohnehin komplett ausgeblendet (mobile Bottom-Nav).
// Positiv auf den schmalen Viewport formuliert: nur ein sicher erkannter
// schmaler Viewport klappt ein; Umgebungen, deren matchMedia nichts weiß
// (jsdom-Stub: immer false), bleiben beim Server-Default "ausgeklappt".
const NARROW_VIEWPORT_QUERY = "(max-width: 1279.98px)";

// Fallback when localStorage is unavailable (private mode, blocked site
// data): the toggle still works for the lifetime of the document.
let memoryValue: boolean | null = null;

const getServerSnapshot = () => false;

function getSnapshot(): boolean {
  try {
    const stored = globalThis.localStorage.getItem(STORAGE_KEY);
    if (stored === "true") return true;
    if (stored === "false") return false;
  } catch {
    if (memoryValue !== null) return memoryValue;
  }
  if (typeof globalThis.matchMedia !== "function") return false;
  return globalThis.matchMedia(NARROW_VIEWPORT_QUERY).matches;
}

function subscribe(onStoreChange: () => void) {
  const handleStorage = (event: StorageEvent) => {
    if (event.key === null || event.key === STORAGE_KEY) onStoreChange();
  };
  globalThis.addEventListener("storage", handleStorage);
  globalThis.addEventListener(LOCAL_CHANGE_EVENT, onStoreChange);
  // While no explicit choice is stored, the default follows the viewport —
  // re-evaluate when the window crosses the breakpoint.
  const media =
    typeof globalThis.matchMedia === "function"
      ? globalThis.matchMedia(NARROW_VIEWPORT_QUERY)
      : null;
  media?.addEventListener("change", onStoreChange);
  return () => {
    globalThis.removeEventListener("storage", handleStorage);
    globalThis.removeEventListener(LOCAL_CHANGE_EVENT, onStoreChange);
    media?.removeEventListener("change", onStoreChange);
  };
}

function writeCollapsed(value: boolean) {
  try {
    globalThis.localStorage.setItem(STORAGE_KEY, String(value));
  } catch {
    memoryValue = value;
  }
  globalThis.dispatchEvent(new Event(LOCAL_CHANGE_EVENT));
}

/**
 * Persisted collapse state of the desktop sidebar (#2825).
 *
 * Per-device via localStorage; without a stored choice the default is
 * viewport-based (collapsed below 1280px). The server snapshot is "expanded",
 * so the first client render after hydration may switch to the rail — the
 * same pattern as useLocalStorageValue.
 */
export function useSidebarCollapsed() {
  const collapsed = useSyncExternalStore(
    subscribe,
    getSnapshot,
    getServerSnapshot,
  );

  const toggleCollapsed = useCallback(() => {
    writeCollapsed(!getSnapshot());
  }, []);

  // Ausklappen ohne Umschalt-Semantik: der Klick auf ein Akkordeon-Icon im
  // eingeklappten Streifen öffnet die Leiste und zählt als bewusste Wahl.
  const expandSidebar = useCallback(() => {
    writeCollapsed(false);
  }, []);

  return { collapsed, toggleCollapsed, expandSidebar };
}
