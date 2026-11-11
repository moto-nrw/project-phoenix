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
// Ermessensfrage sein. Die kleine Variante teilt das Farbvokabular der großen
// (`tone`, ohne Angabe grau); die Form-Props der großen (hint, progressPct,
// action) sind in ihr per Typ ausgeschlossen statt still wirkungslos.
//
// Mit `icon` und `href` deckt die große Variante auch die Kacheln der
// Startseite und der Importvorschau ab, die sich vorher jede selbst gebaut
// haben. Es gibt im Tenant-Portal keine zweite Kennzahl-Kachel mehr.
//
// `variant="row"` zeigt dieselbe verlinkte Kennzahl als kompakte Listenzeile.
//
// `variant="inline"` ist die ruhige Kachel für dichte Raster in Karten (Symbol,
// Wert, Zusatz, Label): ohne Versalien und ohne Farbe. Die Anmeldungen haben
// sie eingeführt und sich dafür eine eigene Komponente gebaut (#5583).
//
// SectionCard bleibt die Antwort für die Karte mit Icon, Überschrift und
// Inhalt; DataField die für ein Label-Wert-Paar.
//
// Tones use the StatusBadge vocabulary. Callers holding the older "amber"
// spelling (getDeltaStatus) map it to "orange" at the boundary.
//
// A tone is one brand color, used twice: the progress bar takes it raw (a bar
// is a fill), the figure takes its text-safe shade from the kit palette (a
// figure is text — the raw hexes miss the contrast minimum on white, brand
// green by a factor of two). Both come from MOTO_COLOR_PALETTE, so this
// component holds no colour values of its own and cannot go stale when the
// palette moves.

import { cn } from "~/lib/utils";
import { listRowClasses } from "./list-row-styles";
import type { ReactNode } from "react";
import Link from "~/components/ui/navigation-link";
import {
  MOTO_COLOR_PALETTE,
  getAccessibleTextColor,
} from "~/components/ui/kit-palette";
import { IconTile } from "~/components/ui/icon-tile";
import { ProgressBar } from "~/components/ui/progress-bar";
import { Skeleton } from "~/components/ui/skeleton";

/**
 * `gray` ist die Farbe „unbekannt" aus dem Standort-Vokabular — ein mittleres
 * Steingrau, das eine Zahl blass wirken lässt. `neutral` ist die Textfarbe
 * der Seite (Dunkelgrau): für eine Kennzahl, die nichts einfärben soll, weil
 * das Symbol daneben die Bedeutung schon trägt (Startseite, #2180).
 */
type StatCardTone = "blue" | "green" | "orange" | "red" | "gray" | "neutral";

const TONE_COLOR: Record<Exclude<StatCardTone, "neutral">, string> = {
  blue: MOTO_COLOR_PALETTE.blue.base,
  green: MOTO_COLOR_PALETTE.green.base,
  orange: MOTO_COLOR_PALETTE.orange.base,
  red: MOTO_COLOR_PALETTE.red.base,
  gray: MOTO_COLOR_PALETTE.stone.base,
};

/** Textfarbe der Zahl; `undefined` lässt die Klasse (Dunkelgrau) gelten. */
function figureStyle(tone: StatCardTone): { color: string } | undefined {
  if (tone === "neutral") return undefined;
  return { color: getAccessibleTextColor(TONE_COLOR[tone]) };
}

function barColor(tone: StatCardTone): string {
  return tone === "neutral" ? TONE_COLOR.gray : TONE_COLOR[tone];
}

type StatCardProps = {
  readonly variant?: "card" | "row";
  readonly label: string;
  readonly value: string | number;
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
  /**
   * Symbol rechts neben der Zahl. Die Startseite und die Importvorschau
   * hatten dafür je eine eigene Kachel — das Symbol gehört in diese hier.
   */
  readonly icon?: ReactNode;
  /** Macht die Kachel zum Link auf die Liste hinter der Zahl. */
  readonly href?: string;
  /** Zeigt statt der Zahl ein Skelett. */
  readonly loading?: boolean;
  /**
   * Erklärung zur Zahl als Titel-Hinweis. Nur für Kennzahlen, deren
   * Rechenweg jemand sonst falsch nachrechnet (Saldo-Veränderung). Kein
   * Ersatz für `hint` — der ist sichtbar, dieser hier nicht.
   */
  readonly title?: string;
};

type StatTileProps = {
  readonly variant: "tile";
  readonly label: string;
  /** Zahl oder kurzer Text. Lange Werte gehören in ein DataField. */
  readonly value: string | number;
  /**
   * Färbt die Zahl, wenn sie mehr sagt als ihre Höhe — etwa offene Fälle, die
   * jemand ansehen muss. Ohne Angabe bleibt sie neutral dunkelgrau.
   */
  readonly tone?: StatCardTone;
};

type StatInlineProps = {
  readonly variant: "inline";
  readonly label: string;
  readonly value: ReactNode;
  /**
   * Zweite Zahl hinter dem Wert, etwa die freien Plätze. Sie bricht in einer
   * schmalen Kachel um, statt über den Rand hinauszulaufen.
   */
  readonly detail?: ReactNode;
  /** Optional, weil die dichten Vierer-Raster ohne Symbol auskommen. */
  readonly icon?: ReactNode;
};

