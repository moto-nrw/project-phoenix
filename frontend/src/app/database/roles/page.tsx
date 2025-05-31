"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Role } from "@/lib/auth-helpers";
import { authService } from "@/lib/auth-service";
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

      // Use authService to fetch roles
      const data = await authService.getRoles();
      
      setRoles(data);
      setError(null);

    } catch (err) {
      console.error("Error loading roles:", err);
      setError("Fehler beim Laden der Rollen. Bitte versuchen Sie es später erneut.");
      setRoles([]);
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
