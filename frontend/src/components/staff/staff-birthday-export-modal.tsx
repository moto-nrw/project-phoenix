"use client";

import { useEffect, useState } from "react";

import { ExportDialog } from "~/components/ui/export-dialog";
import { Input } from "~/components/ui/input";
import { ToggleChip } from "~/components/ui/toggle-chip";
import { useExportRun } from "~/hooks/use-export-run";
import {
  exportStaffBirthdays,
  type StaffBirthdayExportFormat,
} from "~/lib/birthdays-api";
import {
  BIRTHDAY_MONTH_OPTIONS,
  currentBirthdayMonth,
} from "~/lib/student-export-api";

const STAFF_BIRTHDAY_FORMATS: readonly StaffBirthdayExportFormat[] = [
  "pdf",
  "docx",
  "xlsx",
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
  const [title, setTitle] = useState("Geburtstagsliste Personal");
  const [months, setMonths] = useState<string[]>([currentBirthdayMonth()]);

  // Ein neu geöffneter Dialog beginnt wieder beim aktuellen Monat.
  useEffect(() => {
    if (!isOpen) return;
    setTitle("Geburtstagsliste Personal");
    setMonths([currentBirthdayMonth()]);
  }, [isOpen]);

  const toggleMonth = (month: string) => {
    setMonths((current) =>
      current.includes(month)
        ? current.filter((item) => item !== month)
        : [...current, month].sort((a, b) => a.localeCompare(b)),
    );
  };

  const run = useExportRun({
    isOpen,
    onDownload: (format: StaffBirthdayExportFormat) =>
      exportStaffBirthdays({ format, title, months }),
    errorObject: "die Geburtstagsliste",
    successMessage: "Die Geburtstagsliste ist erstellt.",
    onDone: onClose,
    logName: "staff_birthdays",
  });

  return (
    <ExportDialog
      isOpen={isOpen}
      onClose={onClose}
      title="Geburtstagsliste Personal exportieren"
      description="Alle Mitarbeitenden mit hinterlegtem Geburtsdatum. Wer kein Datum hinterlegt hat, fehlt in dieser Liste."
      formats={STAFF_BIRTHDAY_FORMATS}
      run={run}
    >
      <section>
        <Input
          id="staff-birthday-export-title"
          label="Titel"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
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
          <ToggleChip
            pressed={months.length === 0}
            onPressedChange={() => setMonths([])}
            className="shrink-0"
          >
            Ganzes Jahr
          </ToggleChip>
        </div>
        <div className="mt-2 grid grid-cols-4 gap-2 sm:grid-cols-6">
          {BIRTHDAY_MONTH_OPTIONS.map((month) => (
            <ToggleChip
              key={month.value}
              pressed={months.includes(month.value)}
              onPressedChange={() => toggleMonth(month.value)}
              ariaLabel={month.label}
            >
              {month.label.slice(0, 3)}
            </ToggleChip>
          ))}
        </div>
      </section>
    </ExportDialog>
  );
}
