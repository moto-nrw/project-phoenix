import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useLayoutEffect,
  useRef,
  useSyncExternalStore,
  type ReactNode,
  type RefObject,
} from "react";

import { zLayerValue, type ZLayer } from "./overlay-styles";

/**
 * The overlay stack: which kit overlays are open, which one is on top, the
 * page scroll lock, and whether an element lies in an overlay. `Modal`,
 * `SlideOver`, `CoachMark`, `AnchoredPopover` and the calendars of
 * `DatePicker` and `DateRangePicker` register here instead of keeping their
 * own counters or asking the DOM.
 *
 * Order comes from the level each overlay takes in `Z_LAYER`: the coach
 * mark sits above a dialog, a dialog above a side panel. On the same level
 * the overlay opened last is on top. A popover (calendar, anchored popover)
 * opens over everything open at that moment. Its host is the dialog, side
 * panel or tour that was topmost then; one that opens later lies above the
 * popover when it lies above that host. Escape belongs to
 * the topmost overlay only, so one key press never closes two of them.
 *
 * An overlay opened from inside a panel belongs to that panel: a select menu
 * in a calendar, a calendar in an anchored popover. The panel hands itself
 * down through `OverlayOwnerProvider`, and every overlay registered below it
 * records it as owner. A panel recognises its own sub-panels through that
 * mark (`isInsideOwnedOverlay`), not through roles or DOM attributes.
 */

type SurfaceRef = RefObject<Element | null>;

/** Levels that register as overlays. */
export type OverlayLayer = Extract<
  ZLayer,
  "sheet" | "modal" | "coach" | "popover"
>;

interface OverlayEntry {
  readonly layer: OverlayLayer;
  readonly order: number;
  /**
   * For a popover: the dialog, side panel or tour that was topmost when it
   * opened, or null on the page.
   */
  readonly host: OverlayEntry | null;
  readonly takesEscape: boolean;
  /** The overlay's surfaces: its panel, and for a select also its trigger. */
  readonly surfaces: readonly SurfaceRef[];
  /** The panel this overlay was opened from, resolved when asked. */
  readonly ownerRef: RefObject<OverlayEntry | null> | null;
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
  if (a.layer !== "popover" && b.layer !== "popover") {
    const byLevel = zLayerValue(a.layer) - zLayerValue(b.layer);
    return byLevel === 0 ? a.order > b.order : byLevel > 0;
  }
  // A popover opens over everything open at that moment. An overlay that
  // opens after a popover lies above it if it lies above the popover's host:
  // a dialog over a calendar's side panel covers the calendar, a side panel
  // under a calendar's dialog does not.
  const aIsLater = a.order > b.order;
  const later = aIsLater ? a : b;
  const earlier = aIsLater ? b : a;
  const laterIsAbove =
    later.layer === "popover" ||
    earlier.host === null ||
    isAbove(later, earlier.host);
  return aIsLater === laterIsAbove;
}

/** The topmost dialog, side panel or tour: the host of a popover. */
function topmostHost(): OverlayEntry | null {
  const hosts = entries.filter((entry) => entry.layer !== "popover");
  return (
    hosts.find((entry) =>
      hosts.every((other) => other === entry || !isAbove(other, entry)),
    ) ?? null
  );
}

/**
 * Whether a popover lost its place: its host closed or rests, or a dialog,
 * side panel or tour opened above it. A click that opens one of those closes
 * the popover before; opened by key or by code, the popover closes the same
 * way. Popovers stay open over each other; the order decides Escape.
 */
function isCovered(entry: OverlayEntry): boolean {
  if (!entries.includes(entry)) return false;
  if (entry.host !== null && !entries.includes(entry.host)) return true;
  return entries.some(
    (other) => other.layer !== "popover" && isAbove(other, entry),
  );
}

/**
 * Whether `other` takes an Escape that `own` would otherwise take. A dialog,
 * side panel or tour leaves Escape only to another of them: a popover's own
 * Escape listener reaches the dialog below as well, as before popovers were
 * on the stack. A popover leaves Escape to every overlay above it that
 * closes on Escape.
 */
function takesEscapeFrom(other: OverlayEntry, own: OverlayEntry): boolean {
  if (!other.takesEscape) return false;
  if (own.layer !== "popover" && other.layer === "popover") return false;
  return isAbove(other, own);
}

/**
 * Puts an overlay on the stack and returns the call that takes it off. For
 * components, `useOverlay` does this with the overlay's open state.
 */
export function pushOverlay(layer: OverlayLayer): () => void {
  const entry = addEntry(layer, true, [], null);
  return () => removeEntry(entry);
}

function addEntry(
  layer: OverlayLayer,
  takesEscape: boolean,
  surfaces: readonly SurfaceRef[],
  ownerRef: RefObject<OverlayEntry | null> | null,
): OverlayEntry {
  const entry: OverlayEntry = {
    layer,
    order: nextOrder++,
    host: layer === "popover" ? topmostHost() : null,
    takesEscape,
    surfaces,
    ownerRef,
  };
  entries = [...entries, entry];
  emit();
  return entry;
}

