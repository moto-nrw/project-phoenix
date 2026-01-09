"use client";

import { useState, useEffect } from "react";
import { FormModal } from "~/components/ui";
import { SimpleAlert } from "~/components/simple/SimpleAlert";
import { useToast } from "~/contexts/ToastContext";
import { authService } from "~/lib/auth-service";
import type { Role } from "~/lib/auth-helpers";
import {
  getRoleDisplayName,
  getRoleDisplayDescription,
} from "~/lib/auth-helpers";
import type { Teacher } from "~/lib/teacher-api";

// Extracted to reduce duplication - displays role name, description, and permission count
// Exported for testing purposes
export function RoleInfo({ role }: { readonly role: Role }) {
  return (
    <div className="flex-1">
      <div className="font-medium">{getRoleDisplayName(role.name)}</div>
      <div className="text-sm text-gray-600">
        {getRoleDisplayDescription(role.name, role.description)}
      </div>
      {role.permissions && role.permissions.length > 0 && (
        <div className="mt-1 text-xs text-gray-500">
          {role.permissions.length} Berechtigungen
        </div>
      )}
    </div>
  );
}

interface TeacherRoleManagementModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly teacher: Teacher;
  readonly onUpdate: () => void;
}

export function TeacherRoleManagementModal({
  isOpen,
  onClose,
  teacher,
  onUpdate,
}: TeacherRoleManagementModalProps) {
  const { success: toastSuccess } = useToast();
  const [showErrorAlert, setShowErrorAlert] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [showWarningAlert, setShowWarningAlert] = useState(false);
  const [warningMessage, setWarningMessage] = useState("");
  const [allRoles, setAllRoles] = useState<Role[]>([]);
  const [accountRoles, setAccountRoles] = useState<Role[]>([]);
  const [selectedRoles, setSelectedRoles] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"assigned" | "available">(
    "assigned",
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

  // Fetch all roles and account roles
  const fetchRoles = async () => {
    if (!teacher.account_id) {
      showError("Diese pädagogische Fachkraft hat kein verknüpftes Konto");
      return;
    }

    try {
      setLoading(true);

      // Fetch all roles and account roles
      const [allRolesList, accountRolesList] = await Promise.all([
        authService.getRoles(),
        authService.getAccountRoles(teacher.account_id.toString()),
      ]);

      setAllRoles(allRolesList);
      setAccountRoles(accountRolesList);
    } catch (error) {
      console.error("Error fetching roles:", error);
      showError("Fehler beim Laden der Rollen");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen && teacher.account_id) {
      void fetchRoles();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, teacher.account_id]);

  // Filter roles based on search term
  const filteredRoles = allRoles.filter((role) => {
    const searchLower = searchTerm.toLowerCase();
    return (
      role.name.toLowerCase().includes(searchLower) ||
      role.description.toLowerCase().includes(searchLower)
    );
  });

  // Get available roles (not assigned to account)
  const availableRoles = filteredRoles.filter(
    (role) => !accountRoles.some((accountRole) => accountRole.id === role.id),
  );

  // Get assigned roles (assigned to account)
  const assignedRoles =
    activeTab === "assigned"
      ? accountRoles.filter((role) => {
          const searchLower = searchTerm.toLowerCase();
          return (
            role.name.toLowerCase().includes(searchLower) ||
            role.description.toLowerCase().includes(searchLower)
          );
        })
      : accountRoles;

  const handleToggleRole = (roleId: string) => {
    setSelectedRoles((prev) =>
      prev.includes(roleId)
        ? prev.filter((id) => id !== roleId)
        : [...prev, roleId],
    );
  };

  const handleAssignSelected = async () => {
    if (!teacher.account_id) return;

    if (selectedRoles.length === 0) {
      showWarning("Bitte wählen Sie mindestens eine Rolle aus");
      return;
    }

    try {
      setSaving(true);

      // Assign each selected role to the account
      const assignPromises = selectedRoles.map(async (roleId) => {
        await authService.assignRoleToAccount(
          teacher.account_id!.toString(),
          roleId,
        );
      });

      await Promise.all(assignPromises);

      showSuccess("Rollen erfolgreich zum Konto hinzugefügt");
      setSelectedRoles([]);
      setActiveTab("assigned");
      await fetchRoles();
      onUpdate();
    } catch (error) {
      console.error("Error assigning roles:", error);
      showError("Fehler beim Hinzufügen der Rollen");
    } finally {
      setSaving(false);
    }
  };

  const handleRemoveRole = async (roleId: string) => {
    if (!teacher.account_id) return;

    try {
      setSaving(true);

      await authService.removeRoleFromAccount(
        teacher.account_id.toString(),
        roleId,
      );

      showSuccess("Rolle erfolgreich entfernt");
      await fetchRoles();
      onUpdate();
    } catch (error) {
      console.error("Error removing role:", error);
      showError("Fehler beim Entfernen der Rolle");
    } finally {
      setSaving(false);
    }
  };

  // Get button text for available tab
  const getAddButtonText = () => {
    if (saving) return "Wird gespeichert...";
    if (selectedRoles.length > 0) {
      return `${selectedRoles.length} Rollen hinzufügen`;
    }
    return "Wählen Sie Rollen aus";
  };

  if (!teacher.account_id) {
    return (
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Rollen verwalten - ${teacher.name}`}
        size="md"
      >
        <div className="py-8 text-center text-gray-500">
          <p className="mb-2">
            Diese pädagogische Fachkraft hat kein verknüpftes Konto.
          </p>
          <p className="text-sm">
            Erstellen Sie zuerst ein Konto für diese pädagogische Fachkraft, um
            Rollen zuzuweisen.
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
        title={`Rollen verwalten - ${teacher.name}`}
        size="xl"
      >
        <div className="space-y-4">
          {/* Stats */}
          <div className="rounded-lg bg-gray-50 p-4">
            <div className="flex items-center justify-between">
              <span className="text-sm text-gray-600">Zugewiesene Rollen:</span>
              <span className="font-semibold">
                {accountRoles.length}{" "}
                {accountRoles.length === 1 ? "Rolle" : "Rollen"}
              </span>
            </div>
          </div>

          {/* Tabs */}
          <div className="flex border-b border-gray-200">
            <button
              onClick={() => setActiveTab("assigned")}
              className={`border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "assigned"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Zugewiesen ({accountRoles.length})
            </button>
            <button
              onClick={() => setActiveTab("available")}
              className={`border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
                activeTab === "available"
                  ? "border-blue-500 text-blue-600"
                  : "border-transparent text-gray-500 hover:text-gray-700"
              }`}
            >
              Verfügbare Rollen ({availableRoles.length})
            </button>
          </div>

          {/* Search */}
          <input
            type="text"
            placeholder="Rollen suchen..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full rounded-lg border border-gray-300 px-4 py-2 focus:ring-2 focus:ring-blue-500 focus:outline-none"
          />

          {/* Content */}
          {loading ? (
            <div className="py-8 text-center text-gray-500">Laden...</div>
          ) : (
            <>
              {activeTab === "assigned" && (
                <div className="max-h-96 space-y-2 overflow-y-auto">
                  {assignedRoles.length === 0 ? (
                    <p className="py-8 text-center text-gray-500">
                      Keine Rollen zugewiesen
                    </p>
                  ) : (
                    assignedRoles.map((role) => (
                      <div
                        key={role.id}
                        className="flex items-center justify-between rounded-lg border border-gray-200 bg-white p-3"
                      >
                        <RoleInfo role={role} />
                        <button
                          onClick={() => void handleRemoveRole(role.id)}
                          disabled={saving}
                          className="ml-4 text-sm font-medium text-red-600 hover:text-red-800 disabled:opacity-50"
                        >
                          Entfernen
                        </button>
                      </div>
                    ))
                  )}
                </div>
              )}

              {activeTab === "available" && (
                <div className="max-h-80 space-y-2 overflow-y-auto rounded-lg border border-gray-200">
                  {availableRoles.length === 0 ? (
                    <p className="py-8 text-center text-gray-500">
                      Keine verfügbaren Rollen gefunden
                    </p>
                  ) : (
                    availableRoles.map((role) => (
                      <label
                        key={role.id}
                        className="flex cursor-pointer items-center p-3 hover:bg-gray-50"
                      >
                        <input
                          type="checkbox"
                          checked={selectedRoles.includes(role.id)}
                          onChange={() => handleToggleRole(role.id)}
                          className="mr-3 h-4 w-4 rounded text-blue-600 focus:ring-blue-500"
                        />
                        <RoleInfo role={role} />
                      </label>
                    ))
                  )}
                </div>
              )}
            </>
          )}

          {/* Action button for available tab */}
          {activeTab === "available" && (
            <div className="flex justify-end border-t border-gray-200 pt-4">
              <button
                onClick={handleAssignSelected}
                disabled={saving || selectedRoles.length === 0}
                className="rounded-lg bg-blue-600 px-6 py-2 text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {getAddButtonText()}
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
