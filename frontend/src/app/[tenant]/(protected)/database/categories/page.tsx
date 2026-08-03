"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { categoriesConfig } from "@/components/database/configs/categories.config";
import type { ActivityCategory } from "@/lib/activity-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { CategoriesMasterDetail } from "@/components/categories/categories-master-detail";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "DatabaseCategoriesPage" });

/** SWR key of the Stammdaten list (archived included). */
const CATEGORIES_LIST_CACHE_KEY = "database-categories-list";

const CategoryIcon = ({ className }: { className?: string }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
  >
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A2 2 0 013 12V7a4 4 0 014-4z"
    />
  </svg>
);

export default function CategoriesPage() {
  return (
    <Suspense fallback={null}>
      <CategoriesPageContent />
    </Suspense>
  );
}

function CategoriesPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("category");
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);

  const {
    showConfirmModal: showArchiveConfirmModal,
    handleDeleteClick: handleArchiveClick,
    handleDeleteCancel: handleArchiveCancel,
    confirmDelete: confirmArchive,
  } = useDeleteConfirmation();

  const { success: toastSuccess, error: toastError } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(categoriesConfig), []);
  const tenantMutate = useTenantMutate();

  const {
    data: categoriesData,
    isLoading: loading,
    error: categoriesError,
  } = useSWRAuth(CATEGORIES_LIST_CACHE_KEY, async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = categoriesError
    ? "Fehler beim Laden der Kategorien. Bitte versuche es später erneut."
    : null;

  // Only this screen's list is SWR-cached. The category pickers (Termin-
  // Formular, Aktivitäten, Schnellstart) fetch /api/activities/categories
  // when they open, so they pick up a new "Essen" without extra invalidation.
  const refreshCategories = useCallback(
    () => tenantMutate(CATEGORIES_LIST_CACHE_KEY),
    [tenantMutate],
  );

  const filteredCategories = useMemo(() => {
    const categories = categoriesData ?? [];
    let arr = [...categories];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (category) =>
          category.name.toLowerCase().includes(q) ||
          (category.description?.toLowerCase().includes(q) ?? false),
      );
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [categoriesData, searchTerm]);

  const selectedCategory = useMemo(
    () =>
      selectedId
        ? ((categoriesData ?? []).find(
            (category) => category.id === selectedId,
          ) ?? null)
        : null,
    [categoriesData, selectedId],
  );

  const handleSelectCategory = useCallback(
    (id: string | null) => {
      updateUrlParams({ category: id });
    },
    [updateUrlParams],
  );

  const handleCreateCategory = useCallback(
    async (data: Partial<ActivityCategory>) => {
      try {
        if (categoriesConfig.form.transformBeforeSubmit) {
          data = categoriesConfig.form.transformBeforeSubmit(data);
        }
        const created = await service.create(data);
        toastSuccess(
          getDbOperationMessage(
            "create",
            categoriesConfig.name.singular,
            created.name,
          ),
        );
        setShowCreateModal(false);
        await refreshCategories();
      } catch (createError) {
        logger.error("category_create_failed", {
          error:
            createError instanceof Error
              ? createError.message
              : String(createError),
        });
        throw createError;
      }
    },
    [service, refreshCategories, toastSuccess],
  );

  const handleUpdateCategory = useCallback(
    async (data: Partial<ActivityCategory>) => {
      if (!selectedCategory) return;
      try {
        if (categoriesConfig.form.transformBeforeSubmit) {
          data = categoriesConfig.form.transformBeforeSubmit(data);
        }
        await service.update(selectedCategory.id, data);
        toastSuccess(
          getDbOperationMessage(
            "update",
            categoriesConfig.name.singular,
            selectedCategory.name,
          ),
        );
        await refreshCategories();
      } catch (updateError) {
        logger.error("category_update_failed", {
          category_id: selectedCategory.id,
          error:
            updateError instanceof Error
              ? updateError.message
              : String(updateError),
        });
        throw updateError;
      }
    },
    [selectedCategory, service, refreshCategories, toastSuccess],
  );

  const handleArchiveCategory = useCallback(async () => {
    if (!selectedCategory) return;
    const archiveError = await service.delete(selectedCategory.id);
    if (archiveError) {
      toastError(archiveError);
      return;
    }
    toastSuccess(`Kategorie "${selectedCategory.name}" wurde archiviert.`);
    await refreshCategories();
  }, [selectedCategory, service, toastError, toastSuccess, refreshCategories]);

  const handleRestoreCategory = useCallback(async () => {
    if (!selectedCategory) return;
    try {
      const response = await fetch(
        `/api/activities/categories/${selectedCategory.id}/restore`,
        { method: "POST", credentials: "include" },
      );
      if (!response.ok) {
        throw new Error(await response.text());
      }
      toastSuccess(
        `Kategorie "${selectedCategory.name}" wurde wiederhergestellt.`,
      );
      await refreshCategories();
    } catch (restoreError) {
      logger.error("category_restore_failed", {
        category_id: selectedCategory.id,
        error:
          restoreError instanceof Error
            ? restoreError.message
            : String(restoreError),
      });
      toastError(
        getApiErrorMessage(
          restoreError,
          "wiederherstellen",
          "Kategorien",
          "Kategorie konnte nicht wiederhergestellt werden.",
        ),
      );
    }
  }, [selectedCategory, toastError, toastSuccess, refreshCategories]);

  const canShowDetail =
    !loading && (filteredCategories.length > 0 || selectedCategory !== null);

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Kategorien" : ""}
          badge={{
            icon: <CategoryIcon className="h-5 w-5 text-gray-600" />,
            count: filteredCategories.length,
            label: "Kategorien",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Kategorien suchen...",
          }}
          activeFilters={
            searchTerm
              ? [
                  {
                    id: "search",
                    label: `"${searchTerm}"`,
                    onRemove: () => setSearchTerm(""),
                  },
                ]
              : []
          }
          onClearAllFilters={() => setSearchTerm("")}
          actionButton={
            <DatabaseCreateAction
              label="Kategorie"
              ariaLabel="Kategorie erstellen"
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
          <CategoriesMasterDetail
            categories={filteredCategories}
            selectedId={selectedId}
            selectedCategory={selectedCategory}
            onSelect={handleSelectCategory}
            onSaveCategory={handleUpdateCategory}
            onArchiveClick={handleArchiveClick}
            onRestore={handleRestoreCategory}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={<CategoryIcon className="mx-auto h-12 w-12 text-gray-400" />}
          title={
            searchTerm
              ? "Keine Kategorien gefunden"
              : "Keine Kategorien vorhanden"
          }
          description={
            searchTerm
              ? "Versuche andere Suchkriterien."
              : "Es wurden noch keine Kategorien erstellt."
          }
        />
      ) : null}

      <DatabaseFormModal<ActivityCategory>
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        mode="create"
        config={categoriesConfig}
        onSubmit={handleCreateCategory}
      />

      {selectedCategory && (
        <ConfirmationModal
          isOpen={showArchiveConfirmModal}
          onClose={handleArchiveCancel}
          onConfirm={() => confirmArchive(() => void handleArchiveCategory())}
          title="Kategorie archivieren?"
          confirmText="Archivieren"
          cancelText="Abbrechen"
        >
          <p className="text-sm text-gray-700">
            Möchtest du die Kategorie{" "}
            <span className="font-medium">{selectedCategory.name}</span>{" "}
            archivieren? Sie wird für neue Termine und Aktivitäten nicht mehr
            angeboten. Bestehende Einträge behalten sie und bleiben gültig.
          </p>
        </ConfirmationModal>
      )}
    </DatabasePageLayout>
  );
}
