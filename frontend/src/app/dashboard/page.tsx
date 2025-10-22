"use client";

import { useEffect, useState, useRef } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { UserContextProvider } from "~/lib/usercontext-context";
import { fetchWithAuth } from "~/lib/fetch-with-auth";
import type { DashboardAnalytics } from "~/lib/dashboard-helpers";
import { formatRecentActivityTime, getActivityStatusColor, getGroupStatusColor } from "~/lib/dashboard-helpers";
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
const Icon: React.FC<{ path: string; className?: string }> = ({ path, className }) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

// Color theme types and helper
type ColorTheme = {
  overlay: string;
  ring: string;
};

const COLOR_THEMES: Record<string, ColorTheme> = {
  '[#5080D8]': {
    overlay: 'from-blue-50/80 to-cyan-100/80',
    ring: 'ring-blue-200/60'
  },
  '[#83CD2D]': {
    overlay: 'from-green-50/80 to-lime-100/80',
    ring: 'ring-green-200/60'
  },
  '[#FF3130]': {
    overlay: 'from-red-50/80 to-rose-100/80',
    ring: 'ring-red-200/60'
  },
  'orange-500': {
    overlay: 'from-orange-50/80 to-orange-100/80',
    ring: 'ring-orange-200/60'
  },
  'yellow-400': {
    overlay: 'from-yellow-50/80 to-yellow-100/80',
    ring: 'ring-yellow-200/60'
  },
  'emerald': {
    overlay: 'from-emerald-50/80 to-green-100/80',
    ring: 'ring-emerald-200/60'
  },
  'purple': {
    overlay: 'from-purple-50/80 to-violet-100/80',
    ring: 'ring-purple-200/60'
  },
  'indigo': {
    overlay: 'from-indigo-50/80 to-blue-100/80',
    ring: 'ring-indigo-200/60'
  }
};

const DEFAULT_THEME: ColorTheme = {
  overlay: 'from-gray-50/80 to-slate-100/80',
  ring: 'ring-gray-200/60'
};

function getColorTheme(color: string): ColorTheme {
  const matchedKey = Object.keys(COLOR_THEMES).find(key => color.includes(key));
  return matchedKey ? COLOR_THEMES[matchedKey]! : DEFAULT_THEME;
}

// Stat Card Component - matches database page style
interface StatCardProps {
  title: string;
  value: string | number;
  icon: string;
  color: string;
  subtitle?: string;
  loading?: boolean;
}

const StatCard: React.FC<StatCardProps> = ({ title, value, icon, color, subtitle, loading }) => {
  const theme = getColorTheme(color);

  return (
    <div className="relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300">
      <div className={`absolute inset-0 bg-gradient-to-br ${theme.overlay} opacity-[0.03] rounded-3xl pointer-events-none`}></div>
      <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20 pointer-events-none"></div>
      <div className={`absolute inset-0 rounded-3xl ring-1 ring-white/20 ${theme.ring} pointer-events-none`}></div>

      <div className="relative p-4 md:p-6">
        <div className="flex items-start justify-between mb-3">
          <div className={`rounded-2xl bg-gradient-to-br ${color} p-2.5 md:p-3 text-white shadow-lg`}>
            <Icon path={icon} className="h-5 w-5 md:h-6 md:w-6" />
          </div>
          {loading && (
            <div className="h-2 w-2 rounded-full bg-gray-400 animate-pulse"></div>
          )}
        </div>
        <div className="space-y-1">
          <p className="text-xs md:text-sm text-gray-600 font-medium">{title}</p>
          <p className="text-2xl md:text-3xl font-bold text-gray-900">
            {loading ? "..." : value}
          </p>
          {subtitle && <p className="text-xs text-gray-500">{subtitle}</p>}
        </div>
      </div>
    </div>
  );
};

// Info Card Component for lists
interface InfoCardProps {
  title: string;
  children: React.ReactNode;
  icon?: string;
  href?: string;
  linkText?: string;
}

const InfoCard: React.FC<InfoCardProps> = ({ title, children, icon, href, linkText = "Alle" }) => (
  <div className="relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
    <div className="absolute inset-0 bg-gradient-to-br from-gray-50/80 to-slate-100/80 opacity-[0.03] rounded-3xl pointer-events-none"></div>
    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20 pointer-events-none"></div>
    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 pointer-events-none"></div>

    <div className="relative p-4 md:p-6">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          {icon && (
            <div className="rounded-xl bg-gray-100 p-2">
              <Icon path={icon} className="h-4 w-4 md:h-5 md:w-5 text-gray-600" />
            </div>
          )}
          <h3 className="text-base md:text-lg font-semibold text-gray-900">{title}</h3>
        </div>
        {href && (
          <Link href={href} className="flex items-center gap-1 text-xs md:text-sm text-gray-600 hover:text-gray-900 font-medium transition-colors">
            <span>{linkText}</span>
            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
            </svg>
          </Link>
        )}
      </div>
      {children}
    </div>
  </div>
);

