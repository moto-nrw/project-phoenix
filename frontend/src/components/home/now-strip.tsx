"use client";

import type { ReactNode } from "react";
import { ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { Skeleton } from "~/components/ui/skeleton";
import { StatusBadge } from "~/components/ui/status-badge";
import { useBerlinClock } from "~/components/home/home-card-rows";
import { fetchDashboardAnalyticsClient } from "~/lib/dashboard-api";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import { formatStatusDate } from "~/lib/date-helpers";
import type { HomeBlockAccess, HomeBlockContext } from "~/lib/home-blocks";
import { formatMinutesAhead } from "~/lib/home-clock";
import { deriveHomeNow, nowActions, type HomeNowState } from "~/lib/home-now";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { createLogger } from "~/lib/logger";
import { ownShiftService } from "~/lib/shift-api";
import type { OwnAssignment } from "~/lib/shift-helpers";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

const logger = createLogger({ component: "HomeNowStrip" });

/**
 * Die Jetzt-Zone der Startseite (#2180): die eine Zeile, die den Moment
 * trägt. Uhrzeit, was gerade läuft oder als Nächstes kommt, und der eine
 * Weg, der jetzt naheliegt.
 *
 * Sie steht fest über dem Brett und ist kein Baustein: „Was steht jetzt an"
 * ist die Frage, für die es die Startseite gibt, und die Antwort darf nicht
 * davon abhängen, ob jemand eine Karte entfernt hat. Alles darunter bleibt
 * frei anordenbar.
 *
 * Die Abfragen teilen ihre Schlüssel mit den Karten darunter („Mein Tag",
 * „Ablauf des Tages", Kennzahlen): steht die Karte auf der Fläche, kostet
 * die Zone keine zweite Abfrage.
 */
export function NowStrip({
  access,
  context,
}: {
  readonly access: HomeBlockAccess;
  readonly context: HomeBlockContext;
}) {
  const tenantPath = useTenantAwarePath();
  const today = useBerlinToday();
  const now = useBerlinClock();
  // `ownSupervision`, nicht `isSupervising`: Letzteres ist wahr, sobald es
  // einen Schulhof gibt — „fortsetzen" kann man nur, was man selbst führt.
  const { ownSupervision, hasGroups } = useOptionalSupervision();

  // Der eigene Tag: für alle, die selbst betreuen. Ein reines Adminkonto hat
  // keine Einsätze; die Abfrage liefe ins Leere.
  const wantsOwn =
    context.timetableEnabled &&
    access.has("time_tracking:own") &&
    (access.caresForGroups || !access.isAdminScope);
  // Die Schule: für den Adminzuschnitt, wenn es überhaupt Blöcke gibt.
  const wantsSchool =
    access.isAdminScope &&
    context.timetableEnabled &&
    context.detailed &&
    access.has("schedules:read");
  const wantsAnalytics = access.isAdminScope && access.has("groups:read");

  const own = useSWRAuth<OwnAssignment[]>(
    wantsOwn ? `time-tracking-own-assignments-today-${today}` : null,
    () => ownShiftService.getOwnAssignments(today, today),
    { revalidateOnFocus: false, errorRetryCount: 1 },
  );
  const school = useSWRAuth<PlannedTimetableInstance[]>(
    wantsSchool ? "home-day-flow" : null,
    () =>
      timetableOperationsApi.plannedNow({ scope: "day", includeRoster: false }),
    { refreshInterval: 5 * 60 * 1000 },
  );
  const analytics = useSWRAuth<DashboardAnalytics>(
    wantsAnalytics ? "dashboard-analytics" : null,
    fetchDashboardAnalyticsClient,
    { refreshInterval: 5 * 60 * 1000 },
  );

  if (own.error) {
    logger.error("home_now_own_failed", {
      error: own.error instanceof Error ? own.error.message : String(own.error),
    });
  }
  if (school.error) {
    logger.error("home_now_school_failed", {
      error:
        school.error instanceof Error
          ? school.error.message
          : String(school.error),
    });
  }

  const loading =
    now === "" ||
    (wantsOwn && own.data === undefined && !own.error) ||
    (wantsSchool && school.data === undefined && !school.error);

  // Ein Fehler in einer Quelle nimmt die Zone nicht mit: dann trägt sie nur
  // Uhrzeit und Aktionen, und die Karte darunter meldet den Fehler.
  const state = deriveHomeNow({
    now,
    own: own.error ? undefined : own.data?.filter((a) => a.date === today),
    school: school.error ? undefined : school.data,
  });

  const canOpenGroup =
    hasGroups && !context.openCareGroupMode && context.detailed;
  const actions = nowActions({
    isSupervising: ownSupervision === true,
    canOpenGroup,
    canReadUsers: access.has("users:read"),
    tenantPath,
  });

  return (
    <SectionCard testId="home-now">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
        <div className="flex min-w-0 flex-1 items-center gap-4">
          {/* Die Uhr ist der Anker der Zone: alles daneben ist relativ zu ihr
              gemeint („bis 11:00", „in 25 Min"). */}
          <time
            dateTime={`${today}T${now}`}
            className="shrink-0 text-3xl font-semibold text-gray-900 tabular-nums"
          >
            {now}
          </time>
          <span aria-hidden="true" className="h-10 w-px shrink-0 bg-gray-200" />
          {loading ? (
            <div className="min-w-0 flex-1 space-y-2" aria-hidden="true">
              <Skeleton className="h-4 w-2/5 rounded" />
              <Skeleton className="h-3 w-3/5 rounded" />
            </div>
          ) : (
            <NowText state={state} analytics={analytics.data} today={today} />
          )}
        </div>
        {/* Auch auf dem Telefon eine Zeile in Knopfgröße: untereinander in
            voller Breite waren die zwei Wege dort größer als die Uhr, um die
            es in der Zone geht. */}
        {actions.length > 0 && (
          <div className="flex shrink-0 flex-wrap gap-2">
            {actions.map((action, index) => (
              <ButtonLink
                key={action.href}
                href={action.href}
                variant={index === 0 ? "primary" : "outline"}
                size="md"
                className="whitespace-nowrap"
              >
                {action.label}
              </ButtonLink>
            ))}
          </div>
        )}
      </div>
    </SectionCard>
  );
}

function NowText({
  state,
  analytics,
  today,
}: {
  readonly state: HomeNowState;
  readonly analytics: DashboardAnalytics | undefined;
  readonly today: string;
}) {
  switch (state.kind) {
    case "own_running": {
      const { block, next } = state;
      return (
        <Lines
          headline={block.title}
          badges={
            <>
              <StatusBadge tone="green" label="Läuft" />
              {block.isSubstitute && (
                <StatusBadge tone="blue" label="Vertretung" />
              )}
            </>
          }
          detail={[
            block.roomName,
            `bis ${block.endTime}`,
            next
              ? `danach ${next.startTime} ${next.title}`
              : "danach ist für heute nichts mehr geplant",
          ]}
        />
      );
    }
    case "own_next": {
      const { block, minutesAhead } = state;
      return (
        <Lines
          headline={`Als Nächstes: ${block.title}`}
          badges={
            <>
              <StatusBadge
                tone="blue"
                label={formatMinutesAhead(minutesAhead)}
              />
              {block.isSubstitute && (
                <StatusBadge tone="blue" label="Vertretung" />
              )}
            </>
          }
          detail={[`${block.startTime} bis ${block.endTime}`, block.roomName]}
        />
      );
    }
    case "own_done":
      return (
        <Lines
          headline="Für heute ist alles erledigt"
          detail={[
            state.count === 1
              ? "1 Einsatz heute"
              : `${state.count} Einsätze heute`,
          ]}
        />
      );
    case "school": {
      const { running, notStarted, next, minutesAhead, total } = state;
      const headline =
        running > 0
          ? running === 1
            ? "1 Block läuft"
            : `${running} Blöcke laufen`
          : next
            ? "Gerade läuft kein Block"
            : total > 0
              ? "Für heute ist alles vorbei"
              : "Heute ist keine Betreuung geplant";
      return (
        <Lines
          headline={headline}
          badges={
            notStarted > 0 ? (
              <StatusBadge
                tone="orange"
                label={`${notStarted} nicht gestartet`}
              />
            ) : undefined
          }
          detail={[
            analytics ? childrenPresent(analytics.studentsPresent) : "",
            analytics ? staffOnDuty(analytics.supervisorsToday) : "",
            next && minutesAhead !== null
              ? `als Nächstes ${next.startTime} ${next.title} (${formatMinutesAhead(minutesAhead)})`
              : "",
          ]}
        />
      );
    }
    case "plain":
      return (
        <Lines
          headline={formatStatusDate(today)}
          detail={
            analytics
              ? [
                  childrenPresent(analytics.studentsPresent),
                  staffOnDuty(analytics.supervisorsToday),
                ]
              : []
          }
        />
      );
  }
}

function childrenPresent(count: number): string {
  return count === 1 ? "1 Kind da" : `${count} Kinder da`;
}

function staffOnDuty(count: number): string {
  // „vom Team" statt „Kräfte": dasselbe Wort wie der Baustein „Personal
  // heute" und die Seitenleiste, und es liest sich wie gesprochen.
  return `${count} vom Team in Aufsicht`;
}

function Lines({
  headline,
  badges,
  detail,
}: {
  readonly headline: string;
  readonly badges?: ReactNode;
  readonly detail: readonly string[];
}) {
  const text = detail.filter(Boolean).join(" · ");
  return (
    <div className="min-w-0 flex-1">
      {/* Auf dem Handy darf der Satz umbrechen; erst nebeneinander mit den
          Knöpfen wird er auf eine Zeile gekürzt. */}
      <p className="flex flex-wrap items-center gap-2">
        <span className="text-base font-semibold text-gray-900 sm:truncate">
          {headline}
        </span>
        {badges}
      </p>
      {text && <p className="text-sm text-gray-500 sm:truncate">{text}</p>}
    </div>
  );
}
