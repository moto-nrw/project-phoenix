"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  Loader2,
  SquarePen,
  StickyNote,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionHeader } from "~/components/ui/concept-section-header";
import { ArrivalScheduleFormModal } from "./arrival-schedule-form-modal";
import { ArrivalDayEditModal } from "./arrival-day-edit-modal";
import {
  type ArrivalDayData,
  type ArrivalScheduleFormEntry,
  WEEKDAYS,
  formatDateISO,
  formatShortDate,
  getDayData,
  getWeekDays,
  mergeSchedulesWithTemplate,
} from "~/lib/arrival-schedule-helpers";
import {
  type ArrivalData,
  createArrivalException,
  createArrivalNote,
  deleteArrivalException,
  deleteArrivalNote,
  fetchArrivalData,
  updateArrivalException,
  updateArrivalNote,
  updateArrivalSchedules,
} from "~/lib/student-arrival-api";
import { createLogger } from "~/lib/logger";
import type { StudentStatusDay } from "~/lib/student-status-days-api";

const logger = createLogger({ component: "ArrivalScheduleManager" });
const EMPTY_STATUS_DAYS: StudentStatusDay[] = [];

interface ArrivalScheduleManagerProps {
  readonly studentId: string;
  readonly readOnly?: boolean;
  readonly onUpdate?: () => void;
  readonly statusDays?: StudentStatusDay[];
}

