"use client";

import { useEffect, useState, useRef } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { UserContextProvider } from "~/lib/usercontext-context";
import { fetchWithAuth } from "~/lib/fetch-with-auth";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import {
  formatRecentActivityTime,
  getActivityStatusColor,
  getGroupStatusColor,
} from "~/lib/dashboard-helpers";
import { isAdmin } from "~/lib/auth-utils";

import { Loading } from "~/components/ui/loading";
// Helper function to get time-based greeting
function getTimeBasedGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 12) return "Guten Morgen";
  if (hour < 17) return "Guten Tag";
  return "Guten Abend";
}

// Icon component
const Icon: React.FC<{ path: string; className?: string }> = ({
  path,
  className,
}) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

// Color theme types and helper
type ColorTheme = {
  overlay: string;
  ring: string;
};

const COLOR_THEMES: Record<string, ColorTheme> = {
  "[#5080D8]": {
    overlay: "from-blue-50/80 to-cyan-100/80",
    ring: "ring-blue-200/60",
  },
  "[#83CD2D]": {
    overlay: "from-green-50/80 to-lime-100/80",
    ring: "ring-green-200/60",
  },
  "[#FF3130]": {
    overlay: "from-red-50/80 to-rose-100/80",
    ring: "ring-red-200/60",
  },
  "orange-500": {
    overlay: "from-orange-50/80 to-orange-100/80",
    ring: "ring-orange-200/60",
  },
  "yellow-400": {
    overlay: "from-yellow-50/80 to-yellow-100/80",
    ring: "ring-yellow-200/60",
  },
  emerald: {
    overlay: "from-emerald-50/80 to-green-100/80",
    ring: "ring-emerald-200/60",
  },
  purple: {
    overlay: "from-purple-50/80 to-violet-100/80",
    ring: "ring-purple-200/60",
  },
  indigo: {
    overlay: "from-indigo-50/80 to-blue-100/80",
    ring: "ring-indigo-200/60",
  },
};

const DEFAULT_THEME: ColorTheme = {
  overlay: "from-gray-50/80 to-slate-100/80",
  ring: "ring-gray-200/60",
};

function getColorTheme(color: string): ColorTheme {
  const matchedKey = Object.keys(COLOR_THEMES).find((key) =>
    color.includes(key),
  );
  return matchedKey ? COLOR_THEMES[matchedKey]! : DEFAULT_THEME;
}

// Stat Card Component - matches database page style
interface StatCardProps {
  readonly title: string;
  readonly value: string | number;
  readonly icon: string;
  readonly color: string;
  readonly subtitle?: string;
  readonly loading?: boolean;
  readonly href?: string;
}

