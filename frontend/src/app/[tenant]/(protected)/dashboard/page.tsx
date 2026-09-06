"use client";

import { useEffect, useMemo, useState, type ReactNode } from "react";
import { ChevronRight, SlidersHorizontal } from "lucide-react";
import { createLogger } from "~/lib/logger";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantAwarePath } from "~/lib/tenant-path";
import Link from "~/components/ui/navigation-link";
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
  useTenantSlugSafe,
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
import { CustomizeDashboardModal } from "~/components/dashboard/customize-dashboard-modal";
import { Button } from "~/components/ui/button";
import {
  isHomeBlockVisible,
  resolveHomeBlocks,
  type HomeBlockKey,
} from "~/lib/home-blocks";
import { useHomeLayout } from "~/lib/hooks/use-home-layout";

const logger = createLogger({ component: "DashboardPage" });

// Alles, was aus den Betriebszahlen (/analytics) lebt. Ist nichts davon
// sichtbar, wird die Abfrage gar nicht erst gestellt (#2875) — eine
// ausgeblendete Kachel darf keine Last erzeugen.
const ANALYTICS_BLOCKS: readonly HomeBlockKey[] = [
  "tile.students_present",
  "tile.students_in_rooms",
  "tile.students_in_transit",
  "tile.students_on_playground",
  "tile.students_sick",
  "tile.students_excused",
  "tile.students_home",
  "tile.active_activities",
  "tile.capacity_utilization",
  "section.recent_activity",
  "section.current_activities",
  "section.active_groups",
];

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
  const tenantSlug = useTenantSlugSafe();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.replace("/");
    },
  });

  const {
    state: homeLayout,
    save: saveHomeLayout,
    reset: resetHomeLayout,
  } = useHomeLayout();
  const [customizing, setCustomizing] = useState(false);
  const [birthdaysEnabled, setBirthdaysEnabled] = useState(true);

  // Der Betriebsmodus der Schule. Ob es Geburtstage gibt, steht erst in der
  // Antwort, über die wir hier entscheiden — also optimistisch annehmen und
  // die Karte selbst unten auf das echte Kennzeichen warten lassen.
  const blockContext = useMemo(
    () => ({
      detailed: presenceMode !== "binary",
      openCareGroupMode,
      nfcEnabled,
      birthdaysEnabled: true,
    }),
    [presenceMode, openCareGroupMode, nfcEnabled],
  );

  // Nichts holen, was niemand sieht (#2875). Solange die Auswahl aussteht,
  // gilt die Empfehlung; die Seite wartet bewusst nicht auf sie, sonst bliebe
  // sie bei einer hängenden Abfrage leer. Innerhalb einer Sitzung kennt SWR
  // die Auswahl, ausgeblendete Kacheln fragen dann nichts mehr nach.
  const wantsBirthdayData = isHomeBlockVisible(
    blockContext,
    homeLayout.overrides,
    homeLayout.policies,
    "section.birthdays",
  );
  const needsAnalytics = ANALYTICS_BLOCKS.some((key) =>
    isHomeBlockVisible(
      blockContext,
      homeLayout.overrides,
      homeLayout.policies,
      key,
    ),
  );

  // Birthdays live on their own key: they change once a day, while the
  // analytics key below is revalidated by every check-in via SSE (#1542).
  // A failure here must never take the dashboard down, so the card simply
  // stays hidden. Die Abfrage bleibt auch bei einer vorübergehend
  // ausgeschalteten Geburtstagsanzeige aktiv: Nur so erkennt die Startseite,
  // wenn die Schule sie später wieder freigibt.
  const { data: birthdays, isLoading: birthdaysLoading } =
    useSWRAuth<BirthdayOverview>(
      wantsBirthdayData ? "birthday-overview" : null,
      fetchBirthdayOverviewClient,
      { refreshInterval: 30 * 60 * 1000 },
    );

  useEffect(() => {
    setBirthdaysEnabled(true);
  }, [tenantSlug, session?.user?.id]);

  useEffect(() => {
    setBirthdaysEnabled(birthdays?.enabled ?? true);
  }, [birthdays?.enabled]);

  const { adjustable, visible, customized } = useMemo(
    () =>
      resolveHomeBlocks(
        { ...blockContext, birthdaysEnabled },
        homeLayout.overrides,
        homeLayout.policies,
      ),
    [blockContext, birthdaysEnabled, homeLayout.overrides, homeLayout.policies],
  );

  const shown = (key: HomeBlockKey) => visible.has(key);
  // Wer alles abwählt, sieht sonst eine leere Fläche und sucht den Fehler bei
  // sich. Die Startseite sagt stattdessen, wie sie zurückkommt.
  const nothingShown = visible.size === 0;
  // Es gibt immer mindestens eine verfügbare Kachel. Bleibt keine davon
  // persönlich wählbar, hat die Schule sie alle ausgeschaltet.
  const schoolDisabledAllBlocks = nothingShown && adjustable.length === 0;

  const infoCardCount =
    Number(shown("section.recent_activity")) +
    Number(shown("section.current_activities")) +
    Number(shown("section.active_groups"));
  const infoGridColumns =
    infoCardCount === 3
      ? "xl:grid-cols-3"
      : infoCardCount === 2
        ? "xl:grid-cols-2"
        : "xl:grid-cols-1";

  // SWR with "dashboard-analytics" key — automatically revalidated by global SSE
  // when student_checkin, student_checkout, activity_start/end, or
  // dashboard_counts_changed events arrive (see use-global-sse.ts).
  // A null key skips the request entirely when every block that lives on these
  // numbers is hidden (#2875).
  const {
    data: dashboardData,
    isLoading,
    error: swrError,
  } = useSWRAuth<DashboardAnalytics>(
    needsAnalytics ? "dashboard-analytics" : null,
    fetchDashboardAnalyticsClient,
    { refreshInterval: 5 * 60 * 1000 },
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
  const headerStats =
    dashboardData &&
    (shown("tile.students_present") || shown("tile.students_sick"))
      ? [
          formatStatusDate(),
          ...(shown("tile.students_present")
            ? [`${dashboardData.studentsPresent} Kinder anwesend`]
            : []),
          ...(shown("tile.students_sick")
            ? [`${dashboardData.studentsSick} krank`]
            : []),
        ].join(" · ")
      : undefined;

  return (
    <TenantPage
      title={`${greeting}, ${firstName}`}
      prominent
      statsLoading={isLoading}
      stats={headerStats}
      error={
        error
          ? { message: error, keepContent: dashboardData !== undefined }
          : null
      }
      actions={
        <Button
          type="button"
          variant="outline"
          size="md"
          className="gap-2"
          onClick={() => setCustomizing(true)}
        >
          <SlidersHorizontal className="h-4 w-4" aria-hidden="true" />
          Startseite anpassen
        </Button>
      }
    >
      {canReadPhaseExpiryWarnings ? <PhaseExpiryWarnings /> : null}

      {nothingShown ? (
        <EmptyState
          title="Ihre Startseite ist leer"
          description={
            schoolDisabledAllBlocks
              ? homeLayout.canManagePolicies
                ? "Sie haben alle Kacheln für die Schule ausgeschaltet."
                : "Die Schule blendet alle Kacheln aus. Wenden Sie sich an Ihre Leitung."
              : "Sie haben alle Kacheln ausgeblendet."
          }
          action={
            schoolDisabledAllBlocks ? (
              homeLayout.canManagePolicies ? (
                <Button
                  type="button"
                  variant="primary"
                  size="md"
                  onClick={() =>
                    router.push(tenantPath("/settings?tab=startseite"))
                  }
                >
                  Startseite für alle öffnen
                </Button>
              ) : undefined
            ) : (
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={() => setCustomizing(true)}
              >
                Kacheln einblenden
              </Button>
            )
          }
        />
      ) : null}

      {/* Kennzahlen der Einrichtung */}
      <div
        data-testid="dashboard-stats-grid"
        className="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 xl:grid-cols-4"
      >
        {shown("tile.students_present") ? (
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
            href={tenantPath("/students/search")}
          />
        ) : null}
        {shown("tile.students_in_rooms") ? (
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
            href={tenantPath("/students/search")}
          />
        ) : null}
        {shown("tile.students_in_transit") ? (
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
            href={tenantPath("/students/search?status=unterwegs")}
          />
        ) : null}
        {shown("tile.students_on_playground") ? (
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
            href={tenantPath("/students/search?status=schulhof")}
          />
        ) : null}
        {shown("tile.students_sick") ? (
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
            href={tenantPath("/students/search?status=krank")}
          />
        ) : null}
        {shown("tile.students_excused") ? (
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
            href={tenantPath("/students/search?status=entschuldigt")}
          />
        ) : null}
        {shown("tile.students_home") ? (
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
            href={tenantPath("/students/search?status=abwesend")}
          />
        ) : null}
        {shown("tile.active_activities") ? (
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
            href={tenantPath("/activities")}
          />
        ) : null}
        {shown("tile.capacity_utilization") ? (
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
        {shown("section.birthdays") && birthdays?.enabled ? (
          <div className="lg:col-span-2 xl:col-span-full">
            <ListCard title="Geburtstage" concept="birthdays">
              <BirthdayList
                celebrations={birthdays?.celebrations ?? []}
                isLoading={birthdaysLoading}
              />
            </ListCard>
          </div>
        ) : null}

        {/* Recent Activity */}
        {shown("section.recent_activity") ? (
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

        {shown("section.current_activities") ? (
          <ListCard
            title="Laufende Aktivitäten"
            concept="activities"
            href={tenantPath("/activities")}
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
        {shown("section.active_groups") ? (
          <ListCard
            title="Aktive Gruppen"
            concept="groups"
            href={tenantPath("/ogs-groups")}
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

      <CustomizeDashboardModal
        isOpen={customizing}
        onClose={() => setCustomizing(false)}
        adjustable={adjustable}
        visible={visible}
        currentOverrides={homeLayout.overrides}
        customized={customized}
        prescribedCount={Object.keys(homeLayout.policies).length}
        onSave={saveHomeLayout}
        onReset={resetHomeLayout}
      />
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
