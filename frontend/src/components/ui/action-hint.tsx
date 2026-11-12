"use client";

import {
  Children,
  cloneElement,
  isValidElement,
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { Z_INDEX } from "./overlay-styles";

/** Hover time before the bubble appears; a pointer passing by shows nothing. */
export const ACTION_HINT_DELAY_MS = 400;

const VIEWPORT_MARGIN = 8;
const GAP = 6;
/** Time to move the pointer from the control onto the bubble (WCAG 1.4.13). */
const LEAVE_GRACE_MS = 120;

// Ob der letzte Bedienschritt eine Taste war. Nur dann zeigt ein Fokus die
// Blase: ein Klick oder Tipp fokussiert den Knopf auch, löst aber zugleich die
// Aktion aus, eine Blase käme dann zu spät (#6643).
let keyboardModality = false;
let modalityTracking = false;
const pendingHoverCancels = new Set<() => void>();
const blockedHintHandlers = new Map<
  string,
  (anchor: HTMLElement, pointerType: string) => void
>();
const BLOCKED_ACTIVATION_EVENTS = [
  "click",
  "dblclick",
  "auxclick",
  "mousedown",
  "mouseup",
  "pointerdown",
  "pointerup",
  "touchstart",
  "touchend",
  "keydown",
  "keyup",
  "keypress",
] as const;
let blockedActivationTracking = false;

function cancelPendingHoverHints() {
  for (const cancel of [...pendingHoverCancels]) cancel();
}

function trackModality() {
  if (modalityTracking || typeof document === "undefined") return;
  modalityTracking = true;
  document.addEventListener(
    "keydown",
    (event) => {
      if (!event.metaKey && !event.altKey && !event.ctrlKey) {
        keyboardModality = true;
      }
    },
    true,
  );
  document.addEventListener(
    "pointerdown",
    () => {
      keyboardModality = false;
    },
    true,
  );
  document.addEventListener("focusin", cancelPendingHoverHints, true);
}

function isBlockedActivation(event: Readonly<{ type: string; key?: string }>) {
  if (!event.type.startsWith("key")) return true;
  return event.key === "Enter" || event.key === " " || event.key === "Spacebar";
}

function blockDisabledHintActivation(event: Event) {
  const target = event.target;
  if (!(target instanceof Element)) return;
  const anchor = target.closest<HTMLElement>("[data-action-hint-anchor]");
  const id = anchor?.getAttribute("data-action-hint-anchor");
  if (!anchor || !id) return;
  const showHint = blockedHintHandlers.get(id);
  if (!showHint) return;
  if (!isBlockedActivation(event)) return;

  // Dieser Listener läuft auf `window` vor Capture-Handlern eines klickbaren
  // Vorfahren. `aria-disabled` muss diese Sperre selbst nachbilden.
  event.preventDefault();
  event.stopPropagation();
  if (event.type === "pointerdown") {
    keyboardModality = false;
    showHint(anchor, (event as PointerEvent).pointerType);
  }
}

function registerBlockedHint(
  id: string,
  showHint: (anchor: HTMLElement, pointerType: string) => void,
) {
  blockedHintHandlers.set(id, showHint);
  if (!blockedActivationTracking) {
    blockedActivationTracking = true;
    for (const event of BLOCKED_ACTIVATION_EVENTS) {
      window.addEventListener(event, blockDisabledHintActivation, true);
    }
  }
  return () => {
    blockedHintHandlers.delete(id);
    if (blockedHintHandlers.size !== 0 || !blockedActivationTracking) return;
    blockedActivationTracking = false;
    for (const event of BLOCKED_ACTIVATION_EVENTS) {
      window.removeEventListener(event, blockDisabledHintActivation, true);
    }
  };
}

interface HintTargetProps {
  readonly children?: React.ReactNode;
  readonly "aria-describedby"?: string;
  readonly "aria-label"?: string;
  readonly "data-action-hint-anchor"?: string;
  readonly onPointerEnter?: React.PointerEventHandler<HTMLElement>;
  readonly onPointerLeave?: React.PointerEventHandler<HTMLElement>;
  readonly onPointerDown?: React.PointerEventHandler<HTMLElement>;
  readonly onFocus?: React.FocusEventHandler<HTMLElement>;
  readonly onBlur?: React.FocusEventHandler<HTMLElement>;
}

interface ActionHintProps {
  /** One sentence: what happens on click. */
  readonly text: string;
  /** Exactly one interactive element: button, link or a label styled as one. */
  readonly children: React.ReactElement;
  /**
   * Also open at once on a click or a touch or pen tap; a second tap or a
   * tap elsewhere closes it. Only for a blocked control (`disabledReason`),
   * where pressing runs no action and the reason would otherwise stay
   * unreachable on touch.
   */
  readonly showOnTap?: boolean;
}

/**
 * ActionHint: says in one sentence what a click on a control does (#6643).
 *
 * The kit's `Button` and `ButtonLink` render it through their `tooltip` prop;
 * wrap other button-looking controls (`buttonClassName` on `a`/`label`)
 * directly. `Tooltip` explains non-interactive text and also opens on tap;
 * this one belongs to controls:
 *
 * - It appears on mouse hover (after a short delay) and on keyboard focus,
 *   never on touch, where a tap runs the action at once. A blocked control
 *   (`showOnTap`, set by `disabledReason`) also opens on a tap.
 * - It adds no wrapper element, so layout classes on the control keep working.
 * - The bubble is portaled and fixed to the viewport, so cards, tables and
 *   dialogs with `overflow` do not clip it. It flips above the control near
 *   the bottom edge and stays inside the viewport horizontally.
 * - Screen readers get the text as the control's description; the bubble is
 *   only visual. Escape closes it without closing a dialog behind it, and the
 *   pointer may move onto the bubble without closing it (WCAG 1.4.13).
 */
export function ActionHint({
  text,
  children,
  showOnTap = false,
}: ActionHintProps) {
  const id = useId();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const cancel = useCallback(function cancelPendingHover() {
    if (timer.current !== null) {
      clearTimeout(timer.current);
      timer.current = null;
    }
    pendingHoverCancels.delete(cancelPendingHover);
  }, []);
  const hide = useCallback(() => {
    cancel();
    setAnchor(null);
  }, [cancel]);
  const hideSoon = useCallback(() => {
    cancel();
    timer.current = setTimeout(() => {
      timer.current = null;
      setAnchor(null);
    }, LEAVE_GRACE_MS);
  }, [cancel]);
  const showBlockedHint = useCallback(
    (target: HTMLElement, pointerType: string) => {
      // Ein Klick auf einen gesperrten Knopf löst nichts aus: er zeigt den
      // Grund sofort. Ein zweiter Tipp schließt ihn wieder.
      const tap = pointerType !== "mouse";
      cancel();
      setAnchor((open) => (tap && open ? null : target));
    },
    [cancel],
  );

  useEffect(() => {
    trackModality();
    return cancel;
  }, [cancel]);

  useLayoutEffect(() => {
    if (!showOnTap) return;
    return registerBlockedHint(id, showBlockedHint);
  }, [id, showBlockedHint, showOnTap]);

  useEffect(() => {
    if (!anchor) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      // Die erste Escape-Taste schließt nur die Blase, nicht den Dialog
      // dahinter (WCAG 1.4.13).
      event.stopPropagation();
      hide();
    };
    // Ein Tipp neben Knopf und Blase schließt eine angetippte Blase.
    const closeOnOutsidePointer = (event: PointerEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (anchor.contains(target)) return;
      if (
        target instanceof Element &&
        target.closest("[data-action-hint]") !== null
      ) {
        return;
      }
      hide();
    };
    window.addEventListener("keydown", closeOnEscape, true);
    window.addEventListener("pointerdown", closeOnOutsidePointer, true);
    window.addEventListener("scroll", hide, true);
    window.addEventListener("resize", hide);
    return () => {
      window.removeEventListener("keydown", closeOnEscape, true);
      window.removeEventListener("pointerdown", closeOnOutsidePointer, true);
      window.removeEventListener("scroll", hide, true);
      window.removeEventListener("resize", hide);
    };
  }, [anchor, hide]);

  const child = Children.only(children);
  if (!isValidElement<HintTargetProps>(child)) {
    throw new Error("ActionHint requires one React element");
  }
  const props = child.props;
  // Ein Symbolknopf, dessen Name den Hinweis schon enthält, bekommt ihn nicht
  // noch einmal als Beschreibung vorgelesen.
  const describes = !(props["aria-label"] ?? "")
    .toLowerCase()
    .includes(text.trim().toLowerCase());
  const describedBy = describes
    ? [props["aria-describedby"], id].filter(Boolean).join(" ")
    : props["aria-describedby"];

  return (
    <>
      {cloneElement(child, {
        "aria-describedby": describedBy,
        "data-action-hint-anchor": showOnTap ? id : undefined,
        onPointerEnter: (event) => {
          props.onPointerEnter?.(event);
          if (event.pointerType !== "mouse") return;
          const target = event.currentTarget;
          cancel();
          pendingHoverCancels.add(cancel);
          timer.current = setTimeout(() => {
            pendingHoverCancels.delete(cancel);
            timer.current = null;
            setAnchor(target);
          }, ACTION_HINT_DELAY_MS);
        },
        onPointerLeave: (event) => {
          props.onPointerLeave?.(event);
          // Ein Finger „verlässt“ den Knopf beim Abheben; eine angetippte
          // Blase bleibt dann stehen.
          if (showOnTap && event.pointerType !== "mouse") return;
          if (anchor) hideSoon();
          else hide();
        },
        onPointerDown: (event) => {
          props.onPointerDown?.(event);
          if (showOnTap) {
            showBlockedHint(event.currentTarget, event.pointerType);
            return;
          }
          hide();
        },
        onFocus: (event) => {
          props.onFocus?.(event);
          if (keyboardModality) setAnchor(event.currentTarget);
        },
        onBlur: (event) => {
          props.onBlur?.(event);
          hide();
        },
        children: (
          <>
            {props.children}
            {describes && (
              <span hidden id={id}>
                {text}
              </span>
            )}
          </>
        ),
      })}
      {anchor
        ? createPortal(
            <HintBubble
              anchor={anchor}
              text={text}
              onPointerEnter={cancel}
              onPointerLeave={hide}
            />,
            document.body,
          )
        : null}
    </>
  );
}

