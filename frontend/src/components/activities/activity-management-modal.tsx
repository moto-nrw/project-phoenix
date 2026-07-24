"use client";

import { useState, useEffect } from "react";
import type { FormEvent } from "react";
import {
  updateActivity,
  deleteActivity,
  type Activity,
} from "~/lib/activity-api";
import { getDbOperationMessage } from "~/lib/use-notification";
import { useActivityForm } from "~/hooks/useActivityForm";
import { createLogger } from "~/lib/logger";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { SpinnerIcon } from "~/components/ui/icons";
import { getApiErrorMessage } from "~/lib/api-error-message";

const logger = createLogger({ component: "ActivityManagement" });

interface ActivityManagementModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: (message?: string) => void;
  readonly activity: Activity;
  readonly currentStaffId?: string | null;
  readonly readOnly?: boolean;
}

/** Maps delete error to user-friendly German message */
export function getDeleteErrorMessage(err: unknown): string {
  if (!(err instanceof Error)) {
    return "Fehler beim Löschen der Aktivität";
  }
  const message = err.message;
  if (message.includes("students enrolled")) {
    return "Diese Aktivität kann nicht gelöscht werden, da noch Kinder eingeschrieben sind. Bitte entfernen Sie zuerst alle Kinder aus der Aktivität.";
  }
  // Check for ownership/permission error (403 with specific message)
  if (
    message.includes("403") &&
    (message.includes("you can only modify") ||
      message.includes("created or supervise"))
  ) {
    return "Sie können diese Aktivität nicht löschen, da Sie sie nicht erstellt haben und kein Betreuer sind.";
  }
  if (message.includes("401")) {
    return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
  }
  // Generic 403 - could be other permission issues
  if (message.includes("403")) {
    return "Sie haben keine Berechtigung, diese Aktivität zu löschen.";
  }
  return message;
}

// Helper component for delete confirmation footer
function DeleteConfirmFooter({
  isDeleting,
  onCancel,
  onDelete,
}: Readonly<{
  isDeleting: boolean;
  onCancel: () => void;
  onDelete: () => void;
}>) {
  return (
    <div className="flex w-full items-center justify-end gap-3">
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={onCancel}
        disabled={isDeleting}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="danger"
        size="md"
        onClick={onDelete}
        disabled={isDeleting}
        className="min-w-[112px]"
      >
        {isDeleting ? (
          <span className="flex items-center justify-center gap-2">
            <SpinnerIcon />
            Löschen...
          </span>
        ) : (
          "Löschen"
        )}
      </Button>
    </div>
  );
}

// Helper component for normal footer with save/delete buttons
function NormalFooter({
  readOnly,
  isSubmitting,
  isDeleting,
  loading,
  onClose,
  onShowDeleteConfirm,
}: Readonly<{
  readOnly: boolean;
  isSubmitting: boolean;
  isDeleting: boolean;
  loading: boolean;
  onClose: () => void;
  onShowDeleteConfirm: () => void;
}>) {
  return (
    <div className="flex w-full items-center justify-between">
      <div className="flex items-center gap-2">
        {!readOnly && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onShowDeleteConfirm}
            className="text-gray-400 hover:text-red-600"
            disabled={isSubmitting || isDeleting}
            aria-label="Aktivität löschen"
          >
            <svg
              className="h-5 w-5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"
              />
            </svg>
          </Button>
        )}
      </div>

      <div className="flex items-center gap-3">
        <Button
          type="button"
          variant="ghost"
          size="md"
          onClick={onClose}
          disabled={isSubmitting || isDeleting}
        >
          Abbrechen
        </Button>

        {!readOnly && (
          <Button
            type="submit"
            size="md"
            form="activity-management-form"
            disabled={isSubmitting || loading || isDeleting}
            className="min-w-[100px]"
          >
            {isSubmitting ? (
              <span className="flex items-center justify-center gap-2">
                <SpinnerIcon />
                Speichern...
              </span>
            ) : (
              "Speichern"
            )}
          </Button>
        )}
      </div>
    </div>
  );
}

