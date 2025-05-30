"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Role, Permission } from "@/lib/auth-helpers";
import { DatabaseListPage } from "@/components/ui";
import { RoleListItem } from "@/components/auth";

export default function RolesPage() {
  const router = useRouter();
  const [roles, setRoles] = useState<Role[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const fetchRoles = async () => {
    try {
      setLoading(true);
      setError(null);

      // Fetch roles from the API
      const response = await fetch('/api/auth/roles');

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const data = await response.json() as { roles?: unknown[]; data?: unknown[] };

      // Handle different response structures
      const rawRoles = (data.roles ?? data.data ?? []) as Array<{
        id?: unknown;
        name?: unknown;
        description?: unknown;
        created_at?: unknown;
        createdAt?: unknown;
        updated_at?: unknown;
        updatedAt?: unknown;
        permissions?: unknown[];
      }>;
      // Processing API response and raw roles

      // Process and validate roles
      const processedRoles = rawRoles.map((role, index: number) => ({
        id: (role.id !== undefined && role.id !== null && (typeof role.id === 'string' || typeof role.id === 'number')) ? String(role.id) : `role-${index}`, // Ensure ID is a string and always present
        name: (role.name as string) ?? `Rolle ${index + 1}`,
        description: (role.description as string) ?? "",
        createdAt: (role.created_at as string) ?? (role.createdAt as string) ?? new Date().toISOString(),
        updatedAt: (role.updated_at as string) ?? (role.updatedAt as string) ?? new Date().toISOString(),
        permissions: (role.permissions as Permission[]) ?? []
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
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchRoles();
  }, []);

  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectRole = (role: Role) => {
    router.push(`/database/roles/${role.id}`);
  };

  // Apply search filter
  const filteredRoles = roles.filter((role) => {
    if (searchFilter) {
      const searchLower = searchFilter.toLowerCase();
      return (
        role.name.toLowerCase().includes(searchLower) ||
        role.description.toLowerCase().includes(searchLower)
      );
    }
    return true;
  });

  return (
    <DatabaseListPage
      userName={session?.user?.name ?? "Benutzer"}
      title="Rollen verwalten"
      description="Verwalten Sie Systemrollen und Berechtigungen"
      listTitle="Rollenliste"
      searchPlaceholder="Rolle suchen..."
      searchValue={searchFilter}
      onSearchChange={setSearchFilter}
      addButton={{
        label: "Neue Rolle erstellen",
        href: "/database/roles/new"
      }}
      items={filteredRoles}
      loading={loading}
      error={error}
      onRetry={() => fetchRoles()}
      itemLabel={{ singular: "Rolle", plural: "Rollen" }}
      emptyIcon={
        <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
      }
      renderItem={(role: Role) => (
        <RoleListItem
          key={role.id}
          role={role}
          onClick={() => handleSelectRole(role)}
        />
      )}
    />
  );
}
