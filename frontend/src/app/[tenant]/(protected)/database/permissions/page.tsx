"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { permissionsConfig } from "@/lib/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";
import {
  PermissionCreateModal,
  PermissionEditModal,
  PermissionsMasterDetail,
} from "@/components/permissions";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  formatPermissionDisplay,
  localizeDescription,
} from "@/lib/permission-labels";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DatabasePermissionsPage" });

export default function PermissionsPage() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("permission");
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [editError, setEditError] = useState<string | null>(null);
  const [savingPermission, setSavingPermission] = useState(false);

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

  const service = useMemo(() => createCrudService(permissionsConfig), []);

  const fetchPermissions = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setPermissions(arr);
      setError(null);
    } catch (err) {
      logger.error("failed to fetch permissions", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Fehler beim Laden der Berechtigungen. Bitte versuchen Sie es später erneut.",
      );
      setPermissions([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    void fetchPermissions();
  }, [fetchPermissions]);

  const filters: FilterConfig[] = useMemo(() => [], []);

  const activeFilters: ActiveFilter[] = useMemo(
    () =>
      searchTerm
        ? [
            {
              id: "search",
              label: `"${searchTerm}"`,
              onRemove: () => setSearchTerm(""),
            },
          ]
        : [],
    [searchTerm],
  );

  const filteredPermissions = useMemo(() => {
    let arr = [...permissions];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter((p) => {
        const display = formatPermissionDisplay(p.resource, p.action);
        const description = localizeDescription(
          p.resource,
          p.action,
          p.description,
        );
        return (
          p.name.toLowerCase().includes(q) ||
          (p.description?.toLowerCase().includes(q) ?? false) ||
          p.resource.toLowerCase().includes(q) ||
          p.action.toLowerCase().includes(q) ||
          description.toLowerCase().includes(q) ||
          display.toLowerCase().includes(q)
        );
      });
    }
    arr.sort((a, b) => {
      const r = a.resource.localeCompare(b.resource, "de");
      if (r !== 0) return r;
      const a2 = a.action.localeCompare(b.action, "de");
      if (a2 !== 0) return a2;
      return (a.name || "").localeCompare(b.name || "", "de");
    });
    return arr;
  }, [permissions, searchTerm]);

  const selectedPermission = useMemo(
    () =>
      selectedId
        ? (filteredPermissions.find(
            (permission) => permission.id === selectedId,
          ) ?? null)
        : null,
    [filteredPermissions, selectedId],
  );

  const handleSelectPermission = useCallback(
    (id: string | null) => {
      updateUrlParams({ permission: id });
    },
    [updateUrlParams],
  );

  const handleEditClick = useCallback(() => {
    setEditError(null);
    setShowEditModal(true);
  }, []);
  const handleCloseEditModal = useCallback(() => {
    setShowEditModal(false);
    setEditError(null);
  }, []);
  const handleCloseCreateModal = useCallback(() => {
    setShowCreateModal(false);
    setCreateError(null);
  }, []);

  const handleCreatePermission = useCallback(
    async (data: Partial<Permission>) => {
      try {
        setCreateLoading(true);
        setCreateError(null);
        const payload = permissionsConfig.form.transformBeforeSubmit
          ? permissionsConfig.form.transformBeforeSubmit(data)
          : data;
        const created = await service.create(payload);
        const display = `${created.resource}: ${created.action}`;
        toastSuccess(
          getDbOperationMessage(
            "create",
            permissionsConfig.name.singular,
            display,
          ),
        );
        setShowCreateModal(false);
        await fetchPermissions();
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : String(err);
        if (
          errorMessage.includes("duplicate key") ||
          errorMessage.includes("23505")
        ) {
          setCreateError(
            `Die Berechtigung "${data.resource}:${data.action}" existiert bereits. ` +
              `Jede Kombination aus Ressource und Aktion darf nur einmal vorhanden sein. ` +
              `Bitte wählen Sie eine andere Kombination.`,
          );
        } else {
          setCreateError(
            "Fehler beim Erstellen der Berechtigung. Bitte versuchen Sie es erneut.",
          );
        }
      } finally {
        setCreateLoading(false);
      }
    },
    [service, fetchPermissions, toastSuccess],
  );

  const handleUpdatePermission = useCallback(
    async (data: Partial<Permission>) => {
      if (!selectedPermission) return;
      try {
        setSavingPermission(true);
        setEditError(null);
        const payload = permissionsConfig.form.transformBeforeSubmit
          ? permissionsConfig.form.transformBeforeSubmit(data)
          : data;
        await service.update(selectedPermission.id, payload);
        setShowEditModal(false);
        const display = `${selectedPermission.resource}: ${selectedPermission.action}`;
        toastSuccess(
          getDbOperationMessage(
            "update",
            permissionsConfig.name.singular,
            display,
          ),
        );
        await fetchPermissions();
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : String(err);
        if (
          errorMessage.includes("duplicate key") ||
          errorMessage.includes("23505")
        ) {
          setEditError(
            `Die Berechtigung "${data.resource}:${data.action}" existiert bereits. ` +
              `Jede Kombination aus Ressource und Aktion darf nur einmal vorhanden sein. ` +
              `Bitte wählen Sie eine andere Kombination.`,
          );
        } else {
          setEditError(
            "Fehler beim Aktualisieren der Berechtigung. Bitte versuchen Sie es erneut.",
          );
        }
      } finally {
        setSavingPermission(false);
      }
    },
    [selectedPermission, service, fetchPermissions, toastSuccess],
  );

  const handleDeletePermission = useCallback(async () => {
    if (!selectedPermission) return;
    const deleteError = await service.delete(selectedPermission.id);
    if (deleteError) {
      toastError(deleteError);
      return;
    }
    const display = `${selectedPermission.resource}: ${selectedPermission.action}`;
    toastSuccess(
      getDbOperationMessage("delete", permissionsConfig.name.singular, display),
    );
    handleSelectPermission(null);
    await fetchPermissions();
  }, [
    selectedPermission,
    service,
    toastError,
    toastSuccess,
    handleSelectPermission,
    fetchPermissions,
  ]);

  const canShowDetail = !loading && filteredPermissions.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Berechtigungen" : ""}
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
                  d="M5 13l4 4L19 7"
                />
              </svg>
            ),
            count: filteredPermissions.length,
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Berechtigungen suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
          actionButton={
            <DatabaseCreateAction
              label="Berechtigung"
              ariaLabel="Berechtigung erstellen"
              onClick={() => setShowCreateModal(true)}
            />
          }
        />
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <PermissionsMasterDetail
            permissions={filteredPermissions}
            selectedId={selectedId}
            onSelect={handleSelectPermission}
            onEditClick={handleEditClick}
            onDeleteClick={handleDeleteClick}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
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
                d="M15 7a2 2 0 012 2v1a2 2 0 11-4 0V9a2 2 0 012-2m-6 6h3l3 3 3-3 3 3-7 7-5-5v-2a2 2 0 012-2"
              />
            </svg>
          }
          title={
            searchTerm
              ? "Keine Berechtigungen gefunden"
              : "Keine Berechtigungen vorhanden"
          }
          description={
            searchTerm
              ? "Versuchen Sie einen anderen Suchbegriff."
              : "Es wurden noch keine Berechtigungen erstellt."
          }
        />
      ) : null}

      <PermissionCreateModal
        isOpen={showCreateModal}
        onClose={handleCloseCreateModal}
        onCreate={handleCreatePermission}
        loading={createLoading}
        error={createError}
      />

      {selectedPermission && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeletePermission())}
          title="Berechtigung löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie die Berechtigung{" "}
            <span className="font-medium">
              {selectedPermission.resource}: {selectedPermission.action}
            </span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}

      {selectedPermission && (
        <PermissionEditModal
          isOpen={showEditModal}
          onClose={handleCloseEditModal}
          permission={selectedPermission}
          onSave={handleUpdatePermission}
          loading={savingPermission}
          error={editError}
        />
      )}
    </DatabasePageLayout>
  );
}
