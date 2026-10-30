"use client";

// Soll und Ist (#4774): every child with overruns in the Statistik window,
// most overruns first, with its own PDF and Excel export. Read-only; a name
// opens the child, whose Betreuungszeiten tab shows the days.

import { Download, FileSpreadsheet } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import Link from "~/components/ui/navigation-link";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuEntry } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import {
  careTimeOverrunsExportUrl,
  fetchCareTimeOverruns,
  type CareTimeExportFormat,
  type CareTimeOverrunRow,
  type CareTimeOverruns,
} from "~/lib/care-times";
import { formatDate } from "~/lib/date-helpers";
import { downloadBlob, filenameFromDisposition } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "CareTimeOverrunsSection" });

const EXPORT_FORMATS: readonly {
  format: CareTimeExportFormat;
  label: string;
  Icon: typeof Download;
}[] = [
  { format: "pdf", label: "PDF", Icon: Download },
  { format: "xlsx", label: "Excel", Icon: FileSpreadsheet },
];

export function CareTimeOverrunsSection({
  fromISO,
  toISO,
  groupIds,
  schoolClasses,
}: Readonly<{
  fromISO: string;
  toISO: string;
  groupIds: readonly string[];
  schoolClasses: readonly string[];
}>) {
  const tenantPath = useTenantAwarePath();
  const [data, setData] = useState<CareTimeOverruns | null>(null);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);
  const [exporting, setExporting] = useState<CareTimeExportFormat | null>(null);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const { show: showExportError } = useApiErrorDisplay();
  const retryExportRef = useRef<() => void>(() => undefined);
  // Klassennamen dürfen Kommas enthalten; der Schlüssel bleibt deshalb JSON.
  const filterKey = JSON.stringify([groupIds, schoolClasses]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setData(null);
    clearLoadError();
    const [groups, classes] = JSON.parse(filterKey) as [string[], string[]];
    fetchCareTimeOverruns(fromISO, toISO, {
      groupIds: groups,
      schoolClasses: classes,
    })
      .then((result) => {
        if (!cancelled) setData(result);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setData(null);
        logger.error("care_time_overruns_fetch_failed", {
          from: fromISO,
          to: toISO,
          error: error instanceof Error ? error.message : String(error),
        });
        void showLoadError(error, {
          object: "die Liste Soll und Ist",
          retry: () => setReloadKey((key) => key + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [fromISO, toISO, filterKey, reloadKey, showLoadError, clearLoadError]);

  const downloadExport = useCallback(
    async (format: CareTimeExportFormat) => {
      setExporting(format);
      try {
        const res = await transportFetch(
          careTimeOverrunsExportUrl(fromISO, toISO, format, {
            groupIds,
            schoolClasses,
          }),
        );
        if (!res.ok) {
          throw await apiErrorFromResponse(res, "care time export failed");
        }
        downloadBlob(
          await res.blob(),
          filenameFromDisposition(res) ??
            `soll-und-ist-${fromISO}-${toISO}.${format}`,
        );
      } catch (error) {
        logger.error("care_time_overruns_export_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        retryExportRef.current = () => void downloadExport(format);
        void showExportError(error, {
          object: "die Exportdatei",
          retry: () => retryExportRef.current(),
        });
      } finally {
        setExporting(null);
      }
    },
    [fromISO, toISO, groupIds, schoolClasses, showExportError],
  );

  const exportMenu: OverflowMenuEntry[] = [
    { kind: "header", label: "Exportieren" },
    ...EXPORT_FORMATS.map(({ format, label, Icon }) => ({
      label: exporting === format ? "Wird exportiert…" : label,
      icon: <Icon className="h-4 w-4" aria-hidden />,
      onClick: () => void downloadExport(format),
      disabled: !data || exporting !== null,
    })),
  ];

  const columns: DataTableColumn<CareTimeOverrunRow>[] = [
    {
      key: "name",
      header: "Kind",
      render: (row) => (
        <Link
          href={tenantPath(`/students/${row.student_id}?tab=betreuungszeiten`)}
          className="font-medium text-gray-900 hover:underline"
        >
          {row.last_name}, {row.first_name}
        </Link>
      ),
      sortValue: (row) => `${row.last_name} ${row.first_name}`,
      stacked: "title",
    },
    {
      key: "class",
      header: "Klasse",
      render: (row) => row.school_class || "–",
      sortValue: (row) => row.school_class,
    },
    {
      key: "group",
      header: "Gruppe",
      render: (row) => row.group_name || "–",
      sortValue: (row) => row.group_name,
    },
    {
      key: "overruns",
      header: "Überschreitungen",
      align: "right",
      render: (row) => (
        <span className="tabular-nums">{row.summary.overrun_count}</span>
      ),
      sortValue: (row) => row.summary.overrun_count,
    },
    {
      key: "unbooked",
      header: "Davon ohne Buchung",
      align: "right",
      render: (row) => (
        <span className="tabular-nums">{row.summary.unbooked_days}</span>
      ),
      sortValue: (row) => row.summary.unbooked_days,
    },
    {
      key: "extra",
      header: "Minuten über Buchung",
      align: "right",
      render: (row) => (
        <span className="tabular-nums">{row.summary.extra_minutes}</span>
      ),
      sortValue: (row) => row.summary.extra_minutes,
    },
  ];

  const tolerance = data?.tolerance_minutes ?? 15;

  return (
    <SectionCard
      title="Soll und Ist"
      description={`Kinder, die länger oder an anderen Tagen da waren als gebucht. Überschreitung: mehr als ${tolerance} Minuten früher oder später als gebucht, oder da ohne Buchung. Krank, entschuldigt und geschlossen zählt nicht.`}
      actions={
        <OverflowMenu items={exportMenu} ariaLabel="Liste exportieren" />
      }
    >
      <div className="space-y-3">
        <LoadErrorAlert error={load.error} />
        {data && data.data_from > fromISO ? (
          <Alert
            type="info"
            message={`Anwesenheiten werden nicht unbegrenzt aufbewahrt. Verglichen wird ab ${formatDate(data.data_from)}.`}
          />
        ) : null}
        <DataTable
          columns={columns}
          rows={data?.students ?? []}
          getRowKey={(row) => row.student_id}
          defaultSortKey="overruns"
          defaultSortDirection="desc"
          isLoading={loading}
          pageSize={25}
          paginationResetKey={`${fromISO}-${toISO}-${filterKey}`}
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Keine Überschreitungen"
              description="Im gewählten Zeitraum waren alle Kinder nur zu ihren gebuchten Zeiten da."
            />
          }
        />
      </div>
    </SectionCard>
  );
}