const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  color,
  subtitle,
  loading,
  href,
}) => {
  const theme = getColorTheme(color);

  const cardContent = (
    <div className="relative overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-300 group-hover:-translate-y-0.5 group-hover:shadow-[0_16px_40px_rgb(0,0,0,0.14)]">
      <div
        className={`absolute inset-0 bg-gradient-to-br ${theme.overlay} pointer-events-none rounded-3xl opacity-[0.03]`}
      ></div>
      <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
      <div
        className={`absolute inset-0 rounded-3xl ring-1 ring-white/20 ${theme.ring} pointer-events-none`}
      ></div>

      <div className="relative p-4 md:p-6">
        <div className="mb-3 flex items-start justify-between">
          <div
            className={`rounded-2xl bg-gradient-to-br ${color} p-2.5 text-white shadow-lg md:p-3`}
          >
            <Icon path={icon} className="h-5 w-5 md:h-6 md:w-6" />
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
            {loading ? "..." : value}
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
        className="group block rounded-3xl focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2 focus-visible:ring-offset-white"
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
  readonly icon?: string;
  readonly href?: string;
  readonly linkText?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({
  title,
  children,
  icon,
  href,
  linkText,
}) => {
  const cardContent = (
    <div className="relative h-full overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-300 group-hover:-translate-y-0.5 group-hover:shadow-[0_16px_40px_rgb(0,0,0,0.14)]">
      <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-gray-50/80 to-slate-100/80 opacity-[0.03]"></div>
      <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
      <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20"></div>

      <div className="relative p-4 md:p-6">
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            {icon && (
              <div className="rounded-xl bg-gray-100 p-2">
                <Icon
                  path={icon}
                  className="h-4 w-4 text-gray-600 md:h-5 md:w-5"
                />
              </div>
            )}
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
        className="group block rounded-3xl focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500 focus-visible:ring-offset-2 focus-visible:ring-offset-white"
      >
        {cardContent}
      </Link>
    );
  }

  return cardContent;
};

function DashboardContent() {
  const router = useRouter();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.replace("/");
    },
  });

  const [dashboardData, setDashboardData] = useState<DashboardAnalytics | null>(
    null,
  );
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const hasLoadedOnce = useRef(false);

  useEffect(() => {
    const fetchDashboardData = async () => {
      try {
        setIsLoading(true);
        const response = await fetchWithAuth("/api/dashboard/analytics");

        if (!response.ok) {
          console.error(`Dashboard API returned status ${response.status}`);
          throw new Error("Failed to fetch dashboard data");
        }

        const data = (await response.json()) as { data: DashboardAnalytics };
        setDashboardData(data.data);
        setError(null);
        hasLoadedOnce.current = true;
      } catch (err) {
        console.error("Error fetching dashboard data:", err);
        // For initial load, show full error
        if (!hasLoadedOnce.current) {
          setError("Fehler beim Laden der Dashboard-Daten");
        }
        // For background refresh, keep old data and continue silently
      } finally {
        setIsLoading(false);
      }
    };

    if (status === "authenticated" && session) {
      if (session.error === "RefreshTokenExpired") {
        console.log("Session refresh token expired, redirecting to login");
        router.push("/");
        return;
      }

      if (session.user?.token) {
        void fetchDashboardData();
      } else {
        console.log("No valid token in session, redirecting to login");
        router.push("/");
      }

      // Refresh data every 5 minutes
      const interval = setInterval(
        () => {
          void fetchDashboardData();
        },
        5 * 60 * 1000,
      );

      return () => clearInterval(interval);
    }
  }, [status, session, router]);

  if (status === "loading") {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  const firstName = session?.user?.name?.split(" ")[0] ?? "User";
  const greeting = getTimeBasedGreeting();

  return (
    <ResponsiveLayout>
      <div className="mx-auto w-full max-w-7xl">
        {/* Greeting Section */}
        <div className="mb-6 md:mb-8">
          <div className="ml-6">
            <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
              {greeting}, {firstName}!
            </h1>
            <p className="mt-2 text-sm text-gray-600 md:text-base">
              Hier ist die aktuelle Übersicht
            </p>
          </div>
        </div>

        {/* Error Message */}
        {error && (
          <div className="mb-6 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-800">
            {error}
          </div>
        )}

        {/* Main Stats Grid */}
        <div className="mb-6 grid grid-cols-2 gap-3 md:mb-8 md:grid-cols-4 md:gap-4">
          <StatCard
            title="Kinder anwesend"
            value={dashboardData?.studentsPresent ?? 0}
            icon="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
            color="from-[#5080D8] to-[#4070c8]"
            loading={isLoading}
            href="/students/search"
          />
          <StatCard
            title="In Räumen"
            value={dashboardData?.studentsInRooms ?? 0}
            icon="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            color="from-indigo-500 to-indigo-600"
            loading={isLoading}
            href="/students/search"
          />
          <StatCard
            title="Unterwegs"
            value={dashboardData?.studentsInTransit ?? 0}
            icon="M13 10V3L4 14h7v7l9-11h-7z"
            color="from-orange-500 to-orange-600"
            loading={isLoading}
            href="/students/search?status=unterwegs"
          />
          <StatCard
            title="Schulhof"
            value={dashboardData?.studentsOnPlayground ?? 0}
            icon="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707"
            color="from-yellow-400 to-yellow-500"
            loading={isLoading}
            href="/students/search?status=schulhof"
          />
        </div>

        {/* Secondary Stats */}
        <div className="mb-6 grid grid-cols-2 gap-3 md:mb-8 md:grid-cols-4 md:gap-4">
          <StatCard
            title="Aktive Gruppen"
            value={dashboardData?.activeOGSGroups ?? 0}
            icon="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
            color="from-[#83CD2D] to-[#70b525]"
            loading={isLoading}
            href="/ogs-groups"
          />
          <StatCard
            title="Aktive Aktivitäten"
            value={dashboardData?.activeActivities ?? 0}
            icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            color="from-[#FF3130] to-[#e02020]"
            loading={isLoading}
            href="/activities"
          />
          <StatCard
            title="Freie Räume"
            value={dashboardData?.freeRooms ?? 0}
            icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
            color="from-emerald-500 to-green-600"
            loading={isLoading}
            href="/rooms"
          />
          <StatCard
            title="Auslastung"
            value={
              dashboardData
                ? `${Math.round(dashboardData.capacityUtilization * 100)}%`
                : "0%"
            }
            icon="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
            color="from-purple-500 to-purple-600"
            loading={isLoading}
          />
        </div>

        {/* Activity Lists Grid */}
        <div className="grid grid-cols-1 items-stretch gap-4 md:gap-6 lg:grid-cols-2">
          {/* Recent Activity */}
          <InfoCard
            title="Letzte Bewegungen"
            icon="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
          >
            {(() => {
              if (isLoading) {
                return (
                  <div className="space-y-3">
                    {[1, 2, 3].map((i) => (
                      <div
                        key={i}
                        className="h-12 animate-pulse rounded-lg bg-gray-100"
                      ></div>
                    ))}
                  </div>
                );
              }
              const activities = dashboardData?.recentActivity;
              if (!activities || activities.length === 0) {
                return (
                  <p className="py-8 text-center text-sm text-gray-500">
                    Keine aktuellen Bewegungen
                  </p>
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

          {/* Current Activities */}
          <InfoCard
            title="Laufende Aktivitäten"
            icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            href="/activities"
          >
            {(() => {
              if (isLoading) {
                return (
                  <div className="space-y-3">
                    {[1, 2, 3].map((i) => (
                      <div
                        key={i}
                        className="h-14 animate-pulse rounded-lg bg-gray-100"
                      ></div>
                    ))}
                  </div>
                );
              }
              const activities = dashboardData?.currentActivities;
              if (!activities || activities.length === 0) {
                return (
                  <p className="py-8 text-center text-sm text-gray-500">
                    Keine laufenden Aktivitäten
                  </p>
                );
              }
              return (
                <div className="space-y-2">
                  {activities.slice(0, 5).map((activity, idx) => (
                    <div
                      key={`${activity.name}-${activity.category}-${idx}`}
                      className="flex items-center justify-between rounded-xl bg-gray-50/50 p-3 transition-colors hover:bg-gray-100/50"
                    >
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-medium text-gray-900">
                          {activity.name}
                        </p>
                        <p className="text-xs text-gray-500">
                          {activity.category} • {activity.participants}/
                          {activity.maxCapacity} Teilnehmer
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

          {/* Active Groups */}
          <InfoCard
            title="Aktive Gruppen"
            icon="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
            href="/ogs-groups"
          >
            {(() => {
              if (isLoading) {
                return (
                  <div className="space-y-3">
                    {[1, 2, 3].map((i) => (
                      <div
                        key={i}
                        className="h-14 animate-pulse rounded-lg bg-gray-100"
                      ></div>
                    ))}
                  </div>
                );
              }
              const groups = dashboardData?.activeGroupsSummary;
              if (!groups || groups.length === 0) {
                return (
                  <p className="py-8 text-center text-sm text-gray-500">
                    Keine aktiven Gruppen
                  </p>
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

          {/* Betreuer Summary */}
          <InfoCard
            title="Personal heute"
            icon="M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z"
            href="/staff"
          >
            {isLoading ? (
              <div className="h-32 animate-pulse rounded-lg bg-gray-100"></div>
            ) : (
              <div className="grid grid-cols-2 gap-4">
                <div className="rounded-xl border border-gray-200/50 bg-gray-50/50 p-4 transition-colors hover:bg-gray-100/50">
                  <p className="mb-1 text-xs font-medium text-gray-600">
                    Betreuer im Dienst
                  </p>
                  <p className="text-2xl font-bold text-gray-900">
                    {dashboardData?.supervisorsToday ?? 0}
                  </p>
                </div>
                {dashboardData &&
                dashboardData.studentsPresent > 0 &&
                dashboardData.supervisorsToday > 0 ? (
                  <div className="rounded-xl border border-gray-200/50 bg-gray-50/50 p-4 transition-colors hover:bg-gray-100/50">
                    <p className="mb-1 text-xs font-medium text-gray-600">
                      Kinder je Betreuer
                    </p>
                    <p className="text-2xl font-bold text-gray-900">
                      {dashboardData.supervisorsToday > 0
                        ? Math.round(
                            dashboardData.studentsPresent /
                              dashboardData.supervisorsToday,
                          )
                        : "-"}
                    </p>
                    <p className="mt-1 text-xs text-gray-500">
                      Betreuungsschlüssel
                    </p>
                  </div>
                ) : (
                  <div className="rounded-xl border border-gray-200/50 bg-gray-50/50 p-4">
                    <p className="mb-1 text-xs font-medium text-gray-600">
                      Kinder je Betreuer
                    </p>
                    <p className="text-2xl font-bold text-gray-400">-</p>
                    <p className="mt-1 text-xs text-gray-500">Keine Daten</p>
                  </div>
                )}
              </div>
            )}
          </InfoCard>
        </div>
      </div>
    </ResponsiveLayout>
  );
}

// Main Dashboard Page Component
export default function DashboardPage() {
  const router = useRouter();
  const { data: session, status } = useSession();

  // Redirect non-admins to OGS groups (must be in useEffect to avoid SSR issues)
  useEffect(() => {
    if (status !== "loading" && !isAdmin(session)) {
      router.replace("/ogs-groups");
    }
  }, [status, session, router]);

  // Gate access: only admins can view dashboard
  if (status === "loading") {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  // Show nothing while redirecting non-admins
  if (!isAdmin(session)) {
    return null;
  }

  return (
    <UserContextProvider>
      <DashboardContent />
    </UserContextProvider>
  );
}
