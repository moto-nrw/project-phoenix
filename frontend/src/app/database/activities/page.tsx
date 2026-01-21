"use client";

import { useState, useMemo } from "react";
import { useSession } from "~/lib/auth-client";
import { redirect } from "next/navigation";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { activitiesConfig } from "@/lib/database/configs/activities.config";
import type { Activity } from "@/lib/activity-helpers";
import {
  ActivityCreateModal,
  ActivityDetailModal,
  ActivityEditModal,
} from "@/components/activities";
import { ConfirmationModal } from "~/components/ui/modal";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useSWRAuth, mutate } from "~/lib/swr";

export default function ActivitiesPage() {
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");
  const isMobile = useIsMobile();

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [selectedActivity, setSelectedActivity] = useState<Activity | null>(
    null,
  );
  const [detailLoading, setDetailLoading] = useState(false);

  // Delete confirmation modal management
  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation(setShowDetailModal);

  // Secondary management modals (disabled for now)

  const { success: toastSuccess } = useToast();

  // BetterAuth: cookies handle auth, isPending replaces status
  const { data: session, isPending } = useSession();

  // Redirect if not authenticated
  if (!isPending && !session?.user) {
    redirect("/");
  }

  const service = useMemo(() => createCrudService(activitiesConfig), []);

  // Fetch activities with SWR (automatic caching, deduplication, revalidation)
  const {
    data: activitiesData,
    isLoading: loading,
    error: activitiesError,
  } = useSWRAuth("database-activities-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = activitiesError
    ? "Fehler beim Laden der Aktivitäten. Bitte versuchen Sie es später erneut."
    : null;

  // Unique categories
  const uniqueCategories = useMemo(() => {
    const activities = activitiesData ?? [];
    const set = new Set<string>();
    activities.forEach((a) => {
      if (a.category_name) set.add(a.category_name);
    });
    return Array.from(set)
      .sort((a, b) => a.localeCompare(b, "de"))
      .map((c) => ({ value: c, label: c }));
  }, [activitiesData]);

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

  // Derived list (use activitiesData directly to avoid dependency issues)
  const filteredActivities = useMemo(() => {
    const activities = activitiesData ?? [];
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
  }, [activitiesData, searchTerm, categoryFilter]);

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
      await mutate("database-activities-list");
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
      await mutate("database-activities-list");
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
      await mutate("database-activities-list");
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  return (
    <DatabasePageLayout loading={loading} sessionLoading={isPending}>
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
                style={{
                  background:
                    "linear-gradient(135deg, rgb(255, 49, 48) 0%, rgb(224, 32, 32) 100%)",
                  willChange: "transform, opacity",
                  WebkitTransform: "translateZ(0)",
                  transform: "translateZ(0)",
                }}
                aria-label="Aktivität erstellen"
              >
                <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
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
        style={{
          background:
            "linear-gradient(135deg, rgb(255, 49, 48) 0%, rgb(224, 32, 32) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Aktivität erstellen"
      >
        <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
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
            const initials = (activity.name?.slice(0, 2) ?? "AG").toUpperCase();
            const handleClick = () => void handleSelectActivity(activity);
            return (
              <button
                type="button"
                key={activity.id}
                onClick={handleClick}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-red-200/50 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
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
              </button>
            );
          })}
        </div>
      )}

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
          onDeleteClick={handleDeleteClick}
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

      {/* Delete Confirmation Modal */}
      {selectedActivity && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteActivity())}
          title="Aktivität löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie die Aktivität{" "}
            <span className="font-medium">{selectedActivity.name}</span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}

      {/* Success toasts handled globally */}
    </DatabasePageLayout>
  );
}
