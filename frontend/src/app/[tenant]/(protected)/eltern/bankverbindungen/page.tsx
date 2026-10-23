"use client";

import { Suspense, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Download, Landmark, Lock } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { ExportDialog } from "~/components/ui/export-dialog";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { TenantPage } from "~/components/ui/tenant-page";
import { formErrorMessage } from "~/components/ui/form-error";
import { useApiLoadError } from "~/contexts/ToastContext";
import { useExportRun } from "~/hooks/use-export-run";
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

// Excel zuerst: die Liste geht meist an die Abrechnung.
const PAYMENT_EXPORT_FORMATS: readonly PaymentExportFormat[] = [
  "xlsx",
  "pdf",
  "docx",
];

// A school has hundreds of children; rendering every row at once costs a long
// scroll on both layouts and a lot of DOM on a phone.
const ROWS_PER_PAGE = 25;

function BankverbindungenContent() {
  const { data: session, status } = useSession({ required: true });
  // Ladefehler ersetzen die Liste (#2517), nie durch „Noch keine Kinder“;
  // der Export meldet Fehler im offenen Export-Dialog.
  const loadError = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = loadError;
  const [loadFailed, setLoadFailed] = useState(false);
  const [reloadToken, setReloadToken] = useState(0);

  const [rows, setRows] = useState<PaymentOverviewRow[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchValue, setSearchValue] = useState("");
  const [completeness, setCompleteness] = useState<CompletenessFilter>("all");
  const [exportOpen, setExportOpen] = useState(false);

  const canRead = hasPermission(session, "guardians:financial");

  // The fetch effect depends on the session state and the retry counter.
  // The error hooks' callbacks are stable, so the list never refetches on an
  // ordinary render.
  useEffect(() => {
    if (status === "loading" || !canRead) return;
    let cancelled = false;
    setIsLoading(true);
    fetchPaymentOverview()
      .then((data) => {
        if (cancelled) return;
        setRows(data);
        setLoadFailed(false);
        clearLoadError();
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("payment_overview_load_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setLoadFailed(true);
        void showLoadError(error, {
          object: "die Liste der Bankverbindungen",
          retry: () => setReloadToken((token) => token + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [status, canRead, reloadToken, showLoadError, clearLoadError]);

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

  // Statuszeile der Kopfkarte: wie vollständig die Liste ist. Der Tabellen-
  // Untertitel sagt stattdessen, wie viel davon gerade zu sehen ist, sobald
  // Suche oder Filter die Liste verkürzen — so widerspricht keine Zahl den
  // Zeilen darunter.
  const statusLine = `${rows.length} Kinder · ${rows.length - missingCount} mit Bankverbindung · ${missingCount} ohne`;
  const captionText =
    visibleRows.length !== rows.length
      ? `${visibleRows.length} von ${rows.length} Kindern`
      : undefined;

  const exportRun = useExportRun({
    isOpen: exportOpen,
    onDownload: exportPaymentOverview,
    onPrint: (target) => exportPaymentOverview("pdf", target),
    errorObject: "das Herunterladen der Bankverbindungen",
    onDone: () => setExportOpen(false),
    logName: "payment_overview",
  });

  const columns: DataTableColumn<PaymentOverviewRow>[] = [
    {
      key: "student",
      header: "Kind",
      stacked: "title",
      render: (row) => (
        <span className="font-medium text-gray-900">{row.studentName}</span>
      ),
      sortValue: (row) => row.studentName,
    },
    {
      key: "class",
      header: "Klasse",
      stacked: "meta",
      render: (row) => (
        <span className="text-gray-600">{row.schoolClass || "–"}</span>
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
      <TenantPage
        title="Bankverbindungen"
        stats="Kein Zugriff"
        empty={{
          icon: <Lock className="h-12 w-12" />,
          title: "Kein Zugriff auf Bankverbindungen",
          description:
            "Sie brauchen dafür die Berechtigung „Bankverbindungen“. Bitte fragen Sie in der Schulleitung nach.",
        }}
      />
    );
  }

  // Bis der Katalogtext da ist, bleibt das Skelett stehen; danach ersetzt der
  // Fehler die Liste, solange nie eine geladen wurde.
  const failedWithoutData = loadFailed && rows.length === 0;
  const awaitingErrorText =
    loadFailed && formErrorMessage(loadError.error) === null;
  const exportUnavailable = isLoading || loadFailed || rows.length === 0;

  return (
    <TenantPage
      title="Bankverbindungen"
      stats={loadFailed ? null : statusLine}
      statsLoading={isLoading || awaitingErrorText}
      error={failedWithoutData ? loadError.error : null}
      search={{
        value: searchValue,
        onChange: setSearchValue,
        placeholder: "Kind, Klasse oder Kontoinhaber suchen",
      }}
      // Die Auswahl „Alle Kinder / Ohne IBAN“ filtert die Liste darunter und
      // gehört deshalb in die Such- und Filterzeile, als Wertauswahl.
      primaryAction={
        <SegmentedControl<CompletenessFilter>
          ariaLabel="Auswahl der angezeigten Kinder"
          value={completeness}
          onChange={setCompleteness}
          items={[
            { value: "all", label: `Alle Kinder (${rows.length})` },
            { value: "missing", label: `Ohne IBAN (${missingCount})` },
          ]}
        />
      }
      // Exportieren steht im Kebab der Kopfkarte; Format und Hinweis zur
      // Datei stehen im Export-Dialog.
      actions={
        <OverflowMenu
          ariaLabel="Weitere Aktionen"
          disabled={exportUnavailable}
          items={[
            {
              label: "Exportieren",
              icon: <Download className="h-4 w-4" aria-hidden />,
              onClick: () => setExportOpen(true),
              disabled: exportUnavailable,
            },
          ]}
        />
      }
    >
      <Alert
        type="info"
        message="Die IBAN tragen Sie beim Kind ein, im Reiter „Erziehungsberechtigte“. Die Datei enthält die ganzen IBANs und wird protokolliert. Bitte nicht per E-Mail weitergeben."
      />

      {/* A four-column table on a phone pushes the IBAN — the one value the
          page exists for — off screen, so the kit table renders its stacked
          phone layout below md. */}
      <DataTable
        columns={columns}
        rows={visibleRows}
        getRowKey={(row) => row.studentId}
        isLoading={isLoading || failedWithoutData}
        defaultSortKey="student"
        caption={captionText}
        pageSize={ROWS_PER_PAGE}
        paginationResetKey={`${completeness}:${searchValue}`}
        emptyState={emptyState}
        stackedOnMobile
      />
      <ExportDialog
        isOpen={exportOpen}
        onClose={() => setExportOpen(false)}
        title="Bankverbindungen exportieren"
        description="Alle Kinder der Schule mit der ganzen IBAN, auch wenn die Liste gerade gefiltert ist. Der Download wird protokolliert."
        formats={PAYMENT_EXPORT_FORMATS}
        run={exportRun}
      />
    </TenantPage>
  );
}

export default function BankverbindungenPage() {
  return (
    <Suspense fallback={null}>
      <BankverbindungenContent />
    </Suspense>
  );
}
