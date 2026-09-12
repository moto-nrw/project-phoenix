"use client";

import { ChevronRight } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { DetailLoadingSpinner } from "~/components/database/detail-loading-spinner";
import { EditActions } from "~/components/ui/edit-actions";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import type { Permission, Role } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import {
  formatPermissionDisplay,
  localizeAction,
  localizeResource,
} from "~/lib/permission-labels";

const logger = createLogger({ component: "RolePermissionsTab" });

interface RolePermissionsTabProps {
  readonly role: Role;
  /** Der Reiter steht im Bearbeiten-Zustand (Kopf-Aktion „Bearbeiten"). */
  readonly editing: boolean;
  /** Nach erfolgreichem Speichern; der Aufrufer beendet den Bearbeiten-Zustand. */
  readonly onSaved: () => void | Promise<void>;
  readonly onCancelEdit: () => void;
}

type AssignedMap = Record<string, boolean>;

function groupByResource(
  permissions: readonly Permission[],
): Array<[string, Permission[]]> {
  const groups: Record<string, Permission[]> = {};
  for (const permission of permissions) {
    (groups[permission.resource] ??= []).push(permission);
  }
  return Object.entries(groups).sort(([a], [b]) =>
    localizeResource(a).localeCompare(localizeResource(b), "de"),
  );
}

function permissionLabel(permission: Permission): string {
  return formatPermissionDisplay(permission.resource, permission.action);
}

/**
 * Der Reiter „Berechtigungen" einer Rolle (#3116): welche Berechtigungen
 * die Rolle trägt, nach Bereich gruppiert. Bearbeitet wird am Objekt
 * (BAUARTEN-SPEC Bauart 2 Regeln 3 und 4): die Kopf-Aktion „Bearbeiten"
 * schaltet den Reiter um, die Kästchen ändern nur den Entwurf, EIN
 * „Speichern" unten schreibt die Unterschiede. Vorher war das ein Slide-over
 * neben dem Objekt mit eigenem Speichern.
 */
