"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useSession } from "next-auth/react";
import { Button } from "~/components/ui/button";
import { TenantPage } from "~/components/ui/tenant-page";
import { TileCard } from "~/components/ui/tile-card";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { fetchActivities, getCategories } from "~/lib/activity-api";
import {
  formatSupervisorList,
  isActivityCreator,
  type Activity,
  type ActivityCategory,
} from "~/lib/activity-helpers";
import { ActivityManagementModal } from "~/components/activities/activity-management-modal";
import { QuickCreateActivityModal } from "~/components/activities/quick-create-modal";
import { userContextService } from "~/lib/usercontext-api";
import type { Staff } from "~/lib/usercontext-helpers";
import { useToast } from "~/contexts/ToastContext";
import { useSWRAuth } from "~/lib/swr";
import { createLogger } from "~/lib/logger";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { useTenantRouter } from "~/lib/tenant-router";
import { hasRole } from "~/lib/auth-utils";
import { useCollectionTabs } from "~/components/dashboard/use-collection-tabs";
import {
  getTabsForCollection,
  STAFF_FLAT_PAGES,
} from "~/lib/section-navigation";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { NfcModeGuard } from "~/components/tenant/nfc-mode-guard";
import {
  MOBILE_CREATE_FAB_MEDIA_QUERY,
  useFloatingFabOffset,
} from "~/lib/hooks/use-floating-fab-offset";
import { redirect } from "next/navigation";
import { Plus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

const logger = createLogger({ component: "ActivitiesPage" });

// SWR cache key for activities page data
const ACTIVITIES_PAGE_KEY = "activities-page";

// Define interface for the combined page data
interface ActivitiesPageData {
  activities: Activity[];
  categories: ActivityCategory[];
  currentStaff: Staff | null;
}

// Binary-mode tenants can't meaningfully start activities (no active groups,
// no visits). Wrap the real content so the guard runs before any hooks in the
// page's main function — React rules forbid conditional hooks.
export default function ActivitiesPage() {
  return (
    <NfcModeGuard>
      <BinaryModeGuard>
        <ActivitiesPageContent />
      </BinaryModeGuard>
    </NfcModeGuard>
  );
}

function ActivitiesPageContent() {
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("all");
  const [myActivitiesFilter, setMyActivitiesFilter] = useState(false);
  const [filteredActivities, setFilteredActivities] = useState<Activity[]>([]);
  const [selectedActivity, setSelectedActivity] = useState<Activity | null>(
    null,
  );
  const [isManagementModalOpen, setIsManagementModalOpen] = useState(false);
  const [isQuickCreateOpen, setIsQuickCreateOpen] = useState(false);
  const { success: toastSuccess } = useToast();
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();

  // useSWRAuth silently disables the fetch when there is no authenticated
  // session (data stays undefined, isLoading stays false). Require a session
  // here so a direct visit after logout/expiry redirects instead of showing
  // the skeleton forever.
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  // Reiter der Sammlung: die Stammdaten der Aktivität sind ein Reiter hier,
  // kein zweiter Baum („Datenverwaltung").
  const activityTabs = useMemo(
    () =>
      hasRole(session, "admin")
        ? getTabsForCollection(STAFF_FLAT_PAGES.activities.href)
        : [],
    [session],
  );
  const pageTabs = useCollectionTabs(
    STAFF_FLAT_PAGES.activities.href,
    STAFF_FLAT_PAGES.activities.label,
    activityTabs,
    "Bereiche der Aktivitäten",
  );

  // The session callback can keep status "authenticated" while clearing the
  // token and setting session.error (expired refresh token). useSWRAuth never
  // starts without a token, so redirect instead of showing the skeleton
  // forever — same pattern as the dashboard.
  if (
    status === "authenticated" &&
    session &&
    (session.error === "RefreshTokenExpired" || !session.user?.token)
  ) {
    logger.info("invalid session, redirecting to login");
    redirect(tenantPath("/"));
  }

  // Fetch activities, categories, and current staff with SWR
  const {
    data: pageData,
    isLoading,
    error: fetchError,
    mutate: mutatePageData,
  } = useSWRAuth<ActivitiesPageData>(
    ACTIVITIES_PAGE_KEY,
    async () => {
      const [activitiesData, categoriesData, staffData] = await Promise.all([
        fetchActivities(),
        getCategories(),
        userContextService.getCurrentStaff().catch((err) => {
          logger.debug("get_current_staff_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return null;
        }),
      ]);
      return {
        activities: activitiesData,
        categories: categoriesData,
        currentStaff: staffData,
      };
    },
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    },
  );
  const showSkeleton =
    status === "loading" ||
    isLoading ||
    (pageData === undefined && !fetchError);
  useFloatingFabOffset({
    active: !showSkeleton,
    mediaQuery: MOBILE_CREATE_FAB_MEDIA_QUERY,
  });

  // Extract data with defaults (memoized to prevent dependency issues)
  const activities = useMemo(
    () => pageData?.activities ?? [],
    [pageData?.activities],
  );
  const categories = useMemo(
    () => pageData?.categories ?? [],
    [pageData?.categories],
  );
  const currentStaff = useMemo(
    () => pageData?.currentStaff ?? null,
    [pageData?.currentStaff],
  );
  const error = fetchError ? "Fehler beim Laden der Aktivitäten" : null;

  // Apply filters
  useEffect(() => {
    // Ensure activities is an array before filtering
    const activityList = Array.isArray(activities) ? activities : [];
    let filtered = [...activityList];

    // Apply search filter
    if (searchTerm) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (activity) =>
          activity.name.toLowerCase().includes(searchLower) ||
          formatSupervisorList(activity.supervisors)
            .toLowerCase()
            .includes(searchLower) ||
          (activity.category_name?.toLowerCase().includes(searchLower) ??
            false),
      );
    }

    // Apply category filter
    if (categoryFilter !== "all") {
      filtered = filtered.filter(
        (activity) => activity.ag_category_id === categoryFilter,
      );
    }

    // Apply "My Activities" filter
    if (myActivitiesFilter && currentStaff?.id) {
      filtered = filtered.filter((activity) =>
        isActivityCreator(activity, currentStaff.id),
      );
    }

    // Sort activities alphabetically by name
    filtered.sort((a, b) => a.name.localeCompare(b.name, "de"));

    setFilteredActivities(filtered);
  }, [
    searchTerm,
    categoryFilter,
    myActivitiesFilter,
    activities,
    currentStaff,
  ]);

  // Handle activity selection - open management modal
  const handleSelectActivity = (activity: Activity) => {
    setSelectedActivity(activity);
    setIsManagementModalOpen(true);
  };

  // Handle successful management actions (edit/delete)
  const handleManagementSuccess = useCallback(
    async (message?: string) => {
      // Show success toast if provided
      if (message) {
        toastSuccess(message);
      }

      // Trigger SWR refetch to update the cache
      await mutatePageData();
    },
    [mutatePageData, toastSuccess],
  );

  // Prepare filters for PageHeaderWithSearch
  const filters: FilterConfig[] = useMemo(() => {
    const baseFilters: FilterConfig[] = [
      {
        id: "category",
        label: "Kategorie",
        type: "dropdown",
        value: categoryFilter,
        onChange: (value: string | string[]) =>
          setCategoryFilter(value as string),
        options: [
          { value: "all", label: "Alle Kategorien" },
          ...categories.map((cat) => ({
            value: cat.id.toString(),
            label: cat.name,
          })),
        ],
      },
    ];

    // Only show "My Activities" filter if user is a staff member
    if (currentStaff) {
      baseFilters.push({
        id: "myActivities",
        label: "Meine Aktivitäten",
        type: "buttons",
        value: myActivitiesFilter ? "my" : "all",
        onChange: (value: string | string[]) =>
          setMyActivitiesFilter(value === "my"),
        options: [
          { value: "all", label: "Alle" },
          { value: "my", label: "Meine" },
        ],
      });
    }

    return baseFilters;
  }, [categoryFilter, categories, myActivitiesFilter, currentStaff]);

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (categoryFilter !== "all") {
      const category = categories.find(
        (cat) => cat.id.toString() === categoryFilter,
      );
      filters.push({
        id: "category",
        label: category?.name ?? "Kategorie",
        onRemove: () => setCategoryFilter("all"),
      });
    }

    if (myActivitiesFilter) {
      filters.push({
        id: "myActivities",
        label: "Meine Aktivitäten",
        onRemove: () => setMyActivitiesFilter(false),
      });
    }

    return filters;
  }, [searchTerm, categoryFilter, myActivitiesFilter, categories]);

  const hasFilters =
    searchTerm !== "" || categoryFilter !== "all" || myActivitiesFilter;
  const stats = hasFilters
    ? `${filteredActivities.length} von ${activities.length} Aktivitäten · ${categories.length} ${categories.length === 1 ? "Kategorie" : "Kategorien"}`
    : `${activities.length} ${activities.length === 1 ? "Aktivität" : "Aktivitäten"} · ${categories.length} ${categories.length === 1 ? "Kategorie" : "Kategorien"}`;

  return (
    <>
      <TenantPage
        title="Aktivitäten"
        stats={stats}
        statsLoading={showSkeleton}
        tabs={pageTabs}
        actions={
          <Button
            type="button"
            variant="primary"
            size="md"
            className="gap-2"
            onClick={() => setIsQuickCreateOpen(true)}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Aktivität erstellen
          </Button>
        }
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Aktivität suchen…",
        }}
        filters={filters}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setSearchTerm("");
          setCategoryFilter("all");
          setMyActivitiesFilter(false);
        }}
        loading={showSkeleton}
        error={error}
        empty={
          !showSkeleton && !error && filteredActivities.length === 0
            ? {
                icon: <MotoConceptIcon concept="activities" size={48} />,
                title: hasFilters
                  ? "Keine Aktivitäten gefunden"
                  : "Keine Aktivitäten vorhanden",
                description: hasFilters
                  ? "Versuchen Sie andere Suchkriterien oder Filter."
                  : "Es wurden noch keine Aktivitäten erstellt.",
              }
            : null
        }
        overlays={
          <>
            {/* Activity Management Modal */}
            {selectedActivity && (
              <ActivityManagementModal
                isOpen={isManagementModalOpen}
                onClose={() => {
                  setIsManagementModalOpen(false);
                  setSelectedActivity(null);
                }}
                onSuccess={handleManagementSuccess}
                activity={selectedActivity}
                currentStaffId={currentStaff?.id}
                readOnly={
                  !isActivityCreator(selectedActivity, currentStaff?.id)
                }
              />
            )}

            {/* Quick Create Activity Modal */}
            <QuickCreateActivityModal
              isOpen={isQuickCreateOpen}
              onClose={() => setIsQuickCreateOpen(false)}
              onSuccess={() => {
                // Don't close here - let the modal handle its own closing
                handleManagementSuccess().catch(() => {
                  // Error already handled in handleManagementSuccess
                });
              }}
            />
          </>
        }
      >
        {/* Dasselbe Kachelraster wie Räume, Personal und Kinder: gleiche
            Objektart, gleiche Darstellung an allen Breakpoints. */}
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {filteredActivities.map((activity) => {
            const handleClick = () => handleSelectActivity(activity);
            return (
              // Ohne Einblend-Animation: die Kachel ist dieselbe wie in
              // jeder anderen Liste, und eine Liste, die sich nacheinander
              // aufbaut, hält beim Suchen nur auf.
              <TileCard
                key={activity.id}
                onClick={handleClick}
                padding="none"
                ariaLabel={`${activity.name} öffnen`}
              >
                {/* Kein Kebab: die einzige Aktion der Kachel ist das
                    Öffnen, und das tut der Klick auf die Kachel schon.
                    Löschen sitzt im Bearbeiten-Dialog. Ein Menü mit einem
                    Eintrag, der dasselbe tut wie der Klick daneben, ist ein
                    zweiter Weg zum selben Ziel — nicht eine Aktion mehr. */}
                <div className="p-4 sm:p-5">
                  <h3 className="truncate text-base font-bold text-gray-900">
                    {activity.name}
                  </h3>
                  <p className="mt-1 truncate text-sm text-gray-500">
                    <span className="text-gray-400">Erstellt von:</span>{" "}
                    {formatSupervisorList(activity.supervisors)}
                  </p>
                </div>
              </TileCard>
            );
          })}
        </div>
      </TenantPage>

      {/* Mobile FAB Create Button - z-40 to appear below drawer modal (z-50) */}
      <button
        type="button"
        onClick={() => setIsQuickCreateOpen(true)}
        className="group fixed right-4 bottom-24 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-gray-900 text-white shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] transition-[background-color,box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] hover:bg-gray-800 hover:shadow-[0_3px_10px_rgba(15,23,42,0.045),0_0_0_1px_rgba(15,23,42,0.045)] active:bg-gray-950 md:hidden"
        aria-label="Aktivität erstellen"
      >
        <Plus
          className="relative h-6 w-6 transition-transform duration-150 group-active:rotate-90"
          strokeWidth={2.5}
          aria-hidden="true"
        />
      </button>

      {/* Success toasts handled globally */}
    </>
  );
}
