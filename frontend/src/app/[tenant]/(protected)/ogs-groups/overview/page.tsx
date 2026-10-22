"use client";

import { useEffect, useMemo } from "react";
import { ChevronRight } from "lucide-react";
import { RoleGuard } from "~/components/auth/role-guard";
import { OpenCareModeGuard } from "~/components/tenant/open-care-mode-guard";
import { CollectionGrid } from "~/components/ui/collection-grid";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { TileCard } from "~/components/ui/tile-card";
import { ogsGroupHref, OGS_GROUPS_LABEL } from "~/lib/ogs-groups-navigation";
import { useSupervision } from "~/lib/supervision-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import type { NavigationEducationalGroup } from "~/lib/usercontext-helpers";

/**
 * Die Auswahlseite hinter dem Eintrag „Gruppen“ (#4351): alle Gruppen, die
 * die Person sehen darf, die eigenen zuerst und markiert. Die Liste ist
 * dieselbe wie in der Seitenleiste (SupervisionContext), deshalb lädt die
 * Seite nichts Eigenes.
 */
export default function GroupOverviewPage() {
  return (
    <OpenCareModeGuard>
      <RoleGuard
        variant="staffOrAdmin"
        // Wie die Gruppenseite: eine Betreuungsrolle der Schule heißt nicht
        // „user“, die Gruppen liest das Backend mit groups:read (#3469).
        permission="groups:read"
        fallback={<TenantPage title={OGS_GROUPS_LABEL} loading />}
      >
        <GroupOverviewContent />
      </RoleGuard>
    </OpenCareModeGuard>
  );
}

/** Eigene Gruppen zuerst; innerhalb beider Teile bleibt die Reihenfolge. */
function ownGroupsFirst(
  groups: readonly NavigationEducationalGroup[],
): NavigationEducationalGroup[] {
  const own = groups.filter((group) => group.is_personal !== false);
  const other = groups.filter((group) => group.is_personal === false);
  return [...own, ...other];
}

function GroupOverviewContent() {
  const { groups, isLoadingGroups } = useSupervision();
  const router = useTenantRouter();
  const tenantPath = useTenantAwarePath();
  const sortedGroups = useMemo(() => ownGroupsFirst(groups), [groups]);
  const onlyGroup =
    !isLoadingGroups && groups.length === 1 ? groups[0] : undefined;

  // Wer genau eine Gruppe sieht, braucht keine Auswahl.
  useEffect(() => {
    if (onlyGroup) router.replace(ogsGroupHref(onlyGroup.id));
  }, [onlyGroup, router]);

  const ownCount = sortedGroups.filter(
    (group) => group.is_personal !== false,
  ).length;
  const stats =
    groups.length > 0
      ? `${groups.length} ${groups.length === 1 ? "Gruppe" : "Gruppen"} · ${ownCount} eigene`
      : null;

  return (
    <TenantPage
      title={OGS_GROUPS_LABEL}
      stats={stats}
      statsLoading={isLoadingGroups}
      info={
        <>
          „Ihre Gruppe“: Sie sind dort als Gruppenleitung eingetragen.
          „Vertretung“: Sie vertreten die Gruppe gerade.
        </>
      }
      loading={isLoadingGroups || onlyGroup !== undefined}
      loadingLabel="Gruppen werden geladen…"
      empty={
        groups.length === 0
          ? {
              icon: <MotoConceptIcon concept="groups" size={48} />,
              title: "Keine Gruppe sichtbar",
              description:
                "Ihre Leitung kann Sie einer Gruppe als Gruppenleitung zuordnen.",
            }
          : null
      }
    >
      <CollectionGrid as="ul" ariaLabel="Gruppen">
        {sortedGroups.map((group, index) => (
          <li
            key={group.id}
            className="flex"
            // Ziel der Einführungstour (#3748): die erste Gruppe, also die
            // eigene, wenn es eine gibt.
            data-setup-tour={index === 0 ? "group-tile" : undefined}
          >
            <GroupTile
              group={group}
              href={tenantPath(ogsGroupHref(group.id))}
            />
          </li>
        ))}
      </CollectionGrid>
    </TenantPage>
  );
}

function GroupMark({ group }: { readonly group: NavigationEducationalGroup }) {
  if (group.is_personal === false) return null;
  if (group.via_substitution) {
    return <StatusBadge label="Vertretung" tone="blue" compact />;
  }
  return <StatusBadge label="Ihre Gruppe" tone="green" compact />;
}

function GroupTile({
  group,
  href,
}: {
  readonly group: NavigationEducationalGroup;
  readonly href: string;
}) {
  const leaders = group.leaders ?? [];
  return (
    <TileCard href={href} className="flex flex-col gap-3">
      <div className="flex items-start justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2">
          <h2 className="truncate text-base font-bold text-gray-900">
            {group.name}
          </h2>
          <ChevronRight
            className="h-4 w-4 flex-shrink-0 text-gray-300 transition-colors duration-200 md:group-hover:text-gray-500"
            aria-hidden="true"
          />
        </div>
        <GroupMark group={group} />
      </div>
      <div className="space-y-1.5 text-sm text-gray-600">
        <p className="flex items-center gap-1.5">
          <MotoConceptIcon concept="rooms" size={16} tone="neutral" />
          <span className="truncate">{group.room_name ?? "Kein Raum"}</span>
        </p>
        {leaders.length > 0 && (
          <p className="flex items-center gap-1.5">
            <MotoConceptIcon concept="staff" size={16} tone="neutral" />
            <span className="truncate">
              Gruppenleitung: {leaders.join(", ")}
            </span>
          </p>
        )}
      </div>
    </TileCard>
  );
}
