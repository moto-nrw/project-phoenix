"use client";

import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type ClassArrivalException,
  deleteClassArrivalException,
  fetchClassArrivalExceptions,
  upsertClassArrivalException,
} from "~/lib/student-arrival-api";
import { timetableService } from "~/lib/timetable-api";
import { normalizeSchoolClass } from "~/lib/timetable-helpers";

const logger = createLogger({ component: "ClassArrivalExceptionPanel" });

/** The preset reason the "Unterricht fällt aus" button writes. */
const CLASS_ARRIVAL_CANCELLED_REASON = "Unterricht fällt aus";

interface ClassArrivalExceptionPanelProps {
  readonly schoolClass: string;
  /** "Klasse 4a", used in the confirmation toast. */
  readonly classLabel: string;
  readonly onChanged?: () => void;
}

const WEEKDAY_SHORT: Record<number, string> = {
  0: "So",
  1: "Mo",
  2: "Di",
  3: "Mi",
  4: "Do",
  5: "Fr",
  6: "Sa",
};

function exceptionDateLabel(isoDate: string): string {
  const weekday = WEEKDAY_SHORT[parseISODate(isoDate).getDay()] ?? "";
  return `${weekday}, ${formatDate(isoDate)}`;
}

function isValidTime(value: string): boolean {
  return /^\d{2}:\d{2}$/.test(value);
}

/**
 * Earliest planned block start of a date ("HH:MM") or null when the day has
 * no block. Cancelled blocks do not count: the class would arrive into
 * nothing.
 */
function isWeekend(date: Date): boolean {
  const weekday = date.getDay();
  return weekday === 0 || weekday === 6;
}

interface ClassTargetTemplate {
  readonly targetSchoolClass?: string;
  readonly targets?: ReadonlyArray<{ readonly schoolClass?: string }>;
  readonly sourceSchoolClasses?: readonly string[];
}

function appliesToSchoolClass(
  template: ClassTargetTemplate,
  schoolClass: string,
): boolean {
  const classes = [
    template.targetSchoolClass,
    ...(template.targets?.map((target) => target.schoolClass) ?? []),
    ...(template.sourceSchoolClasses ?? []),
  ].filter((value): value is string => value !== undefined);
  return (
    classes.length === 0 ||
    classes.some(
      (schoolClassTarget) =>
        normalizeSchoolClass(schoolClassTarget) ===
        normalizeSchoolClass(schoolClass),
    )
  );
}

async function earliestBlockStart(
  isoDate: string,
  schoolClass: string,
): Promise<string | null> {
  const [week, templates] = await Promise.all([
    timetableService.getWeek(isoDate, isoDate),
    timetableService.getTemplates(),
  ]);
  const templatesByID = new Map(
    templates.templates.map((template) => [template.id, template]),
  );
  const starts = week.instances
    .filter((instance) => {
      if (instance.date !== isoDate || instance.status === "cancelled") {
        return false;
      }
      if (!instance.activityGroupId) {
        return true;
      }
      const template = templatesByID.get(instance.activityGroupId);
      return (
        template !== undefined && appliesToSchoolClass(template, schoolClass)
      );
    })
    .map((instance) => instance.startTime)
    .sort();
  return starts[0] ?? null;
}

/**
 * One date on which a whole class arrives at a different time (#2962).
 * Everybody sees the list; the form appears only for people the school lets
 * set one (operations.class_arrival_exception_editors).
 */
