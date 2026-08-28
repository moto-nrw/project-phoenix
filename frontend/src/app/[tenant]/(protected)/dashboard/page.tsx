"use client";

import type { ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { createLogger } from "~/lib/logger";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantAwarePath } from "~/lib/tenant-path";
import Link from "next/link";
import { UserContextProvider } from "~/lib/usercontext-context";
import { fetchDashboardAnalyticsClient } from "~/lib/dashboard-api";
import { fetchBirthdayOverviewClient } from "~/lib/birthdays-api";
import type { BirthdayOverview } from "~/lib/birthdays-api";
import { BirthdayList } from "~/components/dashboard/birthday-list";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import {
  formatRecentActivityTime,
  getActivityStatusColor,
  getGroupStatusColor,
} from "~/lib/dashboard-helpers";
import { getTimeBasedGreeting } from "~/lib/greeting";
import { useSWRAuth } from "~/lib/swr/hooks";
import { RoleGuard } from "~/components/auth/role-guard";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";
import { DashboardSkeleton } from "./page-skeleton";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { formatStatusDate } from "~/lib/date-helpers";
import { hasEffectiveAdminScope } from "~/lib/auth-utils";

const logger = createLogger({ component: "DashboardPage" });

/**
 * Listenkarte der Startseite: dieselbe Sektionskarte wie überall, mit dem
 * Konzeptsymbol als Vorspann und dem Weiterlink als Aktion. Vorher war das
 * eine eigene Kartenfläche mit eigenem Radius und eigenem Kopf.
 */
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
      className="h-full"
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

