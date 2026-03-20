"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { Clock, UsersRound, ClipboardList } from "lucide-react";
import { createLogger } from "~/lib/logger";
import { useSWRAuth } from "~/lib/swr/hooks";
import { fetchDashboardAnalyticsClient } from "~/lib/dashboard-api";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import {
  formatRecentActivityTime,
  getActivityStatusColor,
  getGroupStatusColor,
} from "~/lib/dashboard-helpers";
import { Loading } from "~/components/ui/loading";
import { HeroCard } from "./hero-card";
import { CompactStats } from "./compact-stats";
import { DashboardListCard } from "./dashboard-list-card";

const logger = createLogger({ component: "DashboardContent" });

function getTimeBasedGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Guten Morgen";
  if (hour < 17) return "Guten Tag";
  return "Guten Abend";
}

export function DashboardContent() {
  const router = useRouter();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.replace("/");
    },
  });

  const {
    data: dashboardData,
    isLoading,
    error: swrError,
  } = useSWRAuth<DashboardAnalytics>(
    "dashboard-analytics",
    fetchDashboardAnalyticsClient,
    { refreshInterval: 5 * 60 * 1000 },
  );

  if (swrError) {
    logger.error("dashboard_fetch_failed", {
      error: swrError instanceof Error ? swrError.message : String(swrError),
    });
  }

  const error = swrError ? "Fehler beim Laden der Dashboard-Daten" : null;

  useEffect(() => {
    if (status === "authenticated" && session) {
      if (session.error === "RefreshTokenExpired") {
        logger.info("session refresh token expired, redirecting to login");
        router.push("/");
        return;
      }

      if (!session.user?.token) {
        logger.info("no valid token in session, redirecting to login");
        router.push("/");
      }
    }
  }, [status, session, router]);

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  const firstName = session?.user?.name?.split(" ")[0] ?? "User";
  const greeting = getTimeBasedGreeting();

  const groups = dashboardData?.activeGroupsSummary ?? [];
  const activities = dashboardData?.currentActivities ?? [];
  const movements = dashboardData?.recentActivity ?? [];
  const hasListData =
    groups.length > 0 || activities.length > 0 || movements.length > 0;

  return (
    <div className="mx-auto w-full max-w-7xl space-y-4 md:space-y-6">
      {/* Greeting */}
      <div className="ml-6">
        <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
          {greeting}, {firstName}!
        </h1>
        <p className="text-muted-foreground mt-1 text-sm">
          Hier ist die aktuelle Übersicht
        </p>
      </div>

      {/* Error */}
      {error && (
        <div className="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
          {error}
        </div>
      )}

      {/* Hero: Student count + breakdown */}
      <HeroCard
        studentsPresent={dashboardData?.studentsPresent ?? 0}
        studentsInRooms={dashboardData?.studentsInRooms ?? 0}
        studentsInTransit={dashboardData?.studentsInTransit ?? 0}
        studentsOnPlayground={dashboardData?.studentsOnPlayground ?? 0}
        loading={isLoading}
      />

      {/* Compact stats row */}
      <CompactStats
        activeOGSGroups={dashboardData?.activeOGSGroups ?? 0}
        activeActivities={dashboardData?.activeActivities ?? 0}
        supervisorsToday={dashboardData?.supervisorsToday ?? 0}
        studentsPresent={dashboardData?.studentsPresent ?? 0}
        loading={isLoading}
      />

      {/* List sections — only rendered when there's data */}
      {(hasListData || isLoading) && (
        <>
          <div className="grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2">
            <DashboardListCard
              title="Aktive Gruppen"
              icon={UsersRound}
              href="/ogs-groups"
              loading={isLoading}
              isEmpty={groups.length === 0}
            >
              <div className="space-y-2">
                {groups.slice(0, 5).map((group) => (
                  <div
                    key={`${group.type}-${group.name}`}
                    className="flex items-center justify-between rounded-lg bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-gray-900">
                        {group.name}
                      </p>
                      <p className="text-muted-foreground text-xs">
                        {group.location} &middot; {group.studentCount} Kinder
                      </p>
                    </div>
                    <div
                      className={`ml-2 h-2.5 w-2.5 flex-shrink-0 rounded-full ${getGroupStatusColor(group.status)}`}
                    />
                  </div>
                ))}
              </div>
            </DashboardListCard>

            <DashboardListCard
              title="Laufende Aktivitäten"
              icon={ClipboardList}
              href="/activities"
              loading={isLoading}
              isEmpty={activities.length === 0}
            >
              <div className="space-y-2">
                {activities.slice(0, 5).map((activity, idx) => (
                  <div
                    key={`${activity.name}-${activity.category}-${idx}`}
                    className="flex items-center justify-between rounded-lg bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-gray-900">
                        {activity.name}
                      </p>
                      <p className="text-muted-foreground text-xs">
                        {activity.category} &middot; {activity.participants}/
                        {activity.maxCapacity} Teilnehmer
                      </p>
                    </div>
                    <div
                      className={`ml-2 h-2.5 w-2.5 flex-shrink-0 rounded-full ${getActivityStatusColor(activity.status)}`}
                    />
                  </div>
                ))}
              </div>
            </DashboardListCard>
          </div>

          <DashboardListCard
            title="Letzte Bewegungen"
            icon={Clock}
            loading={isLoading}
            isEmpty={movements.length === 0}
          >
            <div className="space-y-2">
              {movements.slice(0, 5).map((movement, idx) => {
                const ts = new Date(movement.timestamp).getTime();
                const tsKey = Number.isFinite(ts) ? ts : `idx-${idx}`;
                return (
                  <div
                    key={`${movement.type}-${movement.groupName}-${movement.roomName}-${tsKey}`}
                    className="flex items-center justify-between rounded-lg bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
                  >
                    <div className="min-w-0 flex-1">
                      <p className="flex items-center gap-1.5 text-sm font-medium text-gray-900">
                        <span className="truncate">{movement.groupName}</span>
                        <span className="text-muted-foreground flex-shrink-0">
                          &rarr;
                        </span>
                        <span className="truncate">{movement.roomName}</span>
                      </p>
                      {movement.count > 1 && (
                        <p className="text-muted-foreground text-xs">
                          {movement.count} Kinder
                        </p>
                      )}
                    </div>
                    <span className="text-muted-foreground ml-2 flex-shrink-0 text-xs">
                      {formatRecentActivityTime(movement.timestamp)}
                    </span>
                  </div>
                );
              })}
            </div>
          </DashboardListCard>
        </>
      )}
    </div>
  );
}
