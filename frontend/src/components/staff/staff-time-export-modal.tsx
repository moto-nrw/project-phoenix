"use client";

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { formatOverviewMonth } from "~/components/staff/staff-time-accounts-table";
import {
  DatevConfigIncompleteError,
  fetchDatevExportReport,
  type DatevExportReport,
  type DatevFormat,
} from "~/lib/datev-export-api";
import {
  fetchSFTPStatus,
  transferExportViaSFTP,
  transferFailureMessage,
  type SFTPStatus,
  type TransferOutcome,
} from "~/lib/sftp-export-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffTimeExportModal" });

type Scope = "month" | "year";
type Granularity = "month" | "day";
type FileFormat = "csv" | "xlsx" | DatevFormat;
type TimeFormat = "hhmm" | "decimal";
/** Wohin die Datei geht: auf das eigene Gerät oder an die Gegenstelle. */
type Delivery = "download" | "sftp";
type TransferState =
  | { readonly status: "idle" }
  | { readonly status: "running" }
  | { readonly status: "done"; readonly outcome: TransferOutcome }
  | { readonly status: "error" };
type DatevReportState =
  | { readonly status: "idle"; readonly requestKey: null }
  | { readonly status: "loading"; readonly requestKey: string }
  | {
      readonly status: "ready";
      readonly requestKey: string;
      readonly report: DatevExportReport;
    }
  | { readonly status: "config-incomplete"; readonly requestKey: string }
  | { readonly status: "failed"; readonly requestKey: string };

const isDatevFormat = (format: FileFormat): format is DatevFormat =>
  format === "datev_lodas" || format === "datev_lug";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** Der in der Tabelle angezeigte Monat — Vorauswahl des Zeitraums. */
  readonly year: number;
  readonly month: number;
}

// Chip-Gruppe im Stil der Saldo-Presets der Zeitkonten-Tabelle: sichtbare
// Auswahl statt Dropdown, weil jede Gruppe nur wenige Optionen hat.
function OptionGroup<T extends string>({
  label,
  options,
  value,
  onChange,
  disabled,
}: {
  readonly label: string;
  readonly options: readonly { id: T; label: string }[];
  readonly value: T;
  readonly onChange: (value: T) => void;
  readonly disabled?: boolean;
}) {
  return (
    <div>
      <p className="mb-1.5 text-sm font-medium text-gray-700">{label}</p>
      <div className="flex flex-wrap gap-2">
        {options.map((option) => (
          <Button
            key={option.id}
            type="button"
            size="compact"
            variant={value === option.id && !disabled ? "primary" : "outline"}
            disabled={disabled}
            onClick={() => onChange(option.id)}
          >
            {option.label}
          </Button>
        ))}
      </div>
    </div>
  );
}

// Vorab-Bericht für die DATEV-Formate: dieselbe Berechnung wie die Datei,
// sichtbar VOR dem Download. Eine still unvollständige Lohndatei darf es
// nicht geben — fehlende Personalnummern sperren den Export.
function DatevReportPanel({
  state,
  onRetry,
}: {
  readonly state: DatevReportState;
  readonly onRetry: () => void;
}) {
  if (state.status === "idle" || state.status === "loading") {
    return <p className="text-sm text-gray-500">Bericht wird geladen …</p>;
  }
  if (state.status === "config-incomplete") {
    return (
      <Alert
        type="error"
        message="Die DATEV-Konfiguration ist unvollständig (Lohnarten bzw. Berater-/Mandantennummer). Ohne vollständige Konfiguration wird keine Datei erzeugt. Die Pflege erfolgt auf der Seite Abrechnung."
      />
    );
  }
  if (state.status === "failed") {
    return (
      <div className="space-y-2">
        <Alert
          type="error"
          message="Der DATEV-Bericht konnte nicht geladen werden. Ohne erfolgreichen Vorab-Bericht ist der Export gesperrt."
        />
        <Button
          type="button"
          size="compact"
          variant="outline"
          onClick={onRetry}
        >
          Bericht erneut laden
        </Button>
      </div>
    );
  }
  const { report } = state;
  return (
    <div className="space-y-2 rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
      <p>
        {report.lineCount} Buchungszeilen für {report.staffExported}{" "}
        Mitarbeitende.
      </p>
      {report.staffSkipped.length > 0 && (
        <div>
          <p className="text-moto-orange font-medium">
            {`Export gesperrt: Personalnummer fehlt (${report.staffSkipped.length}):`}
          </p>
          <ul className="list-inside list-disc">
            {report.staffSkipped.map((entry) => (
              <li key={`${entry.lastName}-${entry.firstName}`}>
                {entry.lastName}, {entry.firstName}
              </li>
            ))}
          </ul>
        </div>
      )}
      {report.unconfiguredCategories.length > 0 && (
        <p>
          Ohne Lohnartnummer, daher keine Zeilen:{" "}
          {report.unconfiguredCategories.join(", ")}
        </p>
      )}
      {report.openMonth && (
        <p className="text-moto-orange font-medium">
          Der Monat ist noch nicht abgeschlossen; die Werte sind ein
          Zwischenstand, nicht final.
        </p>
      )}
    </div>
  );
}