function removeEntry(entry: OverlayEntry) {
  if (!entries.includes(entry)) return;
  entries = entries.filter((other) => other !== entry);
  emit();
}

function containsTarget(entry: OverlayEntry, target: Node): boolean {
  return entry.surfaces.some(
    (surface) => surface.current?.contains(target) === true,
  );
}

/** Whether `entry` was opened from inside `owner`, directly or further down. */
function isOwnedBy(entry: OverlayEntry, owner: OverlayEntry): boolean {
  const seen = new Set<OverlayEntry>();
  let current = entry.ownerRef?.current ?? null;
  while (current !== null && !seen.has(current)) {
    if (current === owner) return true;
    seen.add(current);
    current = current.ownerRef?.current ?? null;
  }
  return false;
}

const OverlayOwnerContext =
  createContext<RefObject<OverlayEntry | null> | null>(null);

/** The panel that overlays opened inside `children` belong to. */
export function OverlayOwnerProvider({
  owner,
  children,
}: Readonly<{ owner: OverlayOwner; children: ReactNode }>) {
  return createElement(
    OverlayOwnerContext.Provider,
    { value: owner },
    children,
  );
}

/** Handle of a registered overlay, passed to `OverlayOwnerProvider`. */
export type OverlayOwner = RefObject<OverlayEntry | null>;

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
  /**
   * Whether the overlay closes on Escape. One that does not leaves Escape to
   * the overlay below it.
   */
  readonly takesEscape?: boolean;
  /**
   * The overlay's surface, for `isInsideOverlayAbove` of the ones below. A
   * select passes its trigger and its menu.
   */
  readonly elementRef?: SurfaceRef | readonly SurfaceRef[];
  /**
   * Called when the popover lost its place: its host closed or rests, or a
   * dialog, side panel or tour opened above it. The calendars close then,
   * like on a click outside.
   */
  readonly onCovered?: () => void;
}

/**
 * Registers an overlay while `active`. `isTopmost` answers at event time
 * whether no other open overlay above it takes its Escape (see
 * `takesEscapeFrom`); an Escape handler asks it before it closes anything.
 * `isInsideOverlayAbove` tells that a target lies in an overlay opened over
 * this one, `isInsideOwnedOverlay` that it lies in one opened from inside
 * this one (a calendar in a popover). `owner` goes to `OverlayOwnerProvider`
 * around the overlay's content.
 */
export function useOverlay(
  active: boolean,
  layer: OverlayLayer,
  {
    lockScroll = false,
    takesEscape = true,
    elementRef,
    onCovered,
  }: OverlayOptions = {},
): {
  readonly isTopmost: () => boolean;
  readonly isInsideOverlayAbove: (target: EventTarget | null) => boolean;
  readonly isInsideOwnedOverlay: (target: EventTarget | null) => boolean;
  readonly owner: OverlayOwner;
} {
  const entryRef = useRef<OverlayEntry | null>(null);
  const ownerRef = useContext(OverlayOwnerContext);
  const onCoveredRef = useRef(onCovered);
  useLayoutEffect(() => {
    onCoveredRef.current = onCovered;
  });

  useLayoutEffect(() => {
    if (!active) return undefined;
    const surfaces =
      elementRef === undefined
        ? []
        : Array.isArray(elementRef)
          ? elementRef
          : [elementRef as SurfaceRef];
    const entry = addEntry(layer, takesEscape, surfaces, ownerRef);
    entryRef.current = entry;
    const unsubscribe = subscribe(() => {
      if (isCovered(entry)) onCoveredRef.current?.();
    });
    return () => {
      unsubscribe();
      entryRef.current = null;
      removeEntry(entry);
    };
  }, [active, layer, takesEscape, elementRef, ownerRef]);

  useScrollLock(active && lockScroll);

  const isTopmost = useCallback(() => {
    const own = entryRef.current;
    if (own === null) return false;
    return entries.every(
      (other) => other === own || !takesEscapeFrom(other, own),
    );
  }, []);

  const isInsideOverlayAbove = useCallback((target: EventTarget | null) => {
    const own = entryRef.current;
    if (own === null || !(target instanceof Node)) return false;
    return entries.some(
      (other) =>
        other !== own && isAbove(other, own) && containsTarget(other, target),
    );
  }, []);

  const isInsideOwnedOverlay = useCallback((target: EventTarget | null) => {
    const own = entryRef.current;
    if (own === null || !(target instanceof Node)) return false;
    return entries.some(
      (other) => isOwnedBy(other, own) && containsTarget(other, target),
    );
  }, []);

  return {
    isTopmost,
    isInsideOverlayAbove,
    isInsideOwnedOverlay,
    owner: entryRef,
  };
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
