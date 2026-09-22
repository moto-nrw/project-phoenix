"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { HandPointingIcon } from "@phosphor-icons/react";
import { Button } from "~/components/ui/button";
import { LOCATION_COLORS } from "~/lib/location-helper";

/**
 * Hebt eine Stelle der Seite hervor und erklärt sie in einer Sprechblase
 * (geführte Tour, #2832). Die Seite bleibt bedienbar: Die Abdunklung lässt
 * Klicks durch, damit die Person genau die hervorgehobene Stelle selbst
 * anklickt.
 *
 * Ohne Ziel (Stelle gerade nicht sichtbar) steht die Sprechblase mittig.
 * Liegt über geöffneten Fenstern (`Modal` nutzt z-[9999]).
 */
interface CoachMarkProps {
  readonly target: Element | null;
  /**
   * Die nächste Stelle wird noch gesucht: nur die Abdunklung, keine
   * Sprechblase. So bleibt der Bildschirm beim Stationswechsel ruhig, statt
   * kurz hell aufzublitzen oder die Sprechblase in der Mitte zu zeigen.
   */
  readonly searching?: boolean;
  /**
   * Die Markierung endet über dieser Stelle innerhalb des Ziels, etwa über
   * dem Knopf einer Karte, der erst die nächste Station ist.
   */
  readonly endBefore?: Element | null;
  readonly title: string;
  readonly text: string;
  /** Zum Beispiel „Schritt 2 von 4“. */
  readonly progress?: string;
  /**
   * Was die Person jetzt tun soll, wenn sie die Stelle selbst anklickt (ohne
   * „Weiter“). Steht hervorgehoben mit Hand-Symbol, dazu pulsiert der Rahmen:
   * Ohne diesen Hinweis sucht man im Dialog nach einem Knopf.
   */
  readonly action?: string;
  /** Fehlt, wenn die Person die Stelle selbst anklicken soll. */
  readonly onNext?: () => void;
  readonly nextLabel?: string;
  readonly onBack?: () => void;
  readonly onClose: () => void;
}

interface Box {
  top: number;
  left: number;
  width: number;
  height: number;
}

const PADDING = 6;
const BUBBLE_WIDTH = 320;
const GAP = 12;
const ARROW_SIZE = 12;
const ARROW_CLASSES = {
  top: "-top-1.5 border-t border-l",
  bottom: "-bottom-1.5 border-r border-b",
  left: "-left-1.5 border-b border-l",
  right: "-right-1.5 border-t border-r",
} as const;

