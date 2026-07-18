"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useSession } from "next-auth/react";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
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
import { useTenantAwarePath } from "~/lib/tenant-path";
import { NfcModeGuard } from "~/components/tenant/nfc-mode-guard";
import { ActivitiesSkeleton } from "./page-skeleton";
import { redirect } from "next/navigation";

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
  const [isMobile, setIsMobile] = useState(false);
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

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

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

  // Session loading is gated explicitly via status; the data clause covers
  // the render tick between "authenticated" and SWR starting its first fetch
  // (isLoading is still false there but no data exists yet). Unauthenticated
  // visits redirect via the required-session guard, and expired/tokenless
  // sessions redirect via the effect above, so this cannot show the skeleton
  // indefinitely.
  if (
    status === "loading" ||
    isLoading ||
    (pageData === undefined && !fetchError)
  ) {
    return <ActivitiesSkeleton />;
  }

  return (
    <>
      <div className="w-full">
        {/* PageHeaderWithSearch - Title only on mobile */}
        <div className="relative z-30 mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Aktivitäten" : ""}
            badge={{
              icon: (
                <svg
                  className="h-5 w-5 text-gray-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                  />
                </svg>
              ),
              count: filteredActivities.length,
              label: "Aktivitäten",
            }}
            search={{
              value: searchTerm,
              onChange: setSearchTerm,
              placeholder: "Aktivität suchen...",
            }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => {
              setSearchTerm("");
              setCategoryFilter("all");
              setMyActivitiesFilter(false);
            }}
            actionButton={
              !isMobile && (
                <button
                  type="button"
                  onClick={() => setIsQuickCreateOpen(true)}
                  className="group flex h-10 w-10 items-center justify-center rounded-full bg-gray-900 text-white shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] transition-[background-color,box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] hover:bg-gray-800 hover:shadow-[0_3px_10px_rgba(15,23,42,0.045),0_0_0_1px_rgba(15,23,42,0.045)] active:bg-gray-950"
                  aria-label="Aktivität erstellen"
                >
                  <svg
                    className="relative h-5 w-5 transition-transform duration-150 group-active:rotate-90"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2.5}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 4.5v15m7.5-7.5h-15"
                    />
                  </svg>
                </button>
              )
            }
          />
        </div>

        {/* Mobile FAB Create Button - z-40 to appear below drawer modal (z-50) */}
        <button
          type="button"
          onClick={() => setIsQuickCreateOpen(true)}
          className="group fixed right-4 bottom-24 z-40 flex h-14 w-14 items-center justify-center rounded-full bg-gray-900 text-white shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] transition-[background-color,box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] hover:bg-gray-800 hover:shadow-[0_3px_10px_rgba(15,23,42,0.045),0_0_0_1px_rgba(15,23,42,0.045)] active:bg-gray-950 md:hidden"
          aria-label="Aktivität erstellen"
        >
          <svg
            className="relative h-6 w-6 transition-transform duration-150 group-active:rotate-90"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M12 4.5v15m7.5-7.5h-15"
            />
          </svg>
        </button>

        {/* Error Alert */}
        {error && (
          <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Activity List - Modern Design */}
        {filteredActivities.length > 0 ? (
          <div className="space-y-3">
            {filteredActivities.map((activity, index) => {
              const handleClick = () => handleSelectActivity(activity);
              return (
                <button
                  type="button"
                  key={activity.id}
                  onClick={handleClick}
                  className="moto-content-surface moto-hover-elevated group relative w-full cursor-pointer overflow-hidden rounded-2xl border border-gray-200 bg-white text-left shadow-[0_1px_2px_rgba(15,23,42,0.04),0_0_0_1px_rgba(15,23,42,0.02)] active:shadow-[0_10px_26px_rgba(15,23,42,0.1)]"
                  style={{
                    animationName: "fadeInUp",
                    animationDuration: "0.5s",
                    animationTimingFunction: "ease-out",
                    animationFillMode: "forwards",
                    animationDelay: `${index * 0.05}s`,
                    opacity: 0,
                  }}
                >
                  <div className="pointer-events-none absolute inset-0 rounded-2xl ring-1 ring-transparent transition-[box-shadow] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] md:group-hover:shadow-[inset_0_1px_0_rgba(255,255,255,0.9)]"></div>

                  <div className="relative flex items-center justify-between p-5">
                    {/* Left content */}
                    <div className="min-w-0 flex-1">
                      {/* Activity Name */}
                      <h3 className="inline-block origin-left text-lg font-semibold text-gray-900 transition-[color,transform] duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] motion-reduce:transition-none md:group-hover:scale-[1.025] md:group-hover:text-gray-950 motion-reduce:md:group-hover:scale-100">
                        {activity.name}
                      </h3>

                      {/* Meta info row */}
                      <div className="mt-1 flex items-center gap-4">
                        {/* Creator info */}
                        <p className="text-sm text-gray-500">
                          <span className="text-gray-400">Erstellt von:</span>{" "}
                          {formatSupervisorList(activity.supervisors)}
                        </p>
                      </div>
                    </div>

                    {/* Right content - Edit button (available for all users) */}
                    <div className="ml-4 flex items-center gap-3">
                      {/* Desktop hint */}
                      <span className="hidden text-xs text-gray-400 transition-colors group-hover:text-gray-600 lg:block">
                        Bearbeiten
                      </span>

                      {/* Edit icon indicator (visual only - parent button handles click) */}
                      <span className="relative" aria-hidden="true">
                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gray-100 transition-colors duration-300 md:group-hover:bg-gray-200">
                          <svg
                            className="h-5 w-5 text-gray-600 transition-colors duration-300 md:group-hover:text-gray-900"
                            fill="none"
                            viewBox="0 0 24 24"
                            stroke="currentColor"
                          >
                            <path
                              strokeLinecap="round"
                              strokeLinejoin="round"
                              strokeWidth={2}
                              d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                            />
                          </svg>
                        </div>
                      </span>
                    </div>
                  </div>
                </button>
              );
            })}
          </div>
        ) : (
          <div className="flex min-h-[300px] items-center justify-center">
            <div className="text-center">
              <svg
                className="mx-auto h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01"
                />
              </svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">
                {searchTerm || categoryFilter !== "all"
                  ? "Keine Aktivitäten gefunden"
                  : "Keine Aktivitäten vorhanden"}
              </h3>
              <p className="mt-2 text-sm text-gray-600">
                {searchTerm || categoryFilter !== "all"
                  ? "Versuchen Sie andere Suchkriterien oder Filter."
                  : "Es wurden noch keine Aktivitäten erstellt."}
              </p>
            </div>
          </div>
        )}
      </div>

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
          readOnly={!isActivityCreator(selectedActivity, currentStaff?.id)}
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

      {/* Success toasts handled globally */}
    </>
  );
}
