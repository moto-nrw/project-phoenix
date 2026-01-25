"use client";

import { useState, useEffect, useCallback } from "react";
import { Calendar, Plus, Loader2 } from "lucide-react";
import { PickupScheduleFormModal } from "./pickup-schedule-form-modal";
import { PickupExceptionFormModal } from "./pickup-exception-form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import type {
  PickupData,
  PickupSchedule,
  PickupException,
  BulkPickupScheduleFormData,
  PickupExceptionFormData,
} from "@/lib/pickup-schedule-helpers";
import {
  WEEKDAYS,
  formatPickupTime,
  formatExceptionDate,
  isDateInPast,
  mergeSchedulesWithTemplate,
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
}

export default function PickupScheduleManager({
  studentId,
  readOnly = false,
  onUpdate,
}: PickupScheduleManagerProps) {
  const [pickupData, setPickupData] = useState<PickupData>({
    schedules: [],
    exceptions: [],
  });
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Modal states
  const [isScheduleModalOpen, setIsScheduleModalOpen] = useState(false);
  const [isExceptionModalOpen, setIsExceptionModalOpen] = useState(false);
  const [editingException, setEditingException] = useState<
    PickupException | undefined
  >();
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deletingException, setDeletingException] = useState<
    PickupException | undefined
  >();
  const [isDeleting, setIsDeleting] = useState(false);

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
    setIsExceptionModalOpen(true);
  };

  // Open exception modal for creating
  const handleOpenCreateException = () => {
    setEditingException(undefined);
    setIsExceptionModalOpen(true);
  };

  // Close exception modal
  const handleCloseExceptionModal = () => {
    setIsExceptionModalOpen(false);
    setEditingException(undefined);
  };

  // Get schedule for a weekday
  const getScheduleForWeekday = (
    weekday: number,
  ): PickupSchedule | undefined => {
    return pickupData.schedules.find((s) => s.weekday === weekday);
  };

  // Filter to only show upcoming exceptions
  const upcomingExceptions = pickupData.exceptions.filter(
    (e) => !isDateInPast(e.exceptionDate),
  );

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

      {/* Weekly Schedule Grid */}
      <div className="mb-6">
        <h3 className="mb-3 text-sm font-medium text-gray-700">
          Wöchentlicher Plan
        </h3>
        <div className="grid grid-cols-5 gap-2">
          {WEEKDAYS.map((day) => {
            const schedule = getScheduleForWeekday(day.value);
            return (
              <div
                key={day.value}
                className="rounded-lg border border-gray-200 bg-white p-2 text-center"
              >
                <div className="mb-1 text-xs font-medium text-gray-500">
                  {day.shortLabel}
                </div>
                <div className="text-sm font-semibold text-gray-900">
                  {schedule ? formatPickupTime(schedule.pickupTime) : "—"}
                </div>
                {schedule?.notes && (
                  <div className="mt-1 truncate text-xs text-gray-500">
                    {schedule.notes}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Exceptions Section */}
      <div>
        <div className="mb-3 flex items-center justify-between">
          <h3 className="text-sm font-medium text-gray-700">Ausnahmen</h3>
          {!readOnly && (
            <button
              onClick={handleOpenCreateException}
              className="inline-flex items-center gap-1 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-700"
            >
              <Plus className="h-4 w-4" />
              Hinzufügen
            </button>
          )}
        </div>

        {upcomingExceptions.length === 0 ? (
          <p className="text-sm text-gray-500">
            Keine anstehenden Ausnahmen geplant.
          </p>
        ) : (
          <div className="space-y-2">
            {upcomingExceptions.map((exception) => (
              <div
                key={exception.id}
                className="flex items-center justify-between rounded-lg border border-gray-200 bg-white px-3 py-2"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-gray-900">
                      {formatExceptionDate(exception.exceptionDate)}
                    </span>
                    <span className="text-sm text-gray-500">
                      {exception.pickupTime
                        ? formatPickupTime(exception.pickupTime)
                        : "Kein Abholen"}
                    </span>
                  </div>
                  <div className="truncate text-xs text-gray-500">
                    {exception.reason}
                  </div>
                </div>
                {!readOnly && (
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => handleOpenEditException(exception)}
                      className="rounded p-1 text-gray-500 hover:bg-gray-100 hover:text-gray-700"
                      title="Bearbeiten"
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
                          d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
                        />
                      </svg>
                    </button>
                    <button
                      onClick={() => handleDeleteClick(exception)}
                      className="rounded p-1 text-gray-500 hover:bg-red-50 hover:text-red-600"
                      title="Löschen"
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
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

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
