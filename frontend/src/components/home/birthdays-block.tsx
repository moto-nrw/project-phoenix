"use client";

import { useState } from "react";

import {
  BirthdayList,
  dayMonthLabel,
} from "~/components/dashboard/birthday-list";
import { HOME_CARD_BODY, HomeCardIcon } from "~/components/home/home-card";
import { useHomeCardRows } from "~/components/home/home-card-rows";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { PlanningContextBar } from "~/components/ui/planning-context-bar";
import { SectionCard } from "~/components/ui/section-card";
import {
  fetchBirthdayOverviewClient,
  type BirthdayCelebration,
  type BirthdayOverview,
} from "~/lib/birthdays-api";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useSWRAuth } from "~/lib/swr";

/**
 * So viele Namen passen in die drei Rasterzeilen hohe Karte unter die
 * Wochenleiste, ohne am Rand angeschnitten zu werden.
 */
const MAX_ROWS = 6;

/**
 * Baustein „Geburtstage" (#1542, #3777): die Geburtstage einer Woche von
 * Montag bis Sonntag, mit Blättern in die Wochen davor und danach.
 *
 * Der Rückblick zählt so viel wie die Vorschau: in der OGS wird oft erst
 * nach dem Geburtstag gefeiert, weil das Kind am Tag selbst abgemeldet ist.
 * Wie weit man blättern darf, sagt der Server mit jeder Antwort.
 *
 * Die laufende Woche kommt von der Startseite (`current`, derselbe Schlüssel,
 * über den sie entscheidet, ob es den Baustein überhaupt gibt). Jede andere
 * Woche lädt die Karte selbst.
 */
export function BirthdaysBlock({
  current,
  currentLoading,
  currentError,
}: {
  readonly current: BirthdayOverview | undefined;
  readonly currentLoading: boolean;
  readonly currentError?: Error;
}) {
  // null heißt: die laufende Woche.
  const [weekStart, setWeekStart] = useState<string | null>(null);
  const [allOpen, setAllOpen] = useState(false);

  const other = useSWRAuth<BirthdayOverview>(
    weekStart ? `birthday-overview:${weekStart}` : null,
    () => fetchBirthdayOverviewClient(weekStart),
  );

  const shown = weekStart ? other.data : current;
  const isLoading = weekStart ? other.isLoading && !other.data : currentLoading;
  const bounds = shown ?? current;
  const currentWeekStart = current?.weekStart ?? null;

  const goTo = (target: string) => {
    setWeekStart(target === currentWeekStart ? null : target);
  };
  const base = shown?.weekStart;
  const previous =
    base && bounds && base > bounds.earliestWeekStart
      ? () => goTo(addDays(base, -7))
      : undefined;
  const next =
    base && bounds && base < bounds.latestWeekStart
      ? () => goTo(addDays(base, 7))
      : undefined;

  const celebrations = shown?.celebrations ?? [];
  const today = shown?.today ?? "";
  const isCurrentWeek = weekStart === null;
  // Passt die Woche nicht in die Karte, zählt in der laufenden Woche zuerst,
  // was heute ist und noch kommt: der heutige Geburtstag darf nie hinter
  // den Rest fallen. Die ganze Woche steht im Dialog.
  const { shown: rows } = useHomeCardRows(
    visibleFirst(celebrations, today, isCurrentWeek),
    MAX_ROWS,
  );
  const hiddenCount = celebrations.length - rows.length;

  // Ohne Wochenangabe (ein Server von vor #3777, etwa während eines
  // Deploys) zeigt die Karte die Namen ohne Leiste zum Blättern.
  const hasWeek = Boolean(shown?.weekStart && shown.weekEnd);
  const weekLabel =
    shown && hasWeek
      ? `${relativeWeekLabel(shown.weekStart, currentWeekStart)} · ${rangeLabel(shown)}`
      : undefined;

  return (
    <SectionCard
      title="Geburtstage"
      leading={<HomeCardIcon concept="birthdays" />}
      className="flex h-full flex-col"
      bodyClassName={HOME_CARD_BODY}
    >
      <div className="flex h-full flex-col gap-3">
        {(hasWeek || weekStart) && (
          <PlanningContextBar
            dateLabel={weekLabel}
            onPrevious={previous}
            onNext={next}
            previousLabel="Vorherige Woche"
            nextLabel="Nächste Woche"
            todayLabel="Diese Woche"
            onToday={isCurrentWeek ? undefined : () => setWeekStart(null)}
            withoutContextRow
          />
        )}
        {(weekStart && other.error) || (!current && currentError) ? (
          <Alert
            type="error"
            message="Die Geburtstage konnten nicht geladen werden. Bitte versuchen Sie es noch einmal."
          />
        ) : (
          <div className="min-h-0">
            <BirthdayList
              celebrations={rows}
              today={today}
              isLoading={isLoading}
              emptyTitle="Keine Geburtstage in dieser Woche"
            />
            {hiddenCount > 0 && (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="mt-1 text-gray-600"
                onClick={() => setAllOpen(true)}
              >
                Noch {hiddenCount}{" "}
                {hiddenCount === 1 ? "Geburtstag" : "Geburtstage"} ansehen
              </Button>
            )}
          </div>
        )}
      </div>
      {shown && (
        <Modal
          isOpen={allOpen}
          onClose={() => setAllOpen(false)}
          title={hasWeek ? `Geburtstage ${rangeLabel(shown)}` : "Geburtstage"}
        >
          <BirthdayList
            celebrations={celebrations}
            today={today}
            isLoading={false}
            emptyTitle="Keine Geburtstage in dieser Woche"
          />
        </Modal>
      )}
    </SectionCard>
  );
}

/**
 * In der laufenden Woche beginnt eine zu lange Liste bei heute; vergangene
 * Tage stehen dann nur im Dialog. Andere Wochen lesen sich von Montag an.
 */
function visibleFirst(
  celebrations: readonly BirthdayCelebration[],
  today: string,
  isCurrentWeek: boolean,
): readonly BirthdayCelebration[] {
  if (!isCurrentWeek || celebrations.length <= MAX_ROWS) return celebrations;
  const fromToday = celebrations.filter((entry) => entry.date >= today);
  return fromToday.length > 0 ? fromToday : celebrations;
}

function addDays(isoDate: string, days: number): string {
  const date = parseISODate(isoDate);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

/** "Diese Woche", "Letzte Woche", "In 2 Wochen" … relativ zur laufenden. */
function relativeWeekLabel(
  weekStart: string,
  currentWeekStart: string | null,
): string {
  if (!currentWeekStart) return "Woche";
  const days = Math.round(
    (parseISODate(weekStart).getTime() -
      parseISODate(currentWeekStart).getTime()) /
      (24 * 60 * 60 * 1000),
  );
  const weeks = Math.round(days / 7);
  if (weeks === 0) return "Diese Woche";
  if (weeks === -1) return "Letzte Woche";
  if (weeks === 1) return "Nächste Woche";
  return weeks < 0 ? `Vor ${-weeks} Wochen` : `In ${weeks} Wochen`;
}

/** "28.09.–04.10." aus Montag und Sonntag der Woche. */
function rangeLabel(overview: BirthdayOverview): string {
  return `${dayMonthLabel(overview.weekStart)}–${dayMonthLabel(overview.weekEnd)}`;
}