function StatInline({ label, value, detail, icon }: StatInlineProps) {
  return (
    // Mittig ausgerichtet, damit eine Kachel ohne Zusatzwert neben einer
    // umbrochenen Nachbarkachel nicht oben klebt.
    <div className="moto-content-surface flex items-center rounded-2xl border px-4 py-3">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        {icon == null ? null : <IconTile size="sm" shape="round" icon={icon} />}
        <span className="min-w-0">
          <span
            className={`block text-lg font-semibold text-gray-900 ${
              detail == null ? "leading-none" : "leading-tight"
            }`}
          >
            {value}
            {detail == null ? null : (
              <span className="text-sm font-normal text-gray-500">
                {" "}
                · {detail}
              </span>
            )}
          </span>
          <span className="mt-1 block text-xs font-medium text-gray-500">
            {label}
          </span>
        </span>
      </div>
    </div>
  );
}

export function StatCard(
  props: StatCardProps | StatTileProps | StatInlineProps,
) {
  if (props.variant === "inline") {
    return <StatInline {...props} />;
  }

  if (props.variant === "tile") {
    return (
      <div className="rounded-xl bg-gray-50 px-3 py-2">
        <span
          className="block text-sm font-semibold text-gray-900"
          style={props.tone === undefined ? undefined : figureStyle(props.tone)}
        >
          {props.value}
        </span>
        <span className="block text-xs font-medium text-gray-500">
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
    icon,
    href,
    loading = false,
    title,
  } = props;

  const card =
    props.variant === "row" ? (
      <div
        className={cn(
          listRowClasses(),
          "transition-colors group-hover:bg-gray-100/50",
        )}
      >
        {icon}
        <span className="min-w-0 flex-1 truncate text-sm font-medium text-gray-900">
          {label}
        </span>
        {loading ? (
          <Skeleton className="h-4 w-8" />
        ) : (
          <span
            className="text-sm font-semibold text-gray-900 tabular-nums"
            style={figureStyle(tone)}
          >
            {value}
          </span>
        )}
      </div>
    ) : (
      <div
        title={title}
        // Verlinkt reagiert die Kachel beim Überfahren wie `TileCard`, und
        // jede Kachel hat deren Innenabstand: eine Karte, ein Verhalten (#6565).
        className={`moto-content-surface compact:p-4 relative flex h-full flex-col rounded-2xl border p-5 max-sm:p-4 ${
          href ? "moto-hover-elevated" : ""
        }`}
      >
        {/* Absolute so a tile carrying an action keeps the same label→value
          rhythm as its neighbours in the row. */}
        {action != null ? (
          <div className="absolute top-2.5 right-2.5">{action}</div>
        ) : null}
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            {/* Auch mobil 12 px: 10 px war unter dem Typo-Boden (nichts
              Lesbares unter text-xs, und das nur für Versalien-Labels). */}
            {/* Der rechte Rand hält nur dann Platz frei, wenn oben rechts
              wirklich eine Aktion liegt; sonst nahm er dem Label auf dem
              Telefon neben dem Symbol ein Drittel der Breite. */}
            <p
              className={`text-xs font-semibold tracking-wider break-words text-gray-500 uppercase max-sm:tracking-wide ${
                action != null ? "pr-8" : ""
              }`}
            >
              {label}
            </p>
            {loading ? (
              <Skeleton className="mt-2 h-8 w-16" />
            ) : (
              <p
                className={`mt-2 font-bold text-gray-900 max-sm:mt-1 ${
                  compactValue
                    ? "text-xl whitespace-nowrap"
                    : "text-2xl max-sm:text-lg"
                }`}
                style={figureStyle(tone)}
              >
                {value}
              </p>
            )}
          </div>
          {icon != null ? (
            // Auch auf dem Telefon: ohne Symbol waren vier Kacheln dort nur
            // noch Versalien und Zahl, und die Bedeutung, die das Symbol trägt
            // (die Zahl steht bewusst in Textfarbe), fehlte gerade dort.
            // Kartenkopf-Kachel wie bei SectionCard: auf dem Telefon 36 px
            // statt 40 px, damit das Label daneben Platz behält.
            <IconTile icon={icon} />
          ) : null}
        </div>
        {hint !== undefined && hint !== "" ? (
          <p className="mt-1 text-xs text-gray-500">{hint}</p>
        ) : null}
        {progressPct !== undefined ? (
          <ProgressBar
            value={Math.round(progressPct)}
            max={100}
            label={label}
            color={barColor(tone)}
            size="sm"
            className="mt-3"
          />
        ) : null}
      </div>
    );

  if (href) {
    return (
      <Link
        href={href}
        aria-label={
          props.variant === "row"
            ? `${label}: ${loading ? "…" : value}, Liste öffnen`
            : undefined
        }
        className={
          props.variant === "row"
            ? "focus-visible:ring-moto-blue group block rounded-xl focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2"
            : // Derselbe Fokusring wie `TileCard`, in moto-blue für 3:1 Kontrast (#6565).
              "group focus-visible:outline-moto-blue block h-full cursor-pointer rounded-2xl focus-visible:outline-2 focus-visible:outline-offset-2"
        }
      >
        {card}
      </Link>
    );
  }

  return card;
}
