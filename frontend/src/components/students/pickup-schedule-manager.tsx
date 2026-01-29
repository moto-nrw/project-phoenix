"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import {
  Calendar,
  ChevronLeft,
  ChevronRight,
  SquarePen,
  Loader2,
  StickyNote,
} from "lucide-react";
import { PickupScheduleFormModal } from "./pickup-schedule-form-modal";
import { PickupDayEditModal } from "./pickup-day-edit-modal";
import type {
  PickupData,
  BulkPickupScheduleFormData,
  DayData,
} from "@/lib/pickup-schedule-helpers";
import {
  WEEKDAYS,
  formatPickupTime,
  mergeSchedulesWithTemplate,
  getWeekDays,
  formatShortDate,
  formatDateISO,
  getDayData,
} from "@/lib/pickup-schedule-helpers";
import {
  fetchStudentPickupData,
  updateStudentPickupSchedules,
  createStudentPickupException,
  updateStudentPickupException,
  deleteStudentPickupException,
  createStudentPickupNote,
  updateStudentPickupNote,
  deleteStudentPickupNote,
} from "@/lib/pickup-schedule-api";

interface PickupScheduleManagerProps {
  readonly studentId: string;
  readonly readOnly?: boolean;
  readonly onUpdate?: () => void;
  readonly isSick?: boolean;
}

