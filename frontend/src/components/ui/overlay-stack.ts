import {
  useCallback,
  useLayoutEffect,
  useRef,
  useSyncExternalStore,
} from "react";

import { zLayerValue, type ZLayer } from "./overlay-styles";

/**
 * The overlay stack: which kit overlays are open, which one is on top, the
 * page scroll lock, and whether an element lies in an overlay. `Modal`,
 * `SlideOver` and `CoachMark` register here instead of keeping their own
 * counters or asking the DOM.
 *
 * Order comes from the level each overlay takes in `Z_LAYER`: the coach
 * mark sits above a dialog, a dialog above a side panel. On the same level
 * the overlay opened last is on top. Escape belongs to the topmost overlay
 * only, so one key press never closes two of them.
 */

/** Levels that register as overlays. */
export type OverlayLayer = Extract<ZLayer, "sheet" | "modal" | "coach">;

interface OverlayEntry {
  readonly layer: OverlayLayer;
  readonly order: number;
}

let entries: readonly OverlayEntry[] = [];
let nextOrder = 0;
const listeners = new Set<() => void>();

function emit() {
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function isAbove(a: OverlayEntry, b: OverlayEntry): boolean {
  const byLevel = zLayerValue(a.layer) - zLayerValue(b.layer);
  return byLevel === 0 ? a.order > b.order : byLevel > 0;
}

/**
 * Puts an overlay on the stack and returns the call that takes it off. For
 * components, `useOverlay` does this with the overlay's open state.
 */
export function pushOverlay(layer: OverlayLayer): () => void {
  const entry = addEntry(layer);
  return () => removeEntry(entry);
}

function addEntry(layer: OverlayLayer): OverlayEntry {
  const entry: OverlayEntry = { layer, order: nextOrder++ };
  entries = [...entries, entry];
  emit();
  return entry;
}

function removeEntry(entry: OverlayEntry) {
  if (!entries.includes(entry)) return;
  entries = entries.filter((other) => other !== entry);
  emit();
}

function isModalOpenSnapshot(): boolean {
  return entries.some((entry) => entry.layer === "modal");
}

/** True while a dialog (`Modal` and everything built on it) is open. */
export function useIsModalOpen(): boolean {
  return useSyncExternalStore(subscribe, isModalOpenSnapshot, () => false);
}

interface OverlayOptions {
  /** Locks the page scroll while the overlay is open (see `useScrollLock`). */
  readonly lockScroll?: boolean;
}

/**
 * Registers an overlay while `active`. `isTopmost` answers at event time
 * whether no other open overlay lies above it; an Escape handler asks it
 * before it closes anything.
 */
export function useOverlay(
  active: boolean,
  layer: OverlayLayer,
  { lockScroll = false }: OverlayOptions = {},
): { readonly isTopmost: () => boolean } {
  const entryRef = useRef<OverlayEntry | null>(null);

  useLayoutEffect(() => {
    if (!active) return undefined;
    const entry = addEntry(layer);
    entryRef.current = entry;
    return () => {
      entryRef.current = null;
      removeEntry(entry);
    };
  }, [active, layer]);

  useScrollLock(active && lockScroll);

  const isTopmost = useCallback(() => {
    const own = entryRef.current;
    if (own === null) return false;
    return entries.every((other) => other === own || !isAbove(other, own));
  }, []);

  return { isTopmost };
}

/**
 * Marks the root of a dialog. Menus and pickers opened inside portal into it
 * so the dialog's focus trap keeps them reachable.
 */
const MODAL_SCOPE_ATTRIBUTE = "data-modal-focus-scope";
/** Marks the panel of a `SlideOver`, the portal target inside the panel. */
const SHEET_SCOPE_ATTRIBUTE = "data-date-picker-focus-trap";
/**
 * Marks a surface that may scroll while the page scroll is locked: the
 * dialog body, and panels portaled to the body (menus, calendars).
 */
const SCROLL_AREA_ATTRIBUTE = "data-modal-content";

export const MODAL_SCOPE_SELECTOR = `[${MODAL_SCOPE_ATTRIBUTE}="true"]`;
export const SHEET_SCOPE_SELECTOR = `[${SHEET_SCOPE_ATTRIBUTE}="true"]`;
/** A dialog or a side panel. */
export const OVERLAY_SCOPE_SELECTOR = `${MODAL_SCOPE_SELECTOR}, ${SHEET_SCOPE_SELECTOR}`;

export const MODAL_SCOPE_PROPS = { [MODAL_SCOPE_ATTRIBUTE]: "true" } as const;
export const SHEET_SCOPE_PROPS = { [SHEET_SCOPE_ATTRIBUTE]: "true" } as const;
export const SCROLL_AREA_PROPS = { [SCROLL_AREA_ATTRIBUTE]: "true" } as const;

/** Whether the target lies in a kit dialog. */
export function isInsideModal(target: EventTarget | null): boolean {
  return (
    target instanceof Element && target.closest(MODAL_SCOPE_SELECTOR) !== null
  );
}

// Locks stack: a confirmation over a form modal holds two at once. Only the
// first lock records the page's own value and only the last release restores
// it. Per-lock bookkeeping wrote "hidden" back when the lower modal released
// before the upper one, and the page stayed unscrollable until reload.
let activeLocks = 0;
let overflowBeforeFirstLock = "";

const SCROLL_KEYS = new Set([
  "ArrowUp",
  "ArrowDown",
  "PageUp",
  "PageDown",
  "Home",
  "End",
  " ",
]);

/**
 * Locks the page scroll while `isLocked`. Wheel, touch and scroll keys still
 * work inside surfaces marked with `SCROLL_AREA_PROPS`. Relies on
 * `scrollbar-gutter: stable` in globals.css to prevent layout shift.
 */
export function useScrollLock(isLocked: boolean) {
  const scrollAreas = useRef<WeakSet<Element>>(new WeakSet());

  useLayoutEffect(() => {
    if (!isLocked) return undefined;

    // Capture the prior value so the last release RESTORES it instead of
    // forcing "": closing a modal opened over a page whose inline overflow
    // someone else set would clear it. (The chat locks through
    // `data-chat-page-lock` and CSS, independent of this value.)
    if (activeLocks === 0) {
      overflowBeforeFirstLock = document.documentElement.style.overflow;
    }
    activeLocks += 1;
    document.documentElement.style.overflow = "hidden";

    const updateScrollAreas = () => {
      scrollAreas.current = new WeakSet(
        document.querySelectorAll(`[${SCROLL_AREA_ATTRIBUTE}="true"]`),
      );
    };
    updateScrollAreas();

    // Overlays mount and unmount scroll areas while the lock holds.
    const observer = new MutationObserver(updateScrollAreas);
    observer.observe(document.body, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: [SCROLL_AREA_ATTRIBUTE],
    });

    const isInsideScrollArea = (target: EventTarget | null): boolean => {
      if (!(target instanceof Element)) return false;
      let element: Element | null = target;
      while (element && element !== document.body) {
        if (scrollAreas.current.has(element)) return true;
        element = element.parentElement;
      }
      return false;
    };

    const preventBackgroundScroll = (event: Event) => {
      if (!isInsideScrollArea(event.target)) event.preventDefault();
    };

    const handleKeyDown = (event: KeyboardEvent) => {
      const interactive =
        event.target instanceof Element &&
        event.target.closest(
          'button, a[href], input, textarea, select, [contenteditable]:not([contenteditable="false"]), [role="button"]',
        );
      if (
        SCROLL_KEYS.has(event.key) &&
        !interactive &&
        !isInsideScrollArea(event.target)
      ) {
        event.preventDefault();
      }
    };

    document.addEventListener("wheel", preventBackgroundScroll, {
      passive: false,
    });
    document.addEventListener("touchmove", preventBackgroundScroll, {
      passive: false,
    });
    document.addEventListener("keydown", handleKeyDown);

    return () => {
      activeLocks -= 1;
      if (activeLocks === 0) {
        document.documentElement.style.overflow = overflowBeforeFirstLock;
      }
      document.removeEventListener("wheel", preventBackgroundScroll);
      document.removeEventListener("touchmove", preventBackgroundScroll);
      document.removeEventListener("keydown", handleKeyDown);
      observer.disconnect();
    };
  }, [isLocked]);
}
