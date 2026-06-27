"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ArrowLeft, Plus, UsersRound } from "lucide-react";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { Loading } from "~/components/ui/loading";
import { useIsMobile } from "~/hooks/useIsMobile";
import { createLogger } from "~/lib/logger";
import { useTenantRouter } from "~/lib/tenant-router";
import type { CombinedGroup } from "@/lib/api";
import { combinedGroupService } from "@/lib/api";

const logger = createLogger({ component: "CombinedGroupsPage" });

const COMBINED_GROUPS_BACK_URL = "/database/groups";
const COMBINED_GROUPS_NEW_URL = "/database/groups/combined/new";

// Helper to get access policy label without nested ternary
function getAccessPolicyLabel(policy: string): string {
  const labels: Record<string, string> = {
    all: "Alle",
    first: "Erste Gruppe",
    specific: "Spezifische Gruppe",
  };
  return labels[policy] ?? "Manuell";
}

function getCombinedGroupSubtitle(combinedGroup: CombinedGroup): string {
  const parts = [
    `Zugriffsmethode: ${getAccessPolicyLabel(combinedGroup.access_policy)}`,
  ];

  if (combinedGroup.group_count !== undefined) {
    parts.push(`Gruppen: ${combinedGroup.group_count}`);
  }

  if (combinedGroup.time_until_expiration) {
    parts.push(`Läuft ab in: ${combinedGroup.time_until_expiration}`);
  }

  return parts.join(" | ");
}

function CombinedGroupStatusBadges({
  combinedGroup,
}: Readonly<{ combinedGroup: CombinedGroup }>) {
  if (combinedGroup.is_expired) {
    return (
      <span className="rounded-full bg-red-100 px-2 py-0.5 text-xs text-red-800">
        Abgelaufen
      </span>
    );
  }

  if (combinedGroup.is_active) {
    return (
      <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">
        Aktiv
      </span>
    );
  }

  return null;
}

function CreateCombinedGroupLink({
  compact = false,
}: Readonly<{ compact?: boolean }>) {
  const label = "Neue Kombination erstellen";

  return (
    <Link
      href={COMBINED_GROUPS_NEW_URL}
      aria-label={compact ? label : undefined}
      className={
        compact
          ? "inline-flex h-10 w-10 items-center justify-center rounded-lg bg-gray-900 text-white shadow-sm transition-colors hover:bg-gray-800"
          : "inline-flex h-10 items-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800"
      }
    >
      <Plus className="h-4 w-4" aria-hidden />
      {!compact && <span>{label}</span>}
    </Link>
  );
}

export default function CombinedGroupsPage() {
  const router = useTenantRouter();
  const [combinedGroups, setCombinedGroups] = useState<CombinedGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch combined groups
  const fetchCombinedGroups = useCallback(async () => {
    try {
      setLoading(true);

      // Fetch from the real API using our combined group service
      const data = await combinedGroupService.getCombinedGroups();

      if (data.length === 0) {
        logger.debug("no combined groups returned from API");
      }

      setCombinedGroups(data);
      setError(null);
    } catch (err) {
      logger.error("failed to fetch combined groups", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Fehler beim Laden der Gruppenkombinationen. Bitte versuchen Sie es später erneut.",
      );
      setCombinedGroups([]);
    } finally {
      setLoading(false);
    }
  }, []);

  // Initial data load
  useEffect(() => {
    void fetchCombinedGroups();
  }, [fetchCombinedGroups]);

  const filteredCombinedGroups = useMemo(() => {
    const normalizedSearchTerm = searchTerm.toLowerCase();

    return combinedGroups.filter((combinedGroup) =>
      combinedGroup.name.toLowerCase().includes(normalizedSearchTerm),
    );
  }, [combinedGroups, searchTerm]);

  if (status === "loading" || loading) {
    return <Loading fullPage={false} />;
  }

  const handleSelectCombinedGroup = (combinedGroup: CombinedGroup) => {
    router.push(`/database/groups/combined/${combinedGroup.id}`);
  };

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => void fetchCombinedGroups()}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="-mt-1.5 flex w-full flex-col">
      <MobileBackButton
        href={COMBINED_GROUPS_BACK_URL}
        ariaLabel="Zurück zu Gruppen"
      />

      <div className="mb-4 hidden md:block">
        <Link
          href={COMBINED_GROUPS_BACK_URL}
          className="mb-3 inline-flex items-center gap-2 text-sm font-medium text-gray-600 transition-colors hover:text-gray-900"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden />
          Zurück
        </Link>
        <h1 className="text-2xl font-semibold text-gray-900">
          Gruppenkombinationen
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Gruppenkombination auswählen
        </p>
      </div>

      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Gruppenkombinationen" : ""}
          badge={{
            icon: <UsersRound className="h-5 w-5 text-gray-600" aria-hidden />,
            count: filteredCombinedGroups.length,
            label: "Kombinationen",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Suchen...",
          }}
          actionButton={<CreateCombinedGroupLink />}
          mobileActionButton={<CreateCombinedGroupLink compact />}
        />
      </div>

      <div className="min-h-0 flex-1 pb-4">
        {filteredCombinedGroups.length > 0 ? (
          <div className="overflow-hidden rounded-lg border border-gray-100 bg-white shadow-sm">
            {filteredCombinedGroups.map((combinedGroup) => (
              <DatabaseListItem
                key={combinedGroup.id}
                title={combinedGroup.name || "Unbenannt"}
                subtitle={getCombinedGroupSubtitle(combinedGroup)}
                isSelected={false}
                onSelect={() => handleSelectCombinedGroup(combinedGroup)}
                trailingAccessory={
                  <CombinedGroupStatusBadges combinedGroup={combinedGroup} />
                }
              />
            ))}
          </div>
        ) : (
          <div className="py-8 text-center">
            <p className="text-gray-500">
              {searchTerm
                ? "Keine Ergebnisse gefunden."
                : "Keine Einträge vorhanden."}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
