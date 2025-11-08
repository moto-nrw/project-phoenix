"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { useToast } from "~/contexts/ToastContext";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { activitiesConfig } from "@/lib/database/configs/activities.config";
import type { Activity } from "@/lib/activity-helpers";
import {
  ActivityCreateModal,
  ActivityDetailModal,
  ActivityEditModal,
} from "@/components/activities";

import { Loading } from "~/components/ui/loading";
export default function ActivitiesPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const [isMobile, setIsMobile] = useState(false);

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedActivity, setSelectedActivity] = useState<Activity | null>(
    null,
  );
  const [detailLoading, setDetailLoading] = useState(false);

  // Secondary management modals (disabled for now)

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(activitiesConfig), []);

  // Mobile detection
  useEffect(() => {
    const checkMobile = () => setIsMobile(window.innerWidth < 768);
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Fetch activities
  const fetchActivities = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setActivities(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching activities:", err);
      setError(
        "Fehler beim Laden der Aktivitäten. Bitte versuchen Sie es später erneut.",
      );
      setActivities([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    void fetchActivities();
  }, [fetchActivities]);

  // Unique categories
  const uniqueCategories = useMemo(() => {
    const set = new Set<string>();
    activities.forEach((a) => {
      if (a.category_name) set.add(a.category_name);
    });
    return Array.from(set)
      .sort()
      .map((c) => ({ value: c, label: c }));
  }, [activities]);

  // Filters config
  const filters: FilterConfig[] = useMemo(
    () => [
      {
        id: "category",
        label: "Kategorie",
        type: "dropdown",
        value: categoryFilter,
        onChange: (v) => setCategoryFilter(v as string),
        options: [
          { value: "all", label: "Alle Kategorien" },
          ...uniqueCategories,
        ],
      },
    ],
    [categoryFilter, uniqueCategories],
  );

  const activeFilters: ActiveFilter[] = useMemo(() => {
    const list: ActiveFilter[] = [];
    if (searchTerm)
      list.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    if (categoryFilter !== "all")
      list.push({
        id: "category",
        label: categoryFilter,
        onRemove: () => setCategoryFilter("all"),
      });
    return list;
  }, [searchTerm, categoryFilter]);

  // Derived list
  const filteredActivities = useMemo(() => {
    let arr = [...activities];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (a) =>
          a.name.toLowerCase().includes(q) ||
          (a.category_name?.toLowerCase().includes(q) ?? false) ||
          (a.supervisor_name?.toLowerCase().includes(q) ?? false),
      );
    }
    if (categoryFilter !== "all") {
      arr = arr.filter((a) => a.category_name === categoryFilter);
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [activities, searchTerm, categoryFilter]);

  // Select activity => open detail and fetch fresh
  const handleSelectActivity = async (activity: Activity) => {
    setSelectedActivity(activity);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(activity.id);
      setSelectedActivity(fresh);
    } finally {
      setDetailLoading(false);
    }
  };

  // Create activity
  const handleCreateActivity = async (data: Partial<Activity>) => {
    try {
      setCreateLoading(true);
      // Transform if needed
      if (activitiesConfig.form.transformBeforeSubmit) {
        data = activitiesConfig.form.transformBeforeSubmit(data);
      }
      const created = await service.create(data);
      toastSuccess(
        getDbOperationMessage(
          "create",
          activitiesConfig.name.singular,
          created.name,
        ),
      );
      setShowCreateModal(false);
      await fetchActivities();
    } finally {
      setCreateLoading(false);
    }
  };

  // Update activity
  const handleUpdateActivity = async (data: Partial<Activity>) => {
    if (!selectedActivity) return;
    try {
      setDetailLoading(true);
      if (activitiesConfig.form.transformBeforeSubmit) {
        data = activitiesConfig.form.transformBeforeSubmit(data);
      }
      await service.update(selectedActivity.id, data);
      const name = selectedActivity.name;
      toastSuccess(
        getDbOperationMessage("update", activitiesConfig.name.singular, name),
      );
      const refreshed = await service.getOne(selectedActivity.id);
      setSelectedActivity(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchActivities();
    } catch (e) {
      console.error("Error updating activity", e);
      throw e;
    } finally {
      setDetailLoading(false);
    }
  };

  // Delete activity
  const handleDeleteActivity = async () => {
    if (!selectedActivity) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedActivity.id);
      toastSuccess(
        getDbOperationMessage(
          "delete",
          activitiesConfig.name.singular,
          selectedActivity.name,
        ),
      );
      setShowDetailModal(false);
      setSelectedActivity(null);
      await fetchActivities();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  // Secondary actions (removed in this update)

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="w-full">
        {/* Mobile Back Button */}
        {isMobile && (
          <button
            onClick={() => (window.location.href = "/database")}
            className="relative z-10 mb-3 flex items-center gap-2 text-gray-600 transition-colors duration-200 hover:text-gray-900"
            aria-label="Zurück zur Datenverwaltung"
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 19l-7-7 7-7"
              />
            </svg>
            <span className="text-sm font-medium">Zurück</span>
          </button>
        )}

        {/* Header */}
        <div className="mb-4">
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
              placeholder: "Aktivitäten suchen...",
            }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => {
              setSearchTerm("");
              setCategoryFilter("all");
            }}
            actionButton={
              !isMobile && (
                <button
                  onClick={() => setShowCreateModal(true)}
                  className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                  aria-label="Aktivität erstellen"
                >
                  <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
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
                  <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
                </button>
              )
            }
          />
        </div>

        {/* Mobile FAB */}
        <button
          onClick={() => setShowCreateModal(true)}
          className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgb(255,49,48,0.3)] active:scale-95 md:hidden"
          aria-label="Aktivität erstellen"
        >
          <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
          <svg
            className="pointer-events-none relative h-6 w-6 transition-transform duration-300 group-active:rotate-90"
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
          <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
        </button>

        {/* Error */}
        {error && (
          <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* List */}
        {filteredActivities.length === 0 ? (
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
                  d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
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
        ) : (
          <div className="space-y-3">
            {filteredActivities.map((activity, index) => {
              const initials = (
                activity.name?.slice(0, 2) ?? "AG"
              ).toUpperCase();
              return (
                <div
                  key={activity.id}
                  onClick={() => void handleSelectActivity(activity)}
                  className="group relative cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-red-200/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                  style={{
                    animationName: "fadeInUp",
                    animationDuration: "0.5s",
                    animationTimingFunction: "ease-out",
                    animationFillMode: "forwards",
                    animationDelay: `${index * 0.03}s`,
                    opacity: 0,
                  }}
                >
                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-red-50/80 to-rose-100/80 opacity-[0.03]"></div>
                  <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                  <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-red-200/60"></div>

                  <div className="relative flex items-center gap-4 p-5">
                    <div className="flex-shrink-0">
                      <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                        {initials}
                      </div>
                    </div>

                    <div className="min-w-0 flex-1">
                      <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-red-600">
                        {activity.name}
                      </h3>
                      <div className="mt-1 flex flex-wrap items-center gap-2">
                        {activity.category_name && (
                          <span className="inline-flex items-center rounded-full bg-red-100 px-2 py-1 text-xs font-medium text-red-700">
                            {activity.category_name}
                          </span>
                        )}
                      </div>
                    </div>

                    <div className="flex-shrink-0">
                      <svg
                        className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-red-600"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M9 5l7 7-7 7"
                        />
                      </svg>
                    </div>
                  </div>

                  <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-red-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                </div>
              );
            })}

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
        )}
      </div>

      {/* Create Modal */}
      <ActivityCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateActivity}
        loading={createLoading}
      />

      {/* Detail Modal */}
      {selectedActivity && (
        <ActivityDetailModal
          isOpen={showDetailModal}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedActivity(null);
          }}
          activity={selectedActivity}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteActivity()}
          loading={detailLoading}
        />
      )}

      {/* Edit Modal */}
      {selectedActivity && (
        <ActivityEditModal
          isOpen={showEditModal}
          onClose={() => {
            setShowEditModal(false);
          }}
          activity={selectedActivity}
          onSave={handleUpdateActivity}
          loading={detailLoading}
        />
      )}

      {/* Secondary management modals removed for this release */}

      {/* Success toasts handled globally */}
    </ResponsiveLayout>
  );
}
