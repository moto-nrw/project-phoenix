"use client";

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { DatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import {
  formatDate,
  parseISODate,
  todayISO,
  toISODate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { staffAbsenceService, type StaffAbsenceRow } from "~/lib/staff-api";
import { useTenantRouter } from "~/lib/tenant-router";

const logger = createLogger({ component: "SickReportModal" });

// Minimal staff shape the modal needs. Both StaffScheduleStaff (Dienstplan)
// and the full Staff type (staff detail page) satisfy it structurally, so the
// same modal serves both entry points (#1843).
export interface SickReportStaff {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
}

interface SickReportModalProps {
  readonly isOpen: boolean;
  readonly staff: SickReportStaff;
  readonly onClose: () => void;
  // Fired once the absence was created successfully, so the caller can
  // revalidate the Dienst-/Betreuungs-/Vertretungsplan caches (#1843).
  readonly onCreated: (absence: StaffAbsenceRow) => void;
}

export function SickReportModal({
  isOpen,
  staff,
  onClose,
  onCreated,
}: SickReportModalProps) {
  const router = useTenantRouter();
  const [dateStart, setDateStart] = useState(() => todayISO());
  const [dateEnd, setDateEnd] = useState(() => todayISO());
  const [halfDay, setHalfDay] = useState(false);
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  // Non-null after a successful create — holds the start date the "Zur
  // Vertretung" jump should open the day view on.
  const [createdStart, setCreatedStart] = useState<string | null>(null);

  // Reset the form each time the modal is opened.
  useEffect(() => {
    if (isOpen) {
      const today = todayISO();
      setDateStart(today);
      setDateEnd(today);
      setHalfDay(false);
      setNote("");
      setError(null);
      setCreatedStart(null);
      setSubmitting(false);
    }
  }, [isOpen]);

  const staffName = `${staff.firstName} ${staff.lastName}`;

  const handleStartDateChange = (picked: Date | null) => {
    const nextStart = picked ? toISODate(picked) : "";
    setDateStart(nextStart);
    if (nextStart && (halfDay || !dateEnd || dateEnd < nextStart)) {
      setDateEnd(nextStart);
    }
  };

  const handleHalfDayChange = (checked: boolean) => {
    setHalfDay(checked);
    if (checked && dateStart) {
      setDateEnd(dateStart);
    }
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await staffAbsenceService.createAbsence(staff.id, {
        absence_type: "sick",
        date_start: dateStart,
        date_end: halfDay ? dateStart : dateEnd,
        half_day: halfDay || undefined,
        note: note.trim() || undefined,
      });
      onCreated(created);
      setCreatedStart(dateStart);
    } catch (err) {
      logger.error("sick_report_failed", {
        staff_id: staff.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error && err.message
          ? err.message
          : "Krankmeldung fehlgeschlagen.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  const goToVertretung = () => {
    const day = createdStart ?? dateStart;
    // Neues Drei-Parameter-Vokabular von /vertretung (d/block/verlauf):
    // `?view=day&day=` stammte aus der alten vertretungsplan-view und wird
    // von der neuen Ansicht ignoriert — sie landete immer auf heute statt
    // auf dem Starttag der Krankmeldung.
    router.push(`/vertretung?d=${day}`);
    onClose();
  };

  const isSuccess = createdStart !== null;

  const footer = isSuccess ? (
    <>
      <Button type="button" variant="outline" size="md" onClick={onClose}>
        Schließen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={goToVertretung}
      >
        Zur Vertretung
      </Button>
    </>
  ) : (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        disabled={submitting}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={handleSubmit}
        isLoading={submitting}
        loadingText="Wird gemeldet…"
        disabled={submitting || !dateStart || (!halfDay && !dateEnd)}
      >
        Krank melden
      </Button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Krank melden: ${staffName}`}
      size="md"
      footer={footer}
    >
      {isSuccess ? (
        <div className="space-y-3 text-sm">
          <Alert
            type="success"
            message={
              halfDay
                ? `${staffName} ist am ${formatDate(createdStart)} für einen halben Tag krank gemeldet. Dienst- und Betreuungsplan wurden nicht automatisch geändert.`
                : `${staffName} ist ab dem ${formatDate(createdStart)} als krank gemeldet. Reguläre Schichten wurden storniert und Betreuungsblöcke als abwesend markiert.`
            }
          />
          <p className="text-gray-600">
            Öffne die Vertretung, um offene Betreuungsblöcke neu zu besetzen.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            Die Krankmeldung storniert die geplanten Schichten von {staffName}{" "}
            im gewählten Zeitraum und markiert betroffene Betreuungsblöcke als
            abwesend. Bereits eingetragene Vertretungsschichten müssen manuell
            geprüft werden.
          </p>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className={halfDay ? "sm:col-span-2" : undefined}>
              <span className="mb-2 block text-sm font-medium text-gray-700">
                {halfDay ? "Datum" : "Von"}
              </span>
              <DatePicker
                value={dateStart ? parseISODate(dateStart) : null}
                onChange={handleStartDateChange}
                dropdownPlacement="down"
                placeholder="Datum auswählen"
              />
            </div>
            {!halfDay && (
              <div>
                <span className="mb-2 block text-sm font-medium text-gray-700">
                  Bis
                </span>
                <DatePicker
                  value={dateEnd ? parseISODate(dateEnd) : null}
                  onChange={(picked) =>
                    setDateEnd(picked ? toISODate(picked) : "")
                  }
                  dropdownPlacement="down"
                  placeholder="Datum auswählen"
                />
              </div>
            )}
          </div>

          {/* FieldGroup, not <label>: wrapping the Checkbox row in a label
              would re-dispatch a click onto the first labelable child. A
              dedicated <label> around only the checkbox keeps the toggle
              accessible without swallowing the hint text. */}
          <div>
            <label
              htmlFor="sick-report-half-day"
              className="flex items-center gap-2"
            >
              <Checkbox
                id="sick-report-half-day"
                checked={halfDay}
                onChange={(e) => handleHalfDayChange(e.target.checked)}
              />
              <span className="text-sm font-medium text-gray-800">
                Halber Tag
              </span>
            </label>
            <p className="mt-1 text-xs text-gray-500">
              Halbe Tage gelten für ein einzelnes Datum und ändern den Dienst-
              und Betreuungsplan nicht automatisch.
            </p>
          </div>

          <Input
            name="sick-report-note"
            label="Notiz (optional)"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="z. B. Grippe, Arzttermin …"
          />

          {error && <Alert type="error" message={error} />}
        </div>
      )}
    </FormModal>
  );
}
