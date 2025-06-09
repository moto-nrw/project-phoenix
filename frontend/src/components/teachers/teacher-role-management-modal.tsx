"use client";

import { useState, useEffect } from "react";
import { FormModal, Notification } from "~/components/ui";
import { useNotification } from "~/lib/use-notification";
import { authService } from "~/lib/auth-service";
import type { Role } from "~/lib/auth-helpers";
import type { Teacher } from "~/lib/teacher-api";

interface TeacherRoleManagementModalProps {
  isOpen: boolean;
  onClose: () => void;
  teacher: Teacher;
  onUpdate: () => void;
}

export function TeacherRoleManagementModal({
  isOpen,
  onClose,
  teacher,
  onUpdate,
}: TeacherRoleManagementModalProps) {
  const { notification, showSuccess, showError, showWarning } = useNotification();
  const [allRoles, setAllRoles] = useState<Role[]>([]);
  const [accountRoles, setAccountRoles] = useState<Role[]>([]);
  const [selectedRoles, setSelectedRoles] = useState<string[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeTab, setActiveTab] = useState<"assigned" | "available">("assigned");

  // Debug logging
  useEffect(() => {
    if (isOpen) {
      console.log('TeacherRoleManagementModal - Teacher data:', {
        teacher,
        account_id: teacher.account_id,
        hasAccountId: !!teacher.account_id,
        accountIdType: typeof teacher.account_id,
      });
    }
  }, [isOpen, teacher]);

  // Fetch all roles and account roles
  const fetchRoles = async () => {
    if (!teacher.account_id) {
      showError("Dieser Lehrer hat kein verknüpftes Konto");
      return;
    }

    try {
      setLoading(true);
      
      // Fetch all roles and account roles
      const [allRolesList, accountRolesList] = await Promise.all([
        authService.getRoles(),
        authService.getAccountRoles(teacher.account_id.toString()),
      ]);
      
      console.log("All roles:", allRolesList);
      console.log("Account roles:", accountRolesList);
      
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
    (role) => !accountRoles.some((accountRole) => accountRole.id === role.id)
  );

  // Get assigned roles (assigned to account)
  const assignedRoles = activeTab === "assigned" 
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
        : [...prev, roleId]
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
        await authService.assignRoleToAccount(teacher.account_id!.toString(), roleId);
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
      
      await authService.removeRoleFromAccount(teacher.account_id.toString(), roleId);
      
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

  const footer = activeTab === "available" && selectedRoles.length > 0 && (
    <button
      onClick={handleAssignSelected}
      disabled={saving}
      className="rounded-lg bg-blue-600 px-6 py-2 text-white hover:bg-blue-700 disabled:opacity-50"
    >
      {saving ? "Wird gespeichert..." : `${selectedRoles.length} Rollen hinzufügen`}
    </button>
  );

  if (!teacher.account_id) {
    return (
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Rollen verwalten - ${teacher.name}`}
        size="md"
      >
        <div className="py-8 text-center text-gray-500">
          <p className="mb-2">Dieser Lehrer hat kein verknüpftes Konto.</p>
          <p className="text-sm">
            Erstellen Sie zuerst ein Konto für diesen Lehrer, um Rollen zuzuweisen.
          </p>
        </div>
      </FormModal>
    );
  }

  return (
    <>
      {/* Notification for success/error messages */}
      <Notification notification={notification} className="fixed top-4 right-4 z-[10000] max-w-sm" />
      
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Rollen verwalten - ${teacher.name}`}
        size="xl"
        footer={footer}
      >
        <div className="space-y-4">
          {/* Stats */}
          <div className="rounded-lg bg-gray-50 p-4">
            <div className="flex justify-between items-center">
              <span className="text-sm text-gray-600">Zugewiesene Rollen:</span>
              <span className="font-semibold">
                {accountRoles.length} {accountRoles.length === 1 ? 'Rolle' : 'Rollen'}
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
              Zugewiesen ({accountRoles.length})
            </button>
            <button
              onClick={() => setActiveTab("available")}
              className={`px-4 py-2 text-sm font-medium border-b-2 transition-colors ${
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
            className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />

          {/* Content */}
          {loading ? (
            <div className="text-center py-8 text-gray-500">Laden...</div>
          ) : (
            <>
              {activeTab === "assigned" && (
                <div className="space-y-2 max-h-96 overflow-y-auto">
                  {assignedRoles.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine Rollen zugewiesen
                    </p>
                  ) : (
                    assignedRoles.map((role) => (
                      <div
                        key={role.id}
                        className="flex items-center justify-between p-3 bg-white rounded-lg border border-gray-200"
                      >
                        <div className="flex-1">
                          <div className="font-medium">{role.name}</div>
                          <div className="text-sm text-gray-600">
                            {role.description}
                          </div>
                          {role.permissions && role.permissions.length > 0 && (
                            <div className="text-xs text-gray-500 mt-1">
                              {role.permissions.length} Berechtigungen
                            </div>
                          )}
                        </div>
                        <button
                          onClick={() => void handleRemoveRole(role.id)}
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
                  {availableRoles.length === 0 ? (
                    <p className="text-center py-8 text-gray-500">
                      Keine verfügbaren Rollen gefunden
                    </p>
                  ) : (
                    availableRoles.map((role) => (
                      <label
                        key={role.id}
                        className="flex items-center p-3 hover:bg-gray-50 cursor-pointer"
                      >
                        <input
                          type="checkbox"
                          checked={selectedRoles.includes(role.id)}
                          onChange={() => handleToggleRole(role.id)}
                          className="mr-3 h-4 w-4 text-blue-600 rounded focus:ring-blue-500"
                        />
                        <div className="flex-1">
                          <div className="font-medium">{role.name}</div>
                          <div className="text-sm text-gray-600">
                            {role.description}
                          </div>
                          {role.permissions && role.permissions.length > 0 && (
                            <div className="text-xs text-gray-500 mt-1">
                              {role.permissions.length} Berechtigungen
                            </div>
                          )}
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