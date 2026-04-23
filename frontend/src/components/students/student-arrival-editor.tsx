"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Save } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalData,
  type ArrivalScheduleInput,
  WEEKDAYS,
  fetchArrivalData,
  updateArrivalSchedules,
} from "~/lib/student-arrival-api";
import { createLogger } from "~/lib/logger";
import { cn } from "~/lib/utils";

const logger = createLogger({ component: "StudentArrivalEditor" });

interface StudentArrivalEditorProps {
  studentId: string;
  onSaved?: () => void;
}

type DraftState = Record<number, string>;

function buildDraftFromData(data: ArrivalData | null): DraftState {
  const draft: DraftState = {};
  for (const weekday of WEEKDAYS) {
    draft[weekday.value] = "";
  }
  if (!data) return draft;
  for (const schedule of data.schedules) {
    draft[schedule.weekday] = schedule.expected_arrival.slice(0, 5);
  }
  return draft;
}

function sanitizeTime(value: string): string {
  return value.trim().slice(0, 5);
}

function isValidTime(value: string): boolean {
  if (value === "") return true;
  return /^\d{2}:\d{2}$/.test(value);
}

export function StudentArrivalEditor({
  studentId,
  onSaved,
}: StudentArrivalEditorProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [data, setData] = useState<ArrivalData | null>(null);
  const [draft, setDraft] = useState<DraftState>(() =>
    buildDraftFromData(null),
  );
  const [loading, setLoading] = useState<boolean>(true);
  const [saving, setSaving] = useState<boolean>(false);
  const [loadError, setLoadError] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const freshData = await fetchArrivalData(studentId);
      setData(freshData);
      setDraft(buildDraftFromData(freshData));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to load arrival data", {
        studentId,
        error: message,
      });
      setLoadError(message);
    } finally {
      setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  const isDirty = useMemo(() => {
    if (!data) {
      return Object.values(draft).some((value) => value !== "");
    }
    const original = buildDraftFromData(data);
    return WEEKDAYS.some(({ value }) => draft[value] !== original[value]);
  }, [data, draft]);

  const hasInvalidEntry = useMemo(
    () => WEEKDAYS.some(({ value }) => !isValidTime(draft[value] ?? "")),
    [draft],
  );

  const handleChange = (weekday: number, value: string) => {
    setDraft((prev) => ({ ...prev, [weekday]: sanitizeTime(value) }));
  };

  const handleSave = async () => {
    if (hasInvalidEntry) {
      toastError("Ungueltige Uhrzeit. Format HH:MM.");
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
      await updateArrivalSchedules(studentId, schedules);
      toastSuccess("Ankunftszeiten gespeichert");
      await loadData();
      onSaved?.();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to save arrival schedules", {
        studentId,
        error: message,
      });
      toastError(`Fehler beim Speichern: ${message}`);
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-gray-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden />
        Lade Ankunftszeiten...
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        Ankunftszeiten konnten nicht geladen werden. {loadError}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-700">
            Wochenplan Ankunft
          </h3>
          <p className="mt-0.5 text-xs text-gray-500">
            Eine Zeit pro Wochentag. Leer lassen heisst Kind kommt an dem Tag
            nicht.
          </p>
        </div>
        <Button
          variant="primary"
          onClick={handleSave}
          disabled={!isDirty || saving || hasInvalidEntry}
        >
          {saving ? (
            <>
              <Loader2 className="mr-1.5 h-4 w-4 animate-spin" aria-hidden />
              Speichern...
            </>
          ) : (
            <>
              <Save className="mr-1.5 h-4 w-4" aria-hidden />
              Speichern
            </>
          )}
        </Button>
      </div>
      <div className="space-y-2">
        {WEEKDAYS.map((day) => {
          const value = draft[day.value] ?? "";
          const invalid = !isValidTime(value);
          return (
            <div
              key={day.value}
              className={cn(
                "flex items-center gap-3 rounded-lg border border-gray-200 bg-gray-50 px-4 py-3",
                invalid && "border-red-300 bg-red-50",
              )}
            >
              <label
                htmlFor={`arrival-${day.value}`}
                className="w-28 text-sm font-medium text-gray-700"
              >
                {day.label}
              </label>
              <input
                id={`arrival-${day.value}`}
                type="time"
                value={value}
                onChange={(event) =>
                  handleChange(day.value, event.target.value)
                }
                className="w-32 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-900 focus:border-[#83CD2D] focus:ring-2 focus:ring-[#83CD2D]/30 focus:outline-none"
              />
              {invalid ? (
                <span className="text-xs font-medium text-red-600">
                  Format HH:MM
                </span>
              ) : value === "" ? (
                <span className="text-xs text-gray-400 italic">
                  kein Ankunftstag
                </span>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}