export default function PickupScheduleManager({
  studentId,
  readOnly = false,
  onUpdate,
  isSick = false,
}: PickupScheduleManagerProps) {
  const [pickupData, setPickupData] = useState<PickupData>({
    schedules: [],
    exceptions: [],
    notes: [],
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [weekOffset, setWeekOffset] = useState(0); // 0 = current week

  // Modal states
  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false);
  const [editingDay, setEditingDay] = useState<DayData | null>(null);

  // Compute week data
  const weekDays = useMemo(() => getWeekDays(weekOffset), [weekOffset]);

  // Merge schedule + exceptions + sick + notes for each day
  const dayDataList = useMemo(
    () =>
      weekDays.map((date) =>
        getDayData(
          date,
          pickupData.schedules,
          pickupData.exceptions,
          isSick,
          pickupData.notes,
        ),
      ),
    [
      weekDays,
      pickupData.schedules,
      pickupData.exceptions,
      isSick,
      pickupData.notes,
    ],
  );

  // Load pickup data
  const loadPickupData = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await fetchStudentPickupData(studentId);
      setPickupData(data);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Fehler beim Laden des Abholplans",
      );
    } finally {
      setIsLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    loadPickupData().catch(() => {
      // Error already handled in loadPickupData
    });
  }, [loadPickupData]);

  // Handle schedule update
  const handleUpdateSchedules = async (data: BulkPickupScheduleFormData) => {
    await updateStudentPickupSchedules(studentId, data);
    await loadPickupData();
    onUpdate?.();
    setIsScheduleModalOpen(false);
  };

  // Open day edit modal
  const handleOpenDayEdit = (day: DayData) => {
    if (readOnly || day.weekday === 0) return;
    setEditingDay(day);
  };

  // Refresh day data after changes (keeps modal open with fresh data)
  const refreshAndKeepModal = useCallback(async () => {
    const data = await fetchStudentPickupData(studentId);
    setPickupData(data);
    onUpdate?.();
  }, [studentId, onUpdate]);

  // Day edit modal: exception handlers
  const handleSaveException = useCallback(
    async (params: { pickupTime?: string; reason?: string }) => {
      if (!editingDay) return;
      const dateStr = formatDateISO(editingDay.date);

      if (editingDay.exception) {
        // Update existing exception
        await updateStudentPickupException(studentId, editingDay.exception.id, {
          exceptionDate: dateStr,
          pickupTime: params.pickupTime,
          reason: params.reason,
        });
      } else {
        // Create new exception
        await createStudentPickupException(studentId, {
          exceptionDate: dateStr,
          pickupTime: params.pickupTime,
          reason: params.reason,
        });
      }
      await refreshAndKeepModal();
    },
    [editingDay, studentId, refreshAndKeepModal],
  );

  const handleDeleteException = useCallback(async () => {
    if (!editingDay?.exception) return;
    await deleteStudentPickupException(studentId, editingDay.exception.id);
    await refreshAndKeepModal();
  }, [editingDay, studentId, refreshAndKeepModal]);

  // Day edit modal: note handlers
  const handleCreateNote = useCallback(
    async (content: string) => {
      if (!editingDay) return;
      await createStudentPickupNote(studentId, {
        noteDate: formatDateISO(editingDay.date),
        content,
      });
      await refreshAndKeepModal();
    },
    [editingDay, studentId, refreshAndKeepModal],
  );

  const handleUpdateNote = useCallback(
    async (noteId: string, content: string) => {
      if (!editingDay) return;
      await updateStudentPickupNote(studentId, noteId, {
        noteDate: formatDateISO(editingDay.date),
        content,
      });
      await refreshAndKeepModal();
    },
    [editingDay, studentId, refreshAndKeepModal],
  );

  const handleDeleteNote = useCallback(
    async (noteId: string) => {
      await deleteStudentPickupNote(studentId, noteId);
      await refreshAndKeepModal();
    },
    [studentId, refreshAndKeepModal],
  );

  // Close day edit modal
  const handleCloseDayEdit = () => {
    setEditingDay(null);
  };

  // Navigate weeks
  const goToPreviousWeek = () => setWeekOffset((w) => w - 1);
  const goToNextWeek = () => setWeekOffset((w) => w + 1);

  // Keep editingDay in sync with latest pickupData
  const currentEditingDay = useMemo(() => {
    if (!editingDay) return null;
    return getDayData(
      editingDay.date,
      pickupData.schedules,
      pickupData.exceptions,
      isSick,
      pickupData.notes,
    );
  }, [editingDay, pickupData, isSick]);

  // Show loading state
  if (isLoading && pickupData.schedules.length === 0) {
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
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      {/* Header */}
      <div className="mb-4 flex items-center justify-between gap-2">
        <div className="flex min-w-0 flex-1 items-center gap-2 sm:gap-3">
          <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-[#F78C10]/10 text-[#F78C10] sm:h-10 sm:w-10">
            <Calendar className="h-5 w-5" />
          </div>
          <h2 className="truncate text-base font-semibold text-gray-900 sm:text-lg">
            Abholplan und Notizen
          </h2>
        </div>
        {!readOnly && (
          <button
            onClick={() => setIsScheduleModalOpen(true)}
            className="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
          >
            Bearbeiten
          </button>
        )}
      </div>

      {/* Mobile: Week nav + Vertical list */}
      <div className="sm:hidden">
        <div className="mb-3 flex items-center justify-between">
          <button
            onClick={goToPreviousWeek}
            className="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
            title="Vorherige Woche"
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <div className="flex-1" />
          <button
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

      {/* Desktop: Week info + Arrows inline with grid */}
      <div className="hidden sm:block">
        <div className="flex items-center gap-3">
          <button
            onClick={goToPreviousWeek}
            className="flex-shrink-0 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            title="Vorherige Woche"
          >
            <ChevronLeft className="h-5 w-5" />
          </button>
          <div className="flex-1">
            {/* Day grid */}
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
            onClick={goToNextWeek}
            className="flex-shrink-0 rounded-lg p-2 text-gray-400 hover:bg-gray-100 hover:text-gray-700"
            title="Nächste Woche"
          >
            <ChevronRight className="h-5 w-5" />
          </button>
        </div>
      </div>

      {/* Schedule Edit Modal */}
      <PickupScheduleFormModal
        isOpen={isScheduleModalOpen}
        onClose={() => setIsScheduleModalOpen(false)}
        onSubmit={handleUpdateSchedules}
        initialSchedules={mergeSchedulesWithTemplate(pickupData.schedules)}
      />

      {/* Day Edit Modal */}
      <PickupDayEditModal
        isOpen={editingDay !== null}
        onClose={handleCloseDayEdit}
        day={currentEditingDay}
        studentId={studentId}
        onSaveException={handleSaveException}
        onDeleteException={handleDeleteException}
        onCreateNote={handleCreateNote}
        onUpdateNote={handleUpdateNote}
        onDeleteNote={handleDeleteNote}
      />
    </div>
  );
}

// ============================================
// Day Row Component (Mobile)
// ============================================

interface DayComponentProps {
  readonly day: DayData;
  readonly readOnly: boolean;
  readonly onEditDay: (day: DayData) => void;
}

