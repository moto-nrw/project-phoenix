"use client";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
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
import type { MotoDuotoneTone } from "~/lib/location-helper";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { PhaseExpiryWarnings } from "~/components/enrollment/phase-expiry-warnings";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { hasEffectiveAdminScope } from "~/lib/auth-utils";

const logger = createLogger({ component: "DashboardPage" });

// Stat Card Component - matches database page style
interface StatCardProps {
  readonly title: string;
  readonly value: string | number;
  readonly icon: PhosphorIcon;
  readonly tone: MotoDuotoneTone;
  readonly subtitle?: string;
  readonly loading?: boolean;
  readonly href?: string;
}

const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  tone,
  subtitle,
  loading,
  href,
}) => {
  const cardContent = (
    <div className="moto-content-surface relative overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md transition-all duration-150 group-hover:-translate-y-0.5 group-hover:shadow-sm">
      <div className="relative p-4 md:p-6">
        <div className="mb-3 flex items-start justify-between">
          <div className="p-0.5">
            <MotoDuotoneIcon icon={icon} tone={tone} />
          </div>
          {loading && (
            <div className="h-2 w-2 animate-pulse rounded-full bg-gray-400"></div>
          )}
        </div>
        <div className="space-y-1">
          <p className="text-xs font-medium text-gray-600 md:text-sm">
            {title}
          </p>
          <p className="text-2xl font-bold text-gray-900 md:text-3xl">
            {loading ? "…" : value}
          </p>
          {subtitle && <p className="text-xs text-gray-500">{subtitle}</p>}
        </div>
      </div>
    </div>
  );

  if (href) {
    return (
      <Link
        href={href}
        className="focus-visible:ring-moto-blue group block rounded-2xl focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-white"
      >
        {cardContent}
      </Link>
    );
  }

  return cardContent;
};

// Info Card Component for lists
interface InfoCardProps {
  readonly title: string;
  readonly children: React.ReactNode;
  readonly concept: MotoConceptKey;
  readonly href?: string;
  readonly linkText?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({
  title,
  children,
  concept,
  href,
  linkText,
}) => {
  const cardContent = (
    <div className="moto-content-surface relative h-full overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md transition-all duration-150 group-hover:-translate-y-0.5 group-hover:shadow-sm">
      <div className="relative p-4 md:p-6">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="rounded-xl bg-gray-100 p-2">
              <MotoConceptIcon concept={concept} size={20} />
            </div>
            <h3 className="text-base font-semibold text-gray-900 md:text-lg">
              {title}
            </h3>
          </div>
          {href ? (
            <span className="flex items-center gap-1 text-xs font-medium text-gray-600 transition-colors group-hover:text-gray-900 md:text-sm">
              {linkText ? <span>{linkText}</span> : null}
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M9 5l7 7-7 7"
                />
              </svg>
            </span>
          ) : null}
        </div>
        {children}
      </div>
    </div>
  );

  if (href) {
    return (
      <Link
        href={href}
        className="focus-visible:ring-moto-blue group block h-full rounded-2xl focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-white"
      >
        {cardContent}
      </Link>
    );
  }

  return cardContent;
};

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
    <div className="-mt-1.5 w-full">
      {/* Der Seitentitel steht auf dem Desktop in der Breadcrumb der
          Kopfzeile; PageHeaderWithSearch blendet seine Überschrift ab md aus. */}
      <PageHeaderWithSearch title="Home" />

      {/* Begrüßung: kein zweiter Seitentitel, nur eine Zeile über der
          Übersicht. */}
      <p className="mb-6 text-sm text-gray-600 md:mb-8 md:text-base">
        {greeting}, {firstName}! Hier ist die aktuelle Übersicht.
      </p>

      {error && (
        <div className="mb-6">
          <Alert type="error" message={error} />
        </div>
      )}

      {canReadPhaseExpiryWarnings ? (
        <PhaseExpiryWarnings className="mb-6 md:mb-8" />
      ) : null}

