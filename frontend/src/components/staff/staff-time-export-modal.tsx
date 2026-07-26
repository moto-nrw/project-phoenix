"use client";

import { useState } from "react";

import { Button } from "~/components/ui/button";
import { Modal } from "~/components/ui/modal";
import { formatOverviewMonth } from "~/components/staff/staff-time-accounts-table";

type Scope = "month" | "year";
type Granularity = "month" | "day";
type FileFormat = "csv" | "xlsx";
type TimeFormat = "hhmm" | "decimal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** Der in der Tabelle angezeigte Monat — Vorauswahl des Zeitraums. */
  readonly year: number;
  readonly month: number;
}

// Chip-Gruppe im Stil der Saldo-Presets der Zeitkonten-Tabelle: sichtbare
// Auswahl statt Dropdown, weil jede Gruppe nur zwei Optionen hat.
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

// Export-Dialog der Zeitkonten (#1417 2b). Baut nur die Query — die Zahlen
// kommen vollständig aus dem Backend (dieselbe Monatslogik wie die Tabelle);
// der Download läuft über die Streaming-Proxy-Route.
export function StaffTimeExportModal({ isOpen, onClose, year, month }: Props) {
  const [scope, setScope] = useState<Scope>("month");
  const [granularity, setGranularity] = useState<Granularity>("month");
  const [format, setFormat] = useState<FileFormat>("csv");
  const [timeFormat, setTimeFormat] = useState<TimeFormat>("hhmm");

  const handleExport = () => {
    const params = new URLSearchParams({ year: String(year), format });
    if (scope === "month") {
      params.set("month", String(month));
    }
    params.set("granularity", granularity);
    if (granularity === "month") {
      params.set("time_format", timeFormat);
    }
    globalThis.location.href = `/api/staff/time-tracking/export?${params.toString()}`;
    onClose();
  };

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
          value={scope}
          onChange={setScope}
        />
        <OptionGroup
          label="Detailgrad"
          options={[
            { id: "month", label: "Monatssummen" },
            { id: "day", label: "Einzelne Tage" },
          ]}
          value={granularity}
          onChange={setGranularity}
        />
        <OptionGroup
          label="Dateiformat"
          options={[
            { id: "csv", label: "CSV" },
            { id: "xlsx", label: "Excel" },
          ]}
          value={format}
          onChange={setFormat}
        />
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
        {granularity === "day" && (
          <p className="text-xs text-gray-500">
            Der Tages-Export übernimmt die Darstellung des Einzel-Exports; das
            Zeitformat gilt nur für Monatssummen.
          </p>
        )}
      </div>
    </Modal>
  );
}