function measure(
  target: Element | null,
  endBefore: Element | null = null,
): Box | null {
  if (!target) return null;
  const rect = target.getBoundingClientRect();
  if (rect.width === 0 && rect.height === 0) return null;
  let bottom = rect.bottom;
  if (endBefore) {
    const stop = endBefore.getBoundingClientRect();
    // Nur kürzen, wenn die Stelle wirklich im Ziel liegt.
    if (stop.top > rect.top && stop.top < rect.bottom) bottom = stop.top - GAP;
  }
  return {
    top: rect.top - PADDING,
    left: rect.left - PADDING,
    width: rect.width + PADDING * 2,
    height: bottom - rect.top + PADDING * 2,
  };
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

/**
 * Wo die Sprechblase steht: bevorzugt unter der Stelle, sonst darüber, sonst
 * daneben (große Stellen wie ein ganzes Formular), sonst am unteren Rand
 * über der Stelle. `side` ist die Seite der Sprechblase, an der die
 * Pfeilspitze sitzt; bei „none“ gibt es keine.
 */
function place(
  box: Box,
  width: number,
  height: number,
  viewportWidth: number,
  viewportHeight: number,
): {
  top: number;
  left: number;
  side: "top" | "bottom" | "left" | "right" | "none";
} {
  const margin = 8;
  const horizontal = clamp(box.left, margin, viewportWidth - width - margin);
  const below = box.top + box.height + GAP;
  if (below + height <= viewportHeight - margin) {
    return { top: below, left: horizontal, side: "top" };
  }
  const above = box.top - GAP - height;
  if (above >= margin) {
    return { top: above, left: horizontal, side: "bottom" };
  }
  const vertical = clamp(box.top, margin, viewportHeight - height - margin);
  const leftOf = box.left - GAP - width;
  if (leftOf >= margin) {
    return { top: vertical, left: leftOf, side: "right" };
  }
  const rightOf = box.left + box.width + GAP;
  if (rightOf + width <= viewportWidth - margin) {
    return { top: vertical, left: rightOf, side: "left" };
  }
  return {
    top: viewportHeight - height - margin * 2,
    left: (viewportWidth - width) / 2,
    side: "none",
  };
}

function sameBox(a: Box | null, b: Box | null): boolean {
  return (
    a === b ||
    (a !== null &&
      b !== null &&
      a.top === b.top &&
      a.left === b.left &&
      a.width === b.width &&
      a.height === b.height)
  );
}

export function CoachMark({
  target,
  searching = false,
  endBefore = null,
  title,
  text,
  progress,
  action,
  onNext,
  nextLabel = "Weiter",
  onBack,
  onClose,
}: CoachMarkProps) {
  const [box, setBox] = useState<Box | null>(() => measure(target, endBefore));
  const [bubbleHeight, setBubbleHeight] = useState(0);
  const bubbleRef = useRef<HTMLDivElement>(null);

  // Die Stelle kann scrollen, aufklappen oder sich verschieben; die
  // Hervorhebung folgt ihr jedes Bild.
  useEffect(() => {
    target?.scrollIntoView?.({ block: "center", behavior: "smooth" });
    let frame = 0;
    const follow = () => {
      const next = measure(target, endBefore);
      setBox((current) => (sameBox(current, next) ? current : next));
      frame = requestAnimationFrame(follow);
    };
    frame = requestAnimationFrame(follow);
    return () => cancelAnimationFrame(frame);
  }, [target, endBefore]);

  // Neues Ziel: vor dem nächsten Bild messen, damit Hervorhebung und
  // Sprechblase nicht ein Bild lang an der alten Stelle stehen.
  useLayoutEffect(() => {
    setBox(measure(target, endBefore));
  }, [target, endBefore]);

  useLayoutEffect(() => {
    setBubbleHeight(bubbleRef.current?.offsetHeight ?? 0);
  }, [title, text, action, box]);

  // Radix-Fenster (Slide-over, Drawer) schließen bei einem Zeigerdruck
  // außerhalb; ihr Wächter hört am Dokument. Die Sprechblase liegt außerhalb,
  // gehört aber zur Tour über dem Fenster: ihr Druck erreicht das Dokument
  // deshalb nicht. Ein Klick auf „Weiter“ schließt sonst das Formular.
  useEffect(() => {
    const bubble = bubbleRef.current;
    if (!bubble) return undefined;
    const keepInside = (event: Event) => event.stopPropagation();
    bubble.addEventListener("pointerdown", keepInside);
    return () => bubble.removeEventListener("pointerdown", keepInside);
  }, []);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  if (typeof document === "undefined") return null;

  const viewportWidth = window.innerWidth;
  const viewportHeight = window.innerHeight;
  const width = Math.min(BUBBLE_WIDTH, viewportWidth - 16);
  let bubbleStyle: React.CSSProperties;
  // Die Pfeilspitze zeigt von der Sprechblase auf die Mitte der Stelle.
  let arrow: {
    side: "top" | "bottom" | "left" | "right";
    offset: number;
  } | null = null;
  if (box) {
    const placement = place(
      box,
      width,
      bubbleHeight,
      viewportWidth,
      viewportHeight,
    );
    bubbleStyle = { width, left: placement.left, top: placement.top };
    if (placement.side === "top" || placement.side === "bottom") {
      arrow = {
        side: placement.side,
        offset: clamp(
          box.left + box.width / 2 - placement.left - ARROW_SIZE / 2,
          12,
          width - 12 - ARROW_SIZE,
        ),
      };
    } else if (placement.side === "left" || placement.side === "right") {
      arrow = {
        side: placement.side,
        offset: clamp(
          box.top +
            Math.min(box.height, 80) / 2 -
            placement.top -
            ARROW_SIZE / 2,
          12,
          Math.max(bubbleHeight - 12 - ARROW_SIZE, 12),
        ),
      };
    }
  } else {
    bubbleStyle = {
      width,
      left: (viewportWidth - width) / 2,
      top: Math.max((viewportHeight - bubbleHeight) / 2, 8),
    };
  }

  return createPortal(
    <div className="pointer-events-none fixed inset-0 z-[10000]">
      {box ? (
        <>
          <div
            aria-hidden="true"
            className="ring-moto-green absolute rounded-lg ring-2"
            style={{
              top: box.top,
              left: box.left,
              width: box.width,
              height: box.height,
              boxShadow: "0 0 0 9999px rgba(17, 24, 39, 0.45)",
            }}
          />
          {action && (
            // Pulsierender Außenrahmen: lenkt den Blick auf die Stelle, die
            // angeklickt werden soll. Ohne Bewegung (reduced motion) still.
            <div
              aria-hidden="true"
              className="ring-moto-green/60 absolute rounded-xl ring-4 motion-safe:animate-pulse"
              style={{
                top: box.top - 4,
                left: box.left - 4,
                width: box.width + 8,
                height: box.height + 8,
              }}
            />
          )}
        </>
      ) : (
        <div aria-hidden="true" className="absolute inset-0 bg-gray-900/45" />
      )}
      <div
        ref={bubbleRef}
        role="dialog"
        aria-label={title}
        // Während der Suche bleibt die Sprechblase im Baum, nur unsichtbar:
        // So hängt der Schutz gegen „Klick daneben“ (oben) weiter an ihr.
        hidden={searching}
        className="moto-popover-surface pointer-events-auto absolute flex flex-col gap-3 rounded-xl border p-4"
        style={bubbleStyle}
      >
        {arrow && (
          <span
            aria-hidden="true"
            className={`absolute h-3 w-3 rotate-45 border-gray-200 bg-white ${ARROW_CLASSES[arrow.side]}`}
            style={
              arrow.side === "top" || arrow.side === "bottom"
                ? { left: arrow.offset }
                : { top: arrow.offset }
            }
          />
        )}
        <div className="flex flex-col gap-1">
          {progress && (
            <span className="text-xs text-gray-500">{progress}</span>
          )}
          <p className="text-sm font-semibold text-gray-900">{title}</p>
          <p className="text-sm text-gray-700">{text}</p>
          {action && (
            <p className="mt-1 flex items-center gap-2 text-sm font-medium text-gray-900">
              <HandPointingIcon
                size={18}
                weight="fill"
                aria-hidden
                style={{ color: LOCATION_COLORS.GROUP_ROOM }}
              />
              {action}
            </p>
          )}
        </div>
        <div className="flex items-center justify-between gap-2">
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onClose}
          >
            Tour beenden
          </Button>
          <div className="flex gap-2">
            {onBack && (
              <Button
                type="button"
                variant="outline"
                size="compact"
                onClick={onBack}
              >
                Zurück
              </Button>
            )}
            {onNext && (
              <Button
                type="button"
                variant="primary"
                size="compact"
                onClick={onNext}
              >
                {nextLabel}
              </Button>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body,
  );
}
