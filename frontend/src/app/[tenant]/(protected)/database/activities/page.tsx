"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { activitiesConfig } from "@/components/database/configs/activities.config";
import type { Activity } from "@/lib/activity-helpers";
import { ActivitiesMasterDetail } from "@/components/activities/activities-master-detail";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { NfcModeGuard } from "~/components/tenant/nfc-mode-guard";

const logger = createLogger({ component: "DatabaseActivitiesPage" });

export default function ActivitiesPage() {
  return (
    <NfcModeGuard>
      <Suspense fallback={null}>
        <ActivitiesPageContent />
      </Suspense>
    </NfcModeGuard>
  );
}

function ActivitiesPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("activity");
  const [searchTerm, setSearchTerm] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string>("all");

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [selectedActivityDetail, setSelectedActivityDetail] =
    useState<Activity | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [formResetCounter, setFormResetCounter] = useState(0);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation();

  const { success: toastSuccess, error: toastError } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(activitiesConfig), []);
  const tenantMutate = useTenantMutate();

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

  const selectedActivitySummary = useMemo(
    () =>
      selectedId
        ? (filteredActivities.find((activity) => activity.id === selectedId) ??
          null)
        : null,
    [filteredActivities, selectedId],
  );

  const selectedActivity =
    selectedActivityDetail?.id === selectedActivitySummary?.id
      ? selectedActivityDetail
      : selectedActivitySummary;

  const handleSelectActivity = useCallback(
    (id: string | null) => {
      updateUrlParams({ activity: id });
    },
    [updateUrlParams],
  );

  useEffect(() => {
    if (!selectedId || !selectedActivitySummary) {
      setSelectedActivityDetail(null);
      setDetailLoading(false);
      return;
    }

    setSelectedActivityDetail((current) =>
      current?.id === selectedActivitySummary.id
        ? current
        : selectedActivitySummary,
    );

    let cancelled = false;
    setDetailLoading(true);

    void service
      .getOne(selectedActivitySummary.id)
      .then((fresh) => {
        if (!cancelled) {
          setSelectedActivityDetail(fresh);
        }
      })
      .catch((fetchError: unknown) => {
        logger.error("failed to fetch activity detail", {
          activity_id: selectedActivitySummary.id,
          error:
            fetchError instanceof Error
              ? fetchError.message
              : String(fetchError),
        });
      })
      .finally(() => {
        if (!cancelled) {
          setDetailLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [selectedId, selectedActivitySummary, service]);

  const handleCreateActivity = useCallback(
    async (data: Partial<Activity>) => {
      try {
        const payload = activitiesConfig.form.transformBeforeSubmit
          ? activitiesConfig.form.transformBeforeSubmit(data)
          : data;
        const created = await service.create(payload);
        toastSuccess(
          getDbOperationMessage(
            "create",
            activitiesConfig.name.singular,
            created.name,
          ),
        );
        setShowCreateModal(false);
        await tenantMutate("database-activities-list");
      } catch (createError) {
        logger.error("failed to create activity", {
          error:
            createError instanceof Error
              ? createError.message
              : String(createError),
        });
        throw createError;
      }
    },
    [service, tenantMutate, toastSuccess],
  );

  const handleUpdateActivity = useCallback(
    async (data: Partial<Activity>) => {
      if (!selectedActivity) return;
      try {
        const payload = activitiesConfig.form.transformBeforeSubmit
          ? activitiesConfig.form.transformBeforeSubmit(data)
          : data;
        await service.update(selectedActivity.id, payload);
        const refreshed = await service.getOne(selectedActivity.id);
        setSelectedActivityDetail(refreshed);
        setFormResetCounter((current) => current + 1);
        toastSuccess(
          getDbOperationMessage(
            "update",
            activitiesConfig.name.singular,
            selectedActivity.name,
          ),
        );
        await tenantMutate("database-activities-list");
      } catch (updateError) {
        logger.error("failed to update activity", {
          activity_id: selectedActivity.id,
          error:
            updateError instanceof Error
              ? updateError.message
              : String(updateError),
        });
        throw updateError;
      }
    },
    [selectedActivity, service, tenantMutate, toastSuccess],
  );

  const handleDeleteActivity = useCallback(async () => {
    if (!selectedActivity) return;
    try {
      setDetailLoading(true);
      const deleteError = await service.delete(selectedActivity.id);
      if (deleteError) {
        toastError(deleteError);
        return;
      }
      toastSuccess(
        getDbOperationMessage(
          "delete",
          activitiesConfig.name.singular,
          selectedActivity.name,
        ),
      );
      setSelectedActivityDetail(null);
      setFormResetCounter(0);
      handleSelectActivity(null);
      await tenantMutate("database-activities-list");
    } finally {
      setDetailLoading(false);
    }
  }, [
    selectedActivity,
    service,
    toastError,
    toastSuccess,
    handleSelectActivity,
    tenantMutate,
  ]);

  const canShowDetail = !loading && filteredActivities.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title="Aktivitäten"
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.activities.icon}
                tone={MOTO_CONCEPTS.activities.tone}
                size={20}
              />
            ),
            count: filteredActivities.length,
            label: "Aktivitäten",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Aktivitäten suchen…",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setCategoryFilter("all");
          }}
          actionButton={
            <DatabaseCreateAction
              label="Aktivität"
              ariaLabel="Aktivität erstellen"
              onClick={() => setShowCreateModal(true)}
            />
          }
        />
      </div>

      {error && (
        <div className="mb-6">
          <Alert type="error" message={error} />
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <ActivitiesMasterDetail
            activities={filteredActivities}
            selectedId={selectedId}
            selectedActivity={selectedActivity}
            detailLoading={detailLoading}
            onSelect={handleSelectActivity}
            onSaveActivity={handleUpdateActivity}
            onResetForm={() => setFormResetCounter((current) => current + 1)}
            onDeleteClick={handleDeleteClick}
            formResetKey={`${selectedActivity?.id ?? "none"}:${formResetCounter}`}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.activities.icon}
              tone={MOTO_CONCEPTS.activities.tone}
              size={48}
              className="mx-auto"
            />
          }
          title={
            searchTerm || categoryFilter !== "all"
              ? "Keine Aktivitäten gefunden"
              : "Keine Aktivitäten vorhanden"
          }
          description={
            searchTerm || categoryFilter !== "all"
              ? "Versuchen Sie andere Suchkriterien oder Filter."
              : "Es wurden noch keine Aktivitäten erstellt."
          }
        />
      ) : null}

      <DatabaseFormModal<Activity>
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        mode="create"
        config={activitiesConfig}
        onSubmit={handleCreateActivity}
      />

      {selectedActivity && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteActivity())}
          title="Aktivität löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie die Aktivität{" "}
            <span className="font-medium">{selectedActivity.name}</span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}
    </DatabasePageLayout>
  );
}
