"use client";

// Tagesausnahme einer ganzen Klasse (#2962/#2970): ein Datum, an dem die
// Klasse zu einer anderen Zeit kommt. Der Baustein ist portal-neutral: die
// OGS (Kindersuche) und die Lehrkraft (moto schule) sehen dieselbe Liste und
// dasselbe Formular, nur die Datenquelle und die Rückmeldung unterscheiden
// sich. Beides kommt von außen (`api`, `notify`), damit kein Portal die
// Routen des anderen kennt.

import { useCallback, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { Input } from "~/components/ui/input";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type {
  ClassArrivalException,
  ClassArrivalExceptionInput,
  ClassArrivalExceptionList,
} from "~/lib/student-arrival-api";

const logger = createLogger({ component: "ClassArrivalExceptionPanel" });

/** The preset reason the "Unterricht fällt aus" button writes. */
const CLASS_ARRIVAL_CANCELLED_REASON = "Unterricht fällt aus";

/** Wie weit die Liste vorausschaut (Backend-Standard: 60 Tage). */
const LIST_HORIZON_DAYS = 60;

/** Die Datenquelle des Bausteins; jedes Portal bringt seine eigene mit. */
export interface ClassArrivalExceptionApi {
  readonly list: (schoolClass: string) => Promise<ClassArrivalExceptionList>;
  readonly upsert: (
    schoolClass: string,
    date: string,
    input: ClassArrivalExceptionInput,
  ) => Promise<ClassArrivalException>;
  readonly remove: (schoolClass: string, date: string) => Promise<void>;
  /**
   * Beginn des ersten geplanten Betreuungsblocks der Klasse an dem Tag
   * ("HH:MM"), null wenn es keinen gibt.
   */
  readonly earliestBlockStart: (
    schoolClass: string,
    isoDate: string,
  ) => Promise<string | null>;
}

/** Wohin Erfolg und Fehler gehen (Toast in der OGS, Hinweis im Dialog). */
interface ClassArrivalExceptionNotifier {
  readonly success: (message: string) => void;
  readonly error: (message: string) => void;
}

export interface ClassArrivalExceptionPanelProps {
  readonly schoolClass: string;
  /** "Klasse 4a", steht in der Bestätigung. */
  readonly classLabel: string;
  readonly api: ClassArrivalExceptionApi;
  readonly notify: ClassArrivalExceptionNotifier;
  readonly onChanged?: () => void;
  /** Vorbelegtes Datum, zum Beispiel der angezeigte Tag. */
  readonly defaultDate?: Date | null;
  /**
   * Zeile für Personen, die nichts ändern dürfen. Ohne Text bleibt die
   * Liste ohne Hinweis stehen.
   */
  readonly readOnlyHint?: string;
  /** Zusatzzeile unter einem Eintrag, zum Beispiel wer ihn gemacht hat. */
  readonly originLabel?: (exception: ClassArrivalException) => string | null;
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

function isWeekend(date: Date): boolean {
  const weekday = date.getDay();
  return weekday === 0 || weekday === 6;
}

function startOfToday(): Date {
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  return today;
}

/** Nur ein Datum ab heute darf vorbelegt werden; Vergangenes bleibt leer. */
function usableDefaultDate(candidate: Date | null | undefined): Date | null {
  if (!candidate) return null;
  const today = startOfToday();
  return candidate.getTime() < today.getTime() ? null : candidate;
}

export function ClassArrivalExceptionPanel({
  schoolClass,
  classLabel,
  api,
  notify,
  onChanged,
  defaultDate,
  readOnlyHint,
  originLabel,
}: ClassArrivalExceptionPanelProps) {
  const [exceptions, setExceptions] = useState<ClassArrivalException[]>([]);
  const [canEdit, setCanEdit] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [date, setDate] = useState<Date | null>(() =>
    usableDefaultDate(defaultDate),
  );
  const [time, setTime] = useState("");
  const [reason, setReason] = useState("");
  const [presetPending, setPresetPending] = useState(false);
  const [saving, setSaving] = useState(false);
  const [removing, setRemoving] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const list = await api.list(schoolClass);
    setExceptions(list.exceptions);
    setCanEdit(list.can_edit);
  }, [api, schoolClass]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setLoadError(false);
    api
      .list(schoolClass)
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
  }, [api, schoolClass, loadAttempt]);

  const today = startOfToday();
  const maxDate = new Date(today);
  maxDate.setDate(maxDate.getDate() + LIST_HORIZON_DAYS);
  const isoDate = date ? toISODate(date) : null;
  const canSave =
    isoDate !== null && isValidTime(time) && !presetPending && !saving;

  const applyCancelledPreset = async () => {
    setReason(CLASS_ARRIVAL_CANCELLED_REASON);
    if (!isoDate) {
      notify.error("Bitte zuerst ein Datum wählen.");
      return;
    }
    setPresetPending(true);
    try {
      const start = await api.earliestBlockStart(schoolClass, isoDate);
      if (start) {
        setTime(start);
      } else {
        notify.error(
          "Für diesen Tag ist kein Betreuungsblock geplant. Bitte die Uhrzeit selbst eintragen.",
        );
      }
    } catch (err) {
      logger.warn("class_arrival_exception_preset_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      notify.error(
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
      await api.upsert(schoolClass, isoDate, {
        arrival_time: time,
        reason: reason.trim() === "" ? null : reason.trim(),
      });
      notify.success(
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
      notify.error(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(false);
    }
  };

  const handleRemove = async (exception: ClassArrivalException) => {
    setRemoving(exception.date);
    try {
      await api.remove(schoolClass, exception.date);
      notify.success(`Abweichung am ${formatDate(exception.date)} entfernt`);
      await reload();
      onChanged?.();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("class_arrival_exception_delete_failed", { error: message });
      notify.error(
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
        <p>
          Eine eigene Wochenzeit eines Kindes wird an diesem Tag ersetzt. Eine
          eigene Tagesausnahme eines Kindes bleibt bestehen.
        </p>
        {!loading && !loadError && !canEdit && readOnlyHint ? (
          <p className="font-medium text-gray-700">{readOnlyHint}</p>
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
                id="class-arrival-exception-date"
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
                : CLASS_ARRIVAL_CANCELLED_REASON}
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
              {exceptions.map((exception) => {
                const origin = originLabel?.(exception) ?? null;
                return (
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
                      {origin ? (
                        <div className="text-xs text-gray-500">{origin}</div>
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
                );
              })}
            </ul>
          )}
        </div>
      ) : null}
    </div>
  );
}
