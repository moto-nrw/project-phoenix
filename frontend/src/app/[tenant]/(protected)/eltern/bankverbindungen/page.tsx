"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Landmark, Lock } from "lucide-react";

import { BackButton } from "~/components/ui/back-button";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Skeleton } from "~/components/ui/skeleton";
import { useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import {
  exportPaymentOverview,
  fetchPaymentOverview,
  type PaymentExportFormat,
  type PaymentOverviewRow,
} from "~/lib/guardian-payment-api";
import { createLogger } from "~/lib/logger";
import { LOCATION_COLORS } from "~/lib/location-helper";

const logger = createLogger({ component: "BankverbindungenPage" });

type CompletenessFilter = "all" | "missing";

const FORMAT_LABELS: Record<PaymentExportFormat, string> = {
  pdf: "PDF",
  xlsx: "Excel",
  docx: "Word",
};

// A school has hundreds of children; rendering every row at once costs a long
// scroll on both layouts and a lot of DOM on a phone.
const ROWS_PER_PAGE = 25;

/** One field of a stacked row: a fixed label column so the values line up. */
function RowField({
  label,
  children,
}: Readonly<{ label: string; children: React.ReactNode }>) {
  return (
    <div className="flex gap-3">
      <dt className="w-28 shrink-0 text-gray-500">{label}</dt>
      <dd className="min-w-0 flex-1 text-right">{children}</dd>
    </div>
  );
}

/**
 * The phone layout of the list: one stacked row per child inside a single
 * surface, so the IBAN sits under the name instead of in a fourth column the
 * viewport cannot show.
 */
function PaymentRowList({
  rows,
  caption,
  isLoading,
  emptyState,
}: Readonly<{
  rows: readonly PaymentOverviewRow[];
  caption: string;
  isLoading: boolean;
  emptyState: React.ReactNode;
}>) {
  const [visibleCount, setVisibleCount] = useState(ROWS_PER_PAGE);

  // A narrowed filter must show its first results, not the tail of the
  // previous page.
  const key = rows.length;
  const [lastKey, setLastKey] = useState(key);
  if (key !== lastKey) {
    setLastKey(key);
    setVisibleCount(ROWS_PER_PAGE);
  }

  if (isLoading) {
    return (
      <div className="space-y-3">
        <p className="text-sm text-gray-600">{caption}</p>
        <div className="moto-content-surface rounded-2xl border shadow-sm">
          <ul className="divide-y divide-gray-100">
            {Array.from({ length: 5 }, (_, i) => (
              <li key={i} className="space-y-2 p-4">
                <Skeleton className="h-4 w-40 rounded" />
                <Skeleton className="h-3 w-full rounded" />
                <Skeleton className="h-3 w-2/3 rounded" />
              </li>
            ))}
          </ul>
        </div>
      </div>
    );
  }

  if (rows.length === 0) {
    return (
      <div className="moto-content-surface rounded-2xl border p-6 shadow-sm">
        {emptyState}
      </div>
    );
  }

  const shown = rows.slice(0, visibleCount);

  return (
    <div className="space-y-3">
      <p className="text-sm text-gray-600">{caption}</p>
      <div className="moto-content-surface rounded-2xl border shadow-sm">
        <ul className="divide-y divide-gray-100">
          {shown.map((row) => (
            <li key={row.studentId} className="p-4">
              <div className="flex items-baseline justify-between gap-3">
                <span className="min-w-0 font-medium text-gray-900">
                  {row.studentName}
                </span>
                <span className="shrink-0 text-xs text-gray-500">
                  {row.schoolClass || "—"}
                </span>
              </div>
              <dl className="mt-2 space-y-1 text-sm">
                <RowField label="Kontoinhaber">
                  {row.guardianId ? (
                    <span className="text-gray-900">{row.accountHolder}</span>
                  ) : (
                    <span className="text-gray-500">Nicht zugeordnet</span>
                  )}
                </RowField>
                <RowField label="IBAN">
                  {row.ibanMasked === "" ? (
                    <span style={{ color: LOCATION_COLORS.WARNING }}>
                      Fehlt
                    </span>
                  ) : (
                    <span className="font-mono text-gray-900">
                      {row.ibanMasked}
                    </span>
                  )}
                </RowField>
              </dl>
            </li>
          ))}
        </ul>
      </div>
      {shown.length < rows.length && (
        <Button
          type="button"
          size="md"
          variant="outline"
          className="w-full"
          onClick={() => setVisibleCount((count) => count + ROWS_PER_PAGE)}
        >
          Weitere {Math.min(ROWS_PER_PAGE, rows.length - shown.length)} anzeigen
        </Button>
      )}
    </div>
  );
}

function BankverbindungenContent() {
  const { data: session, status } = useSession({ required: true });
  const toast = useToast();

  const [rows, setRows] = useState<PaymentOverviewRow[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchValue, setSearchValue] = useState("");
  const [completeness, setCompleteness] = useState<CompletenessFilter>("all");
  const [format, setFormat] = useState<PaymentExportFormat>("xlsx");
  const [isExporting, setIsExporting] = useState(false);

  const canRead = hasPermission(session, "guardians:financial");

  // The fetch effect deliberately depends on the session state only. `toast`
  // comes from a context whose value is memoized today, but a list that
  // refetches on every render — and flips back to its loading skeleton while
  // doing so — is not a failure worth risking on that.
  useEffect(() => {
    if (status === "loading" || !canRead) return;
    let cancelled = false;
    setIsLoading(true);
    fetchPaymentOverview()
      .then((data) => {
        if (!cancelled) setRows(data);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("payment_overview_load_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        toast.error(
          error instanceof Error
            ? error.message
            : "Die Liste konnte nicht geladen werden. Bitte noch einmal versuchen.",
        );
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [status, canRead]);

  const missingCount = useMemo(
    () => rows.filter((row) => row.ibanMasked === "").length,
    [rows],
  );

  const visibleRows = useMemo(() => {
    const needle = searchValue.trim().toLowerCase();
    return rows.filter((row) => {
      if (completeness === "missing" && row.ibanMasked !== "") return false;
      if (needle === "") return true;
      return (
        row.studentName.toLowerCase().includes(needle) ||
        row.accountHolder.toLowerCase().includes(needle) ||
        row.schoolClass.toLowerCase().includes(needle)
      );
    });
  }, [rows, searchValue, completeness]);

  // The caption states how complete the list is rather than repeating the page
  // name — two labels sharing the stem "Bankverbindung" would read as two
  // different things. Once a search or filter narrows the list it says how much
  // of it is on screen instead, so the number never contradicts the rows below.
  const captionText = (() => {
    if (isLoading) return "Liste wird geladen…";
    if (visibleRows.length !== rows.length) {
      return `${visibleRows.length} von ${rows.length} Kindern`;
    }
    return `${rows.length} Kinder, davon ${rows.length - missingCount} mit Bankverbindung`;
  })();

  const handleExport = async () => {
    setIsExporting(true);
    try {
      await exportPaymentOverview(format);
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : "Der Export hat nicht geklappt. Bitte noch einmal versuchen.",
      );
    } finally {
      setIsExporting(false);
    }
  };

  const columns: DataTableColumn<PaymentOverviewRow>[] = [
    {
      key: "student",
      header: "Kind",
      render: (row) => (
        <span className="font-medium text-gray-900">{row.studentName}</span>
      ),
      sortValue: (row) => row.studentName,
    },
    {
      key: "class",
      header: "Klasse",
      render: (row) => (
        <span className="text-gray-600">{row.schoolClass || "—"}</span>
      ),
      sortValue: (row) => row.schoolClass,
    },
    {
      key: "holder",
      header: "Kontoinhaber",
      render: (row) =>
        row.guardianId ? (
          <span className="text-gray-900">{row.accountHolder}</span>
        ) : (
          <span className="text-gray-500">Nicht zugeordnet</span>
        ),
      sortValue: (row) => row.accountHolder,
    },
    {
      key: "iban",
      header: "IBAN",
      render: (row) =>
        row.ibanMasked === "" ? (
          <span className="text-sm" style={{ color: LOCATION_COLORS.WARNING }}>
            Fehlt
          </span>
        ) : (
          <span className="font-mono text-sm text-gray-900">
            {row.ibanMasked}
          </span>
        ),
      sortValue: (row) => row.ibanMasked,
    },
  ];

  const emptyState = (
    <EmptyState
      icon={<Landmark className="h-12 w-12" />}
      title={
        completeness === "missing"
          ? "Keine offenen Bankverbindungen"
          : "Noch keine Kinder in der Liste"
      }
      description={
        completeness === "missing"
          ? "Für jedes Kind ist eine IBAN gespeichert."
          : "Sobald Kinder angelegt sind, erscheinen sie hier."
      }
    />
  );

  if (status !== "loading" && !canRead) {
    return (
      <div className="p-4 sm:p-6">
        <EmptyState
          icon={<Lock className="h-12 w-12" />}
          title="Kein Zugriff auf Bankverbindungen"
          description="Sie brauchen dafür die Berechtigung „Bankverbindungen“. Bitte fragen Sie in der Schulleitung nach."
        />
      </div>
    );
  }

  return (
    <div className="pb-10">
      <PageHeaderWithSearch
        title="Bankverbindungen"
        concept="parents"
        search={{
          value: searchValue,
          onChange: setSearchValue,
          placeholder: "Kind, Klasse oder Kontoinhaber suchen",
        }}
      />

      <div className="space-y-4 px-4 sm:px-6">
        <BackButton referrer="/eltern" />

        <p className="max-w-3xl text-sm text-gray-600">
          Von welchem Konto der Beitrag je Kind eingezogen wird. Die IBAN tragen
          Sie beim Kind ein, im Reiter „Erziehungsberechtigte“.
        </p>

        <div className="flex flex-wrap items-end justify-between gap-3">
          <SegmentedControl<CompletenessFilter>
            ariaLabel="Auswahl der angezeigten Kinder"
            value={completeness}
            onChange={setCompleteness}
            items={[
              { value: "all", label: `Alle Kinder (${rows.length})` },
              { value: "missing", label: `Ohne IBAN (${missingCount})` },
            ]}
          />

          <div className="flex flex-wrap items-end gap-2">
            <SegmentedControl<PaymentExportFormat>
              ariaLabel="Dateiformat des Exports"
              value={format}
              onChange={setFormat}
              items={(Object.keys(FORMAT_LABELS) as PaymentExportFormat[]).map(
                (key) => ({ value: key, label: FORMAT_LABELS[key] }),
              )}
            />
            <Button
              type="button"
              size="md"
              onClick={() => void handleExport()}
              disabled={isExporting || rows.length === 0}
            >
              <Download className="mr-1.5 h-4 w-4" aria-hidden />
              {isExporting ? "Wird erstellt…" : "Herunterladen"}
            </Button>
          </div>
        </div>

        <p className="max-w-3xl text-xs text-gray-500">
          Die Datei enthält die ganzen IBANs und wird protokolliert. Bitte nicht
          per E-Mail weitergeben.
        </p>

        {/* A four-column table on a phone pushes the IBAN — the one value the
            page exists for — off screen. Under md the same rows render as a
            stacked list; from md up the table is the denser read. */}
        <div className="md:hidden" data-testid="payment-list-stacked">
          <PaymentRowList
            rows={visibleRows}
            caption={captionText}
            isLoading={isLoading}
            emptyState={emptyState}
          />
        </div>

        <div className="hidden md:block" data-testid="payment-list-table">
          <DataTable
            columns={columns}
            rows={visibleRows}
            getRowKey={(row) => row.studentId}
            isLoading={isLoading}
            defaultSortKey="student"
            caption={captionText}
            pageSize={ROWS_PER_PAGE}
            paginationResetKey={`${completeness}:${searchValue}`}
            emptyState={emptyState}
          />
        </div>
      </div>
    </div>
  );
}

export default function BankverbindungenPage() {
  return (
    <Suspense fallback={null}>
      <BankverbindungenContent />
    </Suspense>
  );
}