      {/* Main Stats Grid */}
      <div
        data-testid="dashboard-stats-grid"
        className="mb-6 grid grid-cols-2 gap-3 md:mb-8 md:grid-cols-3 md:gap-4 xl:grid-cols-4"
      >
        <StatCard
          title="Kinder anwesend"
          value={dashboardData?.studentsPresent ?? 0}
          icon={MOTO_CONCEPTS.present.icon}
          tone={MOTO_CONCEPTS.present.tone}
          loading={isLoading}
          href="/students/search"
        />
        {showRoomSurfaces ? (
          <>
            <StatCard
              title="In Räumen"
              value={dashboardData?.studentsInRooms ?? 0}
              icon={MOTO_CONCEPTS.rooms.icon}
              tone={MOTO_CONCEPTS.rooms.tone}
              loading={isLoading}
              href="/students/search"
            />
            <StatCard
              title="Unterwegs"
              value={dashboardData?.studentsInTransit ?? 0}
              icon={MOTO_CONCEPTS.transit.icon}
              tone={MOTO_CONCEPTS.transit.tone}
              loading={isLoading}
              href="/students/search?status=unterwegs"
            />
          </>
        ) : null}
        <StatCard
          title="Schulhof"
          value={dashboardData?.studentsOnPlayground ?? 0}
          icon={MOTO_CONCEPTS.schoolyard.icon}
          tone={MOTO_CONCEPTS.schoolyard.tone}
          loading={isLoading}
          href="/students/search?status=schulhof"
        />
        <StatCard
          title="Krank"
          value={dashboardData?.studentsSick ?? 0}
          icon={MOTO_CONCEPTS.sick.icon}
          tone={MOTO_CONCEPTS.sick.tone}
          loading={isLoading}
          href="/students/search?status=krank"
        />
        <StatCard
          title="Entschuldigt"
          value={dashboardData?.studentsExcused ?? 0}
          icon={MOTO_CONCEPTS.excused.icon}
          tone={MOTO_CONCEPTS.excused.tone}
          loading={isLoading}
          href="/students/search?status=entschuldigt"
        />
        <StatCard
          title="Zuhause"
          value={dashboardData?.studentsHome ?? 0}
          icon={MOTO_CONCEPTS.home.icon}
          tone={MOTO_CONCEPTS.home.tone}
          loading={isLoading}
          href="/students/search?status=abwesend"
        />
        {showActivitySurfaces ? (
          <StatCard
            title="Aktive Aktivitäten"
            value={dashboardData?.activeActivities ?? 0}
            icon={MOTO_CONCEPTS.activities.icon}
            tone={MOTO_CONCEPTS.activities.tone}
            loading={isLoading}
            href="/activities"
          />
        ) : null}
        {showRoomSurfaces ? (
          <StatCard
            title="Auslastung"
            value={
              dashboardData
                ? `${Math.round(dashboardData.capacityUtilization * 100)}%`
                : "0%"
            }
            icon={MOTO_CONCEPTS.utilization.icon}
            tone={MOTO_CONCEPTS.utilization.tone}
            loading={isLoading}
          />
        ) : null}
      </div>

      {/* Activity Lists Grid */}
      <div
        data-testid="dashboard-info-grid"
        className={`grid grid-cols-1 items-stretch gap-4 md:gap-6 lg:grid-cols-2 ${infoGridColumns}`}
      >
        {/* Geburtstage (#1542) — a full-width strip rather than a half card:
            the list is short, reads horizontally, and never leaves an odd gap
            when the room/activity cards below are hidden per presence mode. */}
        {birthdays?.enabled ? (
          <div className="lg:col-span-2 xl:col-span-full">
            <InfoCard title="Geburtstage" concept="birthdays">
              <BirthdayList
                celebrations={birthdays.celebrations}
                isLoading={birthdaysLoading}
              />
            </InfoCard>
          </div>
        ) : null}

        {/* Recent Activity */}
        {showRoomSurfaces ? (
          <InfoCard title="Letzte Bewegungen" concept="changeHistory">
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
          </InfoCard>
        ) : null}

        {showActivitySurfaces ? (
          <InfoCard
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
          </InfoCard>
        ) : null}

        {/* Active Groups */}
        {!openCareGroupMode ? (
          <InfoCard title="Aktive Gruppen" concept="groups" href="/ogs-groups">
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
          </InfoCard>
        ) : null}
      </div>
    </div>
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
