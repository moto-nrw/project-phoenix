"use client";

import { useState, useEffect, useMemo } from "react";
import { FormModal } from "~/components/ui";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { authService } from "~/lib/auth-service";
import type { Role, Permission } from "~/lib/auth-helpers";

interface RolePermissionManagementModalProps {
  isOpen: boolean;
  onClose: () => void;
  role: Role;
  onUpdate: () => void;
}

export function RolePermissionManagementModal({
  isOpen,
  onClose,
  role,
  onUpdate,
}: RolePermissionManagementModalProps) {
  // Localized labels for resources/actions to avoid exposing raw keys like "roles:create"
  const resourceLabels: Record<string, string> = {
    users: 'Benutzer',
    roles: 'Rollen',
    permissions: 'Berechtigungen',
    activities: 'Aktivitäten',
    rooms: 'Räume',
    groups: 'Gruppen',
    visits: 'Besuche',
    schedules: 'Zeitpläne',
    config: 'Konfiguration',
    feedback: 'Feedback',
    iot: 'Geräte',
    system: 'System',
    admin: 'Administration',
  };
  const actionLabels: Record<string, string> = {
    create: 'Erstellen',
    read: 'Ansehen',
    update: 'Bearbeiten',
    delete: 'Löschen',
    list: 'Auflisten',
    manage: 'Verwalten',
    assign: 'Zuweisen',
    enroll: 'Anmelden',
    '*': 'Alle',
  };

  const getPermissionDisplayName = (p: Permission) => {
    // Prefer explicit description if it's clearly localized; otherwise build from resource/action
    const res = resourceLabels[p.resource] ?? p.resource;
    const act = actionLabels[p.action] ?? p.action;
    return `${res}: ${act}`;
  };
  const [showSuccessAlert, setShowSuccessAlert] = useState(false);
  const [successMessage, setSuccessMessage] = useState("");
  const [showErrorAlert, setShowErrorAlert] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  // Warning alert disabled for now to reduce noise in UI
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [rolePermissions, setRolePermissions] = useState<Permission[]>([]);
  const [assignedMap, setAssignedMap] = useState<Record<string, boolean>>({});
  const [initialAssignedMap, setInitialAssignedMap] = useState<Record<string, boolean>>({});
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const showSuccess = (message: string) => {
    setSuccessMessage(message);
    setShowSuccessAlert(true);
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
      
      console.log("All permissions:", allPerms);
      console.log("Role permissions:", rolePerms);
      
      setAllPermissions(allPerms);
      setRolePermissions(rolePerms);
      const map: Record<string, boolean> = {};
      rolePerms.forEach(p => { map[p.id] = true; });
      setAssignedMap(map);
      setInitialAssignedMap(map);
    } catch (error) {
      console.error("Error fetching permissions:", error);
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
    setAssignedMap(prev => ({ ...prev, [permissionId]: !prev[permissionId] }));
  };

  const hasChanges = useMemo(() => {
    const keys = new Set([...Object.keys(initialAssignedMap), ...Object.keys(assignedMap)]);
    for (const k of keys) {
      if ((initialAssignedMap[k] ?? false) !== (assignedMap[k] ?? false)) return true;
    }
    return false;
  }, [initialAssignedMap, assignedMap]);

  const handleSaveChanges = async () => {
    try {
      setSaving(true);
      const keys = new Set([...Object.keys(initialAssignedMap), ...Object.keys(assignedMap)]);
      const toAssign: string[] = [];
      const toRemove: string[] = [];
      for (const k of keys) {
        const before = initialAssignedMap[k] ?? false;
        const after = assignedMap[k] ?? false;
        if (!before && after) toAssign.push(k);
        if (before && !after) toRemove.push(k);
      }

      await Promise.all([
        ...toAssign.map(id => authService.assignPermissionToRole(role.id, id)),
        ...toRemove.map(id => authService.removePermissionFromRole(role.id, id)),
      ]);

      showSuccess("Berechtigungen aktualisiert");
      await fetchPermissions();
      onUpdate();
      onClose();
    } catch (error) {
      console.error("Error updating role permissions:", error);
      showError("Fehler beim Aktualisieren der Berechtigungen");
    } finally {
      setSaving(false);
    }
  };

  // Unused: handleRemovePermission kept for future quick actions in list

  const footer = (
    <div className="w-full flex gap-2 md:gap-3">
      <button
        type="button"
        onClick={onClose}
        disabled={saving}
        className="flex-1 px-3 md:px-4 py-2 rounded-lg border border-gray-300 text-xs md:text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md md:hover:scale-105 active:scale-100 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={handleSaveChanges}
        disabled={saving || !hasChanges}
        className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-purple-600 text-xs md:text-sm font-medium text-white hover:bg-purple-700 hover:shadow-lg md:hover:scale-105 active:scale-100 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
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
        title={`Berechtigungen verwalten - ${role.name}`}
        size="xl"
        mobilePosition="center"
        footer={footer}
      >
        <div className="space-y-4 md:space-y-6">
          {/* Stats */}
          <div className="rounded-xl border border-gray-100 bg-purple-50/30 p-3 md:p-4">
            <div className="flex justify-between items-center">
              <span className="text-xs md:text-sm text-gray-600">Zugewiesene Berechtigungen</span>
              <span className="text-sm md:text-base font-semibold text-gray-900">
                {rolePermissions.length}
              </span>
            </div>
          </div>

          {/* Hinweiszeile */}
          <div className="flex items-center justify-between text-xs md:text-sm text-gray-600">
            <span>Aktiviere oder deaktiviere Berechtigungen und speichere die Änderungen.</span>
            <span className="hidden md:inline text-gray-500">{rolePermissions.length} zugewiesen</span>
          </div>

          {/* Search */}
          <input
            type="text"
            placeholder="Berechtigungen suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 md:px-4 md:py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
          />

          {/* Content */}
          {loading ? (
            <div className="text-center py-8 text-gray-500">Laden...</div>
          ) : (
            <div className="space-y-1.5 max-h-96 overflow-y-auto rounded-xl border border-gray-100 bg-white">
              {filteredPermissions.length === 0 ? (
                <p className="text-center py-8 text-gray-500">Keine Berechtigungen gefunden</p>
              ) : (
                filteredPermissions.map((permission) => {
                  const checked = !!assignedMap[permission.id];
                  return (
                    <div key={permission.id} className="flex items-center justify-between p-3 md:p-3.5 hover:bg-gray-50">
                      <div className="flex-1 min-w-0 pr-3">
                        <div className="text-sm font-medium text-gray-900">{getPermissionDisplayName(permission)}</div>
                        <div className="text-[11px] md:text-xs text-gray-500 mt-1">Ressource: {resourceLabels[permission.resource] ?? permission.resource} • Aktion: {actionLabels[permission.action] ?? permission.action}</div>
                      </div>
                      <button
                        type="button"
                        role="switch"
                        aria-checked={checked}
                        onClick={() => handleTogglePermission(permission.id)}
                        className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-purple-500 focus:ring-offset-2 ${checked ? 'bg-purple-600' : 'bg-gray-300'}`}
                      >
                        <span className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${checked ? 'translate-x-6' : 'translate-x-1'}`} />
                      </button>
                    </div>
                  );
                })
              )}
            </div>
          )}
        </div>
      </FormModal>
      
      {/* Alert components */}
      {showSuccessAlert && (
        <SimpleAlert
          type="success"
          message={successMessage}
          onClose={() => setShowSuccessAlert(false)}
          autoClose
        />
      )}
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
