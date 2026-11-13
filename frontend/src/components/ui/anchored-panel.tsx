"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  type ReactNode,
  type RefObject,
} from "react";

import { Z_INDEX } from "./overlay-styles";
import { useOverlay, type OverlayOwner } from "./overlay-stack";

/**
 * The engine behind every panel the kit floats at a trigger: `AnchoredPopover`,
 * the menu of `ListboxDropdown` and the calendars of `DatePicker` and
 * `DateRangePicker`. It puts the open panel on the overlay stack and decides,
 * the same way for all of them, what closes it:
 *
 * - **A press outside** (see `outsidePress`; after the target's own
 *   handlers: a field can mark the press before the panel closes). Inside are
 *   the trigger, the panel and every overlay opened from inside the panel
 *   (`OverlayOwnerProvider`): a select menu in a calendar, a calendar in a
 *   popover. Another panel next to it is outside; a press there closes this
 *   one.
 * - **Escape**, when the panel is the topmost overlay that takes Escape and the
 *   key does not come from an overlay above it (an open select keeps its own
 *   Escape). Listened on the window in the capture phase, before the dialog
 *   and side-panel listeners on the document: Escape closes only the panel.
 * - **Scroll** of the page or of a surrounding container: the panel follows
 *   its trigger, or closes where it cannot follow. Scrolling inside the panel
 *   or one of its sub-panels does not count.
 *
 * Placement stays with each panel; the shared distances are in
 * `panel-geometry.ts`.
 */

/** Why the engine closes a panel. */
type PanelCloseReason = "outside" | "escape" | "scroll";

interface AnchoredPanelOptions {
  readonly open: boolean;
  readonly onClose: (reason: PanelCloseReason) => void;
  /** The trigger, or the wrapper around it. */
  readonly anchorRef: RefObject<Element | null>;
  /** The floating panel. */
  readonly panelRef: RefObject<Element | null>;
  /**
   * Whether the engine closes the panel on Escape. Off for a panel whose
   * trigger keeps the focus and handles its keys (a select, a suggestion
   * list): Escape from elsewhere then closes no list.
   */
  readonly closeOnEscape?: boolean;
  /**
   * What outside closes the panel, and lets it through:
   * - `press`: the first touch or mouse press (`AnchoredPopover`, `DatePicker`).
   * - `tap`: the press of a finished tap or click (`mousedown`). A finger that
   *   starts to scroll the page keeps the panel open, and the panel follows
   *   its trigger (select menu, `DateRangePicker`).
   * - `click`: the click, which the panel swallows: for a panel over buttons
   *   that must not fire by accident (`AnchoredPopover` `swallowOutsideClick`).
   */
  readonly outsidePress?: "press" | "tap" | "click";
  /** `follow` moves the panel with its trigger, `close` closes it. */
  readonly scroll: "follow" | "close";
  /** Places the panel again: on resize, and on scroll while following. */
  readonly reposition?: () => void;
  /**
   * Called when the panel lost its place on the overlay stack: its dialog
   * closed or a dialog opened above it (see `useOverlay`).
   */
  readonly onCovered?: () => void;
}

