// Die Kennzahl-Kachel des UI-Kits — eine Zahl, in zwei Größen.
//
// `variant="card"` (Standard) ist die prominente Kachel: Versalien-Label, große
// eingefärbte Zahl, optionaler Hinweis und Fortschrittsbalken. Die Form, die
// Personal-Übersicht, Zeiterfassung und Einrichtungs-Übersicht sich vorher
// jeweils selbst gebaut haben, jedes Mal mit einer anderen Fläche (#2165).
//
// `variant="tile"` ist dieselbe Aussage eine Nummer kleiner: graue Fläche, Zahl
// über dem Wort, dicht genug, dass mehrere davon IN eine Karte passen
// (Klassenansicht, Aufsichten des Schul-Portals). Bewusst dieselbe Komponente
// und keine zweite daneben: „welche Kennzahl-Komponente nehme ich" darf keine
// Ermessensfrage sein, und die Props der großen Variante (tone, hint,
// progressPct, action) sind in der kleinen per Typ ausgeschlossen statt still
// wirkungslos.
//
// InfoCard bleibt die Antwort für die Karte mit Icon, Überschrift und Inhalt;
// DataField die für ein Label-Wert-Paar in einem Detail-Panel.
//
// Tones use the StatusBadge vocabulary. Callers holding the older "amber"
// spelling (getDeltaStatus) map it to "orange" at the boundary.
//
// A tone is one brand color, used twice: the progress bar takes it raw (a bar
// is a fill), the figure takes its accessible shade from location-helper (a
// figure is text — the raw hexes miss the contrast minimum on white, brand
// green by a factor of two). Both come from LOCATION_COLORS, so this component
// holds no palette values of its own and cannot go stale when the palette moves.

import { LOCATION_COLORS, getAccessibleTextColor } from "~/lib/location-helper";

export type StatCardTone = "blue" | "green" | "orange" | "red" | "gray";

const TONE_COLOR: Record<StatCardTone, string> = {
  blue: LOCATION_COLORS.OTHER_ROOM,
  green: LOCATION_COLORS.GROUP_ROOM,
  orange: LOCATION_COLORS.SCHOOLYARD,
  red: LOCATION_COLORS.DANGER,
  gray: LOCATION_COLORS.UNKNOWN,
};

type StatCardProps = {
  readonly variant?: "card";
  readonly label: string;
  readonly value: string;
  readonly hint?: string;
  /** Renders the progress bar when set. Clamped to 0…100. */
  readonly progressPct?: number;
  readonly tone?: StatCardTone;
  /**
   * School-wide sums run to four digits ("1521h 11min") and otherwise wrap
   * mid-value. One step smaller, no wrap.
   */
  readonly compactValue?: boolean;
  /** Optional control in the top-right corner (edit pencil, info button). */
  readonly action?: React.ReactNode;
};

type StatTileProps = {
  readonly variant: "tile";
  readonly label: string;
  /** Zahl oder kurzer Text. Lange Werte gehören in ein DataField. */
  readonly value: string | number;
};

export function StatCard(props: StatCardProps | StatTileProps) {
  if (props.variant === "tile") {
    return (
      <div className="rounded-xl bg-gray-50 px-3 py-2">
        <span className="block text-sm font-semibold text-gray-900">
          {props.value}
        </span>
        <span className="block text-[11px] font-medium text-gray-500">
          {props.label}
        </span>
      </div>
    );
  }

  const {
    label,
    value,
    hint,
    progressPct,
    tone = "gray",
    compactValue,
    action,
  } = props;

  return (
    <div className="moto-content-surface relative flex h-full flex-col rounded-2xl border p-4 shadow-sm sm:p-5">
      {/* Absolute so a tile carrying an action keeps the same label→value
          rhythm as its neighbours in the row. */}
      {action != null ? (
        <div className="absolute top-2.5 right-2.5">{action}</div>
      ) : null}
      <p className="pr-8 text-xs font-semibold tracking-wider text-gray-500 uppercase">
        {label}
      </p>
      <p
        className={`mt-2 font-bold ${
          compactValue ? "text-xl whitespace-nowrap" : "text-2xl"
        }`}
        style={{ color: getAccessibleTextColor(TONE_COLOR[tone]) }}
      >
        {value}
      </p>
      {hint !== undefined && hint !== "" ? (
        <p className="mt-1 text-xs text-gray-500">{hint}</p>
      ) : null}
      {progressPct !== undefined ? (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-gray-100">
          <div
            className="h-full rounded-full transition-all"
            style={{
              width: `${Math.min(100, Math.max(0, progressPct))}%`,
              backgroundColor: TONE_COLOR[tone],
            }}
          />
        </div>
      ) : null}
    </div>
  );
}
