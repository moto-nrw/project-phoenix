"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { Skeleton } from "~/components/ui/skeleton";
import { formatCount } from "~/lib/format-utils";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { createCrudService } from "@/lib/database/service-factory";
import { permissionsConfig } from "@/components/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";
import { PermissionsMasterDetail } from "@/components/permissions/permissions-master-detail";
import {
  formatPermissionDisplay,
  localizeDescription,
} from "@/lib/permission-labels";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DatabasePermissionsPage" });

export default function PermissionsPage() {
  return (
    <Suspense fallback={null}>
      <PermissionsPageContent />
    </Suspense>
  );
}

function PermissionsPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  // The query value only selects an already-loaded row; it never authorizes or
  // pre-fills a permission mutation.
  const selectedId = searchParams.get("permission");
  const [searchTerm, setSearchTerm] = useState("");

  const [permissions, setPermissions] = useState<Permission[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(permissionsConfig), []);

  const fetchPermissions = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setPermissions(arr);
      setError(null);
    } catch (err) {
      logger.error("failed to fetch permissions", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Fehler beim Laden der Berechtigungen. Bitte versuchen Sie es später erneut.",
      );
      setPermissions([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    void fetchPermissions();
  }, [fetchPermissions]);

  const filters: FilterConfig[] = useMemo(() => [], []);

  const activeFilters: ActiveFilter[] = useMemo(
    () =>
      searchTerm
        ? [
            {
              id: "search",
              label: `"${searchTerm}"`,
              onRemove: () => setSearchTerm(""),
            },
          ]
        : [],
    [searchTerm],
  );

  // Statuszeile des Seitenkopfs aus der bereits geladenen Berechtigungsliste.
  const statusLine = useMemo(() => {
    const resources = new Set(permissions.map((p) => p.resource)).size;
    const parts = [
      `${formatCount(permissions.length)} ${permissions.length === 1 ? "Berechtigung" : "Berechtigungen"}`,
    ];
    if (resources > 0) {
      parts.push(
        `${formatCount(resources)} ${resources === 1 ? "Bereich" : "Bereiche"}`,
      );
    }
    return parts.join(" · ");
  }, [permissions]);

  const filteredPermissions = useMemo(() => {
    let arr = [...permissions];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter((p) => {
        const display = formatPermissionDisplay(p.resource, p.action);
        const description = localizeDescription(
          p.resource,
          p.action,
          p.description,
        );
        return (
          p.name.toLowerCase().includes(q) ||
          (p.description?.toLowerCase().includes(q) ?? false) ||
          p.resource.toLowerCase().includes(q) ||
          p.action.toLowerCase().includes(q) ||
          description.toLowerCase().includes(q) ||
          display.toLowerCase().includes(q)
        );
      });
    }
    arr.sort((a, b) => {
      const r = a.resource.localeCompare(b.resource, "de");
      if (r !== 0) return r;
      const a2 = a.action.localeCompare(b.action, "de");
      if (a2 !== 0) return a2;
      return (a.name || "").localeCompare(b.name || "", "de");
    });
    return arr;
  }, [permissions, searchTerm]);

  // Resolve against the unfiltered list so the detail panel survives a search
  // narrowing the visible rows.
  const selectedPermission = useMemo(
    () =>
      selectedId
        ? (permissions.find((permission) => permission.id === selectedId) ??
          null)
        : null,
    [permissions, selectedId],
  );

  const handleSelectPermission = useCallback(
    (id: string | null) => {
      updateUrlParams({ permission: id });
    },
    [updateUrlParams],
  );

  const canShowDetail =
    !loading && (filteredPermissions.length > 0 || selectedPermission !== null);

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      error={error}
      empty={
        filteredPermissions.length === 0 && selectedPermission === null
          ? {
              title: searchTerm
                ? "Keine Berechtigungen gefunden"
                : "Keine Berechtigungen vorhanden",
              description: searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Berechtigungen legt das System an. Bitte wenden Sie sich an moto.",
              icon: (
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.permissions.icon}
                  tone={MOTO_CONCEPTS.permissions.tone}
                  size={48}
                />
              ),
            }
          : null
      }
      className="flex w-full flex-col"
      intro={{
        title: "Berechtigungen",
        description: loading ? <Skeleton className="h-4 w-56" /> : statusLine,
      }}
      search={
        <PageHeaderWithSearch
          embedded
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.permissions.icon}
                tone={MOTO_CONCEPTS.permissions.tone}
                size={20}
              />
            ),
            count: filteredPermissions.length,
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Berechtigungen suchen…",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
        />
      }
    >
      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <PermissionsMasterDetail
            permissions={filteredPermissions}
            selectedId={selectedId}
            selectedPermission={selectedPermission}
            onSelect={handleSelectPermission}
          />
        </div>
      ) : null}
    </DatabasePageLayout>
  );
}
