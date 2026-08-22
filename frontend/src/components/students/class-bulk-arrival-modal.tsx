"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";
import { createLogger } from "~/lib/logger";
import {
  type ArrivalScheduleInput,
  type BulkArrivalFilter,
  WEEKDAYS,
  bulkUpsertArrivalSchedules,
  fetchBulkArrivalScheduleStatus,
  fetchClassArrivalTimes,
} from "~/lib/student-arrival-api";
import { formatDate } from "~/lib/date-helpers";
import { stripClassPrefix } from "~/lib/arrival-schedule-helpers";
import { cn } from "~/lib/utils";

const logger = createLogger({ component: "FilteredBulkArrivalModal" });

interface FilteredBulkArrivalModalProps {
  isOpen: boolean;
  onClose: () => void;
  filter: BulkArrivalFilter;
  filterLabel: string;
  studentsInFilter: Student[];
  onSuccess?: () => void;
}

type DraftState = Record<number, string>;

/** ISO weekday to the day code the class timetable is keyed by. */
const DAY_CODES: Record<number, string> = {
  1: "mon",
  2: "tue",
  3: "wed",
  4: "thu",
  5: "fri",
};

function childCountLabel(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

function initialDraft(): DraftState {
  const draft: DraftState = {};
  for (const day of WEEKDAYS) {
    draft[day.value] = "";
  }
  return draft;
}

function isValidTime(value: string): boolean {
  if (value === "") return true;
  return /^\d{2}:\d{2}$/.test(value);
}

export function FilteredBulkArrivalModal({
  isOpen,
  onClose,
  filter,
  filterLabel,
  studentsInFilter,
  onSuccess,
}: FilteredBulkArrivalModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [draft, setDraft] = useState<DraftState>(initialDraft);
  const [saving, setSaving] = useState(false);
  const [lastChanged, setLastChanged] = useState<string | null>(null);
  const [classTimesLoading, setClassTimesLoading] = useState(false);
  const [classTimesError, setClassTimesError] = useState(false);
  const [classTimesLoadAttempt, setClassTimesLoadAttempt] = useState(0);
  const [collisionCount, setCollisionCount] = useState(0);
  // A school class sets the class timetable once for everyone (#2414); a group
  // is not a class, so there it still sets a time per child.
  const isClassTimetable = filter.type === "school_class";

  // Open with what the class already carries, so nobody has to retype it blind
  // or guess whether anything is set at all (#2414).
  const schoolClass =
    filter.type === "school_class" ? filter.schoolClass : null;

  useEffect(() => {
    if (!isOpen) return;
    setDraft(initialDraft());
    setLastChanged(null);
    setClassTimesError(false);
    if (!schoolClass) {
      setClassTimesLoading(false);
      return;
    }

    setDraft(initialDraft());
    setClassTimesLoading(true);
    let cancelled = false;

    const load = async () => {
      try {
        const current = await fetchClassArrivalTimes(schoolClass);
        if (cancelled) return;
        const next = initialDraft();
        for (const day of WEEKDAYS) {
          next[day.value] = current.times[DAY_CODES[day.value] ?? ""] ?? "";
        }
        setDraft(next);
        setLastChanged(current.updated_at ?? null);
      } catch (err) {
        if (cancelled) return;
        logger.warn("class_arrival_times_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setClassTimesError(true);
      } finally {
        if (!cancelled) setClassTimesLoading(false);
      }
    };
    void load();

    return () => {
      cancelled = true;
    };
  }, [isOpen, schoolClass, classTimesLoadAttempt]);

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    void fetchBulkArrivalScheduleStatus(studentsInFilter.map(({ id }) => id))
      .then((count) => {
        if (!cancelled) setCollisionCount(count);
      })
      .catch((err) => {
        logger.warn("bulk_arrival_schedule_status_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) setCollisionCount(0);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, studentsInFilter]);

  const targetTitle =
    filter.type === "school_class"
      ? // Schools name their classes either "3b" or "Klasse 3b"; without the
        // strip the title reads "für Klasse Klasse 3b".
        `Klasse ${stripClassPrefix(filterLabel)}`
      : filter.type === "group"
        ? `Gruppe ${filterLabel}`
        : filterLabel;

  const hasAnyTime = useMemo(
    () => Object.values(draft).some((value) => value.trim() !== ""),
    [draft],
  );
  const hasInvalidEntry = useMemo(
    () => Object.values(draft).some((value) => !isValidTime(value)),
    [draft],
  );
  const collisionMessage = `${childCountLabel(collisionCount)} ${collisionCount === 1 ? "hat" : "haben"} eigene Ankunftszeiten. ${
    isClassTimetable
      ? "Diese Zeiten bleiben bestehen."
      : "Die gewählten Zeiten können sie ersetzen."
  }`;

  const handleSubmit = async () => {
    if (isClassTimetable && (classTimesLoading || classTimesError)) return;
    if (!hasAnyTime) {
      toastError("Mindestens eine Zeit angeben");
      return;
    }
    if (hasInvalidEntry) {
      toastError("Ungültige Uhrzeit. Format HH:MM.");
      return;
    }

    const schedules: ArrivalScheduleInput[] = WEEKDAYS.filter(
      ({ value }) => (draft[value] ?? "").trim() !== "",
    ).map(({ value }) => ({
      weekday: value,
      expected_arrival: draft[value] ?? "",
    }));

    setSaving(true);
    try {
      await bulkUpsertArrivalSchedules(filter, schedules);
      toastSuccess(
        isClassTimetable
          ? `Unterrichtsschluss für ${targetTitle} gespeichert`
          : `Ankunftszeiten für ${targetTitle} gesetzt (${childCountLabel(studentsInFilter.length)})`,
      );
      onSuccess?.();
      onClose();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("failed to bulk upsert arrival schedules", {
        filter_type: filter.type,
        error: message,
      });
      toastError(`Fehler beim Speichern: ${message}`);
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={
        isClassTimetable
          ? `Unterrichtsschluss für ${targetTitle}`
          : `Ankunftszeiten für ${targetTitle}`
      }
      size="md"
      mobilePosition="bottom"
      footer={
        <div className="flex items-center justify-end gap-2 p-4">
          <Button variant="outline" onClick={onClose} disabled={saving}>
            Abbrechen
          </Button>
          <Button
            variant="success"
            onClick={handleSubmit}
            disabled={
              saving ||
              classTimesLoading ||
              classTimesError ||
              !hasAnyTime ||
              hasInvalidEntry
            }
          >
            {saving ? "Speichern..." : "Speichern"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4 p-4">
        {isClassTimetable ? (
          <div className="space-y-2 text-sm text-gray-600">
            <p>Tragen Sie den Unterrichtsschluss für jeden Wochentag ein.</p>
            <p>
              Die Klassenzeit gilt, wenn ein Kind keine eigene Ankunftszeit hat.
            </p>
            <p>Die Betreuungstage der Kinder ändern sich nicht.</p>
            <p>Leere Felder bleiben unverändert.</p>
            {lastChanged ? (
              <p className="text-gray-500">
                Zuletzt geändert am {formatDate(lastChanged)}.
              </p>
            ) : null}
          </div>
        ) : (
          <p className="text-sm text-gray-600">
            Die Zeiten gelten für {childCountLabel(studentsInFilter.length)} aus{" "}
            {targetTitle}. Leere Felder bleiben unverändert.
          </p>
        )}
        {classTimesLoading ? (
          <Alert type="info" message="Klassenzeiten werden geladen." />
        ) : null}
        {classTimesError ? (
          <Alert
            type="error"
            message="Die Klassenzeiten konnten nicht geladen werden. Bitte versuchen Sie es noch einmal."
            action={
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() =>
                  setClassTimesLoadAttempt((attempt) => attempt + 1)
                }
              >
                Erneut laden
              </Button>
            }
          />
        ) : null}
        {collisionCount > 0 ? (
          <Alert type="info" message={collisionMessage} />
        ) : null}
        <div className="space-y-2">
          {WEEKDAYS.map((day) => {
            const value = draft[day.value] ?? "";
            const invalid = !isValidTime(value);
            return (
              <div
                key={day.value}
                className={cn(
                  "grid grid-cols-[minmax(0,1fr)_8rem] items-center gap-x-3 gap-y-1 rounded-lg border border-gray-200 bg-gray-50 px-4 py-2.5",
                  invalid && "border-red-300 bg-red-50",
                )}
              >
                <label
                  htmlFor={`bulk-arrival-${day.value}`}
                  className="min-w-0 text-sm font-medium text-gray-700"
                >
                  {day.label}
                </label>
                <input
                  id={`bulk-arrival-${day.value}`}
                  type="time"
                  value={value}
                  disabled={
                    isClassTimetable && (classTimesLoading || classTimesError)
                  }
                  onChange={(event) =>
                    setDraft((prev) => ({
                      ...prev,
                      [day.value]: event.target.value.slice(0, 5),
                    }))
                  }
                  className="focus:border-moto-green focus:ring-moto-green/30 w-full rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:ring-2 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400"
                />
                {invalid ? (
                  <span className="col-start-2 text-xs text-red-600">
                    Format HH:MM
                  </span>
                ) : value === "" ? (
                  <span className="col-start-2 text-xs text-gray-400 italic">
                    nicht ändern
                  </span>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </FormModal>
  );
}
