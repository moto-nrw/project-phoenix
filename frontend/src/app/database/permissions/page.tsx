"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "~/lib/auth-client";
import { redirect } from "next/navigation";
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
  PermissionDetailModal,
  PermissionEditModal,
} from "@/components/permissions";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  formatPermissionDisplay,
  localizeAction,
  localizeResource,
} from "@/lib/permission-labels";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";

export default function PermissionsPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  // Modals
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [selectedPermission, setSelectedPermission] =
    useState<Permission | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  // Delete confirmation modal management
  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation(setShowDetailModal);

  const { success: toastSuccess } = useToast();

  // BetterAuth: cookies handle auth, isPending replaces status
  const { data: session, isPending } = useSession();

  // Redirect if not authenticated
  if (!isPending && !session?.user) {
    redirect("/");
  }

  const service = useMemo(() => createCrudService(permissionsConfig), []);

  const fetchPermissions = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setPermissions(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching permissions:", err);
      setError(
        "Fehler beim Laden der Berechtigungen. Bitte versuchen Sie es später erneut.",
      );
      setPermissions([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    fetchPermissions().catch(() => {
      // Error already handled in fetchPermissions
    });
  }, [fetchPermissions]);

  const toDisplay = (p: Permission) =>
    formatPermissionDisplay(p.resource, p.action);

  // Anzeigename + Beschreibung exakt wie in den Daten anzeigen; bei fehlendem Anzeigenamen auf Ressource:Aktion ausweichen
  const displayTitle = (p: Permission) =>
    p.name?.trim() ? p.name : toDisplay(p);

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
      arr = arr.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          (p.description?.toLowerCase().includes(q) ?? false) ||
          p.resource.toLowerCase().includes(q) ||
          p.action.toLowerCase().includes(q),
      );
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

  const handleSelectPermission = async (perm: Permission) => {
    setSelectedPermission(perm);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(perm.id);
      setSelectedPermission(fresh);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleCreatePermission = async (data: Partial<Permission>) => {
    try {
      setCreateLoading(true);
      setCreateError(null);
      // Apply transform before submit to extract resource/action from permissionSelector
      if (permissionsConfig.form.transformBeforeSubmit) {
        data = permissionsConfig.form.transformBeforeSubmit(data);
      }
      const created = await service.create(data);
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
      // Check for duplicate key error and show in modal
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
  };

  const handleUpdatePermission = async (data: Partial<Permission>) => {
    if (!selectedPermission) return;
    try {
      setDetailLoading(true);
      setEditError(null);
      // Apply transform before submit to extract resource/action from permissionSelector
      if (permissionsConfig.form.transformBeforeSubmit) {
        data = permissionsConfig.form.transformBeforeSubmit(data);
      }
      await service.update(selectedPermission.id, data);
      const display = `${selectedPermission.resource}: ${selectedPermission.action}`;
      toastSuccess(
        getDbOperationMessage(
          "update",
          permissionsConfig.name.singular,
          display,
        ),
      );
      const refreshed = await service.getOne(selectedPermission.id);
      setSelectedPermission(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
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
      setDetailLoading(false);
    }
  };

  const handleDeletePermission = async () => {
    if (!selectedPermission) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedPermission.id);
      const display = `${selectedPermission.resource}: ${selectedPermission.action}`;
      toastSuccess(
        getDbOperationMessage(
          "delete",
          permissionsConfig.name.singular,
          display,
        ),
      );
      setShowDetailModal(false);
      setSelectedPermission(null);
      await fetchPermissions();
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
            !isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-pink-500 to-rose-600 text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                style={{
                  background:
                    "linear-gradient(135deg, rgb(236, 72, 153) 0%, rgb(225, 29, 72) 100%)",
                  willChange: "transform, opacity",
                  WebkitTransform: "translateZ(0)",
                  transform: "translateZ(0)",
                }}
                aria-label="Berechtigung erstellen"
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

      <button
        onClick={() => setShowCreateModal(true)}
        className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-pink-500 to-rose-600 text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgba(244,114,182,0.3)] active:scale-95 md:hidden"
        style={{
          background:
            "linear-gradient(135deg, rgb(236, 72, 153) 0%, rgb(225, 29, 72) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Berechtigung erstellen"
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

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {filteredPermissions.length === 0 ? (
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
                d="M15 7a2 2 0 012 2v1a2 2 0 11-4 0V9a2 2 0 012-2m-6 6h3l3 3 3-3 3 3-7 7-5-5v-2a2 2 0 012-2"
              />
            </svg>
            <h3 className="mt-4 text-lg font-medium text-gray-900">
              {searchTerm
                ? "Keine Berechtigungen gefunden"
                : "Keine Berechtigungen vorhanden"}
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              {searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Es wurden noch keine Berechtigungen erstellt."}
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredPermissions.map((perm, index) => {
            const handleClick = () => void handleSelectPermission(perm);
            return (
              <button
                type="button"
                key={perm.id}
                onClick={handleClick}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-indigo-300/60 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                style={{
                  animationName: "fadeInUp",
                  animationDuration: "0.5s",
                  animationTimingFunction: "ease-out",
                  animationFillMode: "forwards",
                  animationDelay: `${index * 0.03}s`,
                  opacity: 0,
                }}
              >
                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-pink-50/80 to-rose-100/80 opacity-[0.03]"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-pink-300/60"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-pink-500 to-rose-600 font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                      {perm.resource?.charAt(0)?.toUpperCase() ?? "P"}
                    </div>
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="truncate text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-pink-600">
                      {displayTitle(perm)}
                    </h3>
                    {perm.description && (
                      <p className="mt-0.5 line-clamp-1 text-sm text-gray-600">
                        {perm.description}
                      </p>
                    )}
                    <div className="mt-1 flex flex-wrap items-center gap-2">
                      <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700">
                        {localizeResource(perm.resource)}
                      </span>
                      <span className="inline-flex items-center rounded-full bg-pink-100 px-2 py-1 text-xs font-medium text-pink-700">
                        {localizeAction(perm.action)}
                      </span>
                    </div>
                  </div>
                  <div className="flex-shrink-0">
                    <svg
                      className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-indigo-600"
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

                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-pink-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
              </button>
            );
          })}
        </div>
      )}

      {/* Create */}
      <PermissionCreateModal
        isOpen={showCreateModal}
        onClose={() => {
          setShowCreateModal(false);
          setCreateError(null);
        }}
        onCreate={handleCreatePermission}
        loading={createLoading}
        error={createError}
      />

      {/* Detail */}
      {selectedPermission && (
        <PermissionDetailModal
          isOpen={showDetailModal}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedPermission(null);
          }}
          permission={selectedPermission}
          onEdit={handleEditClick}
          onDelete={() => void handleDeletePermission()}
          loading={detailLoading}
          onDeleteClick={handleDeleteClick}
        />
      )}

      {/* Delete Confirmation */}
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

      {/* Edit */}
      {selectedPermission && (
        <PermissionEditModal
          isOpen={showEditModal}
          onClose={() => {
            setShowEditModal(false);
            setEditError(null);
          }}
          permission={selectedPermission}
          onSave={handleUpdatePermission}
          loading={detailLoading}
          error={editError}
        />
      )}

      {/* Success toasts handled globally */}
    </DatabasePageLayout>
  );
}
