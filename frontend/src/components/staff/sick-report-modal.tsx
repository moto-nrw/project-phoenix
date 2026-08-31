"use client";

import { useEffect, useState } from "react";

import { formatSignedDuration } from "~/components/staff/staff-time-views";
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
import {
  staffAbsenceService,
  type CompTimeBalancePreview,
  type StaffAbsenceRow,
} from "~/lib/staff-api";
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
  readonly absenceType?: "sick" | "comp_time";
  readonly onClose: () => void;
  // Fired once the absence was created successfully, so the caller can
  // revalidate the Dienst-/Betreuungs-/Vertretungsplan caches (#1843).
  readonly onCreated: (absence: StaffAbsenceRow) => void;
}

export function SickReportModal({
  isOpen,
  staff,
  absenceType = "sick",
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
  // Stundenkonto-Vorschau für Freizeitausgleich (#2873): informiert vor dem
  // Speichern; ein negativer erwarteter Stand verlangt die Bestätigung unten.
  const [preview, setPreview] = useState<CompTimeBalancePreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [overdraftConfirmed, setOverdraftConfirmed] = useState(false);

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
      setPreview(null);
      setOverdraftConfirmed(false);
    }
  }, [isOpen, absenceType]);

  const staffName = `${staff.firstName} ${staff.lastName}`;
  const isCompTime = absenceType === "comp_time";

  // Load the Saldo projection whenever the requested range changes. A changed
  // range also withdraws an already given overdraft confirmation.
  const effectiveEnd = halfDay ? dateStart : dateEnd;
  useEffect(() => {
    if (!isOpen || !isCompTime || !dateStart || !effectiveEnd) {
      return;
    }
    let stale = false;
    setPreviewLoading(true);
    setOverdraftConfirmed(false);
    staffAbsenceService
      .getCompTimePreview(staff.id, {
        dateStart,
        dateEnd: effectiveEnd,
        halfDay,
      })
      .then((result) => {
        if (!stale) setPreview(result);
      })
      .catch((err: unknown) => {
        if (stale) return;
        // Vorschau ist rein informativ: ein Ladefehler blendet den Block aus,
        // die Buchung selbst bleibt möglich und meldet ihre eigenen Fehler.
        setPreview(null);
        logger.error("comp_time_preview_failed", {
          staff_id: staff.id,
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => {
        if (!stale) setPreviewLoading(false);
      });
    return () => {
      stale = true;
    };
  }, [isOpen, isCompTime, staff.id, dateStart, effectiveEnd, halfDay]);

  const isOverdraft =
    isCompTime && preview !== null && preview.projectedBalanceMinutes < 0;

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
        absence_type: absenceType,
        date_start: dateStart,
        date_end: halfDay ? dateStart : dateEnd,
        half_day: halfDay || undefined,
        note: note.trim() || undefined,
      });
      onCreated(created);
      setCreatedStart(dateStart);
    } catch (err) {
      logger.error("managed_absence_create_failed", {
        staff_id: staff.id,
        absence_type: absenceType,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error && err.message
          ? err.message
          : isCompTime
            ? "Freizeitausgleich konnte nicht eingetragen werden."
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
      {!isCompTime && (
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={goToVertretung}
        >
          Zur Vertretung
        </Button>
      )}
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
        loadingText={isCompTime ? "Wird eingetragen…" : "Wird gemeldet…"}
        disabled={
          submitting ||
          !dateStart ||
          (!halfDay && !dateEnd) ||
          (isOverdraft && !overdraftConfirmed)
        }
      >
        {isCompTime ? "Freizeitausgleich eintragen" : "Krank melden"}
      </Button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`${isCompTime ? "Freizeitausgleich eintragen" : "Krank melden"}: ${staffName}`}
      size="md"
      footer={footer}
    >
      {isSuccess ? (
        <div className="space-y-3 text-sm">
          <Alert
            type="success"
            message={
              isCompTime
                ? `${staffName} hat ab dem ${formatDate(createdStart)} Freizeitausgleich. Dafür wird keine Sollzeit gutgeschrieben.${halfDay ? " Die gearbeitete Hälfte muss separat erfasst sein." : ""}${preview ? ` Das Stundenkonto liegt danach bei ${formatSignedDuration(preview.projectedBalanceMinutes)}.` : ""}`
                : halfDay
                  ? `${staffName} ist am ${formatDate(createdStart)} für einen halben Tag krank gemeldet. Dienst- und Betreuungsplan wurden nicht automatisch geändert.`
                  : `${staffName} ist ab dem ${formatDate(createdStart)} als krank gemeldet. Reguläre Schichten wurden storniert und Betreuungsblöcke als abwesend markiert.`
            }
          />
          {!isCompTime && (
            <p className="text-gray-600">
              Öffne die Vertretung, um offene Betreuungsblöcke neu zu besetzen.
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <p className="text-sm text-gray-600">
            {isCompTime ? (
              <>
                Der Freizeitausgleich reduziert das Stundenkonto von {staffName}{" "}
                um das Tagessoll im gewählten Zeitraum. Bei einem halben Tag
                muss die andere Hälfte als Arbeitszeit erfasst sein; nicht
                erfasste Zeit bleibt als Minus bestehen.
              </>
            ) : (
              <>
                Die Krankmeldung storniert die geplanten Schichten von{" "}
                {staffName} im gewählten Zeitraum und markiert betroffene
                Betreuungsblöcke als abwesend. Bereits eingetragene
                Vertretungsschichten müssen manuell geprüft werden.
              </>
            )}
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
              htmlFor="managed-absence-half-day"
              className="flex items-center gap-2"
            >
              <Checkbox
                id="managed-absence-half-day"
                checked={halfDay}
                onChange={(e) => handleHalfDayChange(e.target.checked)}
              />
              <span className="text-sm font-medium text-gray-800">
                Halber Tag
              </span>
            </label>
            <p className="mt-1 text-xs text-gray-500">
              {isCompTime
                ? "Halbe Tage gelten für ein einzelnes Datum. Die gearbeitete Hälfte muss separat erfasst werden."
                : "Halbe Tage gelten für ein einzelnes Datum und ändern den Dienst- und Betreuungsplan nicht automatisch."}
            </p>
          </div>

          {isCompTime && (preview !== null || previewLoading) && (
            <div className="rounded-lg bg-gray-50 p-3">
              {preview === null ? (
                <p className="text-sm text-gray-500">
                  Stundenkonto wird berechnet …
                </p>
              ) : (
                <>
                  <dl className="space-y-1 text-sm">
                    <div className="flex items-center justify-between gap-4">
                      <dt className="text-gray-600">Stundenkonto aktuell</dt>
                      <dd className="font-medium text-gray-900 tabular-nums">
                        {formatSignedDuration(preview.currentBalanceMinutes)}
                      </dd>
                    </div>
                    {preview.futureCommitmentMinutes > 0 && (
                      <div className="flex items-center justify-between gap-4">
                        <dt className="text-gray-600">
                          Bereits geplanter Freizeitausgleich
                        </dt>
                        <dd className="font-medium text-gray-900 tabular-nums">
                          {formatSignedDuration(
                            -preview.futureCommitmentMinutes,
                          )}
                        </dd>
                      </div>
                    )}
                    <div className="flex items-center justify-between gap-4">
                      <dt className="text-gray-600">
                        Abzug für diesen Eintrag
                      </dt>
                      <dd className="font-medium text-gray-900 tabular-nums">
                        {formatSignedDuration(-preview.deductionMinutes)}
                      </dd>
                    </div>
                    <div className="flex items-center justify-between gap-4 border-t border-gray-200 pt-1.5">
                      <dt className="font-medium text-gray-900">
                        Stundenkonto danach
                      </dt>
                      <dd
                        className={`font-semibold tabular-nums ${
                          preview.projectedBalanceMinutes < 0
                            ? "text-moto-red-strong"
                            : "text-gray-900"
                        }`}
                      >
                        {formatSignedDuration(preview.projectedBalanceMinutes)}
                      </dd>
                    </div>
                  </dl>
                  {preview.realizedDeductionMinutes > 0 && (
                    <p className="mt-2 text-xs text-gray-500">
                      Tage bis heute sind im aktuellen Stand schon enthalten.
                    </p>
                  )}
                </>
              )}
            </div>
          )}

          {isOverdraft && (
            <div className="space-y-3">
              <Alert
                type="warning"
                message={`Das Stundenkonto von ${staffName} fällt damit unter null.`}
              />
              <label
                htmlFor="comp-time-overdraft-confirm"
                className="flex items-center gap-2"
              >
                <Checkbox
                  id="comp-time-overdraft-confirm"
                  checked={overdraftConfirmed}
                  onChange={(e) => setOverdraftConfirmed(e.target.checked)}
                />
                <span className="text-sm font-medium text-gray-800">
                  Ich trage den Freizeitausgleich trotzdem ein.
                </span>
              </label>
            </div>
          )}

          <Input
            name="managed-absence-note"
            label="Notiz (optional)"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder={
              isCompTime
                ? "z. B. Überstundenabbau …"
                : "z. B. Grippe, Arzttermin …"
            }
          />

          {error && <Alert type="error" message={error} />}
        </div>
      )}
    </FormModal>
  );
}
