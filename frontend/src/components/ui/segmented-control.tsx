"use client";

import type { CSSProperties } from "react";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

// Segmented single-choice control: a small, fixed set of mutually exclusive
// options rendered as one control.
//
// Fills a real kit gap. `ui/Tabs` is Radix and panel-bound — correct when the
// choice swaps CONTENT, wrong when it is a form value (the Stempeluhr mode) or
// a view parameter that drives fetches rather than panels (Monat/Woche). Both
// of those were hand-rolled `<button>` clusters in the Zeiterfassung before.
//
// Semantics are toggle-buttons (`aria-pressed`), not radios: the options stay
// reachable as plain buttons for keyboard and assistive tech without inheriting
// radiogroup arrow-key navigation, which does not match a two- or three-option
// switcher sitting inline in a toolbar.
//
// Variants:
//   "joined" — segments share one bordered rounded-full container, the active
//              one is solid gray-900. Use for neutral view switchers.
//   "pills"  — separate tinted pills with a gap; the active pill takes its
//              item's brand tone. Use when each option carries its own meaning
//              (Vor Ort = green, Homeoffice = blue, Abwesend = red).
//
// Tones map to LOCATION_COLORS (lib/location-helper.ts) — never generic
// Tailwind hues.

// Not exported: callers reach it through the exported SegmentedControlItem,
// which is the type they actually annotate their items array with.
type SegmentedControlTone = "neutral" | "green" | "blue" | "red";

export interface SegmentedControlItem<T extends string> {
  readonly value: T;
  readonly label: string;
  /** Only honoured by the "pills" variant; "joined" is always neutral. */
  readonly tone?: SegmentedControlTone;
  readonly disabled?: boolean;
}

// Das aktive Segment. `neutral` ist die Regel (hell abgehoben wie in jedem
// anderen Umschalter); die Farbtöne bleiben für die wenigen Fälle, in denen
// die Wahl selbst eine Bedeutung trägt (Arbeitsmodus). Form und Höhe sind in
// allen Fällen dieselben — nur die Tönung wechselt.
const ACTIVE_PILL: Record<
  SegmentedControlTone,
  { className: string; style?: CSSProperties }
> = {
  neutral: { className: "bg-white text-gray-900 shadow-sm" },
  green: {
    className: "shadow-sm",
    style: {
      backgroundColor: MOTO_COLOR_PALETTE.green.soft,
      color: MOTO_COLOR_PALETTE.green.strong,
    },
  },
  blue: {
    className: "shadow-sm",
    style: {
      backgroundColor: MOTO_COLOR_PALETTE.blue.soft,
      color: MOTO_COLOR_PALETTE.blue.base,
    },
  },
  red: {
    className: "shadow-sm",
    style: {
      backgroundColor: MOTO_COLOR_PALETTE.red.soft,
      color: MOTO_COLOR_PALETTE.red.strong,
    },
  },
};

export function SegmentedControl<T extends string>({
  items,
  value,
  onChange,
  variant = "joined",
  fullWidth = false,
  ariaLabel,
  className = "",
}: {
  readonly items: readonly SegmentedControlItem<T>[];
  /** `null` renders every segment inactive — no silent preselection. */
  readonly value: T | null;
  readonly onChange: (next: T) => void;
  readonly variant?: "joined" | "pills";
  /** Stretch to the container; segments share the width and wrap when it is too narrow (modal tab bars). */
  readonly fullWidth?: boolean;
  readonly ariaLabel?: string;
  readonly className?: string;
}) {
  // EINE Höhe und EINE Mindestbreite je Segment, überall. Gemessen am 29.08.
  // trat dasselbe Bauteil in drei Höhen (28, 36, 44 px) und Breiten von 140
  // bis 798 px auf, auf der Zeiterfassung sogar zweimal verschieden auf
  // derselben Seite. Ein Umschalter, der auf jeder Seite anders groß ist,
  // wirkt beim Blättern unruhig, auch wenn jede einzelne Fassung für sich
  // stimmig aussieht.
  //
  // Segment 32 px + Spur-Innenrand 4 px = 40 px Gesamthöhe — dieselbe
  // Bedienhöhe wie jeder Knopf in der Kopfkarte (CONTROL_HEIGHT in
  // tenant-page.tsx). Das `!` ist nötig, weil die Kopfkarte ihre Höhe per
  // Nachfahren-Selektor auf alle Knöpfe legt und die Spur des Umschalters
  // sonst auf 48 px wüchse. Unter sm 36-px-Segmente, also 44 px gesamt —
  // dieselbe Touch-Höhe wie die Nachbarn.
  // Schriftgröße `text-sm`, nicht `text-xs`: ein Bedienelement ist kein
  // Kleingedrucktes (Typo-Boden, TENANT-PAGE-SPEC).
  const base =
    "flex h-8! min-w-[84px] items-center justify-center px-3 text-sm font-medium transition-[background-color,box-shadow,color,opacity] disabled:cursor-not-allowed disabled:opacity-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none max-sm:h-9!";

  // Die frühere „pills"-Fassung (runde, einzeln getönte Pillen ohne Spur) ist
  // aufgegangen: sie war 32 px hoch, die andere 36, und beide standen im
  // selben Portal nebeneinander. Eine Form, eine Höhe — die Tönung bleibt.
  if (variant === "pills") {
    return (
      <div
        role="group"
        aria-label={ariaLabel}
        className={`inline-grid auto-cols-fr grid-flow-col gap-1 rounded-lg bg-gray-100 p-1 ${className}`}
      >
        {items.map((item) => {
          const active = item.value === value;
          const activePill = ACTIVE_PILL[item.tone ?? "neutral"];
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={`${base} rounded-md ${active ? activePill.className : "text-gray-500 hover:text-gray-900"}`}
              style={active ? activePill.style : undefined}
            >
              {item.label}
            </button>
          );
        })}
      </div>
    );
  }

  // Eine Erscheinung für beide Breiten: helle Spur, das aktive Segment hell
  // abgehoben. Vorher färbte die schmale Fassung ihr aktives Segment schwarz
  // und die breite hob es weiß hervor — dasselbe Bauteil in zwei Gestalten.
  //
  // Alle Segmente sind GLEICH BREIT (`auto-cols-fr` bzw. `basis-0`). Vorher
  // richtete sich jedes nach seinem Wort, und weil „Alle", „Belegt" und
  // „Frei" verschieden lang sind, sprang die Markierung in der Breite — der
  // Umschalter wirkte dadurch schief.
  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className={
        fullWidth
          ? `grid w-full auto-cols-fr grid-flow-col gap-1 rounded-lg bg-gray-100 p-1 ${className}`
          : `inline-grid auto-cols-fr grid-flow-col gap-1 rounded-lg bg-gray-100 p-1 ${className}`
      }
    >
      {items.map((item) => {
        const active = item.value === value;
        if (fullWidth) {
          return (
            <button
              key={item.value}
              type="button"
              aria-pressed={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={`${base} rounded-md ${
                active
                  ? "bg-white text-gray-900 shadow-sm"
                  : "text-gray-500 hover:text-gray-700"
              }`}
            >
              {item.label}
            </button>
          );
        }
        return (
          <button
            key={item.value}
            type="button"
            aria-pressed={active}
            disabled={item.disabled}
            onClick={() => onChange(item.value)}
            className={`${base} rounded-md ${
              active
                ? "bg-white text-gray-900 shadow-sm"
                : "text-gray-500 hover:text-gray-700"
            }`}
          >
            {item.label}
          </button>
        );
      })}
    </div>
  );
}
