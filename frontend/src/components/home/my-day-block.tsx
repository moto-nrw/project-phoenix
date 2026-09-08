"use client";

import { useCallback, useState } from "react";
import { ChevronRight } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import Link from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { BlockState } from "~/components/home/day-flow-block";
import {
  HOME_CARD_BODY,
  HomeCardIcon,
} from "~/components/home/home-block-content";
import {
  HomeMoreRow,
  useBerlinClock,
  useHomeCardRows,
} from "~/components/home/home-card-rows";
import { useDayPlanHref, useDayPlanLabel } from "~/lib/hooks/use-day-plan-href";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import { canStartPlannedInstance } from "~/lib/timetable-lifecycle";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const logger = createLogger({ component: "MyDayBlock" });

/**
 * So viele Blöcke passen in eine Karte von drei Rasterzeilen ganz hinein
 * (gemessen: Körper 272 px, Zeile 44 px, Jetzt-Linie 20 px).
 */
const MAX_ROWS = 5;

/**
 * Baustein „Mein Tag" (#2180): der Betreuungstag der Person, so wie ihn der
 * Tagesplan zeigt — nur auf die Blöcke gekürzt, für die sie eingeteilt ist.
 *
 * Das ist die Karte, von der aus eine Betreuungskraft arbeitet: der GANZE
 * Tag mit Zeit, Raum, Kinderzahl und Kolleginnen, Vergangenes gedimmt, die
 * Jetzt-Linie vor dem Block, der als Nächstes zählt, und der Starten-Knopf
 * am eigenen Block, sobald er starten darf. Ein laufender Block führt in
 * seine Kinderliste. Nichts davon muss man im Tagesplan suchen; der bleibt
 * der Weg zu den anderen Tagen und zu den Blöcken der anderen.
 *
 * Dieselbe Abfrage wie „Ablauf des Tages" (gleicher Schlüssel): stehen beide
 * Karten auf dem Brett, fragt die Seite den Server nur einmal.
 */
