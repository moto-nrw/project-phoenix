"use client";

import { useMemo, useState } from "react";
import type { DateRange } from "react-day-picker";

import { RangeCalendarInline } from "~/components/ui/date-range-picker";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { timeTrackingService } from "~/lib/time-tracking-api";

const logger = createLogger({ component: "VacationRequestModal" });

function toIsoDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// Count Mon-Fri days between two dates inclusive — mirrors the backend
// countWorkingDays() helper so the live preview matches what gets stored.
function countWorkingDays(from: Date, to: Date, halfDay: boolean): number {
  if (to.getTime() < from.getTime()) return 0;
  let count = 0;
  const cursor = new Date(from);
  while (cursor.getTime() <= to.getTime()) {
    const dow = cursor.getDay();
    if (dow !== 0 && dow !== 6) count += 1;
    cursor.setDate(cursor.getDate() + 1);
  }
  return halfDay ? count * 0.5 : count;
}

export function VacationRequestModal({
  isOpen,
  onClose,
  onSubmitted,
  remainingDays,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSubmitted: () => void;
  readonly remainingDays: number;
}) {
  const today = useMemo(() => new Date(), []);
  const [range, setRange] = useState<DateRange | undefined>(undefined);
  const [halfDay, setHalfDay] = useState(false);
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const workingDays = useMemo(() => {
    if (!range?.from || !range.to) return 0;
    return countWorkingDays(range.from, range.to, halfDay);
  }, [range, halfDay]);

  const exceedsBalance = workingDays > remainingDays;

  const handleSubmit = async () => {
    if (!range?.from || !range.to) {
      toast.error("Bitte Zeitraum auswählen.");
      return;
    }
    if (workingDays === 0) {
      toast.error("Der gewählte Zeitraum enthält keine Werktage.");
      return;
    }
    setSubmitting(true);
    try {
      await timeTrackingService.requestVacation({
        date_start: toIsoDate(range.from),
        date_end: toIsoDate(range.to),
        half_day: halfDay,
        note: note.trim() || undefined,
      });
      toast.success("Urlaubsantrag gesendet.");
      onSubmitted();
      handleReset();
      onClose();
    } catch (err) {
      logger.error("vacation_request_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error(
        err instanceof Error
          ? err.message
          : "Antrag konnte nicht gesendet werden.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  const handleReset = () => {
    setRange(undefined);
    setHalfDay(false);
    setNote("");
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={() => {
        if (!submitting) {
          handleReset();
          onClose();
        }
      }}
      title="Urlaub beantragen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      footer={
        <div className="flex items-center justify-between gap-3">
          <div className="text-xs text-gray-500">
            {range?.from && range?.to ? (
              <>
                <span className="font-medium text-gray-700">
                  {workingDays} {workingDays === 1 ? "Tag" : "Tage"}
                </span>
                {" beantragt · "}
                <span
                  className={exceedsBalance ? "text-red-600" : "text-gray-500"}
                >
                  {remainingDays} Tage verfügbar
                </span>
              </>
            ) : (
              <span>Wähle einen Zeitraum</span>
            )}
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => {
                handleReset();
                onClose();
              }}
              disabled={submitting}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={handleSubmit}
              disabled={
                submitting || !range?.from || !range.to || workingDays === 0
              }
              className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {submitting ? "Wird gesendet…" : "Antrag senden"}
            </button>
          </div>
        </div>
      }
    >
      <div className="space-y-5">
        <div>
          <p className="mb-2 text-xs font-semibold tracking-wider text-gray-500 uppercase">
            Zeitraum
          </p>
          <div className="rounded-xl border border-gray-200 bg-white p-3">
            <RangeCalendarInline
              value={range}
              onChange={setRange}
              fromMin={today}
            />
          </div>
        </div>

        <div className="flex items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3">
          <div>
            <p className="text-sm font-medium text-gray-800">Halber Tag</p>
            <p className="text-xs text-gray-500">
              Aktivieren, wenn nur ein halber Arbeitstag beansprucht wird.
            </p>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={halfDay}
            onClick={() => setHalfDay((v) => !v)}
            className={`relative h-6 w-11 rounded-full transition-colors ${
              halfDay ? "bg-[#83CD2D]" : "bg-gray-300"
            }`}
          >
            <span
              className={`absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform ${
                halfDay ? "translate-x-5" : "translate-x-0.5"
              }`}
            />
          </button>
        </div>

        <div>
          <label
            htmlFor="vacation-note"
            className="mb-2 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Notiz (optional)
          </label>
          <textarea
            id="vacation-note"
            value={note}
            onChange={(e) => setNote(e.target.value)}
            rows={3}
            maxLength={500}
            placeholder="Zum Beispiel: Vertretung mit Kollegin XY abgestimmt"
            className="w-full resize-none rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 placeholder:text-gray-400 focus:border-[#83CD2D] focus:outline-none"
          />
          <p className="mt-1 text-right text-xs text-gray-400">
            {note.length}/500
          </p>
        </div>

        {exceedsBalance && (
          <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-xs text-red-700">
            Dein Antrag überschreitet den verbleibenden Urlaubsanspruch. Die
            OGS-Leitung kann den Antrag trotzdem genehmigen, du solltest aber
            vorher Rücksprache halten.
          </div>
        )}
      </div>
    </Modal>
  );
}
