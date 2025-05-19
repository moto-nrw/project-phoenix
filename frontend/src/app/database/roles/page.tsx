"use client";

import { useEffect, useState } from "react";
import { DataListPage } from "@/components/dashboard";
import { authService } from "@/lib/auth-service";
import type { Role } from "@/lib/auth-helpers";
import { Button } from "@/components/ui";
import { useRouter } from "next/navigation";

export default function RolesPage() {
  const router = useRouter();
  const [roles, setRoles] = useState<Role[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [roleToDelete, setRoleToDelete] = useState<Role | null>(null);

  const loadRoles = async () => {
    try {
      setIsLoading(true);
      setError(null);
      
      // Fetch roles from the API
      const response = await fetch('/api/auth/roles');
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      const data = await response.json();
      
      // Handle different response structures
      const rawRoles = data.roles || data.data || [];
      console.log("API response:", data);
      console.log("Raw roles:", rawRoles);
      
      // Process and validate roles
      const processedRoles = rawRoles.map((role: any, index: number) => ({
        id: role.id ? String(role.id) : `role-${index}`, // Ensure ID is a string and always present
        name: role.name || `Rolle ${index + 1}`,
        description: role.description || "",
        createdAt: role.created_at || role.createdAt || new Date().toISOString(),
        updatedAt: role.updated_at || role.updatedAt || new Date().toISOString(),
        permissions: role.permissions || []
      }));
      
      console.log("Processed roles:", processedRoles);
      setRoles(processedRoles);
      
    } catch (err) {
      console.error("Error loading roles:", err);
      setError("Fehler beim Laden der Rollen");
      
      // Fallback to sample data if API fails
      const sampleRoles = [
        {
          id: "1",
          name: "Administrator",
          description: "Vollständige Berechtigung zur Verwaltung des Systems",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: "2",
          name: "Lehrer",
          description: "Zugriff auf Schülerdaten und Anwesenheitsmanagement",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
        {
          id: "3",
          name: "Eltern",
          description: "Lesezugriff auf Daten der eigenen Kinder",
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ];
      
      console.log("Using fallback sample roles:", sampleRoles);
      setRoles(sampleRoles);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadRoles();
  }, []);

  const handleDeleteClick = (role: Role) => {
    setRoleToDelete(role);
    setDeleteModalOpen(true);
  };

  const handleDeleteConfirm = async () => {
    if (!roleToDelete) return;

    try {
      setDeleteModalOpen(false);
      
      // Call the API to delete the role
      const response = await fetch(`/api/auth/roles/${roleToDelete.id}`, {
        method: 'DELETE',
      });
      
      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }
      
      // Update the local state to remove the role
      setRoles(roles.filter((r) => r.id !== roleToDelete.id));
      setRoleToDelete(null);
      
      // Reload all roles to ensure consistency with the server
      await loadRoles();
    } catch (err) {
      setError("Fehler beim Löschen der Rolle");
      console.error("Error deleting role:", err);
    }
  };

  const handleSelectRole = (role: Role) => {
    router.push(`/database/roles/${role.id}`);
  };

  // Create a simple list item component for roles
  const RoleListItem = ({ role }: { role: Role }) => (
    <div className="group flex cursor-pointer items-center justify-between rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md">
      <div className="flex flex-col">
        <span className="font-medium">{role.name}</span>
        {role.description && (
          <span className="text-sm text-gray-500">{role.description}</span>
        )}
      </div>
      <div className="flex gap-2 opacity-0 transition-opacity group-hover:opacity-100">
        <Button
          variant="outline"
          size="sm"
          onClick={(e) => {
            e.stopPropagation();
            router.push(`/database/roles/${role.id}`);
          }}
        >
          Details
        </Button>
        <Button
          variant="outline"
          size="sm"
          onClick={(e) => {
            e.stopPropagation();
            router.push(`/database/roles/${role.id}?edit=true`);
          }}
        >
          Bearbeiten
        </Button>
        <Button
          variant="destructive"
          size="sm"
          onClick={(e) => {
            e.stopPropagation();
            handleDeleteClick(role);
          }}
        >
          Löschen
        </Button>
      </div>
    </div>
  );

  // Show loading state
  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => loadRoles()}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  // Custom role rendering function with proper key handling
  const renderRole = (role: Role) => (
    <RoleListItem key={`role-${role.id}`} role={role} />
  );

  return (
    <DataListPage
      title="Rollen verwalten"
      sectionTitle="Rollen verwalten"
      backUrl="/database"
      newEntityLabel="Neue Rolle erstellen"
      newEntityUrl="/database/roles/new"
      data={roles}
      onSelectEntityAction={handleSelectRole}
      renderEntity={renderRole}
      searchTerm={searchTerm}
      onSearchChange={setSearchTerm}
    />
  );
}