// Ergebnis der Übertragung. Ein Fehlschlag wird als Fehlschlag gezeigt — das
// Backend antwortet auch dann mit 200, damit der Eintrag im Protokoll erhalten
// bleibt, also entscheidet hier `transferred`, nicht der HTTP-Status.
function TransferResultPanel({ state }: { readonly state: TransferState }) {
  if (state.status === "idle") return null;
  if (state.status === "running") {
    return <p className="text-sm text-gray-500">Datei wird übertragen …</p>;
  }
  if (state.status === "error") {
    return (
      <Alert
        type="error"
        message="Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal."
      />
    );
  }
  const { outcome } = state;
  if (!outcome.transferred) {
    return (
      <Alert type="error" message={transferFailureMessage(outcome.reason)} />
    );
  }
  return (
    <Alert
      type="success"
      message={`${outcome.filename} wurde übertragen${
        outcome.targetHost ? ` an ${outcome.targetHost}` : ""
      }${outcome.targetDirectory ? `, Ordner ${outcome.targetDirectory}` : ""}.`}
    />
  );
}

// Export-Dialog der Zeitkonten (#1417 2b). Baut nur die Query — die Zahlen
// kommen vollständig aus dem Backend (dieselbe Monatslogik wie die Tabelle);
// der Download läuft über die Streaming-Proxy-Route.
//
// Die Übertragung an die Gegenstelle (#3050) nutzt dieselbe Auswahl und
// dieselbe Datei; sie erzeugt keinen zweiten Export.
export function StaffTimeExportModal({ isOpen, onClose, year, month }: Props) {
  const [scope, setScope] = useState<Scope>("month");
  const [granularity, setGranularity] = useState<Granularity>("month");
  const [format, setFormat] = useState<FileFormat>("csv");
  const [timeFormat, setTimeFormat] = useState<TimeFormat>("hhmm");
  const [delivery, setDelivery] = useState<Delivery>("download");
  const [sftpStatus, setSftpStatus] = useState<SFTPStatus | null>(null);
  const [transferState, setTransferState] = useState<TransferState>({
    status: "idle",
  });

  // Der Status entscheidet, ob die Übertragung überhaupt angeboten wird. Ohne
  // ihn bleibt es beim Download — eine Schaltfläche, die nichts tun kann, ist
  // schlimmer als keine.
  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    fetchSFTPStatus()
      .then((status) => {
        if (!cancelled) setSftpStatus(status);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("sftp_status_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setSftpStatus({ enabled: false, ready: false, missingSettings: [] });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  // Auswahl zurücksetzen, sobald die Übertragung nicht mehr möglich ist.
  useEffect(() => {
    if (sftpStatus && !sftpStatus.ready && delivery === "sftp") {
      setDelivery("download");
    }
  }, [sftpStatus, delivery]);

  const datev = isDatevFormat(format);
  const reportRequestKey = datev ? `${format}:${year}:${month}` : null;
  const [reportState, setReportState] = useState<DatevReportState>({
    status: "idle",
    requestKey: null,
  });
  const [reportAttempt, setReportAttempt] = useState(0);
  const activeReportState: DatevReportState =
    reportRequestKey !== null && reportState.requestKey !== reportRequestKey
      ? { status: "loading", requestKey: reportRequestKey }
      : reportState;

  useEffect(() => {
    if (!isOpen || !isDatevFormat(format)) {
      setReportState({ status: "idle", requestKey: null });
      return;
    }
    const requestKey = `${format}:${year}:${month}`;
    let cancelled = false;
    setReportState({ status: "loading", requestKey });
    fetchDatevExportReport(year, month, format)
      .then((result) => {
        if (!cancelled) {
          setReportState({ status: "ready", requestKey, report: result });
        }
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        if (error instanceof DatevConfigIncompleteError) {
          setReportState({ status: "config-incomplete", requestKey });
          return;
        }
        logger.error("datev_report_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setReportState({ status: "failed", requestKey });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, format, year, month, reportAttempt]);

  const handleExport = () => {
    if (
      datev &&
      (activeReportState.status !== "ready" ||
        activeReportState.report.staffSkipped.length > 0)
    ) {
      return;
    }
    const params = new URLSearchParams({ year: String(year), format });
    if (scope === "month" || datev) {
      params.set("month", String(month));
    }
    if (!datev) {
      params.set("granularity", granularity);
      if (granularity === "month") {
        params.set("time_format", timeFormat);
      }
    }
    globalThis.location.href = `/api/staff/time-tracking/export?${params.toString()}`;
    onClose();
  };

  // Die Übertragung nutzt dieselbe Auswahl. Der Dialog bleibt danach offen,
  // damit das Ergebnis sichtbar ist — auch ein Fehlschlag.
  const handleTransfer = async () => {
    setTransferState({ status: "running" });
    try {
      const outcome = await transferExportViaSFTP({
        year,
        month,
        format,
        wholeYear: !datev && scope === "year",
        granularity: datev ? undefined : granularity,
        timeFormat: !datev && granularity === "month" ? timeFormat : undefined,
      });
      setTransferState({ status: "done", outcome });
    } catch (error: unknown) {
      logger.error("sftp_transfer_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      setTransferState({ status: "error" });
    }
  };

  const datevBlocked =
    datev &&
    (activeReportState.status !== "ready" ||
      activeReportState.report.staffSkipped.length > 0);
  const transferring = transferState.status === "running";
  const exportDisabled = datevBlocked || transferring;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Zeitkonten exportieren"
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" size="md" variant="outline" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            size="md"
            variant="primary"
            disabled={exportDisabled}
            onClick={delivery === "sftp" ? handleTransfer : handleExport}
          >
            {delivery === "sftp"
              ? transferring
                ? "Wird übertragen …"
                : "Übertragen"
              : "Exportieren"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          Export für alle Mitarbeitenden. Die Werte entsprechen der
          Zeitkonten-Tabelle; abgeschlossene Monate tragen den eingefrorenen
          Übertrag. Jeder Export wird im Zugriffsprotokoll vermerkt.
        </p>
        <OptionGroup
          label="Zeitraum"
          options={[
            { id: "month", label: formatOverviewMonth(year, month) },
            { id: "year", label: `Gesamtes Jahr ${year}` },
          ]}
          value={datev ? "month" : scope}
          onChange={setScope}
          disabled={datev}
        />
        <OptionGroup
          label="Detailgrad"
          options={[
            { id: "month", label: "Monatssummen" },
            { id: "day", label: "Einzelne Tage" },
          ]}
          value={datev ? "month" : granularity}
          onChange={setGranularity}
          disabled={datev}
        />
        <OptionGroup
          label="Dateiformat"
          options={[
            { id: "csv", label: "CSV" },
            { id: "xlsx", label: "Excel" },
            { id: "datev_lodas", label: "DATEV LODAS" },
            { id: "datev_lug", label: "DATEV Lohn und Gehalt" },
          ]}
          value={format}
          onChange={setFormat}
        />
        {/*
          Ohne eingeschaltete Schnittstelle gibt es nichts zu wählen: Der
          Download ist dann der einzige Weg, und eine Gruppe "Wohin" mit einer
          einzigen Möglichkeit stellt eine Frage, die keine ist. Erst der
          Schalter in den Einstellungen bringt die Auswahl hervor.

          Eingeschaltet, aber unvollständig, wird dagegen gezeigt — dort hat
          jemand die Übertragung gewollt, und ein stiller Rückfall auf den
          Download würde die halbfertige Einrichtung unsichtbar machen.
        */}
        {sftpStatus?.enabled && (
          <div>
            <OptionGroup
              label="Wohin"
              options={[
                { id: "download", label: "Herunterladen" },
                { id: "sftp", label: "An die Gegenstelle übertragen" },
              ]}
              value={delivery}
              onChange={setDelivery}
              disabled={!sftpStatus.ready}
            />
            {!sftpStatus.ready && (
              <p className="mt-1.5 text-xs text-gray-500">
                Die Übertragung ist eingeschaltet, aber noch nicht vollständig
                eingerichtet. Ein Admin kann die fehlenden Angaben in den
                Einstellungen unter `System` im Bereich `Schnittstellen`
                ergänzen. Solange laden Sie die Datei herunter.
              </p>
            )}
            {delivery === "sftp" && sftpStatus.host && (
              <p className="mt-1.5 text-xs text-gray-500">
                Die Datei geht an {sftpStatus.host}
                {sftpStatus.remoteDirectory
                  ? `, Ordner ${sftpStatus.remoteDirectory}`
                  : ""}
                . Es ist dieselbe Datei wie beim Herunterladen.
              </p>
            )}
          </div>
        )}
        {transferState.status !== "idle" && (
          <TransferResultPanel state={transferState} />
        )}
        {!datev && (
          <OptionGroup
            label="Zeitangaben"
            options={[
              { id: "hhmm", label: "Stunden:Minuten" },
              { id: "decimal", label: "Dezimalstunden" },
            ]}
            value={timeFormat}
            onChange={setTimeFormat}
            disabled={granularity === "day"}
          />
        )}
        {!datev && granularity === "day" && (
          <p className="text-xs text-gray-500">
            Der Tages-Export übernimmt die Darstellung des Einzel-Exports; das
            Zeitformat gilt nur für Monatssummen.
          </p>
        )}
        {datev && (
          <div className="space-y-3">
            <p className="text-xs text-gray-500">
              Export im DATEV-Format (Bewegungsdaten, ANSI-kodiert, ein
              Abrechnungsmonat pro Datei). Stimmen Sie die erste Datei vor dem
              Echtlauf mit Ihrer Lohnbuchhaltung ab: die Zuordnung der Lohnarten
              und bei Lohn und Gehalt die Importbeschreibung legt das Lohnbüro
              fest.
            </p>
            <DatevReportPanel
              state={activeReportState}
              onRetry={() => setReportAttempt((attempt) => attempt + 1)}
            />
          </div>
        )}
      </div>
    </Modal>
  );
}
