"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Calendar, Loader2, Plus, Trash2 } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useToast } from "~/contexts/ToastContext";
import {
  type ArrivalException,
  type ArrivalExceptionInput,
  createArrivalException,
  deleteArrivalException,
  fetchArrivalData,
} from "~/lib/student-arrival-api";
import { createLogger } from "~/lib/logger";
import { cn } from "~/lib/utils";

const logger = createLogger({ component: "StudentAusnahmenTab" });

interface StudentAusnahmenTabProps {
  studentId: string;
  onChanged?: () => void;
}

interface DraftState {
  date: string;
  time: string;
  absent: boolean;
  reason: string;
}

function emptyDraft(): DraftState {
  return { date: "", time: "", absent: false, reason: "" };
}

function formatDateLabel(isoDate: string): string {
  const date = new Date(`${isoDate}T00:00:00`);
  if (Number.isNaN(date.getTime())) return isoDate;
  return date.toLocaleDateString("de-DE", {
    weekday: "short",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

function sortExceptions(list: ArrivalException[]): ArrivalException[] {
  return [...list].sort((a, b) =>
    a.exception_date.localeCompare(b.exception_date),
  );
}

export function StudentAusnahmenTab({
  studentId,
  onChanged,
}: StudentAusnahmenTabProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [exceptions, setExceptions] = useState<ArrivalException[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [draft, setDraft] = useState<DraftState>(emptyDraft);

  const loadData = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const data = await fetchArrivalData(studentId);
      setExceptions(sortExceptions(data.exceptions));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to load arrival exceptions", {
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

  const resetForm = useCallback(() => {
    setDraft(emptyDraft());
    setShowForm(false);
  }, []);

  const draftValid = useMemo(() => {
    if (!draft.date) return false;
    if (draft.absent) return true;
    return /^\d{2}:\d{2}$/.test(draft.time);
  }, [draft]);

  const handleCreate = async () => {
    if (!draftValid) {
      toastError("Datum und Uhrzeit prüfen (HH:MM)");
      return;
    }
    const input: ArrivalExceptionInput = {
      exception_date: draft.date,
      expected_arrival: draft.absent ? null : draft.time,
      reason: draft.reason.trim() ? draft.reason.trim() : null,
    };
    setSaving(true);
    try {
      await createArrivalException(studentId, input);
      toastSuccess("Ausnahme gespeichert");
      resetForm();
      await loadData();
      onChanged?.();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to create arrival exception", {
        studentId,
        error: message,
      });
      toastError(`Fehler beim Speichern: ${message}`);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (exceptionId: number) => {
    setDeletingId(exceptionId);
    try {
      await deleteArrivalException(studentId, exceptionId);
      toastSuccess("Ausnahme gelöscht");
      await loadData();
      onChanged?.();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("Failed to delete arrival exception", {
        studentId,
        exceptionId,
        error: message,
      });
      toastError(`Fehler beim Löschen: ${message}`);
    } finally {
      setDeletingId(null);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12 text-sm text-gray-500">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" aria-hidden />
        Lade Ausnahmen...
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
        Ausnahmen konnten nicht geladen werden. {loadError}
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-700">
            Ankunfts-Ausnahmen
          </h3>
          <p className="mt-0.5 text-xs text-gray-500">
            Einzelne Tage mit abweichender Ankunft (z. B. Wandertag, Arzttermin,
            Hitzefrei). Werden am Tag selbst wirksam.
          </p>
        </div>
        {!showForm ? (
          <Button
            type="button"
            variant="outline"
            onClick={() => setShowForm(true)}
          >
            <Plus className="mr-1.5 h-4 w-4" aria-hidden />
            Neue Ausnahme
          </Button>
        ) : null}
      </div>

      {showForm ? (
        <div className="space-y-3 rounded-xl border border-gray-200 bg-gray-50 p-4">
          <div className="flex items-center gap-3">
            <label
              htmlFor="exception-date"
              className="w-28 text-sm font-medium text-gray-700"
            >
              Datum
            </label>
            <input
              id="exception-date"
              type="date"
              value={draft.date}
              onChange={(event) =>
                setDraft((prev) => ({ ...prev, date: event.target.value }))
              }
              className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-2 focus:ring-[#83CD2D]/30 focus:outline-none"
            />
          </div>
          <div className="flex items-center gap-3">
            <span className="w-28 text-sm font-medium text-gray-700">
              Ankunft
            </span>
            <label className="inline-flex items-center gap-2 text-sm text-gray-700">
              <input
                type="checkbox"
                checked={draft.absent}
                onChange={(event) =>
                  setDraft((prev) => ({
                    ...prev,
                    absent: event.target.checked,
                    time: event.target.checked ? "" : prev.time,
                  }))
                }
                className="h-4 w-4 rounded border-gray-300 text-[#83CD2D] focus:ring-[#83CD2D]/30"
              />
              Kind kommt nicht
            </label>
            <input
              type="time"
              value={draft.time}
              disabled={draft.absent}
              onChange={(event) =>
                setDraft((prev) => ({ ...prev, time: event.target.value }))
              }
              className={cn(
                "rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-2 focus:ring-[#83CD2D]/30 focus:outline-none",
                draft.absent && "opacity-50",
              )}
            />
          </div>
          <div className="flex items-start gap-3">
            <label
              htmlFor="exception-reason"
              className="w-28 pt-1.5 text-sm font-medium text-gray-700"
            >
              Grund
            </label>
            <input
              id="exception-reason"
              type="text"
              maxLength={255}
              value={draft.reason}
              placeholder="Optional"
              onChange={(event) =>
                setDraft((prev) => ({ ...prev, reason: event.target.value }))
              }
              className="flex-1 rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-2 focus:ring-[#83CD2D]/30 focus:outline-none"
            />
          </div>
          <div className="flex items-center justify-end gap-2 pt-1">
            <Button
              type="button"
              variant="outline"
              onClick={resetForm}
              disabled={saving}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              onClick={handleCreate}
              disabled={saving || !draftValid}
            >
              {saving ? (
                <>
                  <Loader2
                    className="mr-1.5 h-4 w-4 animate-spin"
                    aria-hidden
                  />
                  Speichern...
                </>
              ) : (
                "Hinzufügen"
              )}
            </Button>
          </div>
        </div>
      ) : null}

      {exceptions.length === 0 ? (
        <div className="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-500">
          Keine Ausnahmen gesetzt. Standard-Wochenplan gilt.
        </div>
      ) : (
        <ul className="space-y-2" role="list">
          {exceptions.map((exception) => {
            const isAbsent = !exception.expected_arrival;
            return (
              <li
                key={exception.id}
                className="flex items-center gap-3 rounded-lg border border-gray-200 bg-white px-4 py-3"
              >
                <Calendar className="h-4 w-4 text-gray-400" aria-hidden />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-semibold text-gray-900">
                    {formatDateLabel(exception.exception_date)}
                  </div>
                  <div className="mt-0.5 text-xs text-gray-600">
                    {isAbsent ? (
                      <span className="font-medium text-[#F78C10]">
                        Kind kommt nicht
                      </span>
                    ) : (
                      <>
                        Ankunft{" "}
                        <span className="font-semibold text-gray-900">
                          {exception.expected_arrival?.slice(0, 5)}
                        </span>
                      </>
                    )}
                    {exception.reason ? (
                      <>
                        {" · "}
                        <span className="italic">{exception.reason}</span>
                      </>
                    ) : null}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => void handleDelete(exception.id)}
                  disabled={deletingId === exception.id}
                  aria-label="Ausnahme löschen"
                  className="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-50"
                >
                  {deletingId === exception.id ? (
                    <Loader2 className="h-4 w-4 animate-spin" aria-hidden />
                  ) : (
                    <Trash2 className="h-4 w-4" aria-hidden />
                  )}
                </button>
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