export function ActivityManagementModal({
  isOpen,
  onClose,
  onSuccess,
  activity,
  currentStaffId: _currentStaffId,
  readOnly = false,
}: ActivityManagementModalProps) {
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  // Use activity form hook for form state and validation
  const {
    form,
    setForm,
    categories,
    loading,
    error,
    setError,
    handleInputChange,
    validateForm,
  } = useActivityForm(
    {
      name: activity.name,
      category_id: activity.ag_category_id || "",
      max_participants: activity.max_participant?.toString() || "15",
    },
    isOpen,
  );

  // Reset form when activity changes
  useEffect(() => {
    if (isOpen) {
      setForm({
        name: activity.name,
        category_id: activity.ag_category_id || "",
        max_participants: activity.max_participant?.toString() || "15",
      });
      setError(null);
      setShowDeleteConfirm(false);
    }
  }, [isOpen, activity, setForm, setError]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      // Prepare the update data
      const updateData = {
        name: form.name.trim(),
        category_id: Number.parseInt(form.category_id, 10),
        max_participants: Number.parseInt(form.max_participants, 10),
        // Include existing values that might be required
        is_open: activity.is_open_ags || false,
        supervisor_ids: activity.supervisor_id
          ? [Number.parseInt(activity.supervisor_id, 10)]
          : [],
      };

      // Call the update API
      await updateActivity(activity.id, updateData);

      // Get success message
      const successMessage = getDbOperationMessage(
        "update",
        "Aktivität",
        form.name.trim(),
      );

      // Close modal with animation
      onClose();

      // Handle success with message after modal starts closing
      setTimeout(() => {
        if (onSuccess) {
          onSuccess(successMessage);
        }
      }, 100);
    } catch (err) {
      logger.error("activity_update_failed", {
        error: err instanceof Error ? err.message : String(err),
        activity_id: activity.id,
      });
      // Don't console.error for expected errors (403 permission denied, etc.)
      // The error is shown to the user via the UI
      setError(
        getApiErrorMessage(
          err,
          "bearbeiten",
          "Aktivitäten",
          "Failed to update activity",
        ),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDelete = async () => {
    setIsDeleting(true);
    setError(null);

    try {
      await deleteActivity(activity.id);

      // Get success message
      const successMessage = getDbOperationMessage(
        "delete",
        "Aktivität",
        activity.name,
      );

      // Close modal with animation
      onClose();

      // Handle success with message after modal starts closing
      setTimeout(() => {
        if (onSuccess) {
          onSuccess(successMessage);
        }
      }, 100);
    } catch (err) {
      logger.error("activity_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
        activity_id: activity.id,
      });
      // Don't console.error for expected errors (403 permission denied, etc.)
      // The error is shown to the user via the UI
      setError(getDeleteErrorMessage(err));
      setShowDeleteConfirm(false);
    } finally {
      setIsDeleting(false);
    }
  };

  const footer =
    !readOnly && showDeleteConfirm ? (
      <DeleteConfirmFooter
        isDeleting={isDeleting}
        onCancel={() => setShowDeleteConfirm(false)}
        onDelete={handleDelete}
      />
    ) : (
      <NormalFooter
        readOnly={readOnly}
        isSubmitting={isSubmitting}
        isDeleting={isDeleting}
        loading={loading}
        onClose={onClose}
        onShowDeleteConfirm={() => setShowDeleteConfirm(true)}
      />
    );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Aktivität: ${activity.name}`}
      size="sm"
      mobilePosition="center"
      footer={footer}
    >
      {loading ? (
        <ModalLoadingMessage message="Kategorien werden geladen..." />
      ) : (
        <form
          id="activity-management-form"
          onSubmit={handleSubmit}
          className="space-y-4"
        >
          {/* Creator info - positioned at top */}
          <div className="-mx-2 -mt-2 mb-4 border-b border-gray-100 px-2 pb-3 md:-mx-2 md:px-2">
            <p className="text-sm text-gray-500">
              Erstellt von:{" "}
              {activity.supervisors &&
              activity.supervisors.length > 0 &&
              activity.supervisors[0]
                ? (activity.supervisors[0].full_name ?? "Unbekannt")
                : "Unbekannt"}
            </p>
          </div>

          {error && <Alert type="error" message={error} />}

          {/* Activity Name Card - Compact */}
          <div className="relative overflow-hidden rounded-xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4">
            <div className="absolute top-1 right-1 h-12 w-12 rounded-full bg-gray-100/20 blur-xl"></div>
            <div className="relative">
              <label
                htmlFor="name"
                className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
              >
                <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                  <span className="text-[10px] font-bold text-white">1</span>
                </div>
                Aktivitätsname
              </label>
              <input
                id="name"
                name="name"
                value={form.name}
                onChange={handleInputChange}
                placeholder="z.B. Hausaufgaben, Malen, Basteln..."
                className="block w-full rounded-lg border-0 bg-white/80 px-3 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset placeholder:text-gray-400 focus:bg-white focus:ring-2 focus:ring-[#5080D8] focus:ring-inset disabled:cursor-not-allowed disabled:bg-gray-50 md:py-2.5 md:text-sm"
                required
                disabled={readOnly}
                maxLength={255}
              />
            </div>
          </div>

          {/* Category Card - Compact — no overflow-hidden on the card itself: it would clip the CustomSelect menu */}
          <div className="relative rounded-xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4">
            <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-xl">
              <div className="absolute top-1 left-1 h-10 w-10 rounded-full bg-gray-100/20 blur-xl"></div>
            </div>
            <div className="relative">
              <label
                id="category_id-label"
                htmlFor="category_id"
                className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
              >
                <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                  <span className="text-[10px] font-bold text-white">2</span>
                </div>
                Kategorie
              </label>
              <CustomSelect
                id="category_id"
                name="category_id"
                ariaLabelledBy="category_id-label"
                value={form.category_id}
                onChange={(next) => {
                  setForm((prev) => ({ ...prev, category_id: next }));
                  setError(null);
                }}
                options={categories.map((category) => ({
                  value: category.id,
                  label: category.name,
                }))}
                placeholder="Kategorie wählen..."
                required
                disabled={readOnly}
              />
            </div>
          </div>

          {/* Participants Card - Compact */}
          <div className="relative overflow-hidden rounded-xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4">
            <div className="absolute right-1 bottom-1 h-14 w-14 rounded-full bg-gray-100/20 blur-xl"></div>
            <div className="relative">
              <label
                htmlFor="max_participants"
                className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
              >
                <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                  <span className="text-[10px] font-bold text-white">3</span>
                </div>
                Maximale Teilnehmerzahl
              </label>
              <div className="relative flex items-center">
                <button
                  type="button"
                  onClick={() => {
                    const current = Number.parseInt(form.max_participants, 10);
                    if (current > 1) {
                      setForm((prev) => ({
                        ...prev,
                        max_participants: (current - 1).toString(),
                      }));
                    }
                  }}
                  className="absolute left-0 z-10 flex h-full w-12 items-center justify-center rounded-l-lg text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-[#5080D8] focus:outline-none focus:ring-inset active:scale-95 disabled:cursor-not-allowed disabled:opacity-30 md:w-10"
                  disabled={
                    Number.parseInt(form.max_participants, 10) <= 1 || readOnly
                  }
                  aria-label="Teilnehmer reduzieren"
                >
                  <svg
                    className="h-5 w-5 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2.5}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M19.5 12h-15"
                    />
                  </svg>
                </button>

                <input
                  id="max_participants"
                  name="max_participants"
                  type="number"
                  value={form.max_participants}
                  onChange={handleInputChange}
                  min="1"
                  max="50"
                  className="block w-full [appearance:textfield] rounded-lg border-0 bg-white/80 px-14 py-3 text-center text-lg font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset focus:bg-white focus:ring-2 focus:ring-[#5080D8] focus:ring-inset disabled:cursor-not-allowed disabled:bg-gray-50 md:px-12 md:py-2.5 md:text-base [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                  required
                  disabled={readOnly}
                />

                <button
                  type="button"
                  onClick={() => {
                    const current = Number.parseInt(form.max_participants, 10);
                    if (current < 50) {
                      setForm((prev) => ({
                        ...prev,
                        max_participants: (current + 1).toString(),
                      }));
                    }
                  }}
                  className="absolute right-0 z-10 flex h-full w-12 items-center justify-center rounded-r-lg text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-[#5080D8] focus:outline-none focus:ring-inset active:scale-95 disabled:cursor-not-allowed disabled:opacity-30 md:w-10"
                  disabled={
                    Number.parseInt(form.max_participants, 10) >= 50 || readOnly
                  }
                  aria-label="Teilnehmer erhöhen"
                >
                  <svg
                    className="h-5 w-5 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                    strokeWidth={2.5}
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      d="M12 4.5v15m7.5-7.5h-15"
                    />
                  </svg>
                </button>
              </div>
            </div>
          </div>

          {/* Info Card / Delete Confirmation - Compact */}
          {showDeleteConfirm ? (
            <div className="relative overflow-hidden rounded-lg border border-red-200/30 bg-gradient-to-br from-red-50/60 to-rose-50/60 p-3 backdrop-blur-sm">
              <div className="relative flex items-center gap-2">
                <svg
                  className="h-3.5 w-3.5 flex-shrink-0 text-red-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"
                  />
                </svg>
                <p className="text-xs font-medium text-red-700">
                  Diese Aktivität wirklich löschen?
                </p>
              </div>
            </div>
          ) : (
            <div className="relative overflow-hidden rounded-lg border border-gray-200/30 bg-gradient-to-br from-gray-50/60 to-slate-50/60 p-3 backdrop-blur-sm">
              <div className="relative flex items-center gap-2">
                <svg
                  className="h-3.5 w-3.5 flex-shrink-0 text-gray-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <p className="text-xs text-gray-600">
                  {readOnly
                    ? "Sie können nur Aktivitäten bearbeiten, die Sie selbst erstellt haben."
                    : "Änderungen werden sofort wirksam."}
                </p>
              </div>
            </div>
          )}
        </form>
      )}
    </FormModal>
  );
}

function ModalLoadingMessage({ message }: Readonly<{ message: string }>) {
  return (
    <div className="flex items-center justify-center py-12">
      <div className="flex flex-col items-center gap-4">
        <SpinnerIcon className="h-12 w-12 text-[#5080D8]" />
        <p className="text-gray-600">{message}</p>
      </div>
    </div>
  );
}
