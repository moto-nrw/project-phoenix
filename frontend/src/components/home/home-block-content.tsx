"use client";

import type { ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { BirthdayList } from "~/components/dashboard/birthday-list";
import { DayFlowBlock } from "~/components/home/day-flow-block";
import { OpenRequestsBlock } from "~/components/home/open-requests-block";
import { RemindersBlock } from "~/components/home/reminders-block";
import { StaffNoticesBlock } from "~/components/home/staff-notices-block";
import { BetreuungsplanHeuteCard } from "~/components/time-tracking/betreuungsplan-heute-card";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import Link from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { HomeMoreRow, useHomeCardRows } from "~/components/home/home-card-rows";
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
  /**
   * Wohin der Tag fuer diese Person fuehrt: Betreuungskraefte in den
   * Tagesplan, reine Adminkonten in den Betreuungsplan. Die Seitenleiste
   * blendet den Tagesplan fuer sie aus, ein Weiterlink dorthin ginge ins
   * Leere.
   */
  readonly dayPlanHref: string;
}

/**
 * Der scrollende Körper jeder Karte der Startseite.
 *
 * `-mx-1 px-1` gibt dem Schatten von Knöpfen und Zeilen den Platz, den ihm
 * `overflow-y-auto` sonst am Rand abschneidet — daher sah der Knopf „Zur
 * Kenntnis nehmen" aus, als wäre er angeschnitten.
 */
export const HOME_CARD_BODY =
  "moto-scroll-fade mt-4 -mx-1 min-h-0 flex-1 overflow-y-auto px-1";

/**
 * Symbolfläche aller Karten der Startseite: ein Kasten, eine Größe. Vorher
 * trugen die einen ihr Symbol im grauen Kasten und die anderen nackt — auf
 * einer Fläche nebeneinander fällt genau das auf.
 */