export function ClassArrivalExceptionPanel({
  schoolClass,
  classLabel,
  onChanged,
}: ClassArrivalExceptionPanelProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [exceptions, setExceptions] = useState<ClassArrivalException[]>([]);
  const [canEdit, setCanEdit] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [date, setDate] = useState<Date | null>(null);
  const [time, setTime] = useState("");
  const [reason, setReason] = useState("");
  const [presetPending, setPresetPending] = useState(false);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const list = await fetchClassArrivalExceptions(schoolClass);
    setExceptions(list.exceptions);
    setCanEdit(list.can_edit);
  }, [schoolClass]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadError(false);
    fetchClassArrivalExceptions(schoolClass)
      .then((list) => {
        if (cancelled) return;
        setExceptions(list.exceptions);
        setCanEdit(list.can_edit);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.warn("class_arrival_exceptions_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setLoadError(true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [schoolClass, loadAttempt]);

  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const maxDate = new Date(today);
  maxDate.setDate(maxDate.getDate() + 60);
  const isoDate = date ? toISODate(date) : null;
  const canSave =
    isoDate !== null && isValidTime(time) && !presetPending && !saving;

  const applyCancelledPreset = async () => {
    setReason(CLASS_ARRIVAL_CANCELLED_REASON);
    if (!isoDate) {
      toastError("Bitte zuerst ein Datum wählen.");
      return;
    }
    setPresetPending(true);
    try {
      const start = await earliestBlockStart(isoDate, schoolClass);
      if (start) {
        setTime(start);
      } else {
        toastError(
          "Für diesen Tag ist kein Betreuungsblock geplant. Bitte die Uhrzeit selbst eintragen.",
        );
      }
    } catch (err) {
      logger.warn("class_arrival_exception_preset_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toastError(
        "Der Blockbeginn konnte nicht geladen werden. Bitte die Uhrzeit selbst eintragen.",
      );
    } finally {
      setPresetPending(false);
    }
  };

  const handleSave = async () => {
    if (!isoDate || !canSave) return;
    setSaving(true);
    try {
      await upsertClassArrivalException(schoolClass, isoDate, {
        arrival_time: time,
        reason: reason.trim() === "" ? null : reason.trim(),
      });
      toastSuccess(
        `${classLabel} kommt am ${formatDate(isoDate)} um ${time} Uhr`,
      );
      setDate(null);
      setTime("");
      setReason("");
      await reload();
      onChanged?.();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("class_arrival_exception_save_failed", { error: message });
      toastError(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async (exception: ClassArrivalException) => {
    setRemoving(exception.date);
    try {
      await deleteClassArrivalException(schoolClass, exception.date);
      toastSuccess(`Abweichung am ${formatDate(exception.date)} entfernt`);
      await reload();
      onChanged?.();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("class_arrival_exception_delete_failed", { error: message });
      toastError(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setRemoving(null);
    }
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2 text-sm text-gray-600">
        <p>Gilt nur an diesem Tag, für alle Kinder der Klasse.</p>
        <p>Eine eigene Tagesausnahme eines Kindes bleibt bestehen.</p>
        {!loading && !loadError && !canEdit ? (
          <p className="font-medium text-gray-700">
            Ändern kann das die Koordination.
          </p>
        ) : null}
      </div>

      {loading ? (
        <Alert type="info" message="Abweichungen werden geladen." />
      ) : null}
      {loadError ? (
        <Alert
          type="error"
          message="Die Abweichungen konnten nicht geladen werden. Bitte versuchen Sie es noch einmal."
          action={
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setLoadAttempt((attempt) => attempt + 1)}
            >
              Erneut laden
            </Button>
          }
        />
      ) : null}

      {!loading && !loadError && canEdit ? (
        <div className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-4">
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label
                htmlFor="class-arrival-exception-date"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Datum
              </label>
              <DatePicker
                value={date}
                onChange={setDate}
                minDate={today}
                maxDate={maxDate}
                disabledDay={isWeekend}
                placeholder="Datum wählen"
                hideClearButton
                required
              />
            </div>
            <Input
              id="class-arrival-exception-time"
              label="Kommt um"
              type="time"
              value={time}
              onChange={(event) => setTime(event.target.value.slice(0, 5))}
            />
          </div>
          <Input
            id="class-arrival-exception-reason"
            label="Grund (optional)"
            type="text"
            maxLength={255}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="zum Beispiel Wandertag"
          />
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void applyCancelledPreset()}
              disabled={presetPending || saving}
            >
              {presetPending
                ? "Blockbeginn wird geladen..."
                : "Unterricht fällt aus"}
            </Button>
            <Button
              type="button"
              variant="success"
              size="md"
              onClick={() => void handleSave()}
              disabled={!canSave}
            >
              {saving ? "Speichern..." : "Tag speichern"}
            </Button>
          </div>
          <p className="text-xs text-gray-500">
            „Unterricht fällt aus“ setzt die Uhrzeit auf den Beginn des ersten
            Betreuungsblocks an diesem Tag. Sie können sie danach ändern.
          </p>
        </div>
      ) : null}

      {!loading && !loadError ? (
        <div className="space-y-2">
          <h3 className="text-sm font-medium text-gray-900">
            Eingetragene Tage
          </h3>
          {exceptions.length === 0 ? (
            <EmptyState
              title="Keine Abweichung eingetragen"
              description="Die Klasse kommt an jedem Tag zur Klassenzeit."
            />
          ) : (
            <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200">
              {exceptions.map((exception) => (
                <li
                  key={exception.date}
                  className="flex items-center justify-between gap-3 px-3 py-2 text-sm"
                >
                  <div className="min-w-0">
                    <div className="font-medium text-gray-900">
                      {exceptionDateLabel(exception.date)} ·{" "}
                      {exception.arrival_time} Uhr
                    </div>
                    {exception.reason ? (
                      <div className="text-gray-600">{exception.reason}</div>
                    ) : null}
                  </div>
                  {canEdit ? (
                    <Button
                      type="button"
                      variant="outline_danger"
                      size="compact"
                      onClick={() => void handleRemove(exception)}
                      disabled={removing === exception.date}
                    >
                      Entfernen
                    </Button>
                  ) : null}
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}
    </div>
  );
}
