"use client";

import { type ReactNode, useEffect, useState } from "react";
import { ChevronDown } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { formatDate, relativeDaysLabel } from "~/lib/date-helpers";

const HISTORY_STATUS_META: Record<
  string,
  { label: string; tone: StatusBadgeTone }
> = {
  approved: { label: "Freigegeben", tone: "green" },
  rejected: { label: "Abgelehnt", tone: "red" },
  withdrawn: { label: "Zurückgezogen", tone: "gray" },
  auto_applied: { label: "Automatisch übernommen", tone: "gray" },
};

/** Die Anfragearten, die eine Zeile tragen kann. */
export type RequestRowType =
  | "master_data"
  | "care_schedule"
  | "offering"
  | "excused"
  | "enrollment"
  | "direct_correction"
  | "absence";

/**
 * Farbe je Anfrageart, damit eine lange Liste nach Art scanbar ist, ohne dass
 * jede Zeile ihren Typ noch einmal ausschreibt. Alle sieben Werte sind
 * verschiedene Farbtöne: zwei Arten, die nebeneinander stehen können, dürfen
 * nie auf denselben Hex fallen (siehe .claude/rules/frontend-ui-kit.md).
 */
const TYPE_COLOR: Record<RequestRowType, string> = {
  master_data: LOCATION_COLORS.OTHER_ROOM,
  care_schedule: LOCATION_COLORS.GROUP_ROOM,
  offering: LOCATION_COLORS.SCHOOLYARD,
  excused: LOCATION_COLORS.EXCUSED,
  enrollment: LOCATION_COLORS.NOT_ARRIVAL,
  direct_correction: LOCATION_COLORS.UNKNOWN,
  absence: LOCATION_COLORS.TRANSIT,
};

/**
 * Spaltenraster der Zeilen. Zeile und Kopfzeile (RequestRowHeader, unten)
 * teilen sich diese beiden Strings, damit sie nicht auseinander laufen.
 * Unterhalb von `sm` gibt es kein Raster: eine echte Tabelle ist auf einem
 * Telefon unbenutzbar, dort stapelt die Zeile.
 */
const OPEN_ROW_GRID =
  "sm:grid sm:grid-cols-[minmax(0,11rem)_minmax(0,1fr)_auto_1rem] sm:items-center sm:gap-3";
const HISTORY_ROW_GRID =
  "sm:grid sm:grid-cols-[5.5rem_minmax(0,10rem)_minmax(0,1fr)_auto_minmax(0,8rem)_1rem] sm:items-center sm:gap-3";

/** Beschriftung der Art-Pille, wenn der Aufrufer keine eigene mitgibt. */
const TYPE_LABEL: Record<RequestRowType, string> = {
  master_data: "Stammdaten",
  care_schedule: "Betreuungszeiten",
  offering: "Angebote und AGs",
  excused: "Entschuldigung",
  enrollment: "Anmeldung",
  direct_correction: "Direkt-Korrektur",
  absence: "Abwesenheit",
};

/**
 * Die Art als Pille. Farbe und Standardbeschriftung kommen aus der Art;
 * `label` überschreibt den Text, wo die Zeile genauer sein kann als die
 * Kategorie (eine Abwesenheit ist Urlaub, Krank oder Fortbildung).
 */
function TypePill({
  type,
  label,
}: Readonly<{ type?: RequestRowType; label?: string }>) {
  if (!type) return null;
  return (
    <span className="shrink-0">
      <StatusDotBadge
        label={label ?? TYPE_LABEL[type]}
        color={TYPE_COLOR[type]}
        showDot={false}
      />
    </span>
  );
}

function rowAccessibleLabel({
  childName,
  type,
  typeLabel,
  summary,
  status,
  timing,
  open,
}: Readonly<{
  childName: string;
  type?: RequestRowType;
  typeLabel?: string;
  summary?: string;
  status?: string;
  timing?: string | null;
  open: boolean;
}>): string {
  return [
    `Anfrage für ${childName}`,
    typeLabel ?? (type ? TYPE_LABEL[type] : undefined),
    summary,
    status,
    timing,
    open ? "Details ausblenden" : "Details anzeigen",
  ]
    .filter((part): part is string => Boolean(part))
    .join(". ");
}

