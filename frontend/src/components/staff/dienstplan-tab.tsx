"use client";

import { useEffect, useMemo, useState } from "react";

import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { staffScheduleService } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";
import { formatDuration } from "~/lib/time-tracking-helpers";

const logger = createLogger({ component: "DienstplanTab" });

const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];
const WORK_DAYS: readonly number[] = [0, 1, 2, 3, 4];

// Slim Dienstplan tab. Owns only the weekly Soll-Stunden template (per
// weekday target minutes) and the modal that edits it. Aggregations over
// historic Ist-data live in the Übersicht tab; the day-by-day calendar is
// in the Zeiterfassung tab. This split keeps "what should they work" and
// "what did they work" cleanly separated.
export function DienstplanTab({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const [editorOpen, setEditorOpen] = useState(false);

  const {
    data: schedule,
    isLoading,
    mutate: mutateSchedule,
  } = useSWRAuth(`staff-schedule-${staffId}`, () =>
    staffScheduleService.getSchedule(staffId),
  );

  const scheduleEntries = useMemo(() => schedule?.entries ?? [], [schedule]);

  const targetByDow = useMemo(() => {
    const map = new Map<number, number>();
    for (const entry of scheduleEntries) {
      map.set(entry.dayOfWeek, entry.targetMinutes);
    }
    return map;
  }, [scheduleEntries]);

  const initialModalEntries = useMemo(
    () =>
      WORK_DAYS.map((d) => ({
        dayOfWeek: d,
        targetMinutes: targetByDow.get(d) ?? 0,
      })),
    [targetByDow],
  );

  const weeklyTotal = useMemo(
    () => WORK_DAYS.reduce((sum, d) => sum + (targetByDow.get(d) ?? 0), 0),
    [targetByDow],
  );

  if (isLoading) {
    return <Loading fullPage={false} />;
  }

  return (
    <div className="space-y-5">
      <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-6 shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
        <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
          <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
            Wochensoll
          </h3>
          {canEdit && (
            <button
              type="button"
              onClick={() => setEditorOpen(true)}
              className="rounded-full bg-gray-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-gray-800"
            >
              Bearbeiten
            </button>
          )}
        </div>

        <div className="grid grid-cols-1 gap-2 sm:grid-cols-3 lg:grid-cols-5">
          {WORK_DAYS.map((d) => {
            const minutes = targetByDow.get(d) ?? 0;
            return (
              <div
                key={d}
                className="rounded-2xl border border-gray-100 bg-white p-4 text-center"
              >
                <p className="text-xs font-semibold tracking-wider text-gray-400 uppercase">
                  {dayLabels[d]}
                </p>
                <p
                  className={`mt-2 text-lg font-bold tabular-nums ${
                    minutes > 0 ? "text-gray-700" : "text-gray-300"
                  }`}
                >
                  {minutes > 0 ? formatDuration(minutes) : "–"}
                </p>
              </div>
            );
          })}
        </div>

        <div className="mt-4 border-t border-gray-100 pt-4 text-sm text-gray-500">
          Wochensoll insgesamt:{" "}
          <span className="font-bold text-gray-700 tabular-nums">
            {formatDuration(weeklyTotal)}
          </span>
        </div>
      </div>

      <ScheduleEditModal
        isOpen={editorOpen}
        onClose={() => setEditorOpen(false)}
        staffId={staffId}
        initialEntries={initialModalEntries}
        onSaved={() => void mutateSchedule()}
      />
    </div>
  );
}

function ScheduleEditModal({
  isOpen,
  onClose,
  staffId,
  initialEntries,
  onSaved,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly staffId: string;
  readonly initialEntries: Array<{ dayOfWeek: number; targetMinutes: number }>;
  readonly onSaved: () => void;
}) {
  const toast = useToast();
  const [localEntries, setLocalEntries] = useState(initialEntries);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (isOpen) setLocalEntries(initialEntries);
  }, [isOpen, initialEntries]);

  const updateDay = (dayOfWeek: number, minutes: number) => {
    const clamped = Math.max(0, Math.min(720, minutes));
    setLocalEntries((prev) =>
      prev.map((e) =>
        e.dayOfWeek === dayOfWeek ? { ...e, targetMinutes: clamped } : e,
      ),
    );
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const nonZero = localEntries.filter((e) => e.targetMinutes > 0);
      await staffScheduleService.updateSchedule(staffId, nonZero);
      toast.success("Dienstplan gespeichert");
      onSaved();
      onClose();
    } catch (error) {
      logger.error("schedule_save_failed", {
        error: error instanceof Error ? error.message : String(error),
        staff_id: staffId,
      });
      toast.error("Fehler beim Speichern");
    } finally {
      setSaving(false);
    }
  };

  const weeklyTotal = localEntries.reduce((sum, e) => sum + e.targetMinutes, 0);

  const footer = (
    <div className="flex items-center justify-between gap-3">
      <span className="text-xs text-gray-500">
        Wochensoll:{" "}
        <span className="font-bold text-gray-700">
          {formatDuration(weeklyTotal)}
        </span>
      </span>
      <div className="flex gap-2">
        <button
          type="button"
          onClick={onClose}
          disabled={saving}
          className="rounded-full border border-gray-200 bg-white px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-40"
        >
          Abbrechen
        </button>
        <button
          type="button"
          onClick={handleSave}
          disabled={saving}
          className="rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-40"
        >
          {saving ? "Speichern..." : "Speichern"}
        </button>
      </div>
    </div>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Wochensoll bearbeiten"
      footer={footer}
    >
      <div className="space-y-3">
        <p className="text-xs text-gray-500">
          Die vertraglich vereinbarten Sollstunden pro Wochentag. Wochenenden
          sind bewusst ausgeklammert.
        </p>
        {localEntries.map((entry) => {
          const hours = Math.floor(entry.targetMinutes / 60);
          const mins = entry.targetMinutes % 60;
          return (
            <div
              key={entry.dayOfWeek}
              className="flex items-center gap-4 rounded-lg py-1.5"
            >
              <span className="w-10 shrink-0 text-sm font-medium text-gray-600">
                {dayLabels[entry.dayOfWeek]}
              </span>
              <input
                type="number"
                min={0}
                max={12}
                value={hours}
                onChange={(e) => {
                  const h = Math.max(
                    0,
                    Math.min(12, parseInt(e.target.value) || 0),
                  );
                  updateDay(entry.dayOfWeek, h * 60 + mins);
                }}
                className="w-14 rounded-lg border border-gray-200 px-2 py-1.5 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none"
              />
              <span className="text-xs text-gray-400">h</span>
              <select
                value={mins}
                onChange={(e) => {
                  const m = parseInt(e.target.value) || 0;
                  updateDay(entry.dayOfWeek, hours * 60 + m);
                }}
                className="w-16 rounded-lg border border-gray-200 px-2 py-1.5 text-center text-sm text-gray-700 tabular-nums focus:border-gray-400 focus:outline-none"
              >
                <option value={0}>00</option>
                <option value={15}>15</option>
                <option value={30}>30</option>
                <option value={45}>45</option>
              </select>
              <span className="text-xs text-gray-400">min</span>
              <span className="ml-auto text-xs text-gray-500 tabular-nums">
                {formatDuration(entry.targetMinutes)}
              </span>
            </div>
          );
        })}
      </div>
    </Modal>
  );
}
