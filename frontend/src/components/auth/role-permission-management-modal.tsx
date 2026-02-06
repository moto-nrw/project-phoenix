"use client";

import { useState, useEffect, useMemo } from "react";
import { FormModal } from "~/components/ui";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { useToast } from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import {
  localizeAction,
  localizeResource,
  formatPermissionDisplay,
} from "~/lib/permission-labels";
import type { Role, Permission } from "~/lib/auth-helpers";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RolePermissionManagementModal" });

interface RolePermissionManagementModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly role: Role;
  readonly onUpdate: () => void;
}

export function RolePermissionManagementModal({
  isOpen,
  onClose,
  role,
  onUpdate,
}: RolePermissionManagementModalProps) {
  const getPermissionDisplayName = (p: Permission) =>
    formatPermissionDisplay(p.resource, p.action);
  const { success: toastSuccess } = useToast();
  const [showErrorAlert, setShowErrorAlert] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  // Warning alert disabled for now to reduce noise in UI
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [rolePermissions, setRolePermissions] = useState<Permission[]>([]);
  const [assignedMap, setAssignedMap] = useState<Record<string, boolean>>({});
  const [initialAssignedMap, setInitialAssignedMap] = useState<
    Record<string, boolean>
  >({});
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const showSuccess = (message: string) => {
    toastSuccess(message);
  };

  const showError = (message: string) => {
    setErrorMessage(message);
    setShowErrorAlert(true);
  };

  // showWarning helper currently unused

  // Fetch all permissions and role permissions
  const fetchPermissions = async () => {
    try {
      setLoading(true);

      // Fetch all permissions
      const [allPerms, rolePerms] = await Promise.all([
        authService.getPermissions(),
        authService.getRolePermissions(role.id),
      ]);

      logger.debug("permissions loaded", {
        all_count: allPerms.length,
        role_count: rolePerms.length,
      });

      setAllPermissions(allPerms);
      setRolePermissions(rolePerms);
      const map: Record<string, boolean> = {};
      rolePerms.forEach((p) => {
        map[p.id] = true;
      });
      setAssignedMap(map);
      setInitialAssignedMap(map);
    } catch (error) {
      logger.error("failed to fetch permissions", {
        error: error instanceof Error ? error.message : String(error),
      });
      showError("Fehler beim Laden der Berechtigungen");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      void fetchPermissions();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, role.id]);

  // Filter permissions based on search term
  const filteredPermissions = allPermissions.filter((permission) => {
    const searchLower = searchTerm.toLowerCase();
    return (
      permission.name.toLowerCase().includes(searchLower) ||
      permission.description.toLowerCase().includes(searchLower) ||
      permission.resource.toLowerCase().includes(searchLower) ||
      permission.action.toLowerCase().includes(searchLower)
    );
  });

  const handleTogglePermission = (permissionId: string) => {
    setAssignedMap((prev) => ({
      ...prev,
      [permissionId]: !prev[permissionId],
    }));
  };

  const hasChanges = useMemo(() => {
    const keys = new Set([
      ...Object.keys(initialAssignedMap),
      ...Object.keys(assignedMap),
    ]);
    for (const k of keys) {
      if ((initialAssignedMap[k] ?? false) !== (assignedMap[k] ?? false))
        return true;
    }
    return false;
  }, [initialAssignedMap, assignedMap]);

  const handleSaveChanges = async () => {
    try {
      setSaving(true);
      const keys = new Set([
        ...Object.keys(initialAssignedMap),
        ...Object.keys(assignedMap),
      ]);
      const toAssign: string[] = [];
      const toRemove: string[] = [];
      for (const k of keys) {
        const before = initialAssignedMap[k] ?? false;
        const after = assignedMap[k] ?? false;
        if (!before && after) toAssign.push(k);
        if (before && !after) toRemove.push(k);
      }

      await Promise.all([
        ...toAssign.map((id) =>
          authService.assignPermissionToRole(role.id, id),
        ),
        ...toRemove.map((id) =>
          authService.removePermissionFromRole(role.id, id),
        ),
      ]);

      showSuccess("Berechtigungen aktualisiert");
      await fetchPermissions();
      onUpdate();
      onClose();
    } catch (error) {
      logger.error("failed to update role permissions", {
        error: error instanceof Error ? error.message : String(error),
      });
      showError("Fehler beim Aktualisieren der Berechtigungen");
    } finally {
      setSaving(false);
    }
  };

  // Unused: handleRemovePermission kept for future quick actions in list

  const footer = (
    <div className="flex w-full gap-2 md:gap-3">
      <button
        type="button"
        onClick={onClose}
        disabled={saving}
        className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={handleSaveChanges}
        disabled={saving || !hasChanges}
        className="flex-1 rounded-lg bg-purple-600 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-purple-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
      >
        {saving ? "Wird gespeichert..." : "Speichern"}
      </button>
    </div>
  );

  return (
    <>
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Berechtigungen verwalten - ${getRoleDisplayName(role.name)}`}
        size="xl"
        mobilePosition="center"
        footer={footer}
      >
        <div className="space-y-4 md:space-y-6">
          {/* Stats */}
          <div className="rounded-xl border border-gray-100 bg-purple-50/30 p-3 md:p-4">
            <div className="flex items-center justify-between">
              <span className="text-xs text-gray-600 md:text-sm">
                Zugewiesene Berechtigungen
              </span>
              <span className="text-sm font-semibold text-gray-900 md:text-base">
                {rolePermissions.length}
              </span>
            </div>
          </div>

          {/* Hinweiszeile */}
          <div className="flex items-center justify-between text-xs text-gray-600 md:text-sm">
            <span>
              Aktiviere oder deaktiviere Berechtigungen und speichere die
              Änderungen.
            </span>
            <span className="hidden text-gray-500 md:inline">
              {rolePermissions.length} zugewiesen
            </span>
          </div>

          {/* Search */}
          <input
            type="text"
            placeholder="Berechtigungen suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-2 focus:ring-purple-500 focus:outline-none md:px-4 md:py-2"
          />

          {/* Content */}
          {loading ? (
            <div className="py-8 text-center text-gray-500">Laden...</div>
          ) : (
            <div className="max-h-96 space-y-1.5 overflow-y-auto rounded-xl border border-gray-100 bg-white">
              {filteredPermissions.length === 0 ? (
                <p className="py-8 text-center text-gray-500">
                  Keine Berechtigungen gefunden
                </p>
              ) : (
                filteredPermissions.map((permission) => {
                  const checked = !!assignedMap[permission.id];
                  return (
                    <div
                      key={permission.id}
                      className="flex items-center justify-between p-3 hover:bg-gray-50 md:p-3.5"
                    >
                      <div className="min-w-0 flex-1 pr-3">
                        <div className="text-sm font-medium text-gray-900">
                          {getPermissionDisplayName(permission)}
                        </div>
                        <div className="mt-1 text-[11px] text-gray-500 md:text-xs">
                          Ressource: {localizeResource(permission.resource)} •
                          Aktion: {localizeAction(permission.action)}
                        </div>
                      </div>
                      <button
                        type="button"
                        role="switch"
                        aria-checked={checked}
                        onClick={() => handleTogglePermission(permission.id)}
                        className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-purple-500 focus:ring-offset-2 focus:outline-none ${checked ? "bg-purple-600" : "bg-gray-300"}`}
                      >
                        <span
                          className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${checked ? "translate-x-6" : "translate-x-1"}`}
                        />
                      </button>
                    </div>
                  );
                })
              )}
            </div>
          )}
        </div>
      </FormModal>

      {/* Success toasts handled globally */}
      {showErrorAlert && (
        <SimpleAlert
          type="error"
          message={errorMessage}
          onClose={() => setShowErrorAlert(false)}
        />
      )}
      {/* Warning alert intentionally disabled */}
    </>
  );
}
