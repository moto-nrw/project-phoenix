"use client";

import type { ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { BirthdayList } from "~/components/dashboard/birthday-list";
import { DayFlowBlock } from "~/components/home/day-flow-block";
import { OpenRequestsBlock } from "~/components/home/open-requests-block";
import { StaffNoticesBlock } from "~/components/home/staff-notices-block";
import { BetreuungsplanHeuteCard } from "~/components/time-tracking/betreuungsplan-heute-card";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import Link from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import type { BirthdayOverview } from "~/lib/birthdays-api";
import {
  formatRecentActivityTime,
  getActivityStatusColor,
  getGroupStatusColor,
  type DashboardAnalytics,
} from "~/lib/dashboard-helpers";
import type { HomeBlockKey } from "~/lib/home-blocks";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";

/**
 * Der Inhalt eines Bausteins der Startseite (#2180).
 *
 * Ein Baustein weiß nichts davon, wo er steht oder wie breit er ist — das
 * entscheidet die Person im Anpassen-Modus, und das Brett setzt es um. Hier
 * steht nur, was in der Karte steht.
 *
 * Die Kennzahlen und die drei Auswertungslisten teilen sich EINE Abfrage der
 * Betriebszahlen, die die Seite stellt und hierher reicht. Deshalb nehmen sie
 * ihre Daten als Eingabe statt sie selbst zu holen: fünf Kacheln nebeneinander
 * dürfen nicht fünf Abfragen bedeuten. Die vier Bausteine mit eigener Quelle
 * (Mein Tag, Tagesinformationen, Ablauf des Tages, offene Anfragen) laden für
 * sich, weil sie sonst die Seite blockieren würden.
 */
export interface HomeBlockData {
  readonly analytics: DashboardAnalytics | undefined;
  readonly analyticsLoading: boolean;
  readonly birthdays: BirthdayOverview | undefined;
  readonly birthdaysLoading: boolean;
  readonly tenantPath: (path: string) => string;
}

/** Kartenfläche der Listen: füllt ihren Platz, der Inhalt scrollt in ihr. */
function ListCard({
  title,
  concept,
  href,
  linkText = "Ansehen",
  children,
}: Readonly<{
  title: string;
  concept: MotoConceptKey;
  href?: string;
  linkText?: string;
  children: ReactNode;
}>) {
  return (
    <SectionCard
      title={title}
      className="flex h-full flex-col"
      bodyClassName="mt-4 min-h-0 flex-1 overflow-y-auto"
      leading={
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 shadow-sm">
          <MotoConceptIcon concept={concept} size={20} />
        </span>
      }
      actions={
        href ? (
          <Link
            href={href}
            // Der Kartentitel gehört in den Linknamen: „Ansehen" allein sagt
            // in der Vorlesereihenfolge nicht, was man ansieht.
            aria-label={`${title}: ${linkText}`}
            className="flex items-center gap-1 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
          >
            {linkText}
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        ) : undefined
      }
    >
      {children}
    </SectionCard>
  );
}

function RowSkeleton({ withBadge = false }: { readonly withBadge?: boolean }) {
  return (
    <div className="space-y-2" aria-hidden="true">
      {[1, 2, 3].map((i) => (
        <div
          key={i}
          className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3"
        >
          <div className="min-w-0 flex-1 space-y-1.5">
            <div className="h-4 w-2/5 animate-pulse rounded bg-gray-200"></div>
            <div className="h-3 w-1/4 animate-pulse rounded bg-gray-200"></div>
          </div>
          {withBadge ? (
            <div className="h-6 w-16 animate-pulse rounded-full bg-gray-200"></div>
          ) : (
            <div className="ml-2 h-2.5 w-2.5 flex-shrink-0 animate-pulse rounded-full bg-gray-200"></div>
          )}
        </div>
      ))}
    </div>
  );
}

function RecentActivityCard({ data }: { readonly data: HomeBlockData }) {
  const activities = data.analytics?.recentActivity;
  return (
    <ListCard title="Letzte Bewegungen" concept="changeHistory">
      {data.analyticsLoading ? (
        <RowSkeleton withBadge />
      ) : !activities || activities.length === 0 ? (
        <EmptyState className="py-8" title="Keine aktuellen Bewegungen" />
      ) : (
        <div className="space-y-2">
          {activities.slice(0, 5).map((activity, idx) => {
            const ts = new Date(activity.timestamp).getTime();
            const tsKey = Number.isFinite(ts) ? ts : `idx-${idx}`;
            return (
              <div
                key={`${activity.type}-${activity.groupName}-${activity.roomName}-${tsKey}`}
                className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
              >
                <div className="min-w-0 flex-1">
                  <p className="flex items-center gap-1.5 text-sm font-medium text-gray-900">
                    <span className="truncate">{activity.groupName}</span>
                    <ChevronRight
                      className="h-3.5 w-3.5 flex-shrink-0 text-gray-400"
                      aria-hidden="true"
                    />
                    <span className="truncate">{activity.roomName}</span>
                  </p>
                  {activity.count > 1 && (
                    <p className="text-xs text-gray-500">
                      {activity.count} Kinder
                    </p>
                  )}
                </div>
                <span className="ml-2 flex-shrink-0 text-xs text-gray-500">
                  {formatRecentActivityTime(activity.timestamp)}
                </span>
              </div>
            );
          })}
        </div>
      )}
    </ListCard>
  );
}

function CurrentActivitiesCard({ data }: { readonly data: HomeBlockData }) {
  const activities = data.analytics?.currentActivities;
  return (
    <ListCard
      title="Laufende Aktivitäten"
      concept="activities"
      href={data.tenantPath("/activities")}
    >
      {data.analyticsLoading ? (
        <RowSkeleton />
      ) : !activities || activities.length === 0 ? (
        <EmptyState className="py-8" title="Keine laufenden Aktivitäten" />
      ) : (
        <div className="space-y-2">
          {activities.slice(0, 5).map((activity) => (
            <div
              key={activity.id}
              className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-900">
                  {activity.name}
                </p>
                <p className="text-xs text-gray-500">
                  {activity.category} • {activity.participants}
                  {activity.maxCapacity == null
                    ? " Teilnehmer"
                    : `/${activity.maxCapacity} Teilnehmer`}
                </p>
              </div>
              <div
                className={`h-2.5 w-2.5 rounded-full ${getActivityStatusColor(activity.status)} ml-2 flex-shrink-0`}
              ></div>
            </div>
          ))}
        </div>
      )}
    </ListCard>
  );
}

function ActiveGroupsCard({ data }: { readonly data: HomeBlockData }) {
  const groups = data.analytics?.activeGroupsSummary;
  return (
    <ListCard
      title="Aktive Gruppen"
      concept="groups"
      href={data.tenantPath("/ogs-groups")}
    >
      {data.analyticsLoading ? (
        <RowSkeleton />
      ) : !groups || groups.length === 0 ? (
        <EmptyState className="py-8" title="Keine aktiven Gruppen" />
      ) : (
        <div className="space-y-2">
          {groups.slice(0, 5).map((group) => (
            <div
              key={`${group.type}-${group.name}`}
              className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-gray-900">
                  {group.name}
                </p>
                <p className="text-xs text-gray-500">
                  {group.location} • {group.studentCount} Kinder
                </p>
              </div>
              <div
                className={`h-2.5 w-2.5 rounded-full ${getGroupStatusColor(group.status)} ml-2 flex-shrink-0`}
              ></div>
            </div>
          ))}
        </div>
      )}
    </ListCard>
  );
}

/** Kennzahl-Kacheln: Beschriftung, Wert, Symbol und wohin sie führen. */
function StatBlock({
  blockKey,
  data,
}: {
  readonly blockKey: HomeBlockKey;
  readonly data: HomeBlockData;
}) {
  const analytics = data.analytics;
  const tile = (
    label: string,
    value: string | number,
    concept: keyof typeof MOTO_CONCEPTS,
    href?: string,
  ) => (
    <StatCard
      label={label}
      value={value}
      icon={
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS[concept].icon}
          tone={MOTO_CONCEPTS[concept].tone}
        />
      }
      loading={data.analyticsLoading}
      href={href}
    />
  );

  switch (blockKey) {
    case "tile.students_present":
      return tile(
        "Kinder anwesend",
        analytics?.studentsPresent ?? 0,
        "present",
        data.tenantPath("/students/search"),
      );
    case "tile.students_in_rooms":
      return tile(
        "In Räumen",
        analytics?.studentsInRooms ?? 0,
        "rooms",
        data.tenantPath("/students/search"),
      );
    case "tile.students_in_transit":
      return tile(
        "Unterwegs",
        analytics?.studentsInTransit ?? 0,
        "transit",
        data.tenantPath("/students/search?status=unterwegs"),
      );
    case "tile.students_on_playground":
      return tile(
        "Schulhof",
        analytics?.studentsOnPlayground ?? 0,
        "schoolyard",
        data.tenantPath("/students/search?status=schulhof"),
      );
    case "tile.students_sick":
      return tile(
        "Krank",
        analytics?.studentsSick ?? 0,
        "sick",
        data.tenantPath("/students/search?status=krank"),
      );
    case "tile.students_excused":
      return tile(
        "Entschuldigt",
        analytics?.studentsExcused ?? 0,
        "excused",
        data.tenantPath("/students/search?status=entschuldigt"),
      );
    case "tile.students_home":
      return tile(
        "Zuhause",
        analytics?.studentsHome ?? 0,
        "home",
        data.tenantPath("/students/search?status=abwesend"),
      );
    case "tile.active_activities":
      return tile(
        "Aktive Aktivitäten",
        analytics?.activeActivities ?? 0,
        "activities",
        data.tenantPath("/activities"),
      );
    case "tile.capacity_utilization":
      return tile(
        "Auslastung",
        analytics ? `${Math.round(analytics.capacityUtilization * 100)}%` : "0%",
        "utilization",
      );
    default:
      return null;
  }
}

export function HomeBlockContent({
  blockKey,
  data,
}: {
  readonly blockKey: HomeBlockKey;
  readonly data: HomeBlockData;
}) {
  switch (blockKey) {
    case "section.my_day":
      return <BetreuungsplanHeuteCard title="Mein Tag" showEmpty />;
    case "section.staff_notices":
      return <StaffNoticesBlock />;
    case "section.day_flow":
      return <DayFlowBlock />;
    case "section.open_requests":
      return <OpenRequestsBlock />;
    case "section.recent_activity":
      return <RecentActivityCard data={data} />;
    case "section.current_activities":
      return <CurrentActivitiesCard data={data} />;
    case "section.active_groups":
      return <ActiveGroupsCard data={data} />;
    case "section.birthdays":
      return (
        <ListCard title="Geburtstage" concept="birthdays">
          <BirthdayList
            celebrations={data.birthdays?.celebrations ?? []}
            isLoading={data.birthdaysLoading}
          />
        </ListCard>
      );
    default:
      return <StatBlock blockKey={blockKey} data={data} />;
  }
}
