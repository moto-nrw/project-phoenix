"use client";

import { useEffect, useState } from "react";
import { DataListPage } from "@/components/dashboard";
import { authService } from "@/lib/auth-service";
import type { Role } from "@/lib/auth-helpers";
import { Button } from "@/components/ui";
import Link from "next/link";
import { useRouter } from "next/navigation";

export default function RolesPage() {
  const router = useRouter();
  const [roles, setRoles] = useState<Role[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [roleToDelete, setRoleToDelete] = useState<Role | null>(null);

  const loadRoles = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await authService.getRoles();
      setRoles(data);
    } catch (err) {
      setError("Fehler beim Laden der Rollen");
      console.error(err);
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
      await authService.deleteRole(roleToDelete.id);
      await loadRoles();
      setDeleteModalOpen(false);
      setRoleToDelete(null);
    } catch (err) {
      setError("Fehler beim Löschen der Rolle");
      console.error(err);
    }
  };

  const columns = [
    {
      header: "Name",
      accessor: "name" as keyof Role,
      render: (role: Role) => (
        <Link
          href={`/database/roles/${role.id}`}
          className="text-blue-600 hover:underline font-medium"
        >
          {role.name}
        </Link>
      ),
    },
    {
      header: "Beschreibung",
      accessor: "description" as keyof Role,
    },
    {
      header: "Erstellt am",
      accessor: "createdAt" as keyof Role,
      render: (role: Role) => new Date(role.createdAt).toLocaleDateString("de-DE"),
    },
  ];

  const actions = (item: Role) => (
    <div className="flex gap-2">
      <Button
        variant="outline"
        size="sm"
        onClick={() => router.push(`/database/roles/${item.id}`)}
      >
        Details
      </Button>
      <Button
        variant="outline"
        size="sm"
        onClick={() => router.push(`/database/roles/${item.id}?edit=true`)}
      >
        Bearbeiten
      </Button>
      <Button
        variant="destructive"
        size="sm"
        onClick={() => handleDeleteClick(item)}
      >
        Löschen
      </Button>
    </div>
  );

  return (
    <DataListPage
      title="Rollen verwalten"
      items={roles}
      columns={columns}
      searchableFields={["name", "description"]}
      actions={actions}
      createLink="/database/roles/new"
      createButtonText="Neue Rolle"
      isLoading={isLoading}
      error={error}
      emptyMessage="Keine Rollen gefunden"
    />
  );
}