"use client";

import { useState, useEffect } from "react";
import { FormModal } from "~/components/ui";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { useToast } from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import type { Permission } from "~/lib/auth-helpers";
import type { Teacher } from "~/lib/teacher-api";

interface TeacherPermissionManagementModalProps {
  isOpen: boolean;
  onClose: () => void;
  teacher: Teacher;
  onUpdate: () => void;
}

export function TeacherPermissionManagementModal({
  isOpen,
  onClose,
  teacher,
  onUpdate,
}: TeacherPermissionManagementModalProps) {
  const { success: toastSuccess } = useToast();
  const [showErrorAlert, setShowErrorAlert] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [showWarningAlert, setShowWarningAlert] = useState(false);
  const [warningMessage, setWarningMessage] = useState("");
  const [allPermissions, setAllPermissions] = useState<Permission[]>([]);
  const [accountPermissions, setAccountPermissions] = useState<Permission[]>(
    [],
  );
  const [directPermissions, setDirectPermissions] = useState<Permission[]>([]);
  const [selectedPermissions, setSelectedPermissions] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"all" | "direct" | "available">(
    "all",
  );

  const showSuccess = (message: string) => {
    toastSuccess(message);
  };

  const showError = (message: string) => {
    setErrorMessage(message);
    setShowErrorAlert(true);
  };

  const showWarning = (message: string) => {
    setWarningMessage(message);
    setShowWarningAlert(true);
  };

  // Fetch all permissions and account permissions
  const fetchPermissions = async () => {
    if (!teacher.account_id) {
      showError("Dieser Lehrer hat kein verknüpftes Konto");
      return;
    }

    try {
      setLoading(true);

      // Fetch all permissions, account permissions (all), and direct permissions
      const [allPerms, accountPerms, directPerms] = await Promise.all([
        authService.getPermissions(),
        authService.getAccountPermissions(teacher.account_id.toString()),
        authService.getAccountDirectPermissions(teacher.account_id.toString()),
      ]);

      setAllPermissions(allPerms);
      setAccountPermissions(accountPerms);
      setDirectPermissions(directPerms);
    } catch (error) {
      console.error("Error fetching permissions:", error);
      showError("Fehler beim Laden der Berechtigungen");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && teacher.account_id) {
      void fetchPermissions();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, teacher.account_id]);

  // Filter permissions based on search term
  const filterPermissions = (permissions: Permission[]) => {
    const searchLower = searchTerm.toLowerCase();
    return permissions.filter(
      (permission) =>
        permission.name.toLowerCase().includes(searchLower) ||
        permission.description.toLowerCase().includes(searchLower) ||
        permission.resource.toLowerCase().includes(searchLower) ||
        permission.action.toLowerCase().includes(searchLower),
    );
  };

  // Get available permissions (not directly assigned to account)
  const availablePermissions = filterPermissions(
    allPermissions.filter(
      (perm) =>
        !directPermissions.some((directPerm) => directPerm.id === perm.id),
    ),
  );

  // Get filtered permissions based on active tab
  const getDisplayPermissions = () => {
    switch (activeTab) {
      case "all":
        return filterPermissions(accountPermissions);
      case "direct":
        return filterPermissions(directPermissions);
      case "available":
        return availablePermissions;
      default:
        return [];
    }
  };

  const handleTogglePermission = (permissionId: string) => {
    setSelectedPermissions((prev) =>
      prev.includes(permissionId)
        ? prev.filter((id) => id !== permissionId)
        : [...prev, permissionId],
    );
  };

  const handleAssignSelected = async () => {
    if (!teacher.account_id) return;

    if (selectedPermissions.length === 0) {
      showWarning("Bitte wählen Sie mindestens eine Berechtigung aus");
      return;
    }

    try {
      setSaving(true);

      // Assign each selected permission to the account
      const assignPromises = selectedPermissions.map(async (permissionId) => {
        await authService.assignPermissionToAccount(
          teacher.account_id!.toString(),
          permissionId,
        );
      });

      await Promise.all(assignPromises);

      showSuccess("Berechtigungen erfolgreich zum Konto hinzugefügt");
      setSelectedPermissions([]);
      setActiveTab("direct");
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
    if (!teacher.account_id) return;

    try {
      setSaving(true);

      await authService.removePermissionFromAccount(
        teacher.account_id.toString(),
        permissionId,
      );

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

  // Check if a permission is from a role
  const isFromRole = (permission: Permission) => {
    return (
      accountPermissions.some((p) => p.id === permission.id) &&
      !directPermissions.some((p) => p.id === permission.id)
    );
  };

  if (!teacher.account_id) {
    return (
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Berechtigungen verwalten - ${teacher.name}`}
        size="md"
      >
        <div className="py-8 text-center text-gray-500">
          <p className="mb-2">Dieser Lehrer hat kein verknüpftes Konto.</p>
          <p className="text-sm">
            Erstellen Sie zuerst ein Konto für diesen Lehrer, um Berechtigungen
            zuzuweisen.
          </p>
        </div>
      </FormModal>
    );
  }

  return (
    <>
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Berechtigungen verwalten - ${teacher.name}`}
        size="xl"
      >
        <div className="space-y-4">
          {/* Stats */}
          <div className="grid grid-cols-2 gap-4">
            <div className="rounded-lg bg-blue-50 p-4">
              <div className="text-sm text-blue-600">
                Gesamte Berechtigungen:
              </div>
              <div className="text-2xl font-semibold text-blue-800">
                {accountPermissions.length}
              </div>
              <div className="mt-1 text-xs text-blue-600">Inkl. von Rollen</div>
            </div>
            <div className="rounded-lg bg-green-50 p-4">
              <div className="text-sm text-green-600">
                Direkte Berechtigungen:
              </div>
              <div className="text-2xl font-semibold text-green-800">
                {directPermissions.length}
              </div>
              <div className="mt-1 text-xs text-green-600">
                Explizit zugewiesen
              </div>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200">
            <button
              onClick={() => setActiveTab("all")}
              className={`border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "all"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Alle Berechtigungen ({accountPermissions.length})
            </button>
            <button
              onClick={() => setActiveTab("direct")}
              className={`border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "direct"
                  ? "border-green-500 text-green-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Direkte ({directPermissions.length})
            </button>
            <button
              onClick={() => setActiveTab("available")}
              className={`border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "available"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Verfügbar ({availablePermissions.length})
            </button>
          </div>

          {/* Search */}
          <input
            type="text"
            placeholder="Berechtigungen suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 focus:ring-2 focus:ring-blue-500 focus:outline-none"
          />

          {/* Content */}
          {loading ? (
            <div className="py-8 text-center text-gray-500">Laden...</div>
          ) : (
            <div className="max-h-96 space-y-2 overflow-y-auto">
              {activeTab === "available" ? (
                // Available permissions with checkboxes
                getDisplayPermissions().length === 0 ? (
                  <p className="py-8 text-center text-gray-500">
                    Keine verfügbaren Berechtigungen gefunden
                  </p>
                ) : (
                  <div className="rounded-lg border border-gray-200">
                    {getDisplayPermissions().map((permission) => (
                      <label
                        key={permission.id}
                        className="flex cursor-pointer items-center border-b p-3 last:border-b-0 hover:bg-gray-50"
                      >
                        <input
                          type="checkbox"
                          checked={selectedPermissions.includes(permission.id)}
                          onChange={() => handleTogglePermission(permission.id)}
                          className="mr-3 h-4 w-4 rounded text-blue-600 focus:ring-blue-500"
                        />
                        <div className="flex-1">
                          <div className="font-medium">{permission.name}</div>
                          <div className="text-sm text-gray-600">
                            {permission.description}
                          </div>
                          <div className="mt-1 text-xs text-gray-500">
                            Resource: {permission.resource} | Action:{" "}
                            {permission.action}
                          </div>
                        </div>
                      </label>
                    ))}
                  </div>
                )
              ) : // Assigned permissions with remove button (only for direct)
              getDisplayPermissions().length === 0 ? (
                <p className="py-8 text-center text-gray-500">
                  {activeTab === "all"
                    ? "Keine Berechtigungen zugewiesen"
                    : "Keine direkten Berechtigungen zugewiesen"}
                </p>
              ) : (
                getDisplayPermissions().map((permission) => {
                  const fromRole = isFromRole(permission);
                  const isDirect = directPermissions.some(
                    (p) => p.id === permission.id,
                  );

                  return (
                    <div
                      key={permission.id}
                      className="flex items-center justify-between rounded-lg border border-gray-200 bg-white p-3"
                    >
                      <div className="flex-1">
                        <div className="flex items-center gap-2">
                          <div className="font-medium">{permission.name}</div>
                          {fromRole && (
                            <span className="rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-800">
                              Von Rolle
                            </span>
                          )}
                          {isDirect && (
                            <span className="rounded bg-green-100 px-2 py-0.5 text-xs text-green-800">
                              Direkt
                            </span>
                          )}
                        </div>
                        <div className="text-sm text-gray-600">
                          {permission.description}
                        </div>
                        <div className="mt-1 text-xs text-gray-500">
                          Resource: {permission.resource} | Action:{" "}
                          {permission.action}
                        </div>
                      </div>
                      {isDirect && activeTab !== "all" && (
                        <button
                          onClick={() =>
                            void handleRemovePermission(permission.id)
                          }
                          disabled={saving}
                          className="ml-4 text-sm font-medium text-red-600 hover:text-red-800 disabled:opacity-50"
                        >
                          Entfernen
                        </button>
                      )}
                    </div>
                  );
                })
              )}
            </div>
          )}

          {/* Action button for available tab */}
          {activeTab === "available" && (
            <div className="flex justify-end border-t border-gray-200 pt-4">
              <button
                onClick={handleAssignSelected}
                disabled={saving || selectedPermissions.length === 0}
                className="rounded-lg bg-blue-600 px-6 py-2 text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {saving
                  ? "Wird gespeichert..."
                  : selectedPermissions.length > 0
                    ? `${selectedPermissions.length} Berechtigungen hinzufügen`
                    : "Wählen Sie Berechtigungen aus"}
              </button>
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
      {showWarningAlert && (
        <SimpleAlert
          type="warning"
          message={warningMessage}
          onClose={() => setShowWarningAlert(false)}
        />
      )}
    </>
  );
}
