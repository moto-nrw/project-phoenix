"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { DataListPage, ResponsiveLayout } from "@/components/dashboard";
import type { CombinedGroup } from "@/lib/api";
import { combinedGroupService } from "@/lib/api";
import { Loading } from "~/components/ui/loading";

// Helper to get access policy label without nested ternary
function getAccessPolicyLabel(policy: string): string {
  const labels: Record<string, string> = {
    all: "Alle",
    first: "Erste Gruppe",
    specific: "Spezifische Gruppe",
  };
  return labels[policy] ?? "Manuell";
}

export default function CombinedGroupsPage() {
  const router = useRouter();
  const [combinedGroups, setCombinedGroups] = useState<CombinedGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch combined groups
  const fetchCombinedGroups = async () => {
    try {
      setLoading(true);

      try {
        // Fetch from the real API using our combined group service
        const data = await combinedGroupService.getCombinedGroups();

        if (data.length === 0) {
          console.log(
            "No combined groups returned from API, checking connection",
          );
        }

        setCombinedGroups(data);
        setError(null);
      } catch (error_) {
        console.error("API error when fetching combined groups:", error_);
        setError(
          "Fehler beim Laden der Gruppenkombinationen. Bitte versuchen Sie es später erneut.",
        );
        setCombinedGroups([]);
      }
    } catch (err) {
      console.error("Error fetching combined groups:", err);
      setError(
        "Fehler beim Laden der Gruppenkombinationen. Bitte versuchen Sie es später erneut.",
      );
      setCombinedGroups([]);
    } finally {
      setLoading(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchCombinedGroups();
  }, []);

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  const handleSelectCombinedGroup = (combinedGroup: CombinedGroup) => {
    router.push(`/database/groups/combined/${combinedGroup.id}`);
  };

  // Custom renderer for combined group items
  const renderCombinedGroup = (combinedGroup: CombinedGroup) => (
    <div className="flex w-full flex-col transition-transform duration-200 group-hover:translate-x-1">
      <div className="flex items-center justify-between">
        <span className="font-semibold text-gray-900 transition-colors duration-200 group-hover:text-blue-600">
          {combinedGroup.name}
          {combinedGroup.is_active && !combinedGroup.is_expired && (
            <span className="ml-2 rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">
              Aktiv
            </span>
          )}
          {combinedGroup.is_expired && (
            <span className="ml-2 rounded-full bg-red-100 px-2 py-0.5 text-xs text-red-800">
              Abgelaufen
            </span>
          )}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:transform group-hover:text-blue-500"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5l7 7-7 7"
          />
        </svg>
      </div>
      <span className="text-sm text-gray-500">
        Zugriffsmethode: {getAccessPolicyLabel(combinedGroup.access_policy)}
        {combinedGroup.group_count !== undefined &&
          ` | Gruppen: ${combinedGroup.group_count}`}
        {combinedGroup.time_until_expiration &&
          ` | Läuft ab in: ${combinedGroup.time_until_expiration}`}
      </span>
    </div>
  );

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => fetchCombinedGroups()}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  return (
    <DataListPage
      title="Gruppenkombinationen"
      sectionTitle="Gruppenkombination auswählen"
      backUrl="/database/groups"
      newEntityLabel="Neue Kombination erstellen"
      newEntityUrl="/database/groups/combined/new"
      data={combinedGroups}
      onSelectEntityAction={handleSelectCombinedGroup}
      renderEntity={renderCombinedGroup}
    />
  );
}