type RequestReviewCardHistoryBase = {
  readonly decidedAt: string;
  readonly decidedByName?: string;
  readonly reason?: string;
};

/**
 * Eine entschiedene Anfrage (mit Status), eine Direkt-Korrektur der Verwaltung
 * (ohne) oder eine Anfrage, über die woanders entschieden wird. Als Union,
 * damit einer Anfrage-Karte der Status nicht fehlen kann.
 */
type RequestReviewCardHistory =
  | (RequestReviewCardHistoryBase & {
      readonly kind?: "decision";
      readonly status: string;
    })
  | (RequestReviewCardHistoryBase & { readonly kind: "correction" })
  // Eine Anfrage, die diese Karte nur anzeigt: entschieden wird sie auf einer
  // eigenen Seite (#2435). Sie bringt ihre eigene Status-Beschriftung mit,
  // weil ihr Statuswortschatz nicht der der vier Warteschlangen ist, und sie
  // hat noch keine Entscheidung, solange sie offen ist.
  | {
      readonly kind: "readonly";
      readonly label: string;
      readonly tone: StatusBadgeTone;
      readonly decidedAt?: string;
      readonly decidedByName?: string;
      readonly reason?: string;
    };

/**
 * One pending parent change request in the staff Änderungsanfragen queue, in the
 * calm card look of the parent portal. Collapsed by default so a long queue
 * stays scannable: the row shows the child + a one-line summary of what changed;
 * clicking it expands the full "current → requested" diff (passed as children),
 * an optional reason, and Freigeben / Ablehnen. Shared by the master-data and
 * care-schedule review lists. Color is intentionally sparse — monochrome
 * buttons, no filled green/red.
 *
 * Seit dem Tabellenraster (#2413) sind beide Zustände Zeilen einer gemeinsamen
 * Fläche: der eingeklappte Kopf sitzt in einem festen Spaltenraster, damit alle
 * Zeilen einer Liste aneinander ausgerichtet stehen. Auch die Historie klappt
 * auf — vorher stand ihr Änderungspanel dauerhaft offen, drei Einträge füllten
 * den Bildschirm.
 */
