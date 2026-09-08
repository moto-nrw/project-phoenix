"use client";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { InfoCard } from "~/components/ui/info-card";
import Link from "~/components/ui/navigation-link";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StatusBadge } from "~/components/ui/status-badge";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";
import {
  useBerlinClock,
  upcomingFirst,
} from "~/components/home/home-card-rows";
import { ChevronRight } from "lucide-react";
import {
  blockPhase,
  formatMinutesAhead,
  minutesBetween,
} from "~/lib/home-clock";
import { ownShiftService } from "~/lib/shift-api";
import type { OwnAssignment } from "~/lib/shift-helpers";
import { useSWRAuth } from "~/lib/swr";

// Heute geplante Betreuungsplan-Einsätze der eingeloggten Person ("Mein Tag",
// #1844): Ort (Raum) + Aufgabe (Aktivität) + Zeit, plus die Vertretungsplan-
// Zustände (Vertretung, entfällt, du fehlst, unterbesetzt). Bewusst ohne
// Kindernamen (GDPR). Rendert nichts, wenn die Schule keinen Betreuungsplan
// pflegt (leere Liste), damit die Seite nicht mit einer leeren Karte zusteht.
// Ein Ladefehler zeigt dagegen eine Fehlerkarte, damit er nicht wie "keine
// Einsätze" aussieht.
export function BetreuungsplanHeuteCard({
  title = "Heute geplant",
  showEmpty = false,
  href,
  maxRows,
  dense = false,
}: {
  /** Überschrift der Karte. Auf der Startseite heißt derselbe Baustein „Mein Tag". */
  readonly title?: string;
  /**
   * Auf der Zeiterfassung verschwindet die Karte ohne Einsätze, damit die
   * Seite nicht mit einer leeren Karte zusteht. Auf der Startseite ist sie ein
   * gewählter Baustein: dort muss „heute nichts geplant" als Antwort dastehen,
   * sonst sucht die Person den Fehler bei sich (#2180).
   */
  readonly showEmpty?: boolean;
  /**
   * Wohin diese Karte führt: setzt den Weiterlink in der Kopfzeile UND macht
   * die Einsätze anklickbar — auf der Startseite in den Tagesplan, wo der
   * Block auch bedient wird. Ohne Ziel bleibt die Zeile eine Anzeige und
   * sieht auch nicht klickbar aus.
   */
  readonly href?: string;
  /**
   * Wie viele Einsätze höchstens als Zeile erscheinen — der Hinweis auf den
   * Rest ist dabei schon eingerechnet. Auf der Startseite hat die Karte eine
   * feste Höhe: mehr passt nicht ganz hinein, und eine halb sichtbare Zeile am
   * Kartenrand liest sich, als liefe der Baustein aus seiner Karte heraus.
   * Ohne Angabe (Zeiterfassung, ohne Höhenvorgabe) stehen alle Einsätze da.
   */
  readonly maxRows?: number;
  /**
   * Kompakte Zeilen: eine Zeile je Einsatz, Raum als Zusatz dahinter, getönte
   * Kachel. Für die Startseite, wo die Karte eine feste Höhe hat. Ohne das
   * bleibt die gewohnte zweizeilige Form der Zeiterfassung.
   */
  readonly dense?: boolean;
} = {}) {
  // Berlin, not browser-local: the backend defines "today" in Europe/Berlin,
  // and a browser in another timezone around midnight would otherwise fetch
  // yesterday's/tomorrow's assignments and label them "Heute geplant".
  const today = useBerlinToday();
  // Auf einem Handy hat auch die Startseite keine feste Kartenhöhe (eine
  // Spalte, jede Karte so hoch wie ihr Inhalt). Die kompakte einzeilige Form
  // würde dort nur den Raum abschneiden — also gilt sie erst ab der
  // zweispaltigen Ansicht, und gekappt wird auf dem Handy gar nicht. Ab dem
  // laufenden Einsatz beginnt die Karte aber auf jedem Gerät: `maxRows`
  // unterscheidet die Startseite von der Zeiterfassung, nicht die Breite.
  const isPhone = useMediaQuery(BELOW_SM);
  const now = useBerlinClock();
  const compact = dense && !isPhone;
  const onHome = maxRows !== undefined;
  const rowLimit = isPhone ? undefined : maxRows;
  const { data: assignments, error } = useSWRAuth<OwnAssignment[]>(
    `time-tracking-own-assignments-today-${today}`,
    () => ownShiftService.getOwnAssignments(today, today),
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );

  // A fetch failure must stay distinguishable from "keine Einsätze geplant":
  // with only one retry and focus revalidation off, silently rendering the
  // empty-state (null) would hide the employee's schedule until a reload.
  if (error) {
    return (
      <InfoCard
        title={title}
        icon={<MotoConceptIcon concept="carePlan" size={20} />}
      >
        <Alert
          type="error"
          message="Die heutigen Einsätze konnten nicht geladen werden. Bitte die Seite neu laden."
        />
      </InfoCard>
    );
  }

  const blocks = (assignments ?? [])
    .filter((a) => a.date === today)
    .slice()
    .sort((a, b) => a.startTime.localeCompare(b.startTime));

  if (blocks.length === 0) {
    if (!showEmpty) return null;
    return (
      <InfoCard
        title={title}
        icon={<MotoConceptIcon concept="carePlan" size={20} />}
        actions={
          href ? (
            <Link
              href={href}
              aria-label={`${title}: zum Tagesplan`}
              className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
            >
              Zum Tagesplan
              <ChevronRight className="h-4 w-4" aria-hidden="true" />
            </Link>
          ) : undefined
        }
      >
        <EmptyState
          className="py-4"
          title="Heute ist für Sie nichts geplant"
          description="Ihre Einsätze aus dem Betreuungsplan erscheinen hier."
        />
      </InfoCard>
    );
  }

  // Ab dem Einsatz, der gerade läuft — sonst stünde nachmittags immer noch
  // der Frühdienst in der Karte. Auf der Zeiterfassung bleibt der ganze Tag
  // stehen, dort ist die Vergangenheit Teil der Antwort.
  const relevant = onHome
    ? upcomingFirst(blocks, (block) => block.endTime, now)
    : blocks;
  // Der Hinweis auf den Rest kostet selbst eine Zeile Platz: passt nicht
  // alles, steht eine Zeile weniger da, statt dass der Hinweis herausragt.
  const fits = rowLimit === undefined || relevant.length <= rowLimit;
  const shown = fits ? relevant : relevant.slice(0, Math.max(rowLimit - 1, 1));
  const hidden = relevant.length - shown.length;

  return (
    <InfoCard
      title={title}
      icon={<MotoConceptIcon concept="carePlan" size={20} />}
      actions={
        href ? (
          <Link
            href={href}
            aria-label={`${title}: zum Tagesplan`}
            className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
          >
            Zum Tagesplan
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        ) : undefined
      }
    >
      {/* Auf der Startseite hat die Karte eine feste Höhe im Raster. Sie
          scrollt NICHT: eine angeschnittene Zeile am Kartenrand sieht aus, als
          liefe der Baustein aus seiner Karte heraus. Stattdessen stehen so
          viele Einsätze da, wie ganz hineinpassen, und darunter der Rest als
          eine Zeile mit Zahl und Weg. Ohne Höhenvorgabe (Zeiterfassung) steht
          der ganze Tag da, in der gewohnten zweizeiligen Form. */}
      <ul
        className={
          compact
            ? "min-h-0 flex-1 space-y-2 overflow-hidden"
            : "divide-y divide-gray-100"
        }
      >
        {shown.map((block) => (
          <AssignmentRow
            key={block.instanceId}
            block={block}
            href={href}
            dense={compact}
            // Der Zustand relativ zur Uhr gehört auf die Startseite, wo die
            // Karte den Moment trägt. Auf der Zeiterfassung steht der Tag als
            // Liste, und „vorbei" wäre an jeder zweiten Zeile nur Rauschen.
            now={onHome && now !== "" ? now : undefined}
          />
        ))}
      </ul>
      {hidden > 0 && href && (
        <Link
          href={href}
          className="mt-2 block shrink-0 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          {hidden === 1
            ? "Noch 1 Einsatz im Tagesplan"
            : `Noch ${hidden} Einsätze im Tagesplan`}
        </Link>
      )}
    </InfoCard>
  );
}