function DayRow({ day, readOnly, onEditDay }: DayComponentProps) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  const effectiveTime = day.effectiveTime
    ? formatPickupTime(day.effectiveTime)
    : null;

  const hasNotes = !!day.baseSchedule?.notes || day.notes.length > 0;

  return (
    <div
      className={`rounded-lg border px-3 py-2 ${
        day.isToday
          ? "border-[#F78C10] bg-[#F78C10]/5"
          : "border-gray-200 bg-white"
      }`}
    >
      {/* Top row: weekday, time, indicators, edit */}
      <div className="flex items-center gap-3">
        {/* Weekday + Date */}
        <div className="w-16 flex-shrink-0">
          <div
            className={`text-sm font-medium ${
              day.isToday ? "text-[#F78C10]" : "text-gray-700"
            }`}
          >
            {weekdayInfo?.shortLabel} {formatShortDate(day.date)}
          </div>
          {day.isToday && (
            <div className="text-[10px] text-[#F78C10]">heute</div>
          )}
        </div>

        {/* Content */}
        {day.showSick ? (
          <div
            className="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium text-white"
            style={{ backgroundColor: "#EAB308" }}
          >
            <span className="h-1.5 w-1.5 rounded-full bg-white/80" />
            <span>Krank</span>
          </div>
        ) : (
          <>
            {/* Time */}
            <div className="w-12 flex-shrink-0 text-sm font-semibold text-gray-900">
              {effectiveTime ?? "—"}
            </div>

            {/* Exception indicator */}
            <div className="min-w-0 flex-1">
              {day.isException && (
                <span className="inline-flex h-5 w-5 items-center justify-center rounded-full bg-orange-100 text-orange-600">
                  <svg
                    className="h-3 w-3"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                  >
                    <circle cx="10" cy="10" r="5" />
                  </svg>
                </span>
              )}
            </div>

            {/* Edit button */}
            {!readOnly && (
              <button
                onClick={() => onEditDay(day)}
                className="flex-shrink-0 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
                title="Tag bearbeiten"
              >
                <SquarePen className="h-4 w-4" />
              </button>
            )}
          </>
        )}
      </div>

      {/* Notes below (full width) */}
      {!day.showSick && hasNotes && (
        <div className="mt-1.5 space-y-0.5 pl-[76px]">
          {day.baseSchedule?.notes && (
            <div className="flex items-start gap-1 text-xs text-gray-400 italic">
              <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0" />
              <span>{day.baseSchedule.notes}</span>
            </div>
          )}
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
      )}
    </div>
  );
}

// ============================================
// Day Cell Component (Desktop)
// ============================================

function DayCell({ day, readOnly, onEditDay }: DayComponentProps) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  const effectiveTime = day.effectiveTime
    ? formatPickupTime(day.effectiveTime)
    : null;

  return (
    <div
      className={`group relative rounded-lg border p-2 text-center ${
        day.isToday
          ? "border-[#F78C10] bg-[#F78C10]/5"
          : "border-gray-200 bg-white"
      }`}
    >
      {/* Weekday + indicators */}
      <div className="flex items-center justify-center gap-1">
        <div
          className={`text-xs font-medium ${
            day.isToday ? "text-[#F78C10]" : "text-gray-500"
          }`}
        >
          {weekdayInfo?.shortLabel}
        </div>
        {day.isException && (
          <span
            className="flex h-4 w-4 items-center justify-center rounded-full bg-orange-100 text-orange-600"
            title="Abweichende Zeit"
          >
            <svg className="h-2 w-2" viewBox="0 0 20 20" fill="currentColor">
              <circle cx="10" cy="10" r="5" />
            </svg>
          </span>
        )}
      </div>

      {/* Date */}
      <div className="text-xs text-gray-500">{formatShortDate(day.date)}</div>

      {/* Today indicator */}
      {day.isToday && <div className="text-[10px] text-[#F78C10]">heute</div>}

      {/* Content */}
      {day.showSick ? (
        <div
          className="mt-1 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium text-white"
          style={{ backgroundColor: "#EAB308" }}
        >
          <span className="h-1.5 w-1.5 rounded-full bg-white/80" />
          <span>Krank</span>
        </div>
      ) : (
        <>
          {/* Time */}
          <div className="mt-1 text-sm font-semibold text-gray-900">
            {effectiveTime ?? "—"}
          </div>

          {/* Schedule note (recurring weekly) */}
          {day.baseSchedule?.notes && (
            <div className="mt-1 flex items-start justify-center gap-1 text-xs text-gray-400 italic">
              <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0" />
              <span className="text-left">{day.baseSchedule.notes}</span>
            </div>
          )}

          {/* Day-specific notes */}
          {day.notes.map((note) => (
            <div
              key={note.id}
              className="mt-1 flex items-start justify-center gap-1 text-xs text-gray-500"
            >
              <StickyNote className="mt-0.5 h-3 w-3 flex-shrink-0 text-gray-400" />
              <span className="text-left">{note.content}</span>
            </div>
          ))}
        </>
      )}

      {/* Edit button — always visible */}
      {!readOnly && (
        <button
          onClick={() => onEditDay(day)}
          className="absolute top-1 right-1 rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
          title="Tag bearbeiten"
        >
          <SquarePen className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  );
}