export function RequestReviewCard({
  childName,
  summary,
  type,
  typeLabel,
  badge,
  submittedAt,
  submittedByName,
  children,
  history,
  action,
  reason,
  onReasonChange,
  reasonPlaceholder,
  reasonError,
  busy,
  onApprove,
  onReject,
}: Readonly<{
  childName: string;
  summary?: string;
  /** Anfrageart; trägt Farbe und Beschriftung der Art-Pille. */
  type?: RequestRowType;
  /** Ersetzt die Standardbeschriftung der Art-Pille. */
  typeLabel?: string;
  /**
   * Hinweis, der schon in der zugeklappten Zeile stehen muss, etwa die
   * Warnung vor einer Komplett-Abmeldung (#2434).
   */
  badge?: ReactNode;
  /** Einreichungszeitpunkt (ISO); rendert „Eingereicht am …" (#2432). */
  submittedAt?: string;
  /** Wer eingereicht hat; ergänzt die Zeile um „von …" (#2435). */
  submittedByName?: string;
  children?: ReactNode;
  /**
   * Aktion am Fuß der Lese-Karte, etwa der Weg zur Seite, auf der entschieden
   * wird. Nur zusammen mit `history` — die aufklappbare Entscheiden-Karte
   * trägt ihre eigenen Schaltflächen.
   */
  action?: ReactNode;
  history?: RequestReviewCardHistory;
  reason?: string;
  onReasonChange?: (value: string) => void;
  reasonPlaceholder?: string;
  reasonError?: string;
  busy?: boolean;
  onApprove?: () => void;
  onReject?: () => void;
}>) {
  const [open, setOpen] = useState(false);
  const [now, setNow] = useState<Date | null>(null);

  // Die aktuelle Zeit erst im Browser lesen: ein während SSR erzeugtes
  // "heute" kann beim Hydrieren nach Mitternacht schon nicht mehr stimmen.
  // Danach wird die Beschriftung regelmäßig erneuert, damit sie nicht bis zum
  // nächsten Listenabruf auf dem Vortag stehen bleibt.
  useEffect(() => {
    if (!submittedAt) return;
    setNow(new Date());
    const timer = window.setInterval(() => setNow(new Date()), 60_000);
    return () => window.clearInterval(timer);
  }, [submittedAt]);

  // Wie lange die Anfrage schon liegt — die Dringlichkeit der Arbeitsliste.
  const waitingLabel =
    submittedAt && now ? relativeDaysLabel(submittedAt, now) : null;

  if (history) {
    const meta = statusMeta(history);
    // Eine Anfrage, über die woanders entschieden wird, steht auch in der
    // Arbeitsliste — dort ist sie noch nicht entschieden und trägt weder
    // Zeitpunkt noch Person. Sie nimmt deshalb das Raster der Arbeitsliste,
    // sonst stünden ihre Spalten quer zu deren Kopfzeile.
    const decided = Boolean(history.decidedAt);
    return (
      <div data-request-row className="border-b border-gray-100 last:border-0">
        <RowButton
          open={open}
          onToggle={() => setOpen((o) => !o)}
          grid={decided ? HISTORY_ROW_GRID : OPEN_ROW_GRID}
          ariaLabel={rowAccessibleLabel({
            childName,
            type,
            typeLabel,
            summary,
            status: meta?.label,
            timing: decided ? history.decidedAt : waitingLabel,
            open,
          })}
        >
          {history.decidedAt && (
            <span className="hidden text-xs text-gray-500 sm:block">
              {formatDate(history.decidedAt)}
            </span>
          )}
          <span className="truncate text-sm font-semibold text-gray-900">
            {childName}
          </span>
          <span className="flex min-w-0 items-center gap-2">
            <TypePill type={type} label={typeLabel} />
            {summary && (
              <span className="truncate text-sm text-gray-600">{summary}</span>
            )}
            {!decided && meta && (
              <StatusBadge
                label={meta.label}
                tone={meta.tone}
                showDot={false}
              />
            )}
          </span>
          {decided &&
            (meta ? (
              <StatusBadge
                label={meta.label}
                tone={meta.tone}
                showDot={false}
              />
            ) : (
              <span />
            ))}
          <span className="hidden truncate text-xs text-gray-500 sm:block">
            {decided ? (history.decidedByName ?? "") : (waitingLabel ?? "")}
          </span>
        </RowButton>
        {open && (
          <div className="px-4 pb-4 sm:px-5">
            <p className="text-xs text-gray-500">
              {submittedAt
                ? `Eingereicht am ${formatDate(submittedAt)}${
                    submittedByName ? ` von ${submittedByName}` : ""
                  }`
                : ""}
              {submittedAt && history.decidedAt ? " · " : ""}
              {history.decidedAt
                ? `${history.kind === "correction" ? "Geändert am " : "Entschieden am "}${formatDate(history.decidedAt)}${
                    history.decidedByName ? ` von ${history.decidedByName}` : ""
                  }`
                : ""}
            </p>
            {history.reason && (
              <p className="mt-1 text-sm text-gray-600 italic">
                „{history.reason}“
              </p>
            )}
            {children && <div className="mt-2">{children}</div>}
            {action && <div className="mt-3">{action}</div>}
          </div>
        )}
      </div>
    );
  }

  return (
    <div data-request-row className="border-b border-gray-100 last:border-0">
      <RowButton
        open={open}
        onToggle={() => setOpen((o) => !o)}
        grid={OPEN_ROW_GRID}
        ariaLabel={rowAccessibleLabel({
          childName,
          type,
          typeLabel,
          summary,
          timing: waitingLabel,
          open,
        })}
      >
        <span className="truncate text-sm font-semibold text-gray-900">
          {childName}
        </span>
        <span className="flex min-w-0 items-center gap-2">
          <TypePill type={type} label={typeLabel} />
          {badge}
          <span className="hidden truncate text-sm text-gray-500 sm:block">
            {summary}
          </span>
        </span>
        <span className="hidden text-xs text-gray-400 sm:block">
          {waitingLabel ?? ""}
        </span>
      </RowButton>
      {open && (
        <div className="px-4 pb-4 sm:px-5">
          {submittedAt && (
            <p className="text-xs text-gray-500">
              Eingereicht am {formatDate(submittedAt)}
            </p>
          )}
          {children}
          <div className="mt-4 space-y-2">
            <Input
              aria-label="Begründung"
              value={reason ?? ""}
              placeholder={reasonPlaceholder}
              disabled={busy}
              onChange={(e) => onReasonChange?.(e.target.value)}
            />
            {reasonError && (
              <p className="text-moto-red-strong text-xs">{reasonError}</p>
            )}
            <div className="flex justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                disabled={busy}
                onClick={onReject}
              >
                Ablehnen
              </Button>
              <Button
                type="button"
                variant="primary"
                size="md"
                disabled={busy}
                onClick={onApprove}
              >
                Freigeben
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/**
 * Der aufklappbare Zeilenkopf. Trägt das Spaltenraster und den Pfeil; die
 * Zellen kommen als children, damit jede Ansicht ihre eigenen Spalten setzt.
 */
function RowButton({
  open,
  onToggle,
  grid,
  ariaLabel,
  children,
}: Readonly<{
  open: boolean;
  onToggle: () => void;
  grid: string;
  ariaLabel: string;
  children: ReactNode;
}>) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-expanded={open}
      aria-label={ariaLabel}
      className={`flex w-full flex-wrap items-center gap-x-2 gap-y-1 px-4 py-3 text-left transition-colors hover:bg-gray-50/60 sm:px-5 ${grid}`}
    >
      {children}
      <ChevronDown
        className={`ml-auto h-4 w-4 shrink-0 text-gray-400 transition-transform sm:ml-0 ${open ? "rotate-180" : ""}`}
        aria-hidden="true"
      />
    </button>
  );
}

/**
 * Beschriftung und Farbe des Status, je nach Art der Zeile. Eine
 * Direkt-Korrektur hat keinen: sie ist keine entschiedene Anfrage, sondern
 * eine Änderung der Verwaltung selbst (#2436), und die Art-Pille sagt das
 * bereits.
 */
function statusMeta(history: RequestReviewCardHistory): {
  label: string;
  tone: StatusBadgeTone;
} | null {
  if (history.kind === "correction") return null;
  if (history.kind === "readonly") {
    return { label: history.label, tone: history.tone };
  }
  return (
    HISTORY_STATUS_META[history.status] ?? {
      label: history.status,
      tone: "gray",
    }
  );
}

/**
 * The calm "current → requested" panel shown inside an expanded review card,
 * matching the parent-side RequestDiffPanel surface (gray-50, muted uppercase
 * heading). Rows are passed as children so each queue keeps its own layout.
 */
export function ReviewDiffPanel({
  title,
  children,
}: Readonly<{ title: string; children: ReactNode }>) {
  return (
    <div className="mt-3 space-y-2 rounded-lg bg-gray-50 p-3">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {title}
      </p>
      {children}
    </div>
  );
}

/**
 * Die Spaltenüberschriften über einer Zeilenliste. Nutzt dieselben Raster wie
 * die Zeilen selbst und erscheint erst ab `sm` — darunter stapeln die Zeilen
 * und eine Kopfzeile hätte nichts, worüber sie stehen könnte.
 */
export function RequestRowHeader({
  view,
}: Readonly<{ view: "open" | "history" }>) {
  const cell = "truncate";
  return (
    <div
      className={`hidden border-b border-gray-100 px-4 py-2.5 text-xs font-medium text-gray-500 sm:px-5 ${
        view === "history" ? HISTORY_ROW_GRID : OPEN_ROW_GRID
      }`}
    >
      {view === "history" ? (
        <>
          <span className={cell}>Entschieden</span>
          <span className={cell}>Kind</span>
          <span className={cell}>Was geändert wurde</span>
          <span className={cell}>Status</span>
          <span className={cell}>Von</span>
        </>
      ) : (
        <>
          <span className={cell}>Kind</span>
          <span className={cell}>Was geändert werden soll</span>
          <span className={cell}>Wartet seit</span>
        </>
      )}
      <span />
    </div>
  );
}
