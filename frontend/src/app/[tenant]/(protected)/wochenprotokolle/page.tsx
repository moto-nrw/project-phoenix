"use client";

import { Suspense, useMemo } from "react";
import { useSearchParams } from "next/navigation";
import { RoleGuard } from "~/components/auth/role-guard";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  WeekProtocolStatusBadge,
  weekProtocolWeekLabel,
} from "~/components/week-protocols/week-protocol-status";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { ogsGroupHref } from "~/lib/ogs-groups-navigation";
import { useSWRAuth } from "~/lib/swr";
import { useTenantRouter } from "~/lib/tenant-router";
import {
  weekProtocolPath,
  weekProtocolsService,
  type WeekProtocol,
} from "~/lib/week-protocols-api";

// Wochenprotokolle (#4769): die Protokolle der Gruppen, neueste Woche zuerst.
// Ein Protokoll entsteht von selbst am ersten Abend der Woche; anlegen kann
// man es nicht. Mit ?group= zeigt die Seite nur die Protokolle einer Gruppe,
// so verlinkt sie die Gruppenseite.
//
// Bauart 1 (Sammlung): jede Zeile öffnet das Protokoll auf seiner Route.

const columns: DataTableColumn<WeekProtocol>[] = [
  {
    key: "week",
    header: "Woche",
    stacked: "title",
    className: "font-medium text-gray-800",
    render: weekProtocolWeekLabel,
    sortValue: (protocol) => protocol.weekStart,
  },
  {
    key: "group",
    header: "Gruppe",
    render: (protocol) => protocol.groupName,
    sortValue: (protocol) => protocol.groupName,
  },
  {
    key: "status",
    header: "Stand",
    stacked: "meta",
    render: (protocol) => <WeekProtocolStatusBadge protocol={protocol} />,
  },
];

function WeekProtocolsContent() {
  const groupId = useSearchParams().get("group");
  const router = useTenantRouter();
  const { data, error, isLoading, mutate } = useSWRAuth<WeekProtocol[]>(
    `week-protocols-${groupId ?? "all"}`,
    () => weekProtocolsService.list(groupId),
  );
  const loadError = useSwrLoadError(error, "die Wochenprotokolle", mutate);
  const protocols = useMemo(() => data ?? [], [data]);
  const groupName = groupId ? (protocols[0]?.groupName ?? null) : null;
  const drafts = protocols.filter(
    (protocol) => protocol.status === "draft",
  ).length;

  return (
    <TenantPage
      title="Wochenprotokolle"
      back={Boolean(groupId)}
      backHref={groupId ? ogsGroupHref(groupId) : undefined}
      backLabel="Zurück zur Gruppe"
      info="Jeden Abend hält moto den Tag jeder Gruppe fest. Ab Freitagabend ist die Woche ein Entwurf: Die Gruppenleitung ergänzt ihn und schließt ihn ab. Danach gibt es das Protokoll als PDF."
      loading={isLoading && !data}
      error={loadError}
      stats={
        protocols.length > 0 ? (
          <TenantPageStats
            items={[
              ...(groupName ? [{ value: groupName, label: "" }] : []),
              { value: drafts, label: drafts === 1 ? "Entwurf" : "Entwürfe" },
            ]}
          />
        ) : undefined
      }
      empty={
        data && protocols.length === 0
          ? {
              title: "Noch kein Wochenprotokoll",
              description:
                "moto legt das Protokoll am ersten Abend einer Woche an. Ab Freitagabend können Sie es ergänzen.",
            }
          : null
      }
    >
      <DataTable
        columns={columns}
        rows={protocols}
        getRowKey={(protocol) => protocol.id}
        onRowClick={(protocol) => router.push(weekProtocolPath(protocol.id))}
        caption="Wochenprotokolle"
        defaultSortKey="week"
        defaultSortDirection="desc"
        pageSize={20}
      />
    </TenantPage>
  );
}

export default function WeekProtocolsPage() {
  return (
    <RoleGuard
      variant="staffOrAdmin"
      // Die Protokolle liest das Backend mit groups:read, wie die Gruppen.
      permission="groups:read"
      fallback={<TenantPage title="Wochenprotokolle" loading />}
    >
      <Suspense fallback={<TenantPage title="Wochenprotokolle" loading />}>
        <WeekProtocolsContent />
      </Suspense>
    </RoleGuard>
  );
}
