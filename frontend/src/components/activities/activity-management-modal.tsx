"use client";

import React, { useState, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import {
  getCategories,
  updateActivity,
  deleteActivity,
  type ActivityCategory,
  type Activity,
} from "~/lib/activity-api";
import { getDbOperationMessage } from "~/lib/use-notification";
import { useScrollLock } from "~/hooks/useScrollLock";
import {
  createBackdropKeyHandler,
  backdropAriaProps,
  stopPropagation,
  getBackdropClassName,
  getBackdropStyle,
  modalContainerStyle,
  getModalDialogClassName,
  scrollableContentClassName,
  getContentAnimationClassName,
  renderModalCloseButton,
  renderModalLoadingSpinner,
  renderModalErrorAlert,
  renderButtonSpinner,
} from "~/components/ui/modal-utils";

interface ActivityManagementModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: (message?: string) => void;
  readonly activity: Activity;
  readonly currentStaffId?: string | null;
  readonly readOnly?: boolean;
}

interface EditForm {
  name: string;
  category_id: string;
  max_participants: string;
}

export function ActivityManagementModal({
  isOpen,
  onClose,
  onSuccess,
  activity,
  currentStaffId: _currentStaffId,
  readOnly = false,
}: ActivityManagementModalProps) {
  const [form, setForm] = useState<EditForm>({
    name: activity.name,
    category_id: activity.ag_category_id || "",
    max_participants: activity.max_participant?.toString() || "15",
  });
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [isAnimating, setIsAnimating] = useState(false);
  const [isExiting, setIsExiting] = useState(false);

  // Use scroll lock hook
  useScrollLock(isOpen);

  // Handle modal close with animation
  const handleClose = useCallback(() => {
    setIsExiting(true);
    setIsAnimating(false);

    // Delay actual close to allow exit animation
    setTimeout(() => {
      onClose();
    }, 250);
  }, [onClose]);

  // Load categories and reset form when modal opens or activity changes
  useEffect(() => {
    if (isOpen) {
      // Trigger entrance animation with slight delay for smooth effect
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
      void loadCategories();
      // Reset form with current activity values
      setForm({
        name: activity.name,
        category_id: activity.ag_category_id || "",
        max_participants: activity.max_participant?.toString() || "15",
      });
      setError(null);
      setShowDeleteConfirm(false);
    }
  }, [isOpen, activity]);

  // Reset animation states when modal closes
  useEffect(() => {
    if (!isOpen) {
      setIsAnimating(false);
      setIsExiting(false);
    }
  }, [isOpen]);

  const loadCategories = async () => {
    try {
      setLoading(true);
      const categoriesData = await getCategories();
      setCategories(categoriesData ?? []);
    } catch (err) {
      console.error("Failed to load categories:", err);
      setError("Failed to load categories");
    } finally {
      setLoading(false);
    }
  };

  const handleInputChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    const { name, value } = e.target;
    setForm((prev) => ({
      ...prev,
      [name]: value,
    }));
    // Clear error when user starts typing
    if (error) setError(null);
  };

  const validateForm = (): string | null => {
    if (!form.name.trim()) {
      return "Activity name is required";
    }
    if (!form.category_id) {
      return "Please select a category";
    }
    const maxParticipants = Number.parseInt(form.max_participants, 10);
    if (Number.isNaN(maxParticipants) || maxParticipants < 1) {
      return "Max participants must be a positive number";
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
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
      handleClose();

      // Handle success with message after modal starts closing
      setTimeout(() => {
        if (onSuccess) {
          onSuccess(successMessage);
        }
      }, 100);
    } catch (err) {
      console.error("Error updating activity:", err);

      // Extract meaningful error message from API response
      let errorMessage = "Failed to update activity";

      if (err instanceof Error) {
        const message = err.message;

        // Handle specific error cases with user-friendly messages
        if (message.includes("user is not authenticated")) {
          errorMessage =
            "Sie müssen angemeldet sein, um Aktivitäten zu bearbeiten.";
        } else if (message.includes("401")) {
          errorMessage =
            "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
        } else if (message.includes("403")) {
          errorMessage = "Zugriff verweigert. Bitte melden Sie sich erneut an.";
        } else if (message.includes("400")) {
          errorMessage =
            "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.";
        } else {
          errorMessage = message;
        }
      }

      setError(errorMessage);
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
      handleClose();

      // Handle success with message after modal starts closing
      setTimeout(() => {
        if (onSuccess) {
          onSuccess(successMessage);
        }
      }, 100);
    } catch (err) {
      console.error("Error deleting activity:", err);

      let errorMessage = "Fehler beim Löschen der Aktivität";

      if (err instanceof Error) {
        const message = err.message;

        if (message.includes("students enrolled")) {
          errorMessage =
            "Diese Aktivität kann nicht gelöscht werden, da noch Schüler eingeschrieben sind. Bitte entfernen Sie zuerst alle Schüler aus der Aktivität.";
        } else if (message.includes("401") || message.includes("403")) {
          errorMessage =
            "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
        } else {
          errorMessage = message;
        }
      }

      setError(errorMessage);
      setShowDeleteConfirm(false);
    } finally {
      setIsDeleting(false);
    }
  };

  const footer = (
    <>
      {/* Delete Confirmation Mode - Shows only delete options */}
      {!readOnly && showDeleteConfirm ? (
        <div className="flex items-center justify-end">
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={() => setShowDeleteConfirm(false)}
              className="px-4 py-2 text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
              disabled={isDeleting}
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={handleDelete}
              disabled={isDeleting}
              className="flex items-center justify-center gap-2 rounded-lg bg-red-600 px-6 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isDeleting ? (
                <>
                  {renderButtonSpinner()}
                  <span>Löschen...</span>
                </>
              ) : (
                "Löschen"
              )}
            </button>
          </div>
        </div>
      ) : (
        /* Normal Footer - Shows when not in delete mode */
        <div className="flex items-center justify-between">
          {/* Secondary actions left */}
          <div className="flex items-center gap-2">
            {!readOnly && (
              <button
                type="button"
                onClick={() => setShowDeleteConfirm(true)}
                className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-50 hover:text-red-600"
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
              </button>
            )}
          </div>

          {/* Primary actions right */}
          <div className="flex items-center gap-3">
            <button
              type="button"
              onClick={handleClose}
              className="px-4 py-2 text-sm font-medium text-gray-500 transition-colors hover:text-gray-700"
              disabled={isSubmitting || isDeleting}
            >
              Abbrechen
            </button>

            {!readOnly && (
              <button
                type="submit"
                form="activity-management-form"
                disabled={isSubmitting || loading || isDeleting}
                className="flex min-w-[100px] items-center justify-center gap-2 rounded-lg bg-blue-600 px-6 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {isSubmitting ? (
                  <>
                    {renderButtonSpinner()}
                    <span>Speichern...</span>
                  </>
                ) : (
                  "Speichern"
                )}
              </button>
            )}
          </div>
        </div>
      )}
    </>
  );

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isOpen) {
        handleClose();
      }
    };

    if (isOpen) {
      document.addEventListener("keydown", handleEscape);
    }

    return () => {
      document.removeEventListener("keydown", handleEscape);
    };
  }, [isOpen, handleClose]);

  // Handle backdrop click
  const handleBackdropClick = useCallback(
    (e: React.MouseEvent) => {
      if (e.target === e.currentTarget) {
        handleClose();
      }
    },
    [handleClose],
  );

  // Don't return null here - we need to render the success alert even when modal is closed

  const modalContent = (
    <div
      className={getBackdropClassName(isAnimating, isExiting)}
      onClick={handleBackdropClick}
      onKeyDown={createBackdropKeyHandler(handleClose)}
      {...backdropAriaProps}
      style={getBackdropStyle(isAnimating, isExiting)}
    >
      {/* Modal */}
      <div
        className={getModalDialogClassName(isAnimating, isExiting)}
        {...stopPropagation}
        style={modalContainerStyle}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-gray-100 p-4 md:p-6">
          <h3 className="pr-4 text-lg font-semibold text-gray-900 md:text-xl">
            Aktivität: {activity.name}
          </h3>
          {renderModalCloseButton({ onClose: handleClose })}
        </div>

        {/* Content */}
        <div className={scrollableContentClassName} data-modal-content="true">
          <div className={getContentAnimationClassName(isAnimating, isExiting)}>
            {loading ? (
              renderModalLoadingSpinner({
                message: "Kategorien werden geladen...",
              })
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

                {error && renderModalErrorAlert({ message: error })}

                {/* Activity Name Card - Compact */}
                <div className="relative overflow-hidden rounded-xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4">
                  <div className="absolute top-1 right-1 h-12 w-12 rounded-full bg-gray-100/20 blur-xl"></div>
                  <div className="relative">
                    <label
                      htmlFor="name"
                      className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
                    >
                      <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                        <span className="text-[10px] font-bold text-white">
                          1
                        </span>
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
                    />
                  </div>
                </div>

                {/* Category Card - Compact */}
                <div className="relative overflow-hidden rounded-xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-3 md:p-4">
                  <div className="absolute top-1 left-1 h-10 w-10 rounded-full bg-gray-100/20 blur-xl"></div>
                  <div className="relative">
                    <label
                      htmlFor="category_id"
                      className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
                    >
                      <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                        <span className="text-[10px] font-bold text-white">
                          2
                        </span>
                      </div>
                      Kategorie
                    </label>
                    <div className="relative">
                      <select
                        id="category_id"
                        name="category_id"
                        value={form.category_id}
                        onChange={handleInputChange}
                        className="block w-full cursor-pointer appearance-none rounded-lg border-0 bg-white/80 px-3 py-3 pr-10 text-base text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset focus:bg-white focus:ring-2 focus:ring-[#5080D8] focus:ring-inset disabled:cursor-not-allowed disabled:bg-gray-50 md:py-2.5 md:text-sm"
                        required
                        disabled={readOnly}
                      >
                        <option value="">Kategorie wählen...</option>
                        {categories.map((category) => (
                          <option key={category.id} value={category.id}>
                            {category.name}
                          </option>
                        ))}
                      </select>
                      <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                        <svg
                          className="h-5 w-5 text-gray-400 md:h-4 md:w-4"
                          viewBox="0 0 20 20"
                          fill="currentColor"
                          aria-hidden="true"
                        >
                          <path
                            fillRule="evenodd"
                            d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
                            clipRule="evenodd"
                          />
                        </svg>
                      </div>
                    </div>
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
                        <span className="text-[10px] font-bold text-white">
                          3
                        </span>
                      </div>
                      Maximale Teilnehmerzahl
                    </label>
                    <div className="relative flex items-center">
                      <button
                        type="button"
                        onClick={() => {
                          const current = Number.parseInt(
                            form.max_participants,
                            10,
                          );
                          if (current > 1) {
                            setForm((prev) => ({
                              ...prev,
                              max_participants: (current - 1).toString(),
                            }));
                          }
                        }}
                        className="absolute left-0 z-10 flex h-full w-12 items-center justify-center rounded-l-lg text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-[#5080D8] focus:outline-none focus:ring-inset active:scale-95 disabled:cursor-not-allowed disabled:opacity-30 md:w-10"
                        disabled={
                          Number.parseInt(form.max_participants, 10) <= 1 ||
                          readOnly
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
                          const current = Number.parseInt(
                            form.max_participants,
                            10,
                          );
                          if (current < 50) {
                            setForm((prev) => ({
                              ...prev,
                              max_participants: (current + 1).toString(),
                            }));
                          }
                        }}
                        className="absolute right-0 z-10 flex h-full w-12 items-center justify-center rounded-r-lg text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-[#5080D8] focus:outline-none focus:ring-inset active:scale-95 disabled:cursor-not-allowed disabled:opacity-30 md:w-10"
                        disabled={
                          Number.parseInt(form.max_participants, 10) >= 50 ||
                          readOnly
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
          </div>
        </div>

        {/* Footer */}
        {footer && (
          <div className="border-t border-gray-100 bg-gray-50/50 p-4 md:p-6">
            {footer}
          </div>
        )}
      </div>
    </div>
  );

  // Portal render
  if (typeof document !== "undefined" && isOpen) {
    return createPortal(
      <>
        <style>{`
          @keyframes shine {
            0% { transform: translateX(-100%) rotate(12deg); }
            100% { transform: translateX(100%) rotate(12deg); }
          }
        `}</style>
        {modalContent}
      </>,
      document.body,
    );
  }

  return null;
}
