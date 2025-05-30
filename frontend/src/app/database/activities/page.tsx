"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import type { Activity } from "@/lib/activity-helpers";
import { activityService } from "@/lib/activity-service";
import { DatabaseListPage, SelectFilter } from "@/components/ui";
import { ActivityListItem } from "@/components/activities";

export default function ActivitiesPage() {
  const router = useRouter();
  const [activities, setActivities] = useState<Activity[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchFilter, setSearchFilter] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<string | null>(null);
  const [supervisorFilter, setSupervisorFilter] = useState<string | null>(null);

  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch activities with optional filters
  const fetchActivities = async (
    search?: string,
    categoryId?: string | null,
  ) => {
    try {
      setLoading(true);

      // Prepare filters for API call
      const filters = {
        search: search ?? undefined,
        category_id: categoryId ?? undefined,
      };

      try {
        // Fetch from the real API using our activity service
        const data = await activityService.getActivities(filters);


        setActivities(data);
        setError(null);
      } catch (apiErr) {
        console.error("API error when fetching activities:", apiErr);
        setError(
          "Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.",
        );
        setActivities([]);
      }
    } catch (err) {
      console.error("Error fetching activities:", err);
      setError(
        "Fehler beim Laden der Aktivitätsdaten. Bitte versuchen Sie es später erneut.",
      );
      setActivities([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchActivities();
  }, []);

  // Handle search and category filter changes
  useEffect(() => {
    // Debounce search to avoid too many API calls
    const timer = setTimeout(() => {
      void fetchActivities(searchFilter, categoryFilter);
    }, 300);

    return () => clearTimeout(timer);
  }, [searchFilter, categoryFilter]);

  if (status === "loading") {
    return <div />; // Let DatabaseListPage handle the loading state
  }

  const handleSelectActivity = (activity: Activity) => {
    router.push(`/database/activities/${activity.id}`);
  };

  // Apply client-side filters
  const filteredActivities = activities.filter((activity) => {
    if (categoryFilter && activity.ag_category_id !== categoryFilter) return false;
    if (supervisorFilter && activity.supervisor_id !== supervisorFilter) return false;
    return true;
  });

  // Get unique categories from loaded activities
  const categoryOptions = Array.from(
    new Map(
      activities
        .filter(activity => activity.ag_category_id && activity.category_name)
        .map(activity => [
          activity.ag_category_id, 
          { value: activity.ag_category_id, label: activity.category_name! }
        ])
    ).values()
  ).sort((a, b) => a.label.localeCompare(b.label));

  // Get unique supervisors from loaded activities
  const supervisorOptions = Array.from(
    new Map(
      activities
        .filter(activity => activity.supervisor_id && activity.supervisor_name)
        .map(activity => [
          activity.supervisor_id,
          { value: activity.supervisor_id, label: activity.supervisor_name! }
        ])
    ).values()
  ).sort((a, b) => a.label.localeCompare(b.label));

  // Render filters
  const renderFilters = () => (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:gap-4">
      <SelectFilter
        id="categoryFilter"
        label="Kategorie"
        value={categoryFilter}
        onChange={setCategoryFilter}
        options={categoryOptions}
        placeholder="Alle Kategorien"
      />
      <SelectFilter
        id="supervisorFilter"
        label="Leitung"
        value={supervisorFilter}
        onChange={setSupervisorFilter}
        options={supervisorOptions}
        placeholder="Alle Leitungen"
      />
    </div>
  );

  return (
    <DatabaseListPage
      userName={session?.user?.name ?? "Root"}
      title="Aktivitäten auswählen"
      description="Verwalten Sie Aktivitäten und Anmeldungen"
      listTitle="Aktivitätenliste"
      searchPlaceholder="Aktivität suchen..."
      searchValue={searchFilter}
      onSearchChange={setSearchFilter}
      filters={renderFilters()}
      addButton={{
        label: "Neue Aktivität erstellen",
        href: "/database/activities/new"
      }}
      items={filteredActivities}
      loading={loading}
      error={error}
      onRetry={() => fetchActivities()}
      itemLabel={{ singular: "Aktivität", plural: "Aktivitäten" }}
      emptyIcon={
        <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253" />
        </svg>
      }
      renderItem={(activity: Activity) => (
        <ActivityListItem
          key={activity.id}
          activity={activity}
          onClick={() => handleSelectActivity(activity)}
        />
      )}
    />
  );
}
