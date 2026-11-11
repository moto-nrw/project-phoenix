"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { formatCount } from "~/lib/format-utils";
import { Alert } from "~/components/ui/alert";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { createCrudService } from "@/lib/database/service-factory";
import { rolesConfig } from "@/components/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import { getRoleDisplayName } from "@/lib/auth-helpers";
import { isHiddenSystemRole } from "@/lib/role-labels";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { RolesMasterDetail } from "@/components/roles/roles-master-detail";
import { DatabaseFormModal } from "~/components/database/database-form-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { rolesConcept } from "~/lib/concepts/roles";

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
  const [accessRefused, setAccessRefused] = useState(false);
  const latestRoleRequest = useRef(0);
  const removedRoleIds = useRef(new Set<string>());
  const clearSelectionRef = useRef<() => void>(() => undefined);
  // Ladefehler im Gerüst, Schreibfehler im jeweiligen Dialog (#2517).
  const rolesLoad = useApiLoadError();
  const { show: showRolesLoadError, clear: clearRolesLoadError } = rolesLoad;
  const detailLoad = useApiLoadError();
  const { show: showDetailLoadError, clear: clearDetailLoadError } = detailLoad;
  const [detailReload, setDetailReload] = useState(0);
  const createErrors = useApiFormError();
  const deleteErrors = useApiFormError();
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const { clear: clearCreateErrors } = createErrors;
  // Ein neu geöffnetes Anlegen beginnt ohne den Fehler des vorigen Versuchs.
  useEffect(() => {
    if (showCreateModal) clearCreateErrors();
  }, [showCreateModal, clearCreateErrors]);
  const [deletePending, setDeletePending] = useState(false);
  const [selectedRoleDetail, setSelectedRoleDetail] = useState<Role | null>(
    null,
  );
  const [detailLoading, setDetailLoading] = useState(false);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
  } = useDeleteConfirmation();

  const { success: toastSuccess } = useToast();

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const canManagePermissions =
    (isAdmin(session) || hasPermission(session, "roles:manage")) &&
    (isAdmin(session) || hasPermission(session, "roles:read")) &&
    (isAdmin(session) || hasPermission(session, "permissions:read"));

  const service = useMemo(() => createCrudService(rolesConfig), []);

  // Rejects on failure: after a write the caller shows it as the reload of a
  // saved change and keeps the confirmed list (#6652).
  const readRoles = useCallback(async () => {
    const request = ++latestRoleRequest.current;
    try {
      const data = await service.getList({ page: 1, pageSize: 500 });
      if (request !== latestRoleRequest.current) return;
      const arr = Array.isArray(data.data) ? data.data : [];
      // Eltern vergibt die Einladung zur Eltern-App, nicht diese Seite (#4574).
      setRoles(
        arr.filter(
          (role) =>
            !isHiddenSystemRole(role) && !removedRoleIds.current.has(role.id),
        ),
      );
      setAccessRefused(false);
      setLoading(false);
      clearRolesLoadError();
    } catch (error) {
      if (request !== latestRoleRequest.current) return;
      if (
        error instanceof ApiError &&
        (error.status === 401 || error.status === 403)
      ) {
        setAccessRefused(true);
        setRoles([]);
        setSelectedRoleDetail(null);
        setShowCreateModal(false);
        clearSelectionRef.current();
        setLoading(false);
      }
      throw error;
    }
  }, [service, clearRolesLoadError]);

  const fetchRoles = useCallback(async () => {
    const request = latestRoleRequest.current + 1;
    try {
      setLoading(true);
      await readRoles();
    } catch (err) {
      if (request !== latestRoleRequest.current) return;
      logger.error("failed to fetch roles", {
        error: err instanceof Error ? err.message : String(err),
      });
      setRoles([]);
      // Bis der Katalogtext da ist, bleibt das Skelett stehen.
      await showRolesLoadError(err, {
        object: "die Liste der Rollen",
        retry: () => void fetchRoles(),
      });
    } finally {
      setLoading(false);
    }
  }, [readRoles, showRolesLoadError]);

  useEffect(() => {
    void fetchRoles();
  }, [fetchRoles]);
  const rolesAfterWrite = useRefreshAfterWrite(
    "die Anzeige der Rollen",
    readRoles,
  );
  const refreshRoles = rolesAfterWrite.refresh;

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
  const selectedRoleId = selectedRoleSummary?.id;

  const selectedRole =
    selectedRoleDetail?.id === selectedRoleId
      ? selectedRoleDetail
      : selectedRoleSummary;

  const handleSelectRole = useCallback(
    (id: string | null) => {
      updateUrlParams({ role: id });
    },
    [updateUrlParams],
  );

  useLayoutEffect(() => {
    clearSelectionRef.current = () => {
      handleDeleteCancel();
      handleSelectRole(null);
    };
  });

  useEffect(() => {
    if (!selectedRoleId) {
      setSelectedRoleDetail(null);
      setDetailLoading(false);
      return;
    }

    let cancelled = false;
    setDetailLoading(true);
    clearDetailLoadError();

    void service
      .getOne(selectedRoleId)
      .then((fresh) => {
        if (!cancelled) {
          setSelectedRoleDetail(fresh);
        }
      })
      .catch((fetchError: unknown) => {
        logger.error("failed to fetch role detail", {
          role_id: selectedRoleId,
          error:
            fetchError instanceof Error
              ? fetchError.message
              : String(fetchError),
        });
        if (cancelled) return;
        void showDetailLoadError(fetchError, {
          object: "die Rolle",
          retry: () => setDetailReload((value) => value + 1),
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
  }, [
    selectedRoleId,
    service,
    detailReload,
    clearDetailLoadError,
    showDetailLoadError,
  ]);

  // Nach dem Speichern der Berechtigungen im Reiter (#3116): die Zahl in der
  // Liste und das Detail neu laden. Ein Fehler dabei zeigen Liste und Detail
  // selbst an; das Speichern ist da bereits gelungen.
  const handlePermissionsSaved = useCallback(async () => {
    if (!selectedRole) return;
    setDetailReload((value) => value + 1);
    await refreshRoles();
  }, [refreshRoles, selectedRole]);

  const handleCreateRole = useCallback(
    async (data: Partial<Role>) => {
      // Ein Fehler bleibt im Dialog: DatabaseForm zeigt ihn über errorPath,
      // ein vergebener Name (identity.role_name_taken) markiert das Feld.
      const created = await service.create(data);
      toastSuccess(
        `Die Rolle „${getRoleDisplayName(created.name)}“ ist angelegt.`,
      );
      setShowCreateModal(false);
      await refreshRoles();
    },
    [service, refreshRoles, toastSuccess],
  );

  const handleUpdateRole = useCallback(
    async (data: Partial<Role>) => {
      if (!selectedRole) return;
      // Ein Fehler bleibt im Formular (errorPath im Stammdaten-Reiter).
      await service.update(selectedRole.id, data);
      toastSuccess(
        `Die Rolle „${getRoleDisplayName(data.name ?? selectedRole.name)}“ ist gespeichert.`,
      );
      // Liste und Detail neu laden.
      setDetailReload((value) => value + 1);
      await refreshRoles();
    },
    [selectedRole, service, refreshRoles, toastSuccess],
  );

  const handleDeleteRole = useCallback(async () => {
    if (!selectedRole) return;
    setDeletePending(true);
    deleteErrors.clear();
    let deleted: boolean;
    try {
      deleted = await service.remove(selectedRole.id);
    } catch (err) {
      // Der Bestätigungsdialog bleibt offen und nennt den Grund.
      void deleteErrors.show(err, {
        object: "das Löschen der Rolle",
        retry: () => latestDeleteRef.current(),
      });
      return;
    } finally {
      setDeletePending(false);
    }
    if (!deleted) return;
    removedRoleIds.current.add(selectedRole.id);
    setRoles((current) =>
      current.filter((role) => role.id !== selectedRole.id),
    );
    toastSuccess(
      `Die Rolle „${getRoleDisplayName(selectedRole.name)}“ ist gelöscht.`,
    );
    handleDeleteCancel();
    setSelectedRoleDetail(null);
    handleSelectRole(null);
    await refreshRoles();
  }, [
    selectedRole,
    service,
    toastSuccess,
    handleDeleteCancel,
    handleSelectRole,
    refreshRoles,
    deleteErrors,
  ]);

  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDeleteRole();
  });

  const canShowDetail = !loading && filteredRoles.length > 0;

  return (
    <DatabasePageLayout
      loading={
        loading || (accessRefused && !rolesLoad.error && !rolesAfterWrite.error)
      }
      sessionLoading={status === "loading"}
      error={rolesLoad.error ?? (accessRefused ? rolesAfterWrite.error : null)}
      empty={
        filteredRoles.length === 0 &&
        !accessRefused &&
        !rolesLoad.error &&
        !rolesAfterWrite.error
          ? {
              title: searchTerm
                ? "Keine Rollen gefunden"
                : "Keine Rollen vorhanden",
              description: searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Legen Sie die erste Rolle an, um Rechte zu vergeben.",
              icon: (
                <MotoDuotoneIcon
                  icon={rolesConcept.icon}
                  tone={rolesConcept.tone}
                  size={48}
                />
              ),
            }
          : null
      }
      overlays={
        <>
          <DatabaseFormModal<Role>
            isOpen={showCreateModal}
            onClose={() => setShowCreateModal(false)}
            mode="create"
            config={rolesConfig}
            onSubmit={handleCreateRole}
            errorPath={createErrors}
            errorObject="die Rolle"
          />

          {selectedRole && (
            <ConfirmDeleteModal
              isOpen={showDeleteConfirmModal}
              onClose={handleDeleteCancel}
              onConfirm={() => void handleDeleteRole()}
              title="Rolle löschen?"
              description={
                <>
                  Möchten Sie die Rolle{" "}
                  <span className="font-medium">
                    {getRoleDisplayName(selectedRole.name)}
                  </span>{" "}
                  wirklich löschen? Alle Personen mit dieser Rolle verlieren
                  ihre Berechtigungen.
                </>
              }
              gate={{ mode: "twoStep" }}
              loading={deletePending}
              error={deleteErrors.error}
            />
          )}
        </>
      }
      className="flex w-full flex-col"
      intro={{
        title: "Rollen",
        description: statusLine,
        actions: accessRefused ? null : (
          <DatabaseCreateAction
            label="Rolle"
            ariaLabel="Rolle erstellen"
            onClick={() => setShowCreateModal(true)}
          />
        ),
      }}
      searchRow={{
        badge: {
          icon: (
            <MotoDuotoneIcon
              icon={rolesConcept.icon}
              tone={rolesConcept.tone}
              size={20}
            />
          ),
          count: filteredRoles.length,
          label: "Rollen",
        },
        search: {
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Rollen suchen…",
        },
        filters,
        activeFilters,
        onClearAllFilters: () => {
          setSearchTerm("");
        },
      }}
    >
      <LoadErrorAlert error={rolesAfterWrite.error} className="mb-4" />

      {unclassifiedCount > 0 && (
        <div className="mb-6">
          <Alert
            type="warning"
            title={
              unclassifiedCount === 1
                ? "Bei 1 Rolle fehlt „Gilt als“"
                : `Bei ${unclassifiedCount} Rollen fehlt „Gilt als“`
            }
            message="Mitteilungen kommen dort vielleicht nicht an. Öffnen Sie die Rolle, tippen Sie auf „Bearbeiten“ und wählen Sie bei „Gilt als“ einen Eintrag."
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
            detailError={detailLoad.error}
            canManagePermissions={canManagePermissions}
            onSelect={handleSelectRole}
            onSaveRole={handleUpdateRole}
            onDeleteClick={handleDeleteClick}
            onPermissionsSaved={handlePermissionsSaved}
          />
        </div>
      ) : null}
    </DatabasePageLayout>
  );
}
