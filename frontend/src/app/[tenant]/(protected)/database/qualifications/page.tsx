"use client";

// Nachweise aller Mitarbeitenden (#4808): wer welchen Nachweis hat, wann er
// abläuft und welche Pflichtnachweise fehlen. Filterbar nach Status und Art,
// als CSV-Datei für Excel herunterladbar. Eingetragen werden Nachweise im
// Profil der Person (Reiter Stammdaten), die Arten unter Nachweisarten.

import { useMemo, useState } from "react";

import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import {
  qualificationPersonName as personName,
  qualificationsCsv,
} from "~/lib/staff-qualification-csv";
import { formatCount } from "~/lib/format-utils";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  QUALIFICATION_STATUS_LABELS,
  staffQualificationService,
  type QualificationOverviewRow,
  type QualificationStatus,
} from "~/lib/staff-qualification-api";
import { staffQualificationKeys } from "~/lib/swr/staff-keys";
import { useSWRAuth } from "~/lib/swr";
import { useTenantAwarePath } from "~/lib/tenant-path";

const OTHER_KIND = "other";

const STATUS_ORDER: Record<QualificationStatus, number> = {
  missing: 0,
  expired: 1,
  expiring_soon: 2,
  valid: 3,
};

function rowKey(row: QualificationOverviewRow): string {
  return row.qualificationId ?? `missing-${row.staffId}-${row.typeId ?? ""}`;
}

export default function QualificationsPage() {
  const tenantPath = useTenantAwarePath();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState<string>("all");
  const [kind, setKind] = useState<string>("all");

  const { data, isLoading, error, mutate } = useSWRAuth<
    QualificationOverviewRow[]
  >(staffQualificationKeys.overview(), () =>
    staffQualificationService.overview(),
  );
  const loadError = useSwrLoadError(error, "die Nachweise", () => mutate());
  const rows = useMemo(() => data ?? [], [data]);

  const kindOptions = useMemo(() => {
    const names = new Map<string, string>();
    for (const row of rows) {
      if (row.typeId) names.set(row.typeId, row.name);
    }
    return [
      { value: "all", label: "Alle Arten" },
      ...[...names.entries()]
        .sort((a, b) => a[1].localeCompare(b[1], "de"))
        .map(([value, label]) => ({ value, label })),
      { value: OTHER_KIND, label: "Sonstiges" },
    ];
  }, [rows]);

  const filtered = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase("de");
    return rows.filter(
      (row) =>
        (status === "all" || row.status === status) &&
        (kind === "all" ||
          (kind === OTHER_KIND ? row.typeId === null : row.typeId === kind)) &&
        (needle === "" ||
          personName(row).toLocaleLowerCase("de").includes(needle) ||
          row.name.toLocaleLowerCase("de").includes(needle)),
    );
  }, [rows, status, kind, search]);

  const counts = useMemo(() => {
    const byStatus: Record<QualificationStatus, number> = {
      valid: 0,
      expiring_soon: 0,
      expired: 0,
      missing: 0,
    };
    for (const row of rows) byStatus[row.status] += 1;
    return byStatus;
  }, [rows]);

  const filters: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "buttons",
      value: status,
      onChange: (value) => setStatus(String(value)),
      options: [
        { value: "all", label: "Alle" },
        ...(Object.keys(STATUS_ORDER) as QualificationStatus[]).map(
          (value) => ({
            value,
            label: QUALIFICATION_STATUS_LABELS[value].label,
          }),
        ),
      ],
    },
    {
      id: "kind",
      label: "Art",
      type: "dropdown",
      value: kind,
      onChange: (value) => setKind(String(value)),
      options: kindOptions,
    },
  ];
  const activeFilters: ActiveFilter[] = [
    ...(status === "all"
      ? []
      : [
          {
            id: "status",
            label:
              QUALIFICATION_STATUS_LABELS[status as QualificationStatus].label,
            onRemove: () => setStatus("all"),
          },
        ]),
    ...(kind === "all"
      ? []
      : [
          {
            id: "kind",
            label:
              kindOptions.find((option) => option.value === kind)?.label ??
              "Art",
            onRemove: () => setKind("all"),
          },
        ]),
  ];

  const columns: DataTableColumn<QualificationOverviewRow>[] = [
    {
      key: "name",
      header: "Person",
      stacked: "title",
      sortValue: (row) => `${row.lastName} ${row.firstName}`,
      render: (row) => (
        <ButtonLink
          href={tenantPath(`/staff/${row.staffId}?tab=stammdaten`)}
          variant="ghost"
          size="compact"
        >
          {personName(row)}
        </ButtonLink>
      ),
    },
    {
      key: "kind",
      header: "Nachweis",
      sortValue: (row) => row.name,
      render: (row) => (
        <span>
          {row.name}
          {row.mandatory ? (
            <span className="ml-1 text-xs text-gray-500">(Pflicht)</span>
          ) : null}
        </span>
      ),
    },
    {
      key: "expiresOn",
      header: "Gültig bis",
      sortValue: (row) => row.expiresOn,
      render: (row) =>
        row.expiresOn
          ? formatDate(row.expiresOn)
          : row.status === "missing"
            ? "–"
            : "Unbegrenzt",
    },
    {
      key: "status",
      header: "Status",
      stacked: "meta",
      sortValue: (row) => STATUS_ORDER[row.status],
      render: (row) => (
        <StatusBadge
          label={QUALIFICATION_STATUS_LABELS[row.status].label}
          tone={QUALIFICATION_STATUS_LABELS[row.status].tone}
        />
      ),
    },
  ];

  const loading = isLoading && data === undefined;
  const statusLine = loading
    ? undefined
    : [
        `${formatCount(counts.expiring_soon)} laufen bald ab`,
        `${formatCount(counts.expired)} abgelaufen`,
        `${formatCount(counts.missing)} fehlen`,
      ].join(" · ");

  return (
    <TenantPage
      title="Nachweise"
      stats={statusLine}
      statsLoading={loading}
      info="Hier sehen Sie, wer welchen Nachweis hat und wann er abläuft. Nachweise tragen Sie im Profil der Person im Reiter Stammdaten ein."
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Name oder Nachweis suchen…",
      }}
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setStatus("all");
        setKind("all");
      }}
      actions={
        <>
          <ButtonLink
            href={tenantPath("/database/qualification-types")}
            variant="outline"
            size="compact"
          >
            Nachweisarten
          </ButtonLink>
          <Button
            type="button"
            variant="outline"
            size="compact"
            disabled={filtered.length === 0}
            onClick={() =>
              downloadBlob(
                new Blob([qualificationsCsv(filtered)], {
                  type: "text/csv;charset=utf-8",
                }),
                "nachweise.csv",
              )
            }
          >
            Für Excel herunterladen
          </Button>
        </>
      }
      error={loadError}
      back
      backHref="/database"
      backLabel="Zurück zur Datenverwaltung"
    >
      <SectionCard>
        <DataTable
          rows={filtered}
          columns={columns}
          getRowKey={rowKey}
          isLoading={loading}
          defaultSortKey="status"
          stackedOnMobile
          emptyState={
            <EmptyState
              title={
                rows.length === 0
                  ? "Noch keine Nachweise"
                  : "Kein Nachweis gefunden"
              }
              description={
                rows.length === 0
                  ? "Nachweise tragen Sie im Profil der Person im Reiter Stammdaten ein."
                  : "Ändern Sie die Suche oder die Filter."
              }
            />
          }
        />
      </SectionCard>
    </TenantPage>
  );
}