export function ArrivalScheduleManager({
  studentId,
  readOnly = false,
  onUpdate,
  statusDays = EMPTY_STATUS_DAYS,
}: ArrivalScheduleManagerProps) {
  const [arrivalData, setArrivalData] = useState<ArrivalData>({
    schedules: [],
    exceptions: [],
    notes: [],
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [weekOffset, setWeekOffset] = useState(0);

  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false);
  const [editingDay, setEditingDay] = useState<ArrivalDayData | null>(null);

  const weekDays = useMemo(() => getWeekDays(weekOffset), [weekOffset]);
  const statusByDate = useMemo(() => {
    const entries = new Map<string, StudentStatusDay["status"]>();
    for (const day of statusDays) {
      if (!day.cleared_at) {
        entries.set(day.date, day.status);
      }
    }
    return entries;
  }, [statusDays]);

  const dayDataList = useMemo(
    () =>
      weekDays.map((date) =>
        getDayData(
          date,
          arrivalData.schedules,
          arrivalData.exceptions,
          arrivalData.notes,
          false,
          false,
          statusByDate.get(formatDateISO(date)) ?? null,
        ),
      ),
    [
      weekDays,
      arrivalData.schedules,
      arrivalData.exceptions,
      arrivalData.notes,
      statusByDate,
    ],
  );

  const loadArrivalData = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await fetchArrivalData(studentId);
      setArrivalData(data);
    } catch (err) {
      logger.error("arrival_data_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Laden des Ankunftsplans",
      );
    } finally {
      setIsLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    loadArrivalData().catch(() => undefined);
  }, [loadArrivalData]);

  const refreshKeepModal = useCallback(async () => {
    const data = await fetchArrivalData(studentId);
    setArrivalData(data);
    onUpdate?.();
  }, [studentId, onUpdate]);

  const handleUpdateSchedules = async (
    schedules: ArrivalScheduleFormEntry[],
  ) => {
    await updateArrivalSchedules(
      studentId,
      schedules.map((s) => ({
        weekday: s.weekday,
        expected_arrival: s.expected_arrival,
        notes: s.notes ?? null,
      })),
    );
    await loadArrivalData();
    onUpdate?.();
    setIsScheduleModalOpen(false);
  };

  const handleOpenDayEdit = (day: ArrivalDayData) => {
    if (readOnly || day.weekday === 0) return;
    setEditingDay(day);
  };

  const handleSaveException = useCallback(
    async (params: {
      expectedArrival: string | null;
      reason?: string | null;
    }) => {
      if (!editingDay) return;
      const dateStr = formatDateISO(editingDay.date);
      const input = {
        exception_date: dateStr,
        expected_arrival: params.expectedArrival,
        ...(editingDay.exception && params.expectedArrival === null
          ? { clear_expected_arrival: true }
          : {}),
        reason: params.reason ?? null,
      };
      if (editingDay.exception) {
        await updateArrivalException(studentId, editingDay.exception.id, input);
      } else {
        await createArrivalException(studentId, input);
      }
      await refreshKeepModal();
    },
    [editingDay, studentId, refreshKeepModal],
  );

  const handleDeleteException = useCallback(async () => {
    if (!editingDay?.exception) return;
    await deleteArrivalException(studentId, editingDay.exception.id);
    await refreshKeepModal();
  }, [editingDay, studentId, refreshKeepModal]);

  const handleCreateNote = useCallback(
    async (content: string) => {
      if (!editingDay) return;
      await createArrivalNote(studentId, {
        note_date: formatDateISO(editingDay.date),
        content,
      });
      await refreshKeepModal();
    },
    [editingDay, studentId, refreshKeepModal],
  );

  const handleUpdateNote = useCallback(
    async (noteId: number, content: string) => {
      if (!editingDay) return;
      await updateArrivalNote(studentId, noteId, {
        note_date: formatDateISO(editingDay.date),
        content,
      });
      await refreshKeepModal();
    },
    [editingDay, studentId, refreshKeepModal],
  );

  const handleDeleteNote = useCallback(
    async (noteId: number) => {
      await deleteArrivalNote(studentId, noteId);
      await refreshKeepModal();
    },
    [studentId, refreshKeepModal],
  );

  const handleCloseDayEdit = () => setEditingDay(null);

  const goToPreviousWeek = () => setWeekOffset((w) => w - 1);
  const goToNextWeek = () => setWeekOffset((w) => w + 1);

  const currentEditingDay = useMemo(() => {
    if (!editingDay) return null;
    return getDayData(
      editingDay.date,
      arrivalData.schedules,
      arrivalData.exceptions,
      arrivalData.notes,
      false,
      false,
      statusByDate.get(formatDateISO(editingDay.date)) ?? null,
    );
  }, [editingDay, arrivalData, statusByDate]);

  if (isLoading && arrivalData.schedules.length === 0) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-8 w-8 animate-spin text-gray-600" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-red-700">
        {error}
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4">
        <SectionHeader
          title="Ankunftsplan & Notizen"
          icon={<MotoConceptIcon concept="carePlan" size={22} />}
          actions={
            !readOnly ? (
              <button
                type="button"
                onClick={() => setIsScheduleModalOpen(true)}
                className="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
              >
                Bearbeiten
              </button>
            ) : undefined
          }
        />
      </div>

      {/* Mobile */}
      <div className="sm:hidden">
        <div className="mb-3 flex items-center justify-between">
          <button
            type="button"
            onClick={goToPreviousWeek}
            className="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
            title="Vorherige Woche"
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <div className="flex-1" />
          <button
            type="button"
            onClick={goToNextWeek}
            className="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
            title="Nächste Woche"
          >
            <ChevronRight className="h-5 w-5" />
          </button>
        </div>
        <div className="space-y-2">
          {dayDataList.map((day) => (
            <DayRow
              key={formatDateISO(day.date)}
              day={day}
              readOnly={readOnly}
              onEditDay={handleOpenDayEdit}
            />
          ))}
        </div>
      </div>

      {/* Desktop */}
      <div className="hidden sm:block">
        <div className="flex items-center gap-3">
          <button
            type="button"
            onClick={goToPreviousWeek}
            className="flex-shrink-0 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            title="Vorherige Woche"
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <div className="flex-1">
            <div className="grid grid-cols-5 gap-2">
              {dayDataList.map((day) => (
                <DayCell
                  key={formatDateISO(day.date)}
                  day={day}
                  readOnly={readOnly}
                  onEditDay={handleOpenDayEdit}
                />
              ))}
            </div>
          </div>
          <button
            type="button"
            onClick={goToNextWeek}
            className="flex-shrink-0 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            title="Nächste Woche"
          >
            <ChevronRight className="h-5 w-5" />
          </button>
        </div>
      </div>

      <ArrivalScheduleFormModal
        isOpen={isScheduleModalOpen}
        onClose={() => setIsScheduleModalOpen(false)}
        onSubmit={handleUpdateSchedules}
        initialSchedules={mergeSchedulesWithTemplate(arrivalData.schedules)}
      />

      <ArrivalDayEditModal
        isOpen={editingDay !== null}
        onClose={handleCloseDayEdit}
        day={currentEditingDay}
        onSaveException={handleSaveException}
        onDeleteException={handleDeleteException}
        onCreateNote={handleCreateNote}
        onUpdateNote={handleUpdateNote}
        onDeleteNote={handleDeleteNote}
      />
    </div>
  );
}

interface DayComponentProps {
  readonly day: ArrivalDayData;
  readonly readOnly: boolean;
  readonly onEditDay: (day: ArrivalDayData) => void;
}