export function MyDayBlock() {
  const dayPlanHref = useDayPlanHref();
  const dayPlanLabel = useDayPlanLabel();
  const router = useTenantRouter();
  const now = useBerlinClock();
  const { data, error, isLoading, mutate } = useSWRAuth<
    PlannedTimetableInstance[]
  >(
    "home-day-flow",
    () =>
      timetableOperationsApi.plannedNow({
        scope: "day",
        includeRoster: false,
      }),
    { refreshInterval: 5 * 60 * 1000 },
  );

  const [startBusyId, setStartBusyId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const start = useCallback(
    async (instance: PlannedTimetableInstance) => {
      setStartBusyId(instance.id);
      setActionError(null);
      try {
        const result = await timetableOperationsApi.start(instance.id);
        router.push(`/active-supervisions?session=${result.activeGroupId}`);
      } catch (err) {
        logger.error("home_my_day_start_failed", {
          instance_id: instance.id,
          error: err instanceof Error ? err.message : String(err),
        });
        setActionError(
          "Der Block konnte nicht gestartet werden. Bitte noch einmal versuchen.",
        );
        await mutate();
      } finally {
        setStartBusyId(null);
      }
    },
    [mutate, router],
  );

  const mine = [...(data ?? [])]
    .filter((block) => block.isAssigned)
    .sort(
      (a, b) =>
        a.startTime.localeCompare(b.startTime) ||
        a.endTime.localeCompare(b.endTime),
    );
  const { shown, hidden } = useHomeCardRows(mine, MAX_ROWS);
  // Die Jetzt-Linie steht vor dem ersten Block, der noch nicht vorbei ist.
  // Ein Block, der laut Server läuft, zählt als „jetzt", auch über seine
  // geplante Endzeit hinaus. Ohne Uhr (erstes Rendern) gibt es keine Linie.
  const nowIndex =
    now === ""
      ? -1
      : (() => {
          const index = shown.findIndex(
            (block) => block.status === "active" || block.endTime > now,
          );
          return index === -1 ? shown.length : index;
        })();

  return (
    <SectionCard
      title="Mein Tag"
      leading={<HomeCardIcon concept="carePlan" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
      actions={
        <Link
          href={dayPlanHref}
          aria-label={`Mein Tag: ${dayPlanLabel}`}
          className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          {dayPlanLabel}
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      }
    >
      {(() => {
        if (error) {
          return (
            <Alert
              type="error"
              message="Ihr Tag konnte nicht geladen werden. Bitte die Seite neu laden."
            />
          );
        }
        if (isLoading && data === undefined) {
          return (
            <div className="space-y-2" aria-hidden="true">
              {[1, 2, 3, 4].map((i) => (
                <div
                  key={i}
                  className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
                >
                  <div className="h-4 w-20 animate-pulse rounded bg-gray-200"></div>
                  <div className="h-4 flex-1 animate-pulse rounded bg-gray-200"></div>
                </div>
              ))}
            </div>
          );
        }
        if (mine.length === 0) {
          return (
            <EmptyState
              className="py-4"
              title="Heute sind Sie für keinen Block eingeteilt"
              description="Ihre Einsätze aus dem Betreuungsplan erscheinen hier, mit Raum, Kindern und dem Starten-Knopf."
            />
          );
        }
        return (
          <>
            {actionError && <Alert type="error" message={actionError} />}
            <ul className="space-y-1.5">
              {shown.map((block, index) => (
                <BlockRow
                  key={block.id}
                  block={block}
                  now={now}
                  withNowLine={index === nowIndex}
                  dayPlanHref={dayPlanHref}
                  busy={startBusyId === block.id}
                  onStart={start}
                />
              ))}
              {/* Der Tag ist vorbei: die Linie steht unter dem letzten Block,
                  damit „heute war das" eine Antwort bleibt. */}
              {nowIndex === shown.length && hidden === 0 && (
                <li>
                  <NowLine now={now} />
                </li>
              )}
            </ul>
            <HomeMoreRow
              hidden={hidden}
              href={dayPlanHref}
              label="Einsätze im Tagesplan"
            />
          </>
        );
      })()}
    </SectionCard>
  );
}

/** Die Jetzt-Linie: dieselbe wie im Tagesplan, nur schmaler. */
function NowLine({ now }: { readonly now: string }) {
  return (
    <span
      className="flex items-center gap-2 px-1 text-xs font-medium text-gray-500"
      aria-label={`Jetzt, ${now} Uhr`}
    >
      <span className="bg-moto-green h-px flex-1" aria-hidden="true"></span>
      Jetzt · {now} Uhr
      <span className="bg-moto-green h-px flex-1" aria-hidden="true"></span>
    </span>
  );
}

/**
 * Ein Block des eigenen Tages. Zeit, Name, Raum, Kinderzahl und Kolleginnen
 * in einer Zeile mit Untertitel; rechts, was der Block gerade ist — oder
 * der Knopf, ihn zu starten.
 */
function BlockRow({
  block,
  now,
  withNowLine,
  dayPlanHref,
  busy,
  onStart,
}: {
  readonly block: PlannedTimetableInstance;
  readonly now: string;
  readonly withNowLine: boolean;
  readonly dayPlanHref: string;
  readonly busy: boolean;
  readonly onStart: (block: PlannedTimetableInstance) => void;
}) {
  const running = block.status === "active";
  const cancelled = block.status === "cancelled";
  const startable =
    block.status === "planned" && canStartPlannedInstance(block, new Date());
  // Vorbei ist vorbei: gedimmt, damit das Laufende und Kommende von selbst
  // hervortritt — wie im Tagesplan.
  const missed =
    block.status === "planned" && now !== "" && block.endTime <= now;
  const over = block.status === "completed" || missed;
  // Ein laufender Block führt in seine Kinderliste; jeder andere in den
  // Tagesplan, wo er bedient wird.
  const href =
    running && block.activeGroupId
      ? `/active-supervisions?session=${block.activeGroupId}`
      : dayPlanHref;
  const room = block.roomName ?? `Raum ${block.roomId}`;
  // Kinderzahl so knapp wie im Tagesplan: laufend „12 von 18 da", danach
  // nur noch, wer da war, davor, wer erwartet wird.
  const count =
    block.status === "completed"
      ? block.presentStudentsCount
      : block.expectedStudentsCount;
  const children = cancelled
    ? null
    : running
      ? `${block.presentStudentsCount} von ${block.expectedStudentsCount} da`
      : count === 1
        ? "1 Kind"
        : `${count} Kinder`;
  const staff = (block.staffNames ?? [])
    .map((entry) =>
      entry.isSubstitute
        ? `${entry.displayName} (Vertretung)`
        : entry.displayName,
    )
    .join(", ");
  // Was vorbei ist, steht als Wort in der Zeile, nicht als Marke rechts:
  // auf dem Handy nähme die Marke dem Namen den Platz, und „Beendet" muss
  // niemandem ins Auge springen. Rechts steht nur, was JETZT zählt.
  const meta = [
    block.status === "completed" ? "Beendet" : null,
    missed ? "Nicht gestartet" : null,
    room,
    block.groupName,
    children,
    staff || null,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <li>
      {withNowLine && (
        <div className="mb-1.5">
          <NowLine now={now} />
        </div>
      )}
      <div
        className={`flex items-center gap-3 rounded-xl bg-gray-50/50 px-3 py-2 sm:py-1 ${
          over ? "opacity-60" : ""
        }`}
      >
        <Link
          href={href}
          aria-label={`${block.title} ${block.startTime} bis ${block.endTime}: ${running ? "Kinderliste öffnen" : "im Tagesplan öffnen"}`}
          className="flex min-w-0 flex-1 items-center gap-3"
        >
          <span className="w-14 shrink-0 sm:w-16">
            <span className="block text-sm font-semibold text-gray-900 tabular-nums">
              {block.startTime}
            </span>
            <span className="block text-xs text-gray-500 tabular-nums">
              bis {block.endTime}
            </span>
          </span>
          <span className="min-w-0 flex-1">
            <span
              className={`block truncate text-sm text-gray-900 ${running ? "font-semibold" : "font-medium"}`}
            >
              {block.title}
              {block.isSubstitute && (
                <span className="ml-2 text-xs font-normal text-gray-500">
                  Vertretung
                </span>
              )}
            </span>
            <span className="block truncate text-xs text-gray-500">
              {cancelled && block.cancelReason
                ? `Fällt aus · ${block.cancelReason}`
                : meta}
            </span>
          </span>
        </Link>
        <span className="flex shrink-0 items-center gap-2">
          {startable ? (
            <Button
              type="button"
              size="md"
              variant="success"
              disabled={busy}
              onClick={() => onStart(block)}
            >
              {busy ? "Startet..." : "Starten"}
            </Button>
          ) : block.isAbsent ? (
            <StatusBadge tone="gray" label="Sie fehlen" />
          ) : over ? null : (
            <BlockState block={block} now={now} />
          )}
          {running && block.activeGroupId && (
            <ChevronRight
              className="h-4 w-4 text-gray-400"
              aria-hidden="true"
            />
          )}
        </span>
      </div>
    </li>
  );
}