export function RolePermissionsTab({
  role,
  editing,
  onSaved,
  onCancelEdit,
}: RolePermissionsTabProps) {
  const { success: toastSuccess } = useToast();
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [assignedMap, setAssignedMap] = useState<AssignedMap>({});
  const [draftMap, setDraftMap] = useState<AssignedMap>({});
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useFormError();
  const [searchTerm, setSearchTerm] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

  const fetchPermissions = useCallback(async () => {
    try {
      setLoading(true);
      setLoadError(null);
      const [all, assigned] = await Promise.all([
        authService.getPermissions(),
        authService.getRolePermissions(role.id),
      ]);
      const map: AssignedMap = {};
      for (const permission of assigned) map[permission.id] = true;
      setAllPermissions(all);
      setAssignedMap(map);
      setDraftMap(map);
    } catch (error) {
      logger.error("role_permissions_load_failed", {
        role_id: role.id,
        error: error instanceof Error ? error.message : String(error),
      });
      setLoadError("Die Berechtigungen konnten nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, [role.id]);

  useEffect(() => {
    void fetchPermissions();
  }, [fetchPermissions]);

  // Der Entwurf beginnt beim Umschalten in den Bearbeiten-Zustand immer beim
  // gespeicherten Stand; ein Abbruch wirft die Kästchen zurück.
  useEffect(() => {
    if (editing) {
      setDraftMap(assignedMap);
      setSearchTerm("");
      setSaveError(null);
    }
  }, [assignedMap, editing, setSaveError]);

  const assignedCount = useMemo(
    () => allPermissions.filter((p) => assignedMap[p.id] === true).length,
    [allPermissions, assignedMap],
  );

  const filteredPermissions = useMemo(() => {
    const query = searchTerm.trim().toLowerCase();
    if (!query) return allPermissions;
    return allPermissions.filter(
      (permission) =>
        permission.name.toLowerCase().includes(query) ||
        permission.description.toLowerCase().includes(query) ||
        permission.resource.toLowerCase().includes(query) ||
        permission.action.toLowerCase().includes(query) ||
        localizeResource(permission.resource).toLowerCase().includes(query) ||
        localizeAction(permission.action).toLowerCase().includes(query),
    );
  }, [allPermissions, searchTerm]);

  const editGroups = useMemo(
    () => groupByResource(filteredPermissions),
    [filteredPermissions],
  );
  const viewGroups = useMemo(
    () =>
      groupByResource(allPermissions.filter((p) => assignedMap[p.id] === true)),
    [allPermissions, assignedMap],
  );

  const draftCount = useMemo(
    () => allPermissions.filter((p) => draftMap[p.id] === true).length,
    [allPermissions, draftMap],
  );
  const hasChanges = useMemo(
    () =>
      allPermissions.some(
        (p) => (assignedMap[p.id] ?? false) !== (draftMap[p.id] ?? false),
      ),
    [allPermissions, assignedMap, draftMap],
  );
  const allFilteredChecked =
    filteredPermissions.length > 0 &&
    filteredPermissions.every((p) => draftMap[p.id] === true);

  const toggleOne = (permissionId: string) =>
    setDraftMap((current) => ({
      ...current,
      [permissionId]: !current[permissionId],
    }));

  const toggleMany = (permissions: readonly Permission[]) =>
    setDraftMap((current) => {
      const allChecked = permissions.every((p) => current[p.id] === true);
      const next = { ...current };
      for (const p of permissions) next[p.id] = !allChecked;
      return next;
    });

  const handleSave = async () => {
    const toAssign: string[] = [];
    const toRemove: string[] = [];
    for (const permission of allPermissions) {
      const before = assignedMap[permission.id] ?? false;
      const after = draftMap[permission.id] ?? false;
      if (!before && after) toAssign.push(permission.id);
      if (before && !after) toRemove.push(permission.id);
    }
    setSaving(true);
    setSaveError(null);
    try {
      await Promise.all([
        ...toAssign.map((id) =>
          authService.assignPermissionToRole(role.id, id),
        ),
        ...toRemove.map((id) =>
          authService.removePermissionFromRole(role.id, id),
        ),
      ]);
      toastSuccess("Berechtigungen gespeichert.");
      await fetchPermissions();
      await onSaved();
    } catch (error) {
      logger.error("role_permissions_save_failed", {
        role_id: role.id,
        error: error instanceof Error ? error.message : String(error),
      });
      setSaveError("Die Berechtigungen konnten nicht gespeichert werden.");
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return <DetailLoadingSpinner label="Berechtigungen werden geladen..." />;
  }

  if (loadError) {
    return <FormErrorAlert message={loadError} />;
  }

  if (!editing) {
    return (
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          {assignedCount} von {allPermissions.length} Berechtigungen zugewiesen.
        </p>
        {viewGroups.length === 0 ? (
          <p className="text-sm text-gray-500">
            {role.isSystem
              ? "Diese Rolle trägt keine Berechtigungen."
              : "Noch keine Berechtigungen zugewiesen. Mit „Bearbeiten“ oben rechts wählen Sie aus, was diese Rolle darf."}
          </p>
        ) : (
          <div className="moto-content-surface divide-y divide-gray-100 rounded-xl border shadow-sm">
            {viewGroups.map(([resource, permissions]) => (
              <section key={resource} className="px-4 py-3">
                <h4 className="text-sm font-semibold text-gray-900">
                  {localizeResource(resource)}
                  <span className="ml-2 text-xs font-normal text-gray-500">
                    {permissions.length}
                  </span>
                </h4>
                <ul className="mt-1.5 space-y-1">
                  {permissions.map((permission) => (
                    <li key={permission.id} className="text-sm text-gray-700">
                      {permissionLabel(permission)}
                    </li>
                  ))}
                </ul>
              </section>
            ))}
          </div>
        )}
      </div>
    );
  }

  return (
    <form
      className="space-y-4"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        void handleSave();
      }}
    >
      <FormErrorAlert message={saveError} />

      <p className="text-sm text-gray-600">
        {draftCount} von {allPermissions.length} Berechtigungen ausgewählt.
      </p>

      <div className="flex flex-wrap items-end gap-2">
        <div className="min-w-0 flex-1">
          <Input
            controlSize="compact"
            name="role-permissions-search"
            aria-label="Berechtigungen suchen"
            placeholder="Berechtigungen suchen…"
            value={searchTerm}
            onChange={(event) => setSearchTerm(event.target.value)}
            disabled={saving}
          />
        </div>
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={saving || filteredPermissions.length === 0}
          onClick={() => toggleMany(filteredPermissions)}
        >
          {allFilteredChecked ? "Alle abwählen" : "Alle auswählen"}
        </Button>
      </div>

      <div className="moto-content-surface rounded-xl border shadow-sm">
        {editGroups.length === 0 ? (
          <p className="px-4 py-8 text-center text-sm text-gray-500">
            Keine Berechtigungen gefunden.
          </p>
        ) : (
          editGroups.map(([resource, permissions]) => {
            const groupChecked = permissions.filter(
              (p) => draftMap[p.id] === true,
            ).length;
            const allGroupChecked = groupChecked === permissions.length;
            const isCollapsed = collapsed[resource] === true;
            const groupLabel = localizeResource(resource);
            return (
              <section
                key={resource}
                className="border-b border-gray-100 last:border-b-0"
              >
                <div className="flex items-center justify-between gap-2 bg-gray-50 px-3 py-2">
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    className="min-w-0 flex-1 justify-start"
                    aria-expanded={!isCollapsed}
                    onClick={() =>
                      setCollapsed((current) => ({
                        ...current,
                        [resource]: !isCollapsed,
                      }))
                    }
                  >
                    <ChevronRight
                      className={`h-3.5 w-3.5 shrink-0 text-gray-400 transition-transform ${isCollapsed ? "" : "rotate-90"}`}
                      aria-hidden
                    />
                    <span className="truncate text-sm font-semibold text-gray-700">
                      {groupLabel}
                    </span>
                    <span className="text-xs font-normal text-gray-500">
                      {groupChecked}/{permissions.length}
                    </span>
                  </Button>
                  <label
                    htmlFor={`role-permission-group-${resource}`}
                    className="flex shrink-0 cursor-pointer items-center gap-2 text-xs text-gray-600"
                  >
                    <Checkbox
                      id={`role-permission-group-${resource}`}
                      checked={allGroupChecked}
                      disabled={saving}
                      onChange={() => toggleMany(permissions)}
                      aria-label={`Alle Berechtigungen für ${groupLabel} auswählen`}
                    />
                    Alle
                  </label>
                </div>
                {isCollapsed
                  ? null
                  : permissions.map((permission) => (
                      <label
                        key={permission.id}
                        htmlFor={`role-permission-${permission.id}`}
                        className="flex cursor-pointer items-start gap-3 px-3 py-2.5 hover:bg-gray-50"
                      >
                        <Checkbox
                          id={`role-permission-${permission.id}`}
                          checked={draftMap[permission.id] === true}
                          disabled={saving}
                          onChange={() => toggleOne(permission.id)}
                          aria-label={permissionLabel(permission)}
                          className="mt-0.5"
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block text-sm font-medium text-gray-900">
                            {permissionLabel(permission)}
                          </span>
                          <span className="block text-xs text-gray-500">
                            {localizeAction(permission.action)}
                            {permission.description
                              ? ` · ${permission.description}`
                              : ""}
                          </span>
                        </span>
                      </label>
                    ))}
              </section>
            );
          })
        )}
      </div>

      <EditActions
        onCancel={onCancelEdit}
        saving={saving}
        disabled={!hasChanges}
      />
    </form>
  );
}