function DashboardContent() {
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const nfcEnabled = useNFCEnabled();
  const openCareGroupMode = useOpenCareGroupMode();
  const presenceMode = usePresenceMode();
  const showActivitySurfaces = nfcEnabled && presenceMode !== "binary";
  const showRoomSurfaces = presenceMode !== "binary";
  const infoCardCount =
    Number(showRoomSurfaces) +
    Number(showActivitySurfaces) +
    Number(!openCareGroupMode);
  const infoGridColumns =
    infoCardCount === 3
      ? "xl:grid-cols-3"
      : infoCardCount === 2
        ? "xl:grid-cols-2"
        : "xl:grid-cols-1";
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.replace("/");
    },
  });

  // SWR with "dashboard-analytics" key — automatically revalidated by global SSE
  // when student_checkin, student_checkout, activity_start/end, or
  // dashboard_counts_changed events arrive (see use-global-sse.ts)
  const {
    data: dashboardData,
    isLoading,
    error: swrError,
  } = useSWRAuth<DashboardAnalytics>(
    "dashboard-analytics",
    fetchDashboardAnalyticsClient,
    { refreshInterval: 5 * 60 * 1000 },
  );

  // Birthdays live on their own key: they change once a day, while the
  // analytics key above is revalidated by every check-in via SSE (#1542).
  // A failure here must never take the dashboard down, so the card simply
  // stays hidden.
  const { data: birthdays, isLoading: birthdaysLoading } =
    useSWRAuth<BirthdayOverview>(
      "birthday-overview",
      fetchBirthdayOverviewClient,
      { refreshInterval: 30 * 60 * 1000 },
    );

  if (swrError) {
    logger.error("dashboard_fetch_failed", {
      error: swrError instanceof Error ? swrError.message : String(swrError),
    });
  }

  const error = swrError ? "Fehler beim Laden der Dashboard-Daten" : null;

  if (
    status === "authenticated" &&
    session &&
    (session.error === "RefreshTokenExpired" || !session.user?.token)
  ) {
    logger.info("invalid session, redirecting to login");
    redirect(tenantPath("/"));
  }

  const firstName = session?.user?.name?.split(" ")[0] ?? "User";
  const greeting = getTimeBasedGreeting();
  const canReadPhaseExpiryWarnings = hasEffectiveAdminScope(session);

  return (
    <TenantPage
      title={`${greeting}, ${firstName}`}
      prominent
      statsLoading={isLoading}
      stats={`${formatStatusDate()} · ${dashboardData?.studentsPresent ?? 0} Kinder anwesend · ${dashboardData?.studentsSick ?? 0} krank`}
      error={error}
    >
      {canReadPhaseExpiryWarnings ? <PhaseExpiryWarnings /> : null}

      {/* Kennzahlen der Einrichtung */}
      <div
        data-testid="dashboard-stats-grid"
        className="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 xl:grid-cols-4"
      >
        <StatCard
          label="Kinder anwesend"
          value={dashboardData?.studentsPresent ?? 0}
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.present.icon}
              tone={MOTO_CONCEPTS.present.tone}
            />
          }
          loading={isLoading}
          href="/students/search"
        />
        {showRoomSurfaces ? (
          <>
            <StatCard
              label="In Räumen"
              value={dashboardData?.studentsInRooms ?? 0}
              icon={
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.rooms.icon}
                  tone={MOTO_CONCEPTS.rooms.tone}
                />
              }
              loading={isLoading}
              href="/students/search"
            />
            <StatCard
              label="Unterwegs"
              value={dashboardData?.studentsInTransit ?? 0}
              icon={
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.transit.icon}
                  tone={MOTO_CONCEPTS.transit.tone}
                />
              }
              loading={isLoading}
              href="/students/search?status=unterwegs"
            />
          </>
        ) : null}
        <StatCard
          label="Schulhof"
          value={dashboardData?.studentsOnPlayground ?? 0}
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.schoolyard.icon}
              tone={MOTO_CONCEPTS.schoolyard.tone}
            />
          }
          loading={isLoading}
          href="/students/search?status=schulhof"
        />
        <StatCard
          label="Krank"
          value={dashboardData?.studentsSick ?? 0}
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.sick.icon}
              tone={MOTO_CONCEPTS.sick.tone}
            />
          }
          loading={isLoading}
          href="/students/search?status=krank"
        />
        <StatCard
          label="Entschuldigt"
          value={dashboardData?.studentsExcused ?? 0}
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.excused.icon}
              tone={MOTO_CONCEPTS.excused.tone}
            />
          }
          loading={isLoading}
          href="/students/search?status=entschuldigt"
        />
        <StatCard
          label="Zuhause"
          value={dashboardData?.studentsHome ?? 0}
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.home.icon}
              tone={MOTO_CONCEPTS.home.tone}
            />
          }
          loading={isLoading}
          href="/students/search?status=abwesend"
        />
        {showActivitySurfaces ? (
          <StatCard
            label="Aktive Aktivitäten"
            value={dashboardData?.activeActivities ?? 0}
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.activities.icon}
                tone={MOTO_CONCEPTS.activities.tone}
              />
            }
            loading={isLoading}
            href="/activities"
          />
        ) : null}
        {showRoomSurfaces ? (
          <StatCard
            label="Auslastung"
            value={
              dashboardData
                ? `${Math.round(dashboardData.capacityUtilization * 100)}%`
                : "0%"
            }
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.utilization.icon}
                tone={MOTO_CONCEPTS.utilization.tone}
              />
            }
            loading={isLoading}
          />
        ) : null}
      </div>

      {/* Listen der Startseite */}
      <div
        data-testid="dashboard-info-grid"
        className={`grid grid-cols-1 items-stretch gap-6 lg:grid-cols-2 ${infoGridColumns}`}
      >
        {/* Geburtstage (#1542) — a full-width strip rather than a half card:
            the list is short, reads horizontally, and never leaves an odd gap
            when the room/activity cards below are hidden per presence mode. */}
        {birthdays?.enabled ? (
          <div className="lg:col-span-2 xl:col-span-full">
            <ListCard title="Geburtstage" concept="birthdays">
              <BirthdayList
                celebrations={birthdays.celebrations}
                isLoading={birthdaysLoading}
              />
            </ListCard>
          </div>
        ) : null}

        {/* Recent Activity */}
        {showRoomSurfaces ? (
          <ListCard title="Letzte Bewegungen" concept="changeHistory">
            {(() => {
              if (isLoading) {
                // Mirrors the loaded activity row below: same rounded-xl
                // p-3 surface, two text lines left, badge right.
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
                        <div className="h-6 w-16 animate-pulse rounded-full bg-gray-200"></div>
                      </div>
                    ))}
                  </div>
                );
              }
              const activities = dashboardData?.recentActivity;
              if (!activities || activities.length === 0) {
                return (
                  <EmptyState
                    className="py-8"
                    title="Keine aktuellen Bewegungen"
                  />
                );
              }
              return (
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
                            <span className="truncate">
                              {activity.groupName}
                            </span>
                            <svg
                              className="h-3.5 w-3.5 flex-shrink-0 text-gray-400"
                              fill="none"
                              viewBox="0 0 24 24"
                              stroke="currentColor"
                              strokeWidth={2.5}
                            >
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                d="M9 5l7 7-7 7"
                              />
                            </svg>
                            <span className="truncate">
                              {activity.roomName}
                            </span>
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
              );
            })()}
          </ListCard>
        ) : null}

        {showActivitySurfaces ? (
          <ListCard
            title="Laufende Aktivitäten"
            concept="activities"
            href="/activities"
          >
            {(() => {
              if (isLoading) {
                // Mirrors the loaded row: rounded-xl p-3, name + meta line
                // left, status dot right.
                return (
                  <div className="space-y-2" aria-hidden="true">
                    {[1, 2, 3].map((i) => (
                      <div
                        key={i}
                        className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3"
                      >
                        <div className="min-w-0 flex-1 space-y-1.5">
                          <div className="h-4 w-1/2 animate-pulse rounded bg-gray-200"></div>
                          <div className="h-3 w-2/3 animate-pulse rounded bg-gray-200"></div>
                        </div>
                        <div className="ml-2 h-2.5 w-2.5 flex-shrink-0 animate-pulse rounded-full bg-gray-200"></div>
                      </div>
                    ))}
                  </div>
                );
              }
              const activities = dashboardData?.currentActivities;
              if (!activities || activities.length === 0) {
                return (
                  <EmptyState
                    className="py-8"
                    title="Keine laufenden Aktivitäten"
                  />
                );
              }
              return (
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
              );
            })()}
          </ListCard>
        ) : null}

        {/* Active Groups */}
        {!openCareGroupMode ? (
          <ListCard title="Aktive Gruppen" concept="groups" href="/ogs-groups">
            {(() => {
              if (isLoading) {
                // Mirrors the loaded row: rounded-xl p-3, name + meta line
                // left, status dot right.
                return (
                  <div className="space-y-2" aria-hidden="true">
                    {[1, 2, 3].map((i) => (
                      <div
                        key={i}
                        className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3"
                      >
                        <div className="min-w-0 flex-1 space-y-1.5">
                          <div className="h-4 w-1/2 animate-pulse rounded bg-gray-200"></div>
                          <div className="h-3 w-2/3 animate-pulse rounded bg-gray-200"></div>
                        </div>
                        <div className="ml-2 h-2.5 w-2.5 flex-shrink-0 animate-pulse rounded-full bg-gray-200"></div>
                      </div>
                    ))}
                  </div>
                );
              }
              const groups = dashboardData?.activeGroupsSummary;
              if (!groups || groups.length === 0) {
                return (
                  <EmptyState className="py-8" title="Keine aktiven Gruppen" />
                );
              }
              return (
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
              );
            })()}
          </ListCard>
        ) : null}
      </div>
    </TenantPage>
  );
}

// Main Dashboard Page Component
export default function DashboardPage() {
  return (
    <RoleGuard variant="adminOnly" fallback={<DashboardSkeleton />}>
      <UserContextProvider>
        <DashboardContent />
      </UserContextProvider>
    </RoleGuard>
  );
}
