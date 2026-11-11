"use client";

import { EmptyState } from "~/components/ui/empty-state";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { EditActions } from "~/components/ui/edit-actions";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ListGroupHeader } from "~/components/ui/list-group-header";
import { SectionCard } from "~/components/ui/section-card";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import type { Permission, Role } from "~/lib/auth-helpers";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  formatPermissionDisplay,
  isReservedPermission,
  localizeAction,
  localizeDescription,
  localizeResource,
  riskyPermissionHint,
} from "~/lib/permission-labels";
import { Loading } from "~/components/ui/loading";

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

function isReserved(permission: Permission): boolean {
  return isReservedPermission(permission.resource, permission.action);
}

/**
 * Der Warnhinweis unter einem Recht im Bearbeiten-Zustand (#4574): ein
 * gesperrtes Recht, das die Rolle noch aus der Zeit vor der Sperre trägt, oder
 * ein Recht, mit dem jemand Rollen, Konten oder Einstellungen ändern kann.
 */
function permissionHint(permission: Permission): string | undefined {
  if (isReserved(permission)) {
    const owner =
      permission.resource === "admin"
        ? "die Rolle Verwaltung"
        : "das moto-Team";
    return `Nur für ${owner}. Sie können es entfernen, aber nicht wieder vergeben.`;
  }
  return riskyPermissionHint(permission.resource, permission.action);
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
  // Ladefehler vor Ort, Speicherfehler im Formular, beide mit Wiederholen
  // (#2517).
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [completingSave, setSaving] = useState(false);
  const [reloading, setReloading] = useState(false);
  const saving = completingSave || reloading;
  const formRef = useRef<HTMLFormElement>(null);
  const {
    error: saveError,
    show: showSaveError,
    clear: clearSaveError,
  } = useApiFormError(formRef);
  const latestSaveRef = useRef<() => void>(() => undefined);
  // Bis der Katalogtext eines Ladefehlers da ist, bleibt der Ladezustand.
  const [loadFailed, setLoadFailed] = useState(false);
  const [searchTerm, setSearchTerm] = useState("");
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const latestPermissionRequest = useRef(0);
  const draftDirty = useRef(false);
  const clearReloadError = useRef<() => void>(() => undefined);

  const fetchPermissions = useCallback(
    async (includeCatalogue: boolean, options: { rethrow?: boolean } = {}) => {
      const request = ++latestPermissionRequest.current;
      try {
        // Nach dem Speichern bleibt die bestätigte Liste stehen (#6652).
        if (!options.rethrow) {
          setLoading(true);
          setLoadFailed(false);
          clearLoadError();
        }
        const assignedRequest = authService.getRolePermissions(role.id);
        const [assigned, all] = includeCatalogue
          ? await Promise.all([assignedRequest, authService.getPermissions()])
          : [await assignedRequest, undefined];
        if (request !== latestPermissionRequest.current) return;
        const map: AssignedMap = {};
        for (const permission of assigned) map[permission.id] = true;
        setAllPermissions(all ?? assigned);
        setAssignedMap(map);
        if (!draftDirty.current) setDraftMap(map);
        clearReloadError.current();
      } catch (error) {
        if (request !== latestPermissionRequest.current) return;
        logger.error("role_permissions_load_failed", {
          role_id: role.id,
          error: error instanceof Error ? error.message : String(error),
        });
        if (options.rethrow) throw error;
        setLoadFailed(true);
        void showLoadError(error, {
          object: "die Liste der Berechtigungen",
          retry: () => void fetchPermissions(includeCatalogue),
        });
      } finally {
        if (request === latestPermissionRequest.current) setLoading(false);
      }
    },
    [role.id, clearLoadError, showLoadError],
  );

  useEffect(() => {
    void fetchPermissions(editing);
  }, [editing, fetchPermissions]);
  // Nachladen nach dem Speichern: ein Fehler dabei ist ein Ladefehler, kein
  // Fehler des Speicherns (#6652).
  const afterSave = useRefreshAfterWrite(
    "die Anzeige der Berechtigungen",
    async () => {
      setReloading(true);
      try {
        await fetchPermissions(true, { rethrow: true });
      } finally {
        setReloading(false);
      }
    },
  );

  useLayoutEffect(() => {
    clearReloadError.current = afterSave.clear;
  });

  const wasEditing = useRef(false);

  // Der Entwurf beginnt beim Umschalten in den Bearbeiten-Zustand immer beim
  // gespeicherten Stand; ein Abbruch wirft die Kästchen zurück. Nur beim
  // Umschalten: setAssignedMap übergibt bei jedem Laden ein frisches Objekt,
  // und ein Reset auf dessen Identität verwirft einen Klick, der vor dem
  // passiven Effekt des Lade-Commits liegt. Den Entwurf setzt der Ladepfad in
  // fetchPermissions selbst (#3346).
  useEffect(() => {
    const entersEditing = editing && !wasEditing.current;
    wasEditing.current = editing;
    if (!editing) draftDirty.current = false;
    if (!entersEditing) return;
    draftDirty.current = false;
    setDraftMap(assignedMap);
    setSearchTerm("");
    clearSaveError();
  }, [assignedMap, editing, clearSaveError]);

  const assignedCount = useMemo(
    () => allPermissions.filter((p) => assignedMap[p.id] === true).length,
    [allPermissions, assignedMap],
  );

  // Gesperrte Rechte bekommt eine Rolle der Schule nicht (#4574). Trägt sie
  // eins schon, bleibt es sichtbar, damit es entfernt werden kann.
  const selectablePermissions = useMemo(
    () =>
      allPermissions.filter(
        (p) => !isReserved(p) || assignedMap[p.id] === true,
      ),
    [allPermissions, assignedMap],
  );

  const filteredPermissions = useMemo(() => {
    const query = searchTerm.trim().toLowerCase();
    if (!query) return selectablePermissions;
    return selectablePermissions.filter(
      (permission) =>
        permission.name.toLowerCase().includes(query) ||
        permission.description.toLowerCase().includes(query) ||
        permission.resource.toLowerCase().includes(query) ||
        permission.action.toLowerCase().includes(query) ||
        localizeResource(permission.resource).toLowerCase().includes(query) ||
        localizeAction(permission.action, permission.resource)
          .toLowerCase()
          .includes(query),
    );
  }, [selectablePermissions, searchTerm]);

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

  const toggleOne = (permissionId: string) => {
    draftDirty.current = true;
    setDraftMap((current) => ({
      ...current,
      [permissionId]: !current[permissionId],
    }));
  };

  const toggleMany = (permissions: readonly Permission[]) => {
    draftDirty.current = true;
    setDraftMap((current) => {
      const allChecked = permissions.every((p) => current[p.id] === true);
      const next = { ...current };
      for (const p of permissions) next[p.id] = !allChecked;
      return next;
    });
  };

  const handleSave = async () => {
    const permissionIds = allPermissions
      .filter((permission) => draftMap[permission.id] === true)
      .map((permission) => permission.id);
    setSaving(true);
    clearSaveError();
    try {
      await authService.replaceRolePermissions(role.id, permissionIds);
      toastSuccess("Die Berechtigungen der Rolle sind gespeichert.");
    } catch (error) {
      logger.error("role_permissions_save_failed", {
        role_id: role.id,
        error: error instanceof Error ? error.message : String(error),
      });
      void showSaveError(error, {
        object: "das Speichern der Berechtigungen",
        retry: () => latestSaveRef.current(),
      });
      setSaving(false);
      return;
    }
    // The write confirms this selection even if the following reads fail.
    draftDirty.current = false;
    setAssignedMap({ ...draftMap });
    try {
      await afterSave.refresh();
      await onSaved();
    } finally {
      setSaving(false);
    }
  };

  // Wiederholen speichert den aktuellen Entwurf.
  useLayoutEffect(() => {
    latestSaveRef.current = () => void handleSave();
  });

  if (loading || (loadFailed && !loadError)) {
    return <Loading message="Berechtigungen werden geladen..." />;
  }

  if (loadError) {
    return (
      <div className="space-y-4">
        <LoadErrorAlert error={loadError} />
        {editing ? <EditActions onCancel={onCancelEdit} disabled /> : null}
      </div>
    );
  }

  if (!editing) {
    return (
      <div className="space-y-4">
        <LoadErrorAlert error={afterSave.error} />
        <p className="text-sm text-gray-600">
          {editing
            ? `${assignedCount} von ${allPermissions.length} Berechtigungen zugewiesen.`
            : `${assignedCount} ${assignedCount === 1 ? "Berechtigung" : "Berechtigungen"} zugewiesen.`}
        </p>
        {viewGroups.length === 0 ? (
          <p className="text-sm text-gray-500">
            {role.isSystem
              ? "Diese Rolle trägt keine Berechtigungen."
              : "Noch keine Berechtigungen zugewiesen. Mit „Bearbeiten“ oben rechts wählen Sie aus, was diese Rolle darf."}
          </p>
        ) : (
          <SectionCard padding="none">
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
          </SectionCard>
        )}
      </div>
    );
  }

  return (
    <form
      ref={formRef}
      className="space-y-4"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        void handleSave();
      }}
    >
      <LoadErrorAlert error={afterSave.error} />
      <FormErrorAlert message={saveError} />

      <p className="text-sm text-gray-600">
        {draftCount} von {selectablePermissions.length} Berechtigungen
        ausgewählt.
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

      <SectionCard padding="none">
        {editGroups.length === 0 ? (
          <EmptyState title="Keine Berechtigungen gefunden." className="py-8" />
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
                <ListGroupHeader
                  title={groupLabel}
                  addon={
                    <span className="text-xs text-gray-500">
                      {groupChecked}/{permissions.length}
                    </span>
                  }
                  collapsible={{
                    expanded: !isCollapsed,
                    onToggle: () =>
                      setCollapsed((current) => ({
                        ...current,
                        [resource]: !isCollapsed,
                      })),
                  }}
                  meta={
                    <label
                      htmlFor={`role-permission-group-${resource}`}
                      className="flex cursor-pointer items-center gap-2 text-xs text-gray-600"
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
                  }
                />
                {isCollapsed
                  ? null
                  : permissions.map((permission) => {
                      const description = localizeDescription(
                        permission.resource,
                        permission.action,
                        permission.description,
                      );
                      const hint = permissionHint(permission);

                      return (
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
                            aria-describedby={
                              hint
                                ? `role-permission-hint-${permission.id}`
                                : undefined
                            }
                            className="mt-0.5"
                          />
                          <span className="min-w-0 flex-1">
                            <span className="block text-sm font-medium text-gray-900">
                              {permissionLabel(permission)}
                            </span>
                            <span className="block text-xs text-gray-500">
                              {localizeAction(
                                permission.action,
                                permission.resource,
                              )}
                              {description ? ` · ${description}` : ""}
                            </span>
                            {hint ? (
                              <span
                                id={`role-permission-hint-${permission.id}`}
                                className="text-moto-amber-strong mt-0.5 block text-xs font-medium"
                              >
                                {hint}
                              </span>
                            ) : null}
                          </span>
                        </label>
                      );
                    })}
              </section>
            );
          })
        )}
      </SectionCard>

      <EditActions
        onCancel={onCancelEdit}
        saving={saving}
        disabled={!hasChanges}
      />
    </form>
  );
}