function HintBubble({
  anchor,
  text,
  onPointerEnter,
  onPointerLeave,
}: Readonly<{
  anchor: HTMLElement;
  text: string;
  onPointerEnter: () => void;
  onPointerLeave: () => void;
}>) {
  const ref = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number }>();

  useLayoutEffect(() => {
    const bubble = ref.current;
    if (!bubble) return;
    const target = anchor.getBoundingClientRect();
    const own = bubble.getBoundingClientRect();
    const below = target.bottom + GAP;
    const fitsBelow =
      below + own.height <= window.innerHeight - VIEWPORT_MARGIN;
    const top = fitsBelow ? below : target.top - GAP - own.height;
    const centered = target.left + target.width / 2 - own.width / 2;
    const maxLeft = window.innerWidth - VIEWPORT_MARGIN - own.width;
    const left = Math.max(VIEWPORT_MARGIN, Math.min(centered, maxLeft));
    setPosition({ top: Math.max(VIEWPORT_MARGIN, top), left });
  }, [anchor]);

  return (
    <div
      ref={ref}
      data-action-hint=""
      aria-hidden="true"
      onPointerEnter={onPointerEnter}
      onPointerLeave={(event) => {
        // Ein Finger auf der Blase „verlässt“ sie beim Abheben.
        if (event.pointerType === "mouse") onPointerLeave();
      }}
      className="fixed w-max max-w-xs rounded-md bg-gray-900 px-2 py-1 text-xs font-medium text-pretty text-white shadow-lg"
      style={{
        top: position?.top ?? 0,
        left: position?.left ?? 0,
        visibility: position ? "visible" : "hidden",
        zIndex: Z_INDEX.tooltip,
      }}
    >
      {text}
    </div>
  );
}
