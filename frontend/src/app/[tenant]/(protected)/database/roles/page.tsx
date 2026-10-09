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
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { createCrudService } from "@/lib/database/service-factory";
import { rolesConfig } from "@/components/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import { getRoleDisplayName } from "@/lib/auth-helpers";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { RolesMasterDetail } from "@/components/roles/roles-master-detail";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
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

  const fetchRoles = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setRoles(arr);
      clearRolesLoadError();
    } catch (err) {
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
  }, [service, clearRolesLoadError, showRolesLoadError]);

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
    await fetchRoles();
  }, [fetchRoles, selectedRole]);

  const handleCreateRole = useCallback(
    async (data: Partial<Role>) => {
      // Ein Fehler bleibt im Dialog: DatabaseForm zeigt ihn über errorPath,
      // ein vergebener Name (identity.role_name_taken) markiert das Feld.
      const created = await service.create(data);
      toastSuccess(
        `Die Rolle „${getRoleDisplayName(created.name)}“ ist angelegt.`,
      );
      setShowCreateModal(false);
      await fetchRoles();
    },
    [service, fetchRoles, toastSuccess],
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
      await fetchRoles();
    },
    [selectedRole, service, fetchRoles, toastSuccess],
  );

  const handleDeleteRole = useCallback(async () => {
    if (!selectedRole) return;
    setDeletePending(true);
    deleteErrors.clear();
    try {
      const deleted = await service.remove(selectedRole.id);
      if (!deleted) return;
      toastSuccess(
        `Die Rolle „${getRoleDisplayName(selectedRole.name)}“ ist gelöscht.`,
      );
      handleDeleteCancel();
      setSelectedRoleDetail(null);
      handleSelectRole(null);
      await fetchRoles();
    } catch (err) {
      // Der Bestätigungsdialog bleibt offen und nennt den Grund.
      void deleteErrors.show(err, {
        object: "das Löschen der Rolle",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeletePending(false);
    }
  }, [
    selectedRole,
    service,
    toastSuccess,
    handleDeleteCancel,
    handleSelectRole,
    fetchRoles,
    deleteErrors,
  ]);

  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDeleteRole();
  });

  const canShowDetail = !loading && filteredRoles.length > 0;

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      error={rolesLoad.error}
      empty={
        filteredRoles.length === 0
          ? {
              title: searchTerm
                ? "Keine Rollen gefunden"
                : "Keine Rollen vorhanden",
              description: searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Legen Sie die erste Rolle an, um Rechte zu vergeben.",
              icon: (
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.roles.icon}
                  tone={MOTO_CONCEPTS.roles.tone}
                  size={48}
                />
              ),
              action: searchTerm ? undefined : (
                <DatabaseCreateAction
                  label="Rolle"
                  ariaLabel="Rolle erstellen"
                  showMobileFab={false}
                  onClick={() => setShowCreateModal(true)}
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
          embedded
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
