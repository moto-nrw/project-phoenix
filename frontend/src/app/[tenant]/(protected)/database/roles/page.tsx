"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { formatCount } from "~/lib/format-utils";
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
import { rolesConfig } from "@/components/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import { getRoleDisplayName } from "@/lib/auth-helpers";
import { RolesMasterDetail } from "@/components/roles/roles-master-detail";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { RolePermissionManagementModal } from "@/components/auth/role-permission-management-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DatabaseRolesPage" });

export default function RolesPage() {
  return (
    <Suspense fallback={null}>
      <RolesPageContent />
    </Suspense>
  );
}

function RolesPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  // The query value only selects an already-loaded row; role mutations still
  // require authenticated backend authorization and explicit confirmation.
  const selectedId = searchParams.get("role");
  const [searchTerm, setSearchTerm] = useState("");

  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showPermissionModal, setShowPermissionModal] = useState(false);
  const [selectedRoleDetail, setSelectedRoleDetail] = useState<Role | null>(
    null,
  );
  const [detailLoading, setDetailLoading] = useState(false);

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

  const service = useMemo(() => createCrudService(rolesConfig), []);

  const fetchRoles = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setRoles(arr);
      setError(null);
    } catch (err) {
      logger.error("failed to fetch roles", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Fehler beim Laden der Rollen. Bitte versuchen Sie es später erneut.",
      );
      setRoles([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    void fetchRoles();
  }, [fetchRoles]);

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

  // Statuszeile des Seitenkopfs aus der bereits geladenen Rollenliste.
  const statusLine = useMemo(() => {
    const systemRoles = roles.filter((r) => r.isSystem).length;
    const parts = [
      `${formatCount(roles.length)} ${roles.length === 1 ? "Rolle" : "Rollen"}`,
    ];
    if (systemRoles > 0) {
      parts.push(`${formatCount(systemRoles)} vom System`);
    }
    return parts.join(" · ");
  }, [roles]);

  const unclassifiedCount = useMemo(
    () => roles.filter((r) => !r.isSystem && !r.baseRole).length,
    [roles],
  );

  const filteredRoles = useMemo(() => {
    let arr = [...roles];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (r) =>
          r.name.toLowerCase().includes(q) ||
          (r.description?.toLowerCase().includes(q) ?? false),
      );
    }
    arr.sort((a, b) => a.name.localeCompare(b.name, "de"));
    return arr;
  }, [roles, searchTerm]);

  const selectedRoleSummary = useMemo(
    () =>
      selectedId
        ? (filteredRoles.find((role) => role.id === selectedId) ?? null)
        : null,
    [filteredRoles, selectedId],
  );

  const selectedRole =
    selectedRoleDetail?.id === selectedRoleSummary?.id
      ? selectedRoleDetail
      : selectedRoleSummary;

  const handleSelectRole = useCallback(
    (id: string | null) => {
      updateUrlParams({ role: id });
    },
    [updateUrlParams],
  );

  useEffect(() => {
    if (!selectedId || !selectedRoleSummary) {
      setSelectedRoleDetail(null);
      setDetailLoading(false);
      return;
    }

    setSelectedRoleDetail((current) =>
      current?.id === selectedRoleSummary.id ? current : selectedRoleSummary,
    );

    let cancelled = false;
    setDetailLoading(true);

    void service
      .getOne(selectedRoleSummary.id)
      .then((fresh) => {
        if (!cancelled) {
          setSelectedRoleDetail(fresh);
        }
      })
      .catch((fetchError: unknown) => {
        logger.error("failed to fetch role detail", {
          role_id: selectedRoleSummary.id,
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
  }, [selectedId, selectedRoleSummary, service]);

  const handleEditClick = useCallback(() => setShowEditModal(true), []);
  const handleCloseEditModal = useCallback(() => setShowEditModal(false), []);
  const handleManagePermissions = useCallback(
    () => setShowPermissionModal(true),
    [],
  );

  const handleCreateRole = useCallback(
    async (data: Partial<Role>) => {
      try {
        const created = await service.create(data);
        toastSuccess(
          getDbOperationMessage(
            "create",
            rolesConfig.name.singular,
            created.name,
          ),
        );
        setShowCreateModal(false);
        await fetchRoles();
      } catch (createError) {
        const errorMessage =
          createError instanceof Error
            ? createError.message
            : String(createError);
        logger.error("role_create_failed", { error: errorMessage });
        if (
          errorMessage.includes("duplicate key") ||
          errorMessage.includes("23505")
        ) {
          throw new Error(
            `Eine Rolle mit dem Namen "${data.name ?? ""}" existiert bereits. ` +
              `Bitte wählen Sie einen anderen Namen.`,
            { cause: createError },
          );
        }
        throw createError;
      }
    },
    [service, fetchRoles, toastSuccess],
  );

  const handleUpdateRole = useCallback(
    async (data: Partial<Role>) => {
      if (!selectedRole) return;
      try {
        await service.update(selectedRole.id, data);
        const refreshed = await service.getOne(selectedRole.id);
        setSelectedRoleDetail(refreshed);
        setShowEditModal(false);
        toastSuccess(
          getDbOperationMessage(
            "update",
            rolesConfig.name.singular,
            getRoleDisplayName(selectedRole.name),
          ),
        );
        await fetchRoles();
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : String(err);
        logger.error("role_update_failed", {
          role_id: selectedRole.id,
          error: errorMessage,
        });
        if (
          errorMessage.includes("duplicate key") ||
          errorMessage.includes("23505")
        ) {
          throw new Error(
            `Eine Rolle mit dem Namen "${data.name ?? ""}" existiert bereits. ` +
              `Bitte wählen Sie einen anderen Namen.`,
            { cause: err },
          );
        }
        throw err;
      }
    },
    [selectedRole, service, fetchRoles, toastSuccess],
  );

  const handleDeleteRole = useCallback(async () => {
    if (!selectedRole) return;
    try {
      setDetailLoading(true);
      const deleteError = await service.delete(selectedRole.id);
      if (deleteError) {
        toastError(deleteError);
        return;
      }
      toastSuccess(
        getDbOperationMessage(
          "delete",
          rolesConfig.name.singular,
          getRoleDisplayName(selectedRole.name),
        ),
      );
      setSelectedRoleDetail(null);
      handleSelectRole(null);
      await fetchRoles();
    } finally {
      setDetailLoading(false);
    }
  }, [
    selectedRole,
    service,
    toastError,
    toastSuccess,
    handleSelectRole,
    fetchRoles,
  ]);

  const canShowDetail = !loading && filteredRoles.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="flex w-full flex-col"
      intro={{
        kicker: "Datenverwaltung",
        title: "Rollen",
        description: loading ? <Skeleton className="h-4 w-44" /> : statusLine,
        actions: (
          <DatabaseCreateAction
            label="Rolle"
            ariaLabel="Rolle erstellen"
            onClick={() => setShowCreateModal(true)}
          />
        ),
      }}
      search={
        <PageHeaderWithSearch
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.roles.icon}
                tone={MOTO_CONCEPTS.roles.tone}
                size={20}
              />
            ),
            count: filteredRoles.length,
            label: "Rollen",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Rollen suchen…",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
        />
      }
    >
      {error && (
        <div className="mb-6">
          <Alert type="error" message={error} />
        </div>
      )}

      {unclassifiedCount > 0 && (
        <div className="mb-6">
          <Alert
            type="warning"
            title={
              unclassifiedCount === 1
                ? "1 Rolle hat keine Systemrollen-Zuordnung"
                : `${unclassifiedCount} Rollen haben keine Systemrollen-Zuordnung`
            }
            message="Ankündigungen werden möglicherweise nicht korrekt zugestellt. Bitte bearbeiten Sie die betroffenen Rollen und wählen Sie eine Systemrolle aus."
          />
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <RolesMasterDetail
            roles={filteredRoles}
            selectedId={selectedId}
            selectedRole={selectedRole}
            detailLoading={detailLoading}
            onSelect={handleSelectRole}
            onEditClick={handleEditClick}
            onDeleteClick={handleDeleteClick}
            onManagePermissions={handleManagePermissions}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.roles.icon}
              tone={MOTO_CONCEPTS.roles.tone}
              size={48}
              className="mx-auto"
            />
          }
          title={
            searchTerm ? "Keine Rollen gefunden" : "Keine Rollen vorhanden"
          }
          description={
            searchTerm
              ? "Versuchen Sie einen anderen Suchbegriff."
              : "Es wurden noch keine Rollen erstellt."
          }
        />
      ) : null}

      <DatabaseFormModal<Role>
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        mode="create"
        config={rolesConfig}
        onSubmit={handleCreateRole}
      />

      {selectedRole && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteRole())}
          title="Rolle löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie die Rolle{" "}
            <span className="font-medium">
              {getRoleDisplayName(selectedRole.name)}
            </span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}

      {selectedRole && (
        <DatabaseFormModal<Role>
          isOpen={showEditModal}
          onClose={handleCloseEditModal}
          mode="edit"
          config={rolesConfig}
          initialData={selectedRole}
          onSubmit={handleUpdateRole}
        />
      )}

      {selectedRole && (
        <RolePermissionManagementModal
          isOpen={showPermissionModal}
          onClose={() => setShowPermissionModal(false)}
          role={selectedRole}
          onUpdate={async () => {
            await fetchRoles();
            const refreshed = await service.getOne(selectedRole.id);
            setSelectedRoleDetail(refreshed);
          }}
        />
      )}
    </DatabasePageLayout>
  );
}
