"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import { Download, FileSpreadsheet, FileText } from "lucide-react";

import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  exportStaffBirthdays,
  type StaffBirthdayExportFormat,
} from "~/lib/birthdays-api";
import {
  BIRTHDAY_MONTH_OPTIONS,
  currentBirthdayMonth,
} from "~/lib/student-export-api";

const logger = createLogger({ component: "StaffBirthdayExportModal" });

const FORMAT_OPTIONS: Array<{
  value: StaffBirthdayExportFormat;
  label: string;
  icon: ReactNode;
}> = [
  { value: "pdf", label: "PDF", icon: <FileText className="h-4 w-4" /> },
  { value: "docx", label: "DOCX", icon: <FileText className="h-4 w-4" /> },
  {
    value: "xlsx",
    label: "XLSX",
    icon: <FileSpreadsheet className="h-4 w-4" />,
  },
];

/**
 * Staff Geburtstagsliste export (#1542).
 *
 * Same month picker and defaults as the child list so the two feel like one
 * feature, but with no column picker: the document carries name and birth date
 * and nothing else, which is deliberate rather than pending.
 */
export function StaffBirthdayExportModal({
  isOpen,
  onClose,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}) {
  const toast = useToast();
  const [format, setFormat] = useState<StaffBirthdayExportFormat>("pdf");
  const [title, setTitle] = useState("Geburtstagsliste Personal");
  const [months, setMonths] = useState<string[]>([currentBirthdayMonth()]);
  const [exporting, setExporting] = useState(false);

  const toggleMonth = (month: string) => {
    setMonths((current) =>
      current.includes(month)
        ? current.filter((item) => item !== month)
        : [...current, month].sort((a, b) => a.localeCompare(b)),
    );
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      await exportStaffBirthdays({ format, title, months });
      toast.success("Export wurde erstellt.");
      onClose();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Export fehlgeschlagen";
      logger.error("staff_birthday_export_failed", { error: message });
      toast.error(message);
    } finally {
      setExporting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={exporting}
        className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={() => void handleExport()}
        disabled={exporting}
        className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        <Download className="h-4 w-4" aria-hidden />
        {exporting ? "Exportiert..." : "Exportieren"}
      </button>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Geburtstagsliste Personal exportieren"
      closeLabel="Export schließen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      isDismissDisabled={exporting}
      footer={footer}
    >
      <p className="mb-5 text-sm text-gray-500">
        Alle Mitarbeitenden mit hinterlegtem Geburtsdatum. Wer kein Datum
        hinterlegt hat, fehlt in dieser Liste.
      </p>
      <div className="space-y-5">
        <section>
          <label
            htmlFor="staff-birthday-export-title"
            className="text-sm font-medium text-gray-900"
          >
            Titel
          </label>
          <input
            id="staff-birthday-export-title"
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          />
        </section>

        <section>
          <p className="text-sm font-medium text-gray-900">Format</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {FORMAT_OPTIONS.map((item) => (
              <button
                key={item.value}
                type="button"
                onClick={() => setFormat(item.value)}
                className={`inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                  format === item.value
                    ? "border-gray-900 bg-gray-950 text-white"
                    : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                }`}
              >
                {item.icon}
                {item.label}
              </button>
            ))}
          </div>
        </section>

        <section>
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-medium text-gray-900">Geburtsmonate</p>
              <p className="text-xs text-gray-500">
                {months.length === 0
                  ? "Alle Geburtstage des Jahres werden aufgelistet."
                  : "Nur Personen, die in den gewählten Monaten Geburtstag haben."}
              </p>
            </div>
            <button
              type="button"
              onClick={() => setMonths([])}
              aria-pressed={months.length === 0}
              className={`inline-flex h-8 shrink-0 items-center rounded-md border px-2.5 text-xs font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                months.length === 0
                  ? "border-gray-900 bg-gray-950 text-white"
                  : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
              }`}
            >
              Ganzes Jahr
            </button>
          </div>
          <div className="mt-2 grid grid-cols-4 gap-2 sm:grid-cols-6">
            {BIRTHDAY_MONTH_OPTIONS.map((month) => {
              const selected = months.includes(month.value);
              return (
                <button
                  key={month.value}
                  type="button"
                  onClick={() => toggleMonth(month.value)}
                  aria-label={month.label}
                  aria-pressed={selected}
                  className={`inline-flex h-9 items-center justify-center rounded-lg border text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                    selected
                      ? "border-gray-900 bg-gray-950 text-white"
                      : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                  }`}
                >
                  {month.label.slice(0, 3)}
                </button>
              );
            })}
          </div>
        </section>
      </div>
    </Modal>
  );
}