function DayRow({ day, readOnly, onEditDay }: DayComponentProps) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  const hasNotes = !!day.baseSchedule?.notes || day.notes.length > 0;

  return (
    <div
      className={
        day.isToday
          ? "border-moto-green bg-moto-green/5 rounded-lg border px-3 py-2"
          : "moto-content-surface rounded-lg border px-3 py-2"
      }
    >
      <div className="flex items-center gap-3">
        <div className="w-16 flex-shrink-0">
          <div
            className={
              day.isToday
                ? "text-moto-green text-sm font-medium"
                : "text-sm font-medium text-gray-700"
            }
          >
            {weekdayInfo?.shortLabel} {formatShortDate(day.date)}
          </div>
          {day.isToday ? (
            <div className="text-moto-green text-[10px]">heute</div>
          ) : null}
        </div>
        <div className="w-16 flex-shrink-0 text-sm font-semibold text-gray-900">
          {day.showSick || day.showExcused || day.isAbsent ? (
            <span className="text-gray-300">-</span>
          ) : (
            (day.effectiveTime ?? "-")
          )}
        </div>
        <div className="min-w-0 flex-1">
          {day.showSick || day.showExcused ? (
            <span className="inline-flex items-center rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600">
              kommt nicht
            </span>
          ) : day.isAbsent ? (
            <span className="text-moto-orange bg-moto-orange/10 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
              kommt nicht
            </span>
          ) : day.isException ? (
            <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-orange-100 text-orange-600">
              <svg className="h-3 w-3" viewBox="0 0 20 20" fill="currentColor">
                <circle cx="10" cy="10" r="5" />
              </svg>
            </span>
          ) : null}
        </div>
        {!readOnly ? (
          <button
            type="button"
            onClick={() => onEditDay(day)}
            className="flex-shrink-0 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
            title="Tag bearbeiten"
          >
            <SquarePen className="h-4 w-4" />
          </button>
        ) : null}
      </div>
      {hasNotes ? (
        <div className="mt-1.5 space-y-0.5 pl-[76px]">
          {day.baseSchedule?.notes ? (
            <div className="flex items-start gap-1 text-xs text-gray-400 italic">
              <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0" />
              <span>{day.baseSchedule.notes}</span>
            </div>
          ) : null}
          {day.notes.map((note) => (
            <div
              key={note.id}
              className="flex items-start gap-1 text-xs text-gray-500"
            >
              <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0 text-gray-400" />
              <span>{note.content}</span>
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function DayCell({ day, readOnly, onEditDay }: DayComponentProps) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  return (
    <div
      className={
        day.isToday
          ? "group border-moto-green bg-moto-green/5 relative rounded-lg border p-2 text-center"
          : "group moto-content-surface relative rounded-lg border p-2 text-center"
      }
    >
      <div className="flex items-center justify-center gap-1">
        <div
          className={
            day.isToday
              ? "text-moto-green text-xs font-medium"
              : "text-xs font-medium text-gray-500"
          }
        >
          {weekdayInfo?.shortLabel}
        </div>
        {day.isException ? (
          <span
            className="flex h-4 w-4 items-center justify-center rounded-full bg-orange-100 text-orange-600"
            title="Abweichende Ankunft"
          >
            <svg className="h-2 w-2" viewBox="0 0 20 20" fill="currentColor">
              <circle cx="10" cy="10" r="5" />
            </svg>
          </span>
        ) : null}
      </div>
      <div className="text-xs text-gray-500">{formatShortDate(day.date)}</div>
      {day.isToday ? (
        <div className="text-moto-green text-[10px]">heute</div>
      ) : null}

      {day.showSick || day.showExcused ? (
        <div className="mt-1 inline-flex items-center rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600">
          kommt nicht
        </div>
      ) : day.isAbsent ? (
        <div className="text-moto-orange bg-moto-orange/10 mt-1 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
          kommt nicht
        </div>
      ) : (
        <div className="mt-1 text-sm font-semibold text-gray-900">
          {day.effectiveTime ?? "-"}
        </div>
      )}

      {day.baseSchedule?.notes ? (
        <div className="mt-1 flex items-start justify-center gap-1 text-xs text-gray-400 italic">
          <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0" />
          <span className="text-left">{day.baseSchedule.notes}</span>
        </div>
      ) : null}

      {day.notes.map((note) => (
        <div
          key={note.id}
          className="mt-1 flex items-start justify-center gap-1 text-xs text-gray-500"
        >
          <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0 text-gray-400" />
          <span className="text-left">{note.content}</span>
        </div>
      ))}

      {!readOnly ? (
        <button
          type="button"
          onClick={() => onEditDay(day)}
          className="absolute top-1 right-1 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
          title="Tag bearbeiten"
        >
          <SquarePen className="h-3.5 w-3.5" />
        </button>
      ) : null}
    </div>
  );
}
