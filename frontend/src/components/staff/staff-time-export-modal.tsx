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
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffTimeExportModal" });

type Scope = "month" | "year";
type Granularity = "month" | "day";
type FileFormat = "csv" | "xlsx" | DatevFormat;
type TimeFormat = "hhmm" | "decimal";
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
          <p className="font-medium text-[#F78C10]">
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
        <p className="font-medium text-[#F78C10]">
          Der Monat ist noch nicht abgeschlossen; die Werte sind ein
          Zwischenstand, nicht final.
        </p>
      )}
    </div>
  );
}

// Export-Dialog der Zeitkonten (#1417 2b). Baut nur die Query — die Zahlen
// kommen vollständig aus dem Backend (dieselbe Monatslogik wie die Tabelle);
// der Download läuft über die Streaming-Proxy-Route.
export function StaffTimeExportModal({ isOpen, onClose, year, month }: Props) {
  const [scope, setScope] = useState<Scope>("month");
  const [granularity, setGranularity] = useState<Granularity>("month");
  const [format, setFormat] = useState<FileFormat>("csv");
  const [timeFormat, setTimeFormat] = useState<TimeFormat>("hhmm");

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

  const exportDisabled =
    datev &&
    (activeReportState.status !== "ready" ||
      activeReportState.report.staffSkipped.length > 0);

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
            onClick={handleExport}
          >
            Exportieren
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
