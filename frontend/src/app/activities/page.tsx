"use client";

import { useState, useEffect, useMemo } from "react";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
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
import { Loading } from "~/components/ui/loading";

export default function ActivitiesPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState("all");
  const [myActivitiesFilter, setMyActivitiesFilter] = useState(false);
  const [filteredActivities, setFilteredActivities] = useState<Activity[]>([]);
  const [selectedActivity, setSelectedActivity] = useState<Activity | null>(
    null,
  );
  const [isManagementModalOpen, setIsManagementModalOpen] = useState(false);
  const [isQuickCreateOpen, setIsQuickCreateOpen] = useState(false);
  const [currentStaff, setCurrentStaff] = useState<Staff | null>(null);
  const { success: toastSuccess } = useToast();
  const [isMobile, setIsMobile] = useState(false);

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Load activities, categories and current user on mount
  useEffect(() => {
    const loadData = async () => {
      try {
        setLoading(true);
        // Load activities, categories and current user in parallel
        const [activitiesData, categoriesData, staffData] = await Promise.all([
          fetchActivities(),
          getCategories(),
          userContextService.getCurrentStaff().catch(() => null), // Fail silently if not a staff member
        ]);
        setActivities(activitiesData);
        setCategories(categoriesData);
        setFilteredActivities(activitiesData);
        setCurrentStaff(staffData);
        setError(null);
      } catch (err) {
        setError("Fehler beim Laden der Aktivitäten");
        console.error("Error loading activities:", err);
      } finally {
        setLoading(false);
      }
    };

    void loadData();
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

  // Handle edit button click - same as selecting activity
  const handleEditActivity = (e: React.MouseEvent, activity: Activity) => {
    e.stopPropagation(); // Prevent duplicate modal opening
    handleSelectActivity(activity);
  };

  // Handle successful management actions (edit/delete)
  const handleManagementSuccess = async (message?: string) => {
    // Show success toast if provided
    if (message) {
      toastSuccess(message);
    }

    // Reload activities to show updated data
    try {
      const activitiesData = await fetchActivities();
      setActivities(activitiesData);
      setFilteredActivities(activitiesData);
    } catch (err) {
      console.error("Error reloading activities:", err);
    }
  };

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

  if (loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
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
                  onClick={() => setIsQuickCreateOpen(true)}
                  className="group flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                  style={{
                    background:
                      "linear-gradient(135deg, rgb(255, 49, 48) 0%, rgb(224, 32, 32) 100%)",
                    willChange: "transform, opacity",
                    WebkitTransform: "translateZ(0)",
                    transform: "translateZ(0)",
                  }}
                  aria-label="Aktivität erstellen"
                >
                  <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                  <svg
                    className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90"
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
                  <div className="absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
                </button>
              )
            }
          />
        </div>

        {/* Mobile FAB Create Button */}
        <button
          onClick={() => setIsQuickCreateOpen(true)}
          className="group fixed right-4 bottom-24 z-[9999] flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] text-white shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 hover:shadow-[0_8px_40px_rgb(255,49,48,0.3)] active:scale-95 md:hidden"
          style={{
            background:
              "linear-gradient(135deg, rgb(255, 49, 48) 0%, rgb(224, 32, 32) 100%)",
            willChange: "transform, opacity",
            WebkitTransform: "translateZ(0)",
            transform: "translateZ(0)",
          }}
          aria-label="Aktivität erstellen"
        >
          <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
          <svg
            className="relative h-6 w-6 transition-transform duration-300 group-active:rotate-90"
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
          <div className="absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
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
              return (
                <div
                  key={activity.id}
                  onClick={() => handleSelectActivity(activity)}
                  className="group relative cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-red-200/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                  style={{
                    animationName: "fadeInUp",
                    animationDuration: "0.5s",
                    animationTimingFunction: "ease-out",
                    animationFillMode: "forwards",
                    animationDelay: `${index * 0.05}s`,
                    opacity: 0,
                  }}
                >
                  {/* Modern gradient overlay */}
                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-red-50/80 to-rose-100/80 opacity-[0.03]"></div>
                  {/* Subtle inner glow */}
                  <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  {/* Modern border highlight */}
                  <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-red-200/60"></div>

                  <div className="relative flex items-center justify-between p-5">
                    {/* Left content */}
                    <div className="min-w-0 flex-1">
                      {/* Activity Name */}
                      <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-red-600">
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

                      {/* Edit icon button */}
                      <button
                        onClick={(e) => handleEditActivity(e, activity)}
                        className="relative"
                        aria-label="Aktivität bearbeiten"
                      >
                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gray-100 transition-all duration-200 md:group-hover:scale-110 md:group-hover:bg-red-100">
                          <svg
                            className="h-5 w-5 text-gray-600 transition-colors md:group-hover:text-red-600"
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

                        {/* Ripple effect on hover */}
                        <div className="absolute inset-0 scale-0 rounded-full bg-red-200/20 transition-transform duration-300 md:group-hover:scale-100"></div>
                      </button>
                    </div>
                  </div>

                  {/* Glowing border effect */}
                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-red-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                </div>
              );
            })}

            {/* Add fadeInUp animation */}
            <style jsx>{`
              @keyframes fadeInUp {
                from {
                  opacity: 0;
                  transform: translateY(20px);
                }
                to {
                  opacity: 1;
                  transform: translateY(0);
                }
              }
            `}</style>
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
              <h3 className="mt-4 text-xl font-bold text-gray-900">
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
          readOnly={false}
        />
      )}

      {/* Quick Create Activity Modal */}
      <QuickCreateActivityModal
        isOpen={isQuickCreateOpen}
        onClose={() => setIsQuickCreateOpen(false)}
        onSuccess={() => {
          // Don't close here - let the modal handle its own closing
          void handleManagementSuccess(); // Reload activities
        }}
      />

      {/* Success toasts handled globally */}
    </ResponsiveLayout>
  );
}