export function useAnchoredPanel({
  open,
  onClose,
  anchorRef,
  panelRef,
  closeOnEscape = true,
  outsidePress = "press",
  scroll,
  reposition,
  onCovered,
}: AnchoredPanelOptions): {
  /** Hand it to `OverlayOwnerProvider` around the panel's content. */
  readonly owner: OverlayOwner;
} {
  // Trigger and panel together are the overlay's surface: an Escape on the
  // trigger of an open select belongs to that select, not to a calendar
  // below it.
  const surfaces = useMemo(() => [anchorRef, panelRef], [anchorRef, panelRef]);
  const { isTopmost, isInsideOverlayAbove, isInsideOwnedOverlay, owner } =
    useOverlay(open, "popover", {
      takesEscape: closeOnEscape,
      elementRef: surfaces,
      onCovered,
    });

  const onCloseRef = useRef(onClose);
  const repositionRef = useRef(reposition);
  useLayoutEffect(() => {
    onCloseRef.current = onClose;
    repositionRef.current = reposition;
  });
  const close = useCallback((reason: PanelCloseReason) => {
    onCloseRef.current(reason);
  }, []);

  // One press fires pointerdown, touchstart and mousedown; it closes once.
  // The mark lasts until the press ends, so a caller that keeps the panel
  // open still gets the next press.
  const pressClosedRef = useRef(false);
  useLayoutEffect(() => {
    pressClosedRef.current = false;
  }, [open]);

  /** The panel, its trigger, or a sub-panel opened from the panel. */
  const isInside = useCallback(
    (target: EventTarget | null) =>
      target instanceof Node &&
      (anchorRef.current?.contains(target) === true ||
        panelRef.current?.contains(target) === true ||
        isInsideOwnedOverlay(target)),
    [anchorRef, isInsideOwnedOverlay, panelRef],
  );

  useEffect(() => {
    if (!open) return undefined;
    const handlePress = (event: Event) => {
      if (pressClosedRef.current || isInside(event.target)) return;
      pressClosedRef.current = true;
      close("outside");
    };
    const endPress = () => {
      pressClosedRef.current = false;
    };
    const handleClick = (event: MouseEvent) => {
      if (isInside(event.target)) return;
      event.preventDefault();
      event.stopPropagation();
      close("outside");
    };
    const pressEvents =
      outsidePress === "press"
        ? (["pointerdown", "touchstart", "mousedown"] as const)
        : outsidePress === "tap"
          ? (["mousedown"] as const)
          : ([] as const);
    const endEvents = ["mouseup", "touchend", "touchcancel"] as const;
    if (outsidePress === "click") {
      document.addEventListener("click", handleClick, true);
    }
    for (const type of pressEvents) {
      document.addEventListener(type, handlePress, { passive: true });
    }
    for (const type of endEvents) {
      document.addEventListener(type, endPress, true);
    }
    return () => {
      document.removeEventListener("click", handleClick, true);
      for (const type of pressEvents) {
        document.removeEventListener(type, handlePress);
      }
      for (const type of endEvents) {
        document.removeEventListener(type, endPress, true);
      }
    };
  }, [close, isInside, open, outsidePress]);

  useEffect(() => {
    if (!open || !closeOnEscape) return undefined;
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || !isTopmost()) return;
      if (isInsideOverlayAbove(event.target)) return;
      event.preventDefault();
      event.stopImmediatePropagation();
      close("escape");
    };
    window.addEventListener("keydown", handleEscape, true);
    return () => window.removeEventListener("keydown", handleEscape, true);
  }, [close, closeOnEscape, isInsideOverlayAbove, isTopmost, open]);

  useEffect(() => {
    if (!open) return undefined;
    const place = () => repositionRef.current?.();
    const handleScroll = (event: Event) => {
      const target = event.target;
      if (
        target instanceof Node &&
        (panelRef.current?.contains(target) === true ||
          isInsideOwnedOverlay(target))
      ) {
        return;
      }
      if (scroll === "close") close("scroll");
      else place();
    };
    // A trigger in a dialog that is still sliding in reaches its place only
    // when the animation ends. The panel's own hover transitions do not count.
    const placeAfterMotion = (event: Event) => {
      if (
        event.target instanceof Node &&
        anchorRef.current !== null &&
        event.target.contains(anchorRef.current)
      ) {
        place();
      }
    };
    window.addEventListener("resize", place);
    window.addEventListener("scroll", handleScroll, true);
    window.addEventListener("animationend", placeAfterMotion, true);
    window.addEventListener("transitionend", placeAfterMotion, true);
    return () => {
      window.removeEventListener("resize", place);
      window.removeEventListener("scroll", handleScroll, true);
      window.removeEventListener("animationend", placeAfterMotion, true);
      window.removeEventListener("transitionend", placeAfterMotion, true);
    };
  }, [anchorRef, close, isInsideOwnedOverlay, open, panelRef, scroll]);

  return { owner };
}

// The level of select menus portaled from inside a surface that itself lies
// above `Z_INDEX.menu`, such as an `AnchoredPopover` (#6569).
const MenuLayerContext = createContext<number>(Z_INDEX.menu);

/** Lifts the menus of all selects inside `children` to `zIndex`. */
export function MenuLayerProvider({
  zIndex,
  children,
}: Readonly<{ zIndex: number; children: ReactNode }>) {
  return (
    <MenuLayerContext.Provider value={zIndex}>
      {children}
    </MenuLayerContext.Provider>
  );
}

/** The level for a select menu at this place in the tree. */
export function useMenuLayer(): number {
  return useContext(MenuLayerContext);
}
