"use client";

import Link from "~/components/ui/navigation-link";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import {
  HOME_CARD_BODY,
  HomeCardIcon,
} from "~/components/home/home-block-content";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatMinutesAhead, minutesBetween } from "~/lib/home-clock";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import { useSWRAuth } from "~/lib/swr";
import { useDayPlanHref, useDayPlanLabel } from "~/lib/hooks/use-day-plan-href";
import {
  HomeCardLink,
  HomeMoreRow,
  upcomingFirst,
  useBerlinClock,
  useHomeCardRows,
} from "~/components/home/home-card-rows";

/** So viele Blöcke passen in eine Karte dieser Höhe ganz hinein. */
const MAX_BLOCKS = 3;

/**
 * Baustein „Ablauf des Tages" (#2180): der Rahmen des Tages, nicht nur die
 * eigenen Einsätze. Der GANZE Tag mit seinen Zuständen — was gelaufen ist,
 * was gerade läuft, was noch kommt; gearbeitet wird weiterhin im Tagesplan,
 * dorthin führt der Weiterlink.
 *
 * Bewusst `scope: "day"` statt des Standardfensters: das liefert nur, was in
 * den nächsten 15 Minuten beginnt, und lässt einen gerade laufenden Block
 * ganz weg. Für einen Baustein, der „Ablauf des Tages" heißt, ist das die
 * falsche Frage.
 *
 * Bewusst ohne Kinderlisten: die Startseite ist eine Übersicht, und die
 * Namen gehören in den Tagesplan (GDPR).
 */
export function DayFlowBlock() {
  // Reine Adminkonten haben keinen Tagesplan in der Navigation; für sie
  // führt der Weiterlink in den Betreuungsplan im Planungsbereich.
  const dayPlanHref = useDayPlanHref();
  const dayPlanLabel = useDayPlanLabel();
  const { data, error, isLoading } = useSWRAuth<PlannedTimetableInstance[]>(
    "home-day-flow",
    () =>
      timetableOperationsApi.plannedNow({
        scope: "day",
        includeRoster: false,
      }),
    { refreshInterval: 5 * 60 * 1000 },
  );

  const now = useBerlinClock();
  const blocks = data ?? [];
  // Ab dem Block, der gerade läuft: sonst stünde um 16 Uhr immer noch die
  // Frühbetreuung in der Karte.
  // Ein Block, der laut Server noch läuft, zählt als „jetzt", auch wenn seine
  // geplante Endzeit vorbei ist — die Jetzt-Zone zählt ihn ebenfalls mit.
  const relevant = upcomingFirst(
    blocks,
    (block) => (block.status === "active" ? "24:00" : block.endTime),
    now,
  );
  const { shown, hidden } = useHomeCardRows(relevant, MAX_BLOCKS);

  return (
    <SectionCard
      title="Ablauf des Tages"
      leading={<HomeCardIcon concept="carePlan" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      actions={
        <HomeCardLink
          href={dayPlanHref}
          label={`Ablauf des Tages: ${dayPlanLabel}`}
        >
          {dayPlanLabel}
        </HomeCardLink>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Der Ablauf des Tages konnte nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && data === undefined) {
          return (
            <div className="space-y-2" aria-hidden="true">
              {[1, 2, 3].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
                >
                  <div className="h-4 w-24 animate-pulse rounded bg-gray-200"></div>
                  <div className="h-4 flex-1 animate-pulse rounded bg-gray-200"></div>
                </div>
              ))}
            </div>
          );
        }
        if (blocks.length === 0) {
          return (
            <EmptyState
              className="py-4"
              title="Gerade steht nichts an"
              description="Hier erscheinen die Blöcke des Betreuungsplans, sobald der nächste beginnt."
            />
          );
        }
        return (
          <>
            <ul className="space-y-2">
              {shown.map((block) => (
                <li key={block.id}>
                  {/* Jede Zeile führt in den Tagesplan, wo der Block bedient
                      wird — wie die Zeilen der Nachbarkarten. */}
                  <Link
                    href={dayPlanHref}
                    aria-label={`${block.title}: im Tagesplan öffnen`}
                    className={`flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2 transition-colors hover:bg-gray-100/50 ${
                      block.status === "completed" ? "opacity-60" : ""
                    }`}
                  >
                    <span className="w-24 flex-shrink-0 text-sm font-medium text-gray-900 tabular-nums">
                      {block.startTime}–{block.endTime}
                    </span>
                    {/* Eine Zeile je Block: zweizeilige Einträge lassen in
                        eine Karte dieser Höhe nur zwei ganz hinein, und zwei
                        Blöcke sind kein Tagesablauf. */}
                    <p className="min-w-0 flex-1 truncate text-sm">
                      <span className="font-medium text-gray-900">
                        {block.title}
                      </span>
                      <span className="text-gray-500">
                        {" · "}
                        {[
                          block.roomName,
                          `${block.presentStudentsCount}/${block.expectedStudentsCount} Kinder`,
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </span>
                    </p>
                    <BlockState block={block} now={now} />
                  </Link>
                </li>
              ))}
            </ul>
            <HomeMoreRow hidden={hidden} href={dayPlanHref} label="Blöcke" />
          </>
        );
      })()}
    </SectionCard>
  );
}

/**
 * Der Zustand eines Blocks relativ zur Uhr — dieselben Wörter wie im
 * Tagesplan („Läuft", „Nicht gestartet", „Fällt aus"), und für das Kommende
 * die Zeit bis zum Beginn statt eines „Geplant", das nichts sagt. „Mein Tag"
 * nutzt dieselbe Marke, damit beide Karten dieselbe Sprache sprechen.
 */
export function BlockState({
  block,
  now,
}: {
  readonly block: PlannedTimetableInstance;
  readonly now: string;
}) {
  if (block.status === "cancelled") {
    return <StatusBadge tone="red" label="Fällt aus" />;
  }
  if (block.status === "active") {
    return <StatusBadge tone="green" label="Läuft" />;
  }
  if (block.status === "completed") {
    return <StatusBadge tone="gray" label="Beendet" />;
  }
  if (block.isOverdue || (now !== "" && block.startTime <= now)) {
    return <StatusBadge tone="orange" label="Nicht gestartet" />;
  }
  // Ohne Uhr (vor dem ersten Rendern im Browser) gibt es keinen Abstand.
  if (now === "") return null;
  return (
    <StatusBadge
      tone="blue"
      label={formatMinutesAhead(minutesBetween(now, block.startTime))}
    />
  );
}
