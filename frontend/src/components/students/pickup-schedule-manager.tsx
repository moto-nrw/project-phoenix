"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import { mutate } from "swr";
import {
  ChevronLeft,
  ChevronRight,
  SquarePen,
  Loader2,
  StickyNote,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionHeader } from "~/components/ui/concept-section-header";
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
import { createLogger } from "~/lib/logger";
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
import type { StudentStatusDay } from "~/lib/student-status-days-api";

const logger = createLogger({ component: "PickupScheduleManager" });
const EMPTY_STATUS_DAYS: StudentStatusDay[] = [];

/** Invalidate SWR caches that contain pickup time data (OGS groups, dashboard). */
function invalidatePickupCaches() {
  try {
    mutate(
      (key) =>
        typeof key === "string" &&
        (key.includes("ogs-students-") || key.includes("dashboard")),
    ).catch(() => {});
  } catch {
    // SWR cache not available (e.g. in tests)
  }
}

interface PickupScheduleManagerProps {
  readonly studentId: string;
  readonly readOnly?: boolean;
  readonly onUpdate?: () => void;
  readonly isSick?: boolean;
  readonly isExcused?: boolean;
  readonly statusDays?: StudentStatusDay[];
}

export default function PickupScheduleManager({
  studentId,
  readOnly = false,
  onUpdate,
  isSick = false,
  isExcused = false,
  statusDays = EMPTY_STATUS_DAYS,
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
  const statusByDate = useMemo(() => {
    const entries = new Map<string, StudentStatusDay["status"]>();
    for (const day of statusDays) {
      if (!day.cleared_at) {
        entries.set(day.date, day.status);
      }
    }
    return entries;
  }, [statusDays]);

  // Merge schedule + exceptions + sick / excused + notes for each day
  const dayDataList = useMemo(
    () =>
      weekDays.map((date) =>
        getDayData(
          date,
          pickupData.schedules,
          pickupData.exceptions,
          isSick,
          pickupData.notes,
          isExcused,
          statusByDate.get(formatDateISO(date)) ?? null,
        ),
      ),
    [
      weekDays,
      pickupData.schedules,
      pickupData.exceptions,
      isSick,
      isExcused,
      pickupData.notes,
      statusByDate,
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
      logger.error("pickup_data_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
      setError(
        err instanceof Error ? err.message : "Fehler beim Laden des Gehplans",
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
    invalidatePickupCaches();
  };

  // Open day edit modal
  const handleOpenDayEdit = (day: DayData) => {
    if (readOnly || day.weekday === 0) return;
    setEditingDay(day);
  };

  // Refresh day data after changes (keeps modal open with fresh data)
  // Also invalidates OGS groups SWR caches so pickup times update on navigation back
  const refreshAndKeepModal = useCallback(async () => {
    const data = await fetchStudentPickupData(studentId);
    setPickupData(data);
    onUpdate?.();
    invalidatePickupCaches();
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
          ...(params.pickupTime === undefined ? { clearPickupTime: true } : {}),
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
      isExcused,
      statusByDate.get(formatDateISO(editingDay.date)) ?? null,
    );
  }, [editingDay, pickupData, isSick, isExcused, statusByDate]);

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
      <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-lg border px-4 py-3">
        {error}
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
      {/* Header */}
      <div className="mb-4">
        <SectionHeader
          title="Gehplan & Notizen"
          icon={<MotoConceptIcon concept="pickup" size={22} />}
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

      {/* Mobile: Week nav + Vertical list */}
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

      {/* Desktop: Week info + Arrows inline with grid */}
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
            type="button"
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
          ? "border-moto-orange bg-moto-orange/5"
          : "border-gray-200 bg-white"
      }`}
    >
      {/* Top row: weekday, time, indicators, edit */}
      <div className="flex items-center gap-3">
        {/* Weekday + Date */}
        <div className="w-16 flex-shrink-0">
          <div
            className={`text-sm font-medium ${
              day.isToday ? "text-moto-orange" : "text-gray-700"
            }`}
          >
            {weekdayInfo?.shortLabel} {formatShortDate(day.date)}
          </div>
          {day.isToday && (
            <div className="text-moto-orange text-[10px]">heute</div>
          )}
        </div>

        {/* Content */}
        {day.showSick || day.showExcused ? (
          <div className="inline-flex items-center rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600">
            <span>keine Abholung</span>
          </div>
        ) : (
          <>
            {/* Time */}
            <div className="w-12 flex-shrink-0 text-sm font-semibold text-gray-900">
              {effectiveTime ?? "-"}
            </div>

            {/* Exception indicator */}
            <div className="min-w-0 flex-1">
              {day.isException && (
                <span className="bg-moto-orange/15 text-moto-orange inline-flex h-5 w-5 items-center justify-center rounded-full">
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
                type="button"
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
      {!day.showSick && !day.showExcused && hasNotes && (
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
          ? "border-moto-orange bg-moto-orange/5"
          : "border-gray-200 bg-white"
      }`}
    >
      {/* Weekday + indicators */}
      <div className="flex items-center justify-center gap-1">
        <div
          className={`text-xs font-medium ${
            day.isToday ? "text-moto-orange" : "text-gray-500"
          }`}
        >
          {weekdayInfo?.shortLabel}
        </div>
        {day.isException && (
          <span
            className="bg-moto-orange/15 text-moto-orange flex h-4 w-4 items-center justify-center rounded-full"
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
      {day.isToday && <div className="text-moto-orange text-[10px]">heute</div>}

      {/* Content */}
      {day.showSick || day.showExcused ? (
        <div className="mt-1 inline-flex items-center rounded-full border border-gray-200 bg-gray-50 px-2 py-0.5 text-xs font-medium text-gray-600">
          <span>keine Abholung</span>
        </div>
      ) : (
        <>
          {/* Time */}
          <div className="mt-1 text-sm font-semibold text-gray-900">
            {effectiveTime ?? "-"}
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
          type="button"
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
