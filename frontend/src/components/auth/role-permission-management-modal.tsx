"use client";

import { useState, useEffect } from "react";
import { FormModal, Notification } from "~/components/ui";
import { useNotification } from "~/lib/use-notification";
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
  const { notification, showSuccess, showError, showWarning } = useNotification();
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [rolePermissions, setRolePermissions] = useState<Permission[]>([]);
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"assigned" | "available">("assigned");

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

  // Get available permissions (not assigned to role)
  const availablePermissions = filteredPermissions.filter(
    (perm) => !rolePermissions.some((rolePerm) => rolePerm.id === perm.id)
  );

  // Get assigned permissions (assigned to role)
  const assignedPermissions = activeTab === "assigned" 
    ? rolePermissions.filter((permission) => {
        const searchLower = searchTerm.toLowerCase();
        return (
          permission.name.toLowerCase().includes(searchLower) ||
          permission.description.toLowerCase().includes(searchLower) ||
          permission.resource.toLowerCase().includes(searchLower) ||
          permission.action.toLowerCase().includes(searchLower)
        );
      })
    : rolePermissions;

  const handleTogglePermission = (permissionId: string) => {
    setSelectedPermissions((prev) =>
      prev.includes(permissionId)
        ? prev.filter((id) => id !== permissionId)
        : [...prev, permissionId]
    );
  };

  const handleAssignSelected = async () => {
    if (selectedPermissions.length === 0) {
      showWarning("Bitte wählen Sie mindestens eine Berechtigung aus");
      return;
    }

    try {
      setSaving(true);
      
      // Assign each selected permission to the role
      const assignPromises = selectedPermissions.map(async (permissionId) => {
        await authService.assignPermissionToRole(role.id, permissionId);
      });
      
      await Promise.all(assignPromises);
      
      showSuccess("Berechtigungen erfolgreich zur Rolle hinzugefügt");
      setSelectedPermissions([]);
      setActiveTab("assigned");
      await fetchPermissions();
      onUpdate();
    } catch (error) {
      console.error("Error assigning permissions:", error);
      showError("Fehler beim Hinzufügen der Berechtigungen");
    } finally {
      setSaving(false);
    }
  };

  const handleRemovePermission = async (permissionId: string) => {
    try {
      setSaving(true);
      
      await authService.removePermissionFromRole(role.id, permissionId);
      
      showSuccess("Berechtigung erfolgreich entfernt");
      await fetchPermissions();
      onUpdate();
    } catch (error) {
      console.error("Error removing permission:", error);
      showError("Fehler beim Entfernen der Berechtigung");
    } finally {
      setSaving(false);
    }
  };

  const footer = activeTab === "available" && selectedPermissions.length > 0 && (
    <button
      onClick={handleAssignSelected}
      disabled={saving}
      className="rounded-lg bg-blue-600 px-6 py-2 text-white hover:bg-blue-700 disabled:opacity-50"
    >
      {saving ? "Wird gespeichert..." : `${selectedPermissions.length} Berechtigungen hinzufügen`}
    </button>
  );

  return (
    <>
      {/* Notification for success/error messages */}
      <Notification notification={notification} className="fixed top-4 right-4 z-[10000] max-w-sm" />
      
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Berechtigungen verwalten - ${role.name}`}
        size="xl"
        footer={footer}
      >
        <div className="space-y-4">
          {/* Stats */}
          <div className="rounded-lg bg-gray-50 p-4">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-600">Zugewiesene Berechtigungen:</span>
              <span className="font-semibold">
                {rolePermissions.length} Berechtigungen
              </span>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200">
            <button
              onClick={() => setActiveTab("assigned")}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "assigned"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Zugewiesen ({rolePermissions.length})
            </button>
            <button
              onClick={() => setActiveTab("available")}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
                activeTab === "available"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Verfügbare Berechtigungen ({availablePermissions.length})
            </button>
          </div>

          {/* Search */}
          <input
            type="text"
            placeholder="Berechtigungen suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />

          {/* Content */}
          {loading ? (
            <div className="text-center py-8 text-gray-500">Laden...</div>
          ) : (
            <>
              {activeTab === "assigned" && (
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {assignedPermissions.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine Berechtigungen zugewiesen
                    </p>
                  ) : (
                    assignedPermissions.map((permission) => (
                      <div
                        key={permission.id}
                        className="flex items-center justify-between p-3 bg-white rounded-lg border border-gray-200"
                      >
                        <div className="flex-1">
                          <div className="font-medium">{permission.name}</div>
                          <div className="text-sm text-gray-600">
                            {permission.description}
                          </div>
                          <div className="text-xs text-gray-500 mt-1">
                            Resource: {permission.resource} | Action: {permission.action}
                          </div>
                        </div>
                        <button
                          onClick={() => void handleRemovePermission(permission.id)}
                          disabled={saving}
                          className="text-red-600 hover:text-red-800 text-sm font-medium disabled:opacity-50 ml-4"
                        >
                          Entfernen
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}

              {activeTab === "available" && (
                <div className="space-y-2 max-h-80 overflow-y-auto border border-gray-200 rounded-lg">
                  {availablePermissions.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine verfügbaren Berechtigungen gefunden
                    </p>
                  ) : (
                    availablePermissions.map((permission) => (
                      <label
                        key={permission.id}
                        className="flex items-center p-3 hover:bg-gray-50 cursor-pointer"
                      >
                        <input
                          type="checkbox"
                          checked={selectedPermissions.includes(permission.id)}
                          onChange={() => handleTogglePermission(permission.id)}
                          className="mr-3 h-4 w-4 text-blue-600 rounded focus:ring-blue-500"
                        />
                        <div className="flex-1">
                          <div className="font-medium">{permission.name}</div>
                          <div className="text-sm text-gray-600">
                            {permission.description}
                          </div>
                          <div className="text-xs text-gray-500 mt-1">
                            Resource: {permission.resource} | Action: {permission.action}
                          </div>
                        </div>
                      </label>
                    ))
                  )}
                </div>
              )}
            </>
          )}
        </div>
      </FormModal>
    </>
  );
}