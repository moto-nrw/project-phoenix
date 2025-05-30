"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Group } from "@/lib/api";
import { groupService } from "@/lib/api";
import { DatabaseListPage } from "@/components/ui";
import { GroupListItem } from "@/components/groups";

export default function GroupsPage() {
  const router = useRouter();
  const [groups, setGroups] = useState<Group[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
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
        console.log("Groups page received:", data);

        if (data.length === 0 && !search) {
          console.log("No groups returned from API, checking connection");
        }

        setGroups(data);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching groups:", apiErr);
        
        // Check if it's a 403 Forbidden error
        const errorMessage = apiErr instanceof Error ? apiErr.message : String(apiErr);
        if (errorMessage.includes("403")) {
          setError(
            "Sie haben keine Berechtigung, diese Seite anzusehen. Bitte wenden Sie sich an einen Administrator, um die erforderlichen Berechtigungen zu erhalten.",
          );
        } else {
          setError(
            "Fehler beim Laden der Gruppendaten. Bitte versuchen Sie es später erneut.",
          );
        }
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

  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectGroup = (group: Group) => {
    router.push(`/database/groups/${group.id}`);
  };

  return (
    <DatabaseListPage
      userName={session?.user?.name ?? "Root"}
      title="Gruppen auswählen"
      description="Verwalten Sie Gruppen und Raumzuweisungen"
      listTitle="Gruppenliste"
      searchPlaceholder="Gruppe suchen..."
      searchValue={searchFilter}
      onSearchChange={setSearchFilter}
      addButton={{
        label: "Neue Gruppe erstellen",
        href: "/database/groups/new"
      }}
      items={groups}
      loading={loading}
      error={error}
      onRetry={() => fetchGroups()}
      itemLabel={{ singular: "Gruppe", plural: "Gruppen" }}
      emptyIcon={
        <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
        </svg>
      }
      renderItem={(group: Group) => (
        <GroupListItem
          key={group.id}
          group={group}
          onClick={() => handleSelectGroup(group)}
        />
      )}
    />
  );
}
