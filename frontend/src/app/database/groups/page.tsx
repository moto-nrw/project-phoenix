"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { DataListPage } from "@/components/dashboard";
import { GroupListItem } from "@/components/groups";
import type { Group } from "@/lib/api";
import { groupService } from "@/lib/api";

export default function GroupsPage() {
  const router = useRouter();
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/login");
    },
  });

  // Function to fetch groups with optional filters
  const fetchGroups = async (search?: string) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
      };

      try {
        // Fetch from the real API using our group service
        const data = await groupService.getGroups(filters);

        if (data.length === 0 && !search) {
          console.log("No groups returned from API, checking connection");
        }

        setGroups(data);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching groups:", apiErr);
        setError(
          "Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.",
        );
        setGroups([]);
      }
    } catch (err) {
      console.error("Error fetching groups:", err);
      setError(
        "Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.",
      );
      setGroups([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchGroups();
  }, []);

  // Handle search filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchGroups(searchFilter);
    }, 300);

    return () => clearTimeout(timer);
  }, [searchFilter]);

  // We use the API-based search, so we pass an empty search to the DataListPage
  // to avoid duplicate client-side filtering

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectGroup = (group: Group) => {
    router.push(`/database/groups/${group.id}`);
  };

  const renderGroup = (group: Group) => (
    <GroupListItem group={group} onClick={() => handleSelectGroup(group)} />
  );

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => fetchGroups()}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  // Create a handler for the search input in DataListPage
  const handleSearchChange = (searchTerm: string) => {
    setSearchFilter(searchTerm);
  };

  return (
    <DataListPage
      title="Gruppenauswahl"
      sectionTitle="Gruppe auswählen"
      backUrl="/database"
      newEntityLabel="Neue Gruppe erstellen"
      newEntityUrl="/database/groups/new"
      data={groups}
      onSelectEntityAction={handleSelectGroup}
      renderEntity={renderGroup}
      searchTerm={searchFilter}
      onSearchChange={handleSearchChange}
    />
  );
}