export function HomeCardIcon({
  concept,
}: {
  readonly concept: MotoConceptKey;
}) {
  return (
    <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 shadow-sm">
      <MotoConceptIcon concept={concept} size={20} />
    </span>
  );
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
      bodyClassName={HOME_CARD_BODY}
      leading={<HomeCardIcon concept={concept} />}
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

/**
 * Platzhalterzeilen in der Höhe der echten: drei zweizeilige Blöcke ragen aus
 * einer Karte dieser Höhe heraus, und beim Laden blitzt ein Scrollbalken auf.
 */
function RowSkeleton() {
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

/** So viele Zeilen passen in eine Karte dieser Höhe ganz hinein. */
const MAX_LIST_ROWS = 3;

function RecentActivityCard({ data }: { readonly data: HomeBlockData }) {
  const activities = data.analytics?.recentActivity;
  const { shown } = useHomeCardRows(activities ?? [], MAX_LIST_ROWS);
  return (
    <ListCard title="Letzte Bewegungen" concept="changeHistory">
      {data.analyticsLoading ? (
        <RowSkeleton />
      ) : !activities || activities.length === 0 ? (
        <EmptyState className="py-4" title="Keine aktuellen Bewegungen" />
      ) : (
        <div className="space-y-2">
          {shown.map((activity, idx) => {
            const ts = new Date(activity.timestamp).getTime();
            const tsKey = Number.isFinite(ts) ? ts : `idx-${idx}`;
            return (
              // Diese Zeile fuehrt nirgendwohin: eine Bewegung ist ein
              // Ereignis, keine Seite. Darum auch kein Hover-Effekt, der
              // etwas anderes verspricht.
              <div
                key={`${activity.type}-${activity.groupName}-${activity.roomName}-${tsKey}`}
                className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2"
              >
                <p className="flex min-w-0 flex-1 items-center gap-1.5 text-sm">
                  <span className="truncate font-medium text-gray-900">
                    {activity.groupName}
                  </span>
                  <ChevronRight
                    className="h-3.5 w-3.5 flex-shrink-0 text-gray-400"
                    aria-hidden="true"
                  />
                  <span className="truncate text-gray-500">
                    {activity.roomName}
                  </span>
                  {activity.count > 1 && (
                    <span className="flex-shrink-0 text-gray-500">
                      · {activity.count} Kinder
                    </span>
                  )}
                </p>
                <span className="flex-shrink-0 text-xs text-gray-500">
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
  const { shown, hidden } = useHomeCardRows(activities ?? [], MAX_LIST_ROWS);
  return (
    <ListCard
      title="Laufende Aktivitäten"
      concept="activities"
      href={data.tenantPath("/activities")}
    >
      {data.analyticsLoading ? (
        <RowSkeleton />
      ) : !activities || activities.length === 0 ? (
        <EmptyState className="py-4" title="Keine laufenden Aktivitäten" />
      ) : (
        <div className="space-y-2">
          {shown.map((activity) => (
            <Link
              key={activity.id}
              href={data.tenantPath("/activities")}
              aria-label={`${activity.name}: Aktivitäten öffnen`}
              className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2 transition-colors hover:bg-gray-100/50"
            >
              <p className="min-w-0 flex-1 truncate text-sm">
                <span className="font-medium text-gray-900">
                  {activity.name}
                </span>
                <span className="text-gray-500">
                  {" · "}
                  {activity.category} · {activity.participants}
                  {activity.maxCapacity == null
                    ? " Teilnehmer"
                    : `/${activity.maxCapacity} Teilnehmer`}
                </span>
              </p>
              <div
                className={`h-2.5 w-2.5 rounded-full ${getActivityStatusColor(activity.status)} ml-2 flex-shrink-0`}
              ></div>
            </Link>
          ))}
          <HomeMoreRow
            hidden={hidden}
            href={data.tenantPath("/activities")}
            label="Aktivitäten"
          />
        </div>
      )}
    </ListCard>
  );
}

/**
 * Was in einer laufenden Betreuung steht: eine Betreuungsgruppe, eine
 * Aktivität oder eine spontane Runde. Der Server unterscheidet das seit jeher
 * im Feld `type`; auf der Karte stand bisher nur der Name, und „Kochen" unter
 * der Überschrift „Aktive Gruppen" liest sich wie ein Fehler.
 */
const ACTIVE_GROUP_KIND: Record<string, string> = {
  ogs_group: "Gruppe",
  activity: "Aktivität",
  spontaneous: "Spontan",
};

function ActiveGroupsCard({ data }: { readonly data: HomeBlockData }) {
  const groups = data.analytics?.activeGroupsSummary;
  const { shown, hidden } = useHomeCardRows(groups ?? [], MAX_LIST_ROWS);
  return (
    <ListCard
      title="Laufende Betreuung"
      concept="groups"
      href={data.tenantPath("/ogs-groups")}
    >
      {data.analyticsLoading ? (
        <RowSkeleton />
      ) : !groups || groups.length === 0 ? (
        <EmptyState className="py-4" title="Es läuft gerade nichts" />
      ) : (
        <div className="space-y-2">
          {shown.map((group) => (
            <Link
              key={`${group.type}-${group.name}`}
              href={data.tenantPath("/ogs-groups")}
              aria-label={`${group.name}: Betreuung öffnen`}
              className="flex items-center justify-between gap-3 rounded-xl bg-gray-50/50 px-3 py-2 transition-colors hover:bg-gray-100/50"
            >
              <p className="min-w-0 flex-1 truncate text-sm">
                <span className="font-medium text-gray-900">{group.name}</span>
                <span className="text-gray-500">
                  {" · "}
                  {[
                    ACTIVE_GROUP_KIND[group.type],
                    group.location,
                    `${group.studentCount} Kinder`,
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              </p>
              <div
                className={`h-2.5 w-2.5 rounded-full ${getGroupStatusColor(group.status)} ml-2 flex-shrink-0`}
              ></div>
            </Link>
          ))}
          <HomeMoreRow
            hidden={hidden}
            href={data.tenantPath("/ogs-groups")}
            label="Gruppen"
          />
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
        analytics
          ? `${Math.round(analytics.capacityUtilization * 100)}%`
          : "0%",
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
      return (
        <BetreuungsplanHeuteCard
          title="Mein Tag"
          showEmpty
          href={data.dayPlanHref}
          // Kompakte Zeilen nur hier: die Karte hat auf der Fläche eine feste
          // Höhe. Drei passen ganz hinein; ist es mehr, treten zwei Zeilen
          // plus der Hinweis auf den Rest an ihre Stelle. Gemessen, nicht
          // geschätzt. Auf der Zeiterfassung bleibt die gewohnte Ansicht.
          dense
          maxRows={3}
        />
      );
    case "section.staff_notices":
      return <StaffNoticesBlock />;
    case "section.day_flow":
      return <DayFlowBlock />;
    case "section.open_requests":
      return <OpenRequestsBlock />;
    case "section.reminders":
      return <RemindersBlock />;
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