function DashboardContent() {
  const router = useRouter();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const [dashboardData, setDashboardData] = useState<DashboardAnalytics | null>(null);
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

        const data = await response.json() as { data: DashboardAnalytics };
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
      const sessionWithError = session as typeof session & { error?: string };
      if (sessionWithError.error === "RefreshTokenExpired") {
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
      const interval = setInterval(() => {
        void fetchDashboardData();
      }, 5 * 60 * 1000);

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
      <div className="w-full max-w-7xl mx-auto">
        {/* Greeting Section - Mobile optimized with underline */}
        <div className="mb-6 md:mb-8">
          <div className="ml-6">
            <div className="relative inline-block pb-3">
              <h1 className="text-2xl md:text-3xl font-bold text-gray-900">
                {greeting}, {firstName}!
              </h1>
              {/* Underline indicator - matches tab style */}
              <div
                className="absolute bottom-0 left-0 h-0.5 bg-gray-900 rounded-full"
                style={{ width: '70%' }}
              />
            </div>
            <p className="text-gray-600 text-sm md:text-base mt-3">
              Hier ist die aktuelle Übersicht
            </p>
          </div>
        </div>

        {/* Error Message */}
        {error && (
          <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded-2xl text-red-800 text-sm">
            {error}
          </div>
        )}

        {/* Main Stats Grid */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-6 md:mb-8">
          <StatCard
            title="Kinder anwesend"
            value={dashboardData?.studentsPresent ?? 0}
            icon="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
            color="from-[#5080D8] to-[#4070c8]"
            loading={isLoading}
          />
          <StatCard
            title="In Räumen"
            value={dashboardData?.studentsInRooms ?? 0}
            icon="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            color="from-indigo-500 to-indigo-600"
            loading={isLoading}
          />
          <StatCard
            title="Aktive Gruppen"
            value={dashboardData?.activeOGSGroups ?? 0}
            icon="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
            color="from-[#83CD2D] to-[#70b525]"
            loading={isLoading}
          />
          <StatCard
            title="Aktivitäten"
            value={dashboardData?.activeActivities ?? 0}
            icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            color="from-[#FF3130] to-[#e02020]"
            loading={isLoading}
          />
        </div>

        {/* Secondary Stats */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 md:gap-4 mb-6 md:mb-8">
          <StatCard
            title="Unterwegs"
            value={dashboardData?.studentsInTransit ?? 0}
            icon="M13 10V3L4 14h7v7l9-11h-7z"
            color="from-orange-500 to-orange-600"
            subtitle="zwischen Räumen"
            loading={isLoading}
          />
          <StatCard
            title="Schulhof"
            value={dashboardData?.studentsOnPlayground ?? 0}
            icon="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707"
            color="from-yellow-400 to-yellow-500"
            subtitle="im Freien"
            loading={isLoading}
          />
          <StatCard
            title="Freie Räume"
            value={dashboardData?.freeRooms ?? 0}
            icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
            color="from-emerald-500 to-green-600"
            subtitle="verfügbar"
            loading={isLoading}
          />
          <StatCard
            title="Auslastung"
            value={dashboardData ? `${Math.round(dashboardData.capacityUtilization * 100)}%` : "0%"}
            icon="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
            color="from-purple-500 to-purple-600"
            subtitle="Kapazität"
            loading={isLoading}
          />
        </div>

        {/* Activity Lists Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6">
          {/* Recent Activity */}
          <InfoCard
            title="Letzte Bewegungen"
            icon="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
          >
            {isLoading ? (
              <div className="space-y-3">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="h-12 bg-gray-100 rounded-lg animate-pulse"></div>
                ))}
              </div>
            ) : dashboardData?.recentActivity && dashboardData.recentActivity.length > 0 ? (
              <div className="space-y-2">
                {dashboardData.recentActivity.slice(0, 5).map((activity, index) => (
                  <div key={index} className="flex items-center justify-between p-3 rounded-xl bg-gray-50/50 hover:bg-gray-100/50 transition-colors">
                    <div className="flex-1 min-w-0">
                      <p className="flex items-center gap-1.5 text-sm font-medium text-gray-900">
                        <span className="truncate">{activity.groupName}</span>
                        <svg className="h-3.5 w-3.5 flex-shrink-0 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
                        </svg>
                        <span className="truncate">{activity.roomName}</span>
                      </p>
                      {activity.count > 1 && (
                        <p className="text-xs text-gray-500">{activity.count} Kinder</p>
                      )}
                    </div>
                    <span className="text-xs text-gray-500 ml-2 flex-shrink-0">
                      {formatRecentActivityTime(activity.timestamp)}
                    </span>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-gray-500 text-center py-8">Keine aktuellen Bewegungen</p>
            )}
          </InfoCard>

          {/* Current Activities */}
          <InfoCard
            title="Laufende Aktivitäten"
            icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
            href="/activities"
            linkText="Meine"
          >
            {isLoading ? (
              <div className="space-y-3">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="h-14 bg-gray-100 rounded-lg animate-pulse"></div>
                ))}
              </div>
            ) : dashboardData?.currentActivities && dashboardData.currentActivities.length > 0 ? (
              <div className="space-y-2">
                {dashboardData.currentActivities.slice(0, 5).map((activity, index) => (
                  <div key={index} className="flex items-center justify-between p-3 rounded-xl bg-gray-50/50 hover:bg-gray-100/50 transition-colors">
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 truncate">{activity.name}</p>
                      <p className="text-xs text-gray-500">
                        {activity.category} • {activity.participants}/{activity.maxCapacity} Teilnehmer
                      </p>
                    </div>
                    <div className={`h-2.5 w-2.5 rounded-full ${getActivityStatusColor(activity.status)} flex-shrink-0 ml-2`}></div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-gray-500 text-center py-8">Keine laufenden Aktivitäten</p>
            )}
          </InfoCard>

          {/* Active Groups */}
          <InfoCard
            title="Aktive Gruppen"
            icon="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
            href="/ogs_groups"
            linkText="Meine"
          >
            {isLoading ? (
              <div className="space-y-3">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="h-14 bg-gray-100 rounded-lg animate-pulse"></div>
                ))}
              </div>
            ) : dashboardData?.activeGroupsSummary && dashboardData.activeGroupsSummary.length > 0 ? (
              <div className="space-y-2">
                {dashboardData.activeGroupsSummary.slice(0, 5).map((group, index) => (
                  <div key={index} className="flex items-center justify-between p-3 rounded-xl bg-gray-50/50 hover:bg-gray-100/50 transition-colors">
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium text-gray-900 truncate">{group.name}</p>
                      <p className="text-xs text-gray-500">
                        {group.location} • {group.studentCount} Kinder
                      </p>
                    </div>
                    <div className={`h-2.5 w-2.5 rounded-full ${getGroupStatusColor(group.status)} flex-shrink-0 ml-2`}></div>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-gray-500 text-center py-8">Keine aktiven Gruppen</p>
            )}
          </InfoCard>

          {/* Betreuer Summary */}
          <InfoCard
            title="Personal heute"
            icon="M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z"
            href="/staff"
          >
            {isLoading ? (
              <div className="h-32 bg-gray-100 rounded-lg animate-pulse"></div>
            ) : (
              <div className="grid grid-cols-2 gap-4">
                <div className="p-4 rounded-xl bg-gray-50/50 border border-gray-200/50 hover:bg-gray-100/50 transition-colors">
                  <p className="text-xs text-gray-600 font-medium mb-1">Betreuer im Dienst</p>
                  <p className="text-2xl font-bold text-gray-900">{dashboardData?.supervisorsToday ?? 0}</p>
                </div>
                {dashboardData && dashboardData.studentsPresent > 0 && dashboardData.supervisorsToday > 0 ? (
                  <div className="p-4 rounded-xl bg-gray-50/50 border border-gray-200/50 hover:bg-gray-100/50 transition-colors">
                    <p className="text-xs text-gray-600 font-medium mb-1">Kinder je Betreuer</p>
                    <p className="text-2xl font-bold text-gray-900">
                      {dashboardData.supervisorsToday > 0
                        ? Math.round(dashboardData.studentsPresent / dashboardData.supervisorsToday)
                        : '-'}
                    </p>
                    <p className="text-xs text-gray-500 mt-1">Betreuungsschlüssel</p>
                  </div>
                ) : (
                  <div className="p-4 rounded-xl bg-gray-50/50 border border-gray-200/50">
                    <p className="text-xs text-gray-600 font-medium mb-1">Kinder je Betreuer</p>
                    <p className="text-2xl font-bold text-gray-400">-</p>
                    <p className="text-xs text-gray-500 mt-1">Keine Daten</p>
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

  // Gate access: only admins can view dashboard
  if (status === "loading") {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  if (!isAdmin(session)) {
    // Redirect non-admins to OGS groups (mobile default)
    if (typeof window !== "undefined") {
      router.replace("/ogs_groups");
    }
    return null;
  }

  return (
    <UserContextProvider>
      <DashboardContent />
    </UserContextProvider>
  );
}
