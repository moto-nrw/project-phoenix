"use client";

import { useEffect, useState } from "react";

import { ISODatePicker } from "~/components/ui/date-picker";
import { ExportDialog } from "~/components/ui/export-dialog";
import { useExportRun } from "~/hooks/use-export-run";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import {
  downloadBlob,
  filenameFromDisposition,
  openBlobForPrint,
} from "~/lib/file-download";

type AttendanceExportFormat = "pdf" | "docx" | "xlsx";

const ATTENDANCE_EXPORT_FORMATS: readonly AttendanceExportFormat[] = [
  "pdf",
  "docx",
  "xlsx",
];

function shiftISODate(iso: string, days: number): string {
  const date = parseISODate(iso);
  date.setDate(date.getDate() + days);
  return toISODate(date);
}

async function downloadAttendanceExport(
  studentId: string,
  from: string,
  to: string,
  format: AttendanceExportFormat,
  printTarget?: Window,
): Promise<void> {
  const params = new URLSearchParams({ format, from, to });
  const response = await transportFetch(
    `/api/students/${encodeURIComponent(studentId)}/attendance-history/export?${params.toString()}`,
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "attendance export failed");
  }
  const blob = await response.blob();
  if (printTarget) {
    openBlobForPrint(blob, printTarget);
    return;
  }
  downloadBlob(
    blob,
    filenameFromDisposition(response) ?? `anwesenheit.${format}`,
  );
}

/**
 * Export of one child's Anwesenheitsprotokoll (#4748). The period can only
 * cover the days the page shows: the school sets how many
 * (`visibleDays`), and nothing lies in the future. The backend checks the
 * same limits.
 */
export function AttendanceExportDialog({
  isOpen,
  onClose,
  studentId,
  studentName,
  today,
  visibleDays,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly studentId: string;
  readonly studentName: string;
  /** Today in Berlin, "YYYY-MM-DD". */
  readonly today: string;
  /** How many days back the school shows the protocol. */
  readonly visibleDays: number;
}) {
  const earliest = shiftISODate(today, -(Math.max(visibleDays, 1) - 1));
  const [from, setFrom] = useState(earliest);
  const [to, setTo] = useState(today);

  // Jeder neue Export beginnt mit allen sichtbaren Tagen.
  useEffect(() => {
    if (!isOpen) return;
    setFrom(earliest);
    setTo(today);
  }, [isOpen, earliest, today]);

  const rangeError = (() => {
    if (!from || !to) return "Bitte wählen Sie Von und Bis aus.";
    if (to < from) return "Das Bis-Datum liegt vor dem Von-Datum.";
    if (from < earliest || to > today)
      return `Es lassen sich nur die letzten ${visibleDays} Tage exportieren.`;
    return null;
  })();

  const run = useExportRun({
    isOpen,
    onDownload: (format: AttendanceExportFormat) =>
      downloadAttendanceExport(studentId, from, to, format),
    onPrint: (target) =>
      downloadAttendanceExport(studentId, from, to, "pdf", target),
    validate: () => rangeError,
    errorObject: "die Exportdatei",
    onDone: onClose,
    logName: "attendance_history",
  });

  return (
    <ExportDialog
      isOpen={isOpen}
      onClose={onClose}
      title="Anwesenheitsprotokoll exportieren"
      description={
        studentName
          ? `Anwesenheit von ${studentName} im gewählten Zeitraum.`
          : "Anwesenheit des Kindes im gewählten Zeitraum."
      }
      formats={ATTENDANCE_EXPORT_FORMATS}
      run={run}
      blocked={rangeError !== null}
    >
      <section>
        <p className="text-sm font-medium text-gray-900">Zeitraum</p>
        <div className="mt-2 grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label
              htmlFor="attendance-export-from"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Von
            </label>
            <ISODatePicker
              id="attendance-export-from"
              controlSize="lg"
              value={from}
              min={earliest}
              max={to || today}
              onChange={setFrom}
              calendarLayout="popover"
            />
          </div>
          <div>
            <label
              htmlFor="attendance-export-to"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Bis
            </label>
            <ISODatePicker
              id="attendance-export-to"
              controlSize="lg"
              value={to}
              min={from || earliest}
              max={today}
              onChange={setTo}
              calendarLayout="popover"
            />
          </div>
        </div>
        <p className="mt-2 text-xs text-gray-500">
          {rangeError ??
            `Möglich sind die Tage vom ${formatDate(earliest)} bis heute.`}
        </p>
      </section>
    </ExportDialog>
  );
}
