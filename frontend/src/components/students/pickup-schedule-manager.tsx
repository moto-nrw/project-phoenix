"use client";

import { useState, useEffect, useCallback, useMemo } from "react";
import {
  Calendar,
  ChevronLeft,
  ChevronRight,
  Plus,
  Loader2,
} from "lucide-react";
import { PickupScheduleFormModal } from "./pickup-schedule-form-modal";
import { PickupExceptionFormModal } from "./pickup-exception-form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import type {
  PickupData,
  PickupException,
  BulkPickupScheduleFormData,
  PickupExceptionFormData,
  DayData,
} from "@/lib/pickup-schedule-helpers";
import {
  WEEKDAYS,
  formatPickupTime,
  formatExceptionDate,
  mergeSchedulesWithTemplate,
  getWeekDays,
  formatShortDate,
  formatWeekRange,
  formatDateISO,
  getDayData,
  getCalendarWeek,
} from "@/lib/pickup-schedule-helpers";
import {
  fetchStudentPickupData,
  updateStudentPickupSchedules,
  createStudentPickupException,
  updateStudentPickupException,
  deleteStudentPickupException,
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
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [weekOffset, setWeekOffset] = useState(0); // 0 = current week

  // Modal states
  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false);
  const [isExceptionModalOpen, setIsExceptionModalOpen] = useState(false);
  const [editingException, setEditingException] = useState<
    PickupException | undefined
  >();
  const [defaultExceptionDate, setDefaultExceptionDate] = useState<
    string | undefined
  >();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletingException, setDeletingException] = useState<
    PickupException | undefined
  >();
  const [isDeleting, setIsDeleting] = useState(false);

  // Compute week data
  const weekDays = useMemo(() => getWeekDays(weekOffset), [weekOffset]);
  const weekStart = weekDays[0]!;
  const weekEnd = weekDays[4]!;

  // Merge schedule + exceptions + sick for each day
  const dayDataList = useMemo(
    () =>
      weekDays.map((date) =>
        getDayData(date, pickupData.schedules, pickupData.exceptions, isSick),
      ),
    [weekDays, pickupData.schedules, pickupData.exceptions, isSick],
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

  // Handle create exception
  const handleCreateException = async (data: PickupExceptionFormData) => {
    await createStudentPickupException(studentId, data);
    await loadPickupData();
    onUpdate?.();
    setIsExceptionModalOpen(false);
    setDefaultExceptionDate(undefined);
  };

  // Handle update exception
  const handleUpdateException = async (data: PickupExceptionFormData) => {
    if (!editingException) return;
    await updateStudentPickupException(studentId, editingException.id, data);
    await loadPickupData();
    onUpdate?.();
    setIsExceptionModalOpen(false);
    setEditingException(undefined);
  };

  // Handle delete exception
  const handleDeleteClick = (exception: PickupException) => {
    setDeletingException(exception);
    setShowDeleteModal(true);
  };

  const handleConfirmDelete = async () => {
    if (!deletingException) return;

    setIsDeleting(true);
    try {
      await deleteStudentPickupException(studentId, deletingException.id);
      await loadPickupData();
      onUpdate?.();
      setShowDeleteModal(false);
      setDeletingException(undefined);
    } catch (err) {
      alert(
        err instanceof Error ? err.message : "Fehler beim Löschen der Ausnahme",
      );
    } finally {
      setIsDeleting(false);
    }
  };

  const handleCancelDelete = () => {
    setShowDeleteModal(false);
    setDeletingException(undefined);
  };

  // Open exception modal for editing
  const handleOpenEditException = (exception: PickupException) => {
    setEditingException(exception);
    setDefaultExceptionDate(undefined);
    setIsExceptionModalOpen(true);
  };

  // Open exception modal for creating (optionally with pre-filled date)
  const handleOpenCreateException = (date?: Date) => {
    setEditingException(undefined);
    if (date) {
      // Only allow future dates
      const today = new Date();
      today.setHours(0, 0, 0, 0);
      if (date >= today) {
        setDefaultExceptionDate(formatDateISO(date));
      } else {
        setDefaultExceptionDate(undefined);
      }
    } else {
      setDefaultExceptionDate(undefined);
    }
    setIsExceptionModalOpen(true);
  };

  // Close exception modal
  const handleCloseExceptionModal = () => {
    setIsExceptionModalOpen(false);
    setEditingException(undefined);
    setDefaultExceptionDate(undefined);
  };

  // Navigate weeks
  const goToPreviousWeek = () => setWeekOffset((w) => w - 1);
  const goToNextWeek = () => setWeekOffset((w) => w + 1);
  const goToCurrentWeek = () => setWeekOffset(0);

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
            Abholplan
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
          <button
            onClick={goToCurrentWeek}
            className="flex flex-col items-center gap-0.5"
            title="Zur aktuellen Woche"
          >
            <span className="rounded bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
              KW {getCalendarWeek(weekStart)}
            </span>
            <span className="text-sm font-medium text-gray-600">
              {formatWeekRange(weekStart, weekEnd)}
            </span>
          </button>
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
              onEditException={handleOpenEditException}
              onDeleteException={handleDeleteClick}
              onCreateException={() => handleOpenCreateException(day.date)}
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
            {/* Week header */}
            <button
              onClick={goToCurrentWeek}
              className="mb-2 flex w-full items-center justify-center gap-2"
              title="Zur aktuellen Woche"
            >
              <span className="rounded bg-gray-100 px-2 py-0.5 text-sm font-semibold text-gray-700">
                KW {getCalendarWeek(weekStart)}
              </span>
              <span className="text-sm text-gray-600">
                {formatWeekRange(weekStart, weekEnd)}
              </span>
            </button>
            {/* Day grid */}
            <div className="grid grid-cols-5 gap-2">
              {dayDataList.map((day) => (
                <DayCell
                  key={formatDateISO(day.date)}
                  day={day}
                  readOnly={readOnly}
                  onEditException={handleOpenEditException}
                  onDeleteException={handleDeleteClick}
                  onCreateException={() => handleOpenCreateException(day.date)}
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

      {/* Footer Actions */}
      {!readOnly && (
        <div className="mt-4 flex items-center justify-end border-t border-gray-100 pt-4">
          <button
            onClick={() => handleOpenCreateException()}
            className="inline-flex items-center gap-1 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-700"
          >
            <Plus className="h-4 w-4" />
            Ausnahme
          </button>
        </div>
      )}

      {/* Schedule Edit Modal */}
      <PickupScheduleFormModal
        isOpen={isScheduleModalOpen}
        onClose={() => setIsScheduleModalOpen(false)}
        onSubmit={handleUpdateSchedules}
        initialSchedules={mergeSchedulesWithTemplate(pickupData.schedules)}
      />

      {/* Exception Modal */}
      <PickupExceptionFormModal
        isOpen={isExceptionModalOpen}
        onClose={handleCloseExceptionModal}
        onSubmit={
          editingException ? handleUpdateException : handleCreateException
        }
        initialData={editingException}
        mode={editingException ? "edit" : "create"}
        defaultDate={defaultExceptionDate}
      />

      {/* Delete Confirmation Modal */}
      <ConfirmationModal
        isOpen={showDeleteModal}
        onClose={handleCancelDelete}
        onConfirm={handleConfirmDelete}
        title="Ausnahme löschen"
        confirmText={isDeleting ? "Wird gelöscht..." : "Löschen"}
        cancelText="Abbrechen"
        isConfirmLoading={isDeleting}
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p>
          Möchten Sie die Ausnahme für{" "}
          <strong>
            {deletingException
              ? formatExceptionDate(deletingException.exceptionDate)
              : ""}
          </strong>{" "}
          wirklich löschen?
        </p>
      </ConfirmationModal>
    </div>
  );
}

// ============================================
// Day Row Component (Mobile)
// ============================================

interface DayComponentProps {
  readonly day: DayData;
  readonly readOnly: boolean;
  readonly onEditException: (exception: PickupException) => void;
  readonly onDeleteException: (exception: PickupException) => void;
  readonly onCreateException: () => void;
}

function DayRow({
  day,
  readOnly,
  onEditException,
  onDeleteException,
}: DayComponentProps) {
  const weekdayInfo = WEEKDAYS[day.weekday - 1];
  const effectiveTime = day.effectiveTime
    ? formatPickupTime(day.effectiveTime)
    : null;

  return (
    <div
      className={`flex items-center gap-3 rounded-lg border px-3 py-2 ${
        day.isToday
          ? "border-[#F78C10] bg-[#F78C10]/5"
          : "border-gray-200 bg-white"
      }`}
    >
      {/* Weekday + Date */}
      <div className="w-16 flex-shrink-0">
        <div
          className={`text-sm font-medium ${
            day.isToday ? "text-[#F78C10]" : "text-gray-700"
          }`}
        >
          {weekdayInfo?.shortLabel} {formatShortDate(day.date)}
        </div>
        {day.isToday && <div className="text-[10px] text-[#F78C10]">heute</div>}
      </div>

      {/* Content */}
      {day.showSick ? (
        <div className="inline-flex items-center gap-1 rounded-full bg-pink-100 px-2 py-0.5 text-xs font-medium text-pink-700">
          <span className="h-1.5 w-1.5 rounded-full bg-pink-500" />
          Krank
        </div>
      ) : (
        <>
          {/* Time */}
          <div className="w-12 flex-shrink-0 text-sm font-semibold text-gray-900">
            {effectiveTime ?? "—"}
          </div>

          {/* Exception indicator + notes */}
          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            {day.isException && day.exception && (
              <span
                className="flex h-5 w-5 flex-shrink-0 cursor-pointer items-center justify-center rounded-full bg-orange-100 text-orange-600"
                onClick={() => !readOnly && onEditException(day.exception!)}
                title={readOnly ? day.exception.reason : "Ausnahme bearbeiten"}
              >
                <svg
                  className="h-3 w-3"
                  viewBox="0 0 20 20"
                  fill="currentColor"
                >
                  <circle cx="10" cy="10" r="5" />
                </svg>
              </span>
            )}
            {day.effectiveNotes && (
              <span className="truncate text-sm text-gray-500">
                {day.effectiveNotes}
              </span>
            )}
          </div>

          {/* Edit/Delete for exceptions */}
          {!readOnly && day.isException && day.exception && (
            <button
              onClick={() => onDeleteException(day.exception!)}
              className="flex-shrink-0 rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600"
              title="Ausnahme löschen"
            >
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
          )}
        </>
      )}
    </div>
  );
}

// ============================================
// Day Cell Component (Desktop)
// ============================================

function DayCell({
  day,
  readOnly,
  onEditException,
  onDeleteException,
}: DayComponentProps) {
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
      {/* Weekday + Exception indicator */}
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
            className="flex h-4 w-4 cursor-pointer items-center justify-center rounded-full bg-orange-100 text-orange-600"
            onClick={() =>
              !readOnly && day.exception && onEditException(day.exception)
            }
            title={day.exception?.reason ?? "Ausnahme"}
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
        <div className="mt-1 inline-flex items-center gap-1 rounded-full bg-pink-100 px-2 py-0.5 text-xs font-medium text-pink-700">
          <span className="h-1.5 w-1.5 rounded-full bg-pink-500" />
          Krank
        </div>
      ) : (
        <>
          {/* Time */}
          <div className="mt-1 text-sm font-semibold text-gray-900">
            {effectiveTime ?? "—"}
          </div>

          {/* Notes */}
          {day.effectiveNotes && (
            <div
              title={day.effectiveNotes}
              className={`mt-1 truncate text-xs ${
                day.isException ? "text-orange-600" : "text-gray-500"
              }`}
            >
              {day.effectiveNotes}
            </div>
          )}
        </>
      )}

      {/* Delete button for exceptions (hover only on desktop) */}
      {!readOnly && day.isException && day.exception && (
        <button
          onClick={() => onDeleteException(day.exception!)}
          className="absolute top-1 right-1 hidden rounded p-1 text-gray-400 group-hover:block hover:bg-red-50 hover:text-red-600"
          title="Ausnahme löschen"
        >
          <svg
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      )}
    </div>
  );
}
