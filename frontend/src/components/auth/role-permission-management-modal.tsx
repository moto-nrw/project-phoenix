"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { ChevronRight, Check, Minus } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { useToast } from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import {
  localizeAction,
  localizeResource,
  formatPermissionDisplay,
} from "~/lib/permission-labels";
import type { Role, Permission } from "~/lib/auth-helpers";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
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
  const [errorMessage, setErrorMessage] = useState("");
  const errorRef = useScrollToError(errorMessage);
  // Warning alert disabled for now to reduce noise in UI
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [assignedMap, setAssignedMap] = useState<Record<string, boolean>>({});
  const [initialAssignedMap, setInitialAssignedMap] = useState<
    Record<string, boolean>
  >({});
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [collapsedGroups, setCollapsedGroups] = useState<
    Record<string, boolean>
  >({});

  const showError = (message: string) => {
    setErrorMessage(message);
  };

  const fetchPermissions = async () => {
    try {
      setErrorMessage("");
      setLoading(true);

      const [allPerms, rolePerms] = await Promise.all([
        authService.getPermissions(),
        authService.getRolePermissions(role.id),
      ]);

      logger.debug("permissions loaded", {
        all_count: allPerms.length,
        role_count: rolePerms.length,
      });

      setAllPermissions(allPerms);
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

  const filteredPermissions = useMemo(() => {
    const searchLower = searchTerm.toLowerCase();
    return allPermissions.filter(
      (permission) =>
        permission.name.toLowerCase().includes(searchLower) ||
        permission.description.toLowerCase().includes(searchLower) ||
        permission.resource.toLowerCase().includes(searchLower) ||
        permission.action.toLowerCase().includes(searchLower) ||
        localizeResource(permission.resource)
          .toLowerCase()
          .includes(searchLower) ||
        localizeAction(permission.action).toLowerCase().includes(searchLower),
    );
  }, [allPermissions, searchTerm]);

  const groupedPermissions = useMemo(() => {
    const groups: Record<string, Permission[]> = {};
    for (const permission of filteredPermissions) {
      const resource = permission.resource;
      groups[resource] ??= [];
      groups[resource].push(permission);
    }
    return Object.entries(groups).sort(([a], [b]) =>
      localizeResource(a).localeCompare(localizeResource(b), "de"),
    );
  }, [filteredPermissions]);

  const handleTogglePermission = (permissionId: string) => {
    setAssignedMap((prev) => ({
      ...prev,
      [permissionId]: !prev[permissionId],
    }));
  };

  // Shared toggle: checks state inside the updater so the callback has no `assignedMap` dependency
  const handleTogglePermissions = useCallback((permissions: Permission[]) => {
    setAssignedMap((prev) => {
      const allChecked = permissions.every((p) => !!prev[p.id]);
      const next = { ...prev };
      for (const p of permissions) {
        next[p.id] = !allChecked;
      }
      return next;
    });
  }, []);

  const toggleGroupCollapsed = (resource: string) => {
    setCollapsedGroups((prev) => ({
      ...prev,
      [resource]: !prev[resource],
    }));
  };

  const totalAssignedCount = useMemo(
    () => allPermissions.filter((p) => !!assignedMap[p.id]).length,
    [allPermissions, assignedMap],
  );

  const allFilteredChecked = useMemo(
    () =>
      filteredPermissions.length > 0 &&
      filteredPermissions.every((p) => !!assignedMap[p.id]),
    [filteredPermissions, assignedMap],
  );

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
      setErrorMessage("");
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

      toastSuccess("Berechtigungen aktualisiert");
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
        className="bg-moto-purple hover:bg-moto-purple-strong flex-1 rounded-lg px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
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
          {errorMessage && (
            <div ref={errorRef}>
              <Alert type="error" message={errorMessage} />
            </div>
          )}

          {/* Stats */}
          <div className="bg-moto-purple-soft/30 rounded-xl border border-gray-100 p-3 md:p-4">
            <div className="flex items-center justify-between">
              <span className="text-xs text-gray-600 md:text-sm">
                Zugewiesene Berechtigungen
              </span>
              <span className="text-sm font-semibold text-gray-900 md:text-base">
                {totalAssignedCount} / {allPermissions.length}
              </span>
            </div>
          </div>

          {/* Search + Select All */}
          <div className="flex items-center gap-2">
            <input
              type="text"
              placeholder="Berechtigungen suchen..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              className="focus:ring-moto-purple min-w-0 flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-2 focus:outline-none md:px-4 md:py-2"
            />
            <button
              type="button"
              onClick={() => handleTogglePermissions(filteredPermissions)}
              disabled={filteredPermissions.length === 0}
              className="hover:border-moto-purple/40 hover:bg-moto-purple-soft shrink-0 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm"
            >
              {allFilteredChecked ? "Alle abwählen" : "Alle auswählen"}
            </button>
          </div>

          {/* Permission groups */}
          {loading ? (
            <div className="py-8 text-center text-gray-500">Laden...</div>
          ) : (
            <div className="max-h-96 overflow-y-auto rounded-xl border border-gray-100 bg-white">
              {groupedPermissions.length === 0 ? (
                <p className="py-8 text-center text-gray-500">
                  Keine Berechtigungen gefunden
                </p>
              ) : (
                groupedPermissions.map(([resource, permissions]) => {
                  const groupAssignedCount = permissions.filter(
                    (p) => !!assignedMap[p.id],
                  ).length;
                  const allGroupChecked =
                    groupAssignedCount === permissions.length;
                  const someGroupChecked =
                    !allGroupChecked && groupAssignedCount > 0;
                  const isCollapsed = !!collapsedGroups[resource];

                  return (
                    <div key={resource}>
                      {/* Resource group header */}
                      <div className="sticky top-0 z-10 flex items-center justify-between border-b border-gray-100 bg-gray-50 px-3 py-2 md:px-4">
                        <button
                          type="button"
                          onClick={() => toggleGroupCollapsed(resource)}
                          className="flex min-w-0 flex-1 items-center gap-2"
                        >
                          <ChevronRight
                            className={`h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform duration-200 ${isCollapsed ? "" : "rotate-90"}`}
                            strokeWidth={2.5}
                          />
                          <span className="text-xs font-semibold text-gray-700 md:text-sm">
                            {localizeResource(resource)}
                          </span>
                          <span className="text-[10px] text-gray-400 md:text-xs">
                            {groupAssignedCount}/{permissions.length}
                          </span>
                        </button>
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            handleTogglePermissions(permissions);
                          }}
                          className="hover:border-moto-purple/40 hover:bg-moto-purple-soft relative ml-2 flex h-5 w-5 shrink-0 items-center justify-center rounded border border-gray-300 bg-white transition-colors md:h-[18px] md:w-[18px]"
                          title={
                            allGroupChecked
                              ? `Alle ${localizeResource(resource)}-Berechtigungen abwählen`
                              : `Alle ${localizeResource(resource)}-Berechtigungen auswählen`
                          }
                        >
                          {allGroupChecked && (
                            <Check
                              className="text-moto-purple h-3 w-3"
                              strokeWidth={3}
                            />
                          )}
                          {someGroupChecked && (
                            <Minus
                              className="text-moto-purple h-3 w-3"
                              strokeWidth={3}
                            />
                          )}
                        </button>
                      </div>

                      {/* Permission rows */}
                      {!isCollapsed &&
                        permissions.map((permission) => {
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
                                  Ressource:{" "}
                                  {localizeResource(permission.resource)} •
                                  Aktion: {localizeAction(permission.action)}
                                </div>
                              </div>
                              <button
                                type="button"
                                role="switch"
                                aria-label={`${getPermissionDisplayName(permission)} zuweisen`}
                                aria-checked={checked}
                                onClick={() =>
                                  handleTogglePermission(permission.id)
                                }
                                className={`focus:ring-moto-purple relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-offset-2 focus:outline-none ${checked ? "bg-moto-purple" : "bg-gray-300"}`}
                              >
                                <span
                                  className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${checked ? "translate-x-6" : "translate-x-1"}`}
                                />
                              </button>
                            </div>
                          );
                        })}
                    </div>
                  );
                })
              )}
            </div>
          )}
        </div>
      </FormModal>
      {/* Warning alert intentionally disabled */}
    </>
  );
}