function AssignmentRow({
  block,
  href,
  dense,
  now,
}: {
  readonly block: OwnAssignment;
  readonly href?: string;
  readonly dense: boolean;
  /** Uhrzeit „HH:MM" für den Zustand der Zeile; ohne Angabe kein Zustand. */
  readonly now?: string;
}) {
  const dimmed = block.cancelled || block.isAbsent;
  const phase =
    now !== undefined && !dimmed
      ? blockPhase(block.startTime, block.endTime, now)
      : null;
  // Zwei Zeilenformen für zwei Orte. Auf der Zeiterfassung steht der ganze Tag
  // in einer Karte ohne Höhengrenze: dort bleibt der Eintrag zweizeilig, mit
  // dem Raum unter der Aufgabe. Auf der Startseite hat die Karte eine feste
  // Höhe — zweizeilige Einträge lassen dort nur einen ganz hineinpassen, und
  // ein einzelner Einsatz ist kein Tag.
  const rowClass = dense
    ? "flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
    : "flex flex-col gap-1 py-3 first:pt-0 last:pb-0 sm:flex-row sm:items-start sm:gap-4";
  const secondary = [
    block.roomName,
    block.groupName !== block.title ? block.groupName : "",
  ]
    .filter(Boolean)
    .join(" · ");
  const content = (
    <>
      <span
        className={`${dense ? "w-24" : "w-28"} flex-shrink-0 text-sm font-medium tabular-nums ${
          dimmed ? "text-gray-400 line-through" : "text-gray-900"
        }`}
      >
        {block.startTime}–{block.endTime}
      </span>
      {dense ? (
        <p className="min-w-0 flex-1 truncate text-sm">
          <span
            className={`font-medium ${dimmed ? "text-gray-400" : "text-gray-900"}`}
          >
            {block.title}
          </span>
          {secondary && <span className="text-gray-500"> · {secondary}</span>}
        </p>
      ) : (
        <div className="min-w-0 flex-1">
          <p
            className={`truncate text-sm font-medium ${
              dimmed ? "text-gray-400" : "text-gray-900"
            }`}
          >
            {block.title}
            {block.groupName && block.groupName !== block.title && (
              <span className="font-normal text-gray-500">
                {" "}
                · {block.groupName}
              </span>
            )}
          </p>
          {block.roomName && (
            <p className="mt-0.5 flex items-center gap-1 text-xs text-gray-500">
              <MotoConceptIcon concept="rooms" size={16} />
              {block.roomName}
            </p>
          )}
        </div>
      )}
      {/* Kit StatusBadge — the local Badge copy carried its own tone map of the
          same brand hexes. Abwesend and Unterbesetzt both land on the orange
          tone; they never appear on the same block. */}
      <div
        className={`flex flex-wrap items-center gap-1.5 ${
          dense ? "shrink-0 justify-end" : ""
        }`}
      >
        {block.isSubstitute && <StatusBadge tone="blue" label="Vertretung" />}
        {/* Dieselben Wörter wie im Tagesplan und in der Jetzt-Zone: „Läuft"
            für den Moment, die Zeit bis zum Beginn für das Kommende. */}
        {phase === "running" && <StatusBadge tone="green" label="Läuft" />}
        {phase === "upcoming" && (
          <StatusBadge
            tone="blue"
            label={formatMinutesAhead(minutesBetween(now!, block.startTime))}
          />
        )}
        {block.cancelled && (
          <StatusBadge
            tone="red"
            label={
              block.cancelReason
                ? `Entfällt · ${block.cancelReason}`
                : "Entfällt"
            }
          />
        )}
        {block.isAbsent && (
          <StatusBadge
            tone="orange"
            label={
              block.absenceReason
                ? `Abwesend · ${block.absenceReason}`
                : "Abwesend"
            }
          />
        )}
        {block.understaffedAck && !block.cancelled && (
          <StatusBadge tone="orange" label="Unterbesetzt" />
        )}
      </div>
    </>
  );

  // Mit Ziel ist die Zeile ein Link und sieht auch so aus; ohne Ziel bleibt
  // sie eine Anzeige — nichts, was sich anfassen lässt, ohne etwas zu tun.
  return (
    <li className={href ? undefined : rowClass}>
      {href ? (
        <Link
          href={href}
          aria-label={`${block.title}: im Tagesplan öffnen`}
          className={`${rowClass} transition-colors hover:bg-gray-100/50`}
        >
          {content}
        </Link>
      ) : (
        content
      )}
    </li>
  );
}
