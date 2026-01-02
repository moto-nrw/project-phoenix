"use client";

import React, { useState, useEffect, useCallback } from "react";
import { createPortal } from "react-dom";
import { getCategories, type ActivityCategory } from "~/lib/activity-api";
import { getDbOperationMessage } from "~/lib/use-notification";
import { useScrollLock } from "~/hooks/useScrollLock";
import { useToast } from "~/contexts/ToastContext";
import {
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
  getApiErrorMessage,
} from "~/components/ui/modal-utils";

interface QuickCreateActivityModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: () => void;
}

interface QuickCreateForm {
  name: string;
  category_id: string;
  max_participants: string;
}

export function QuickCreateActivityModal({
  isOpen,
  onClose,
  onSuccess,
}: QuickCreateActivityModalProps) {
  const { success: toastSuccess } = useToast();
  const [form, setForm] = useState<QuickCreateForm>({
    name: "",
    category_id: "",
    max_participants: "15",
  });
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
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

  // Load categories and manage animations when modal opens
  useEffect(() => {
    if (isOpen) {
      // Trigger entrance animation with slight delay for smooth effect
      setTimeout(() => {
        setIsAnimating(true);
      }, 10);
      void loadCategories();
      // Reset form when modal opens
      setForm({
        name: "",
        category_id: "",
        max_participants: "15",
      });
      setError(null);
      // Don't reset success alert here - it should persist after modal closes
    }
  }, [isOpen]);

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
      // Prepare the request data
      const requestData = {
        name: form.name.trim(),
        category_id: Number.parseInt(form.category_id, 10),
        max_participants: Number.parseInt(form.max_participants, 10),
      };

      // Call the quick-create API endpoint
      const response = await fetch("/api/activities/quick-create", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(requestData),
      });

      if (!response.ok) {
        throw new Error(`Failed to create activity: ${response.status}`);
      }

      await response.json();

      // Show success notification
      toastSuccess(
        getDbOperationMessage("create", "Aktivität", form.name.trim()),
      );

      // Handle success
      if (onSuccess) {
        onSuccess();
      }

      // Close modal immediately - success alert will persist independently
      handleClose();
    } catch (err) {
      console.error("Error creating activity:", err);
      setError(
        getApiErrorMessage(
          err,
          "erstellen",
          "Aktivitäten",
          "Failed to create activity",
        ),
      );
    } finally {
      setIsSubmitting(false);
    }
  };

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

  // Don't return null here - we need to render the success alert even when modal is closed

  const modalContent = (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center"
      style={{ position: "fixed", top: 0, left: 0, right: 0, bottom: 0 }}
    >
      {/* Backdrop button - native button for accessibility (keyboard + click support) */}
      <button
        type="button"
        onClick={handleClose}
        aria-label="Hintergrund - Klicken zum Schließen"
        className={`absolute inset-0 cursor-default border-none bg-transparent p-0 ${getBackdropClassName(isAnimating, isExiting).replace("fixed inset-0 z-[9999] flex items-center justify-center", "")}`}
        style={getBackdropStyle(isAnimating, isExiting)}
      />
      {/* Modal */}
      <div
        className={getModalDialogClassName(isAnimating, isExiting)}
        style={modalContainerStyle}
      >
        {/* Header */}
        <div className="flex items-center justify-between border-b border-gray-100 p-4 md:p-6">
          <h3 className="pr-4 text-lg font-semibold text-gray-900 md:text-xl">
            Aktivität erstellen
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
                id="quick-create-form"
                onSubmit={handleSubmit}
                className="space-y-6"
              >
                {error && renderModalErrorAlert({ message: error })}

                {/* Activity Name Card */}
                <div className="relative overflow-hidden rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5">
                  <div className="absolute top-2 right-2 h-16 w-16 rounded-full bg-gray-100/20 blur-2xl"></div>
                  <div className="absolute bottom-2 left-2 h-12 w-12 rounded-full bg-slate-100/20 blur-xl"></div>
                  <div className="relative">
                    <label
                      htmlFor="name"
                      className="mb-3 block flex items-center gap-2 text-sm font-semibold text-gray-700"
                    >
                      <div className="flex h-5 w-5 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                        <span className="text-xs font-bold text-white">1</span>
                      </div>
                      Aktivitätsname
                    </label>
                    <input
                      id="name"
                      name="name"
                      value={form.name}
                      onChange={handleInputChange}
                      placeholder="z.B. Hausaufgaben, Malen, Basteln..."
                      className="block w-full rounded-xl border-0 bg-white/80 px-4 py-3.5 text-base text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset placeholder:text-gray-400 focus:bg-white focus:ring-2 focus:ring-gray-700 focus:ring-inset"
                      required
                    />
                  </div>
                </div>

                {/* Category Card */}
                <div className="relative overflow-hidden rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5">
                  <div className="absolute top-2 left-2 h-14 w-14 rounded-full bg-gray-100/20 blur-2xl"></div>
                  <div className="relative">
                    <label
                      htmlFor="category_id"
                      className="mb-3 block flex items-center gap-2 text-sm font-semibold text-gray-700"
                    >
                      <div className="flex h-5 w-5 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                        <span className="text-xs font-bold text-white">2</span>
                      </div>
                      Kategorie
                    </label>
                    <div className="relative">
                      <select
                        id="category_id"
                        name="category_id"
                        value={form.category_id}
                        onChange={handleInputChange}
                        className="block w-full cursor-pointer appearance-none rounded-xl border-0 bg-white/80 px-4 py-3.5 pr-10 text-base text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset focus:bg-white focus:ring-2 focus:ring-gray-700 focus:ring-inset"
                        required
                      >
                        <option value="">Kategorie wählen...</option>
                        {/* Categories are fetched from backend. Expected values:
                      - Gruppenraum
                      - Hausaufgaben
                      - Kreatives/Musik
                      - Bewegen/Entspannen
                      - Natur
                      - HW/Technik
                      - Spielen
                      - Lernen
                  */}
                        {categories.map((category) => (
                          <option key={category.id} value={category.id}>
                            {category.name}
                          </option>
                        ))}
                      </select>
                      <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                        <svg
                          className="h-5 w-5 text-gray-400"
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

                {/* Participants Card */}
                <div className="relative overflow-hidden rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5">
                  <div className="absolute right-2 bottom-2 h-20 w-20 rounded-full bg-gray-100/20 blur-2xl"></div>
                  <div className="relative">
                    <label
                      htmlFor="max_participants"
                      className="mb-3 block flex items-center gap-2 text-sm font-semibold text-gray-700"
                    >
                      <div className="flex h-5 w-5 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                        <span className="text-xs font-bold text-white">3</span>
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
                        className="absolute left-0 z-10 flex h-full w-14 items-center justify-center rounded-l-xl text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-gray-700 focus:outline-none focus:ring-inset disabled:cursor-not-allowed disabled:opacity-30"
                        disabled={
                          Number.parseInt(form.max_participants, 10) <= 1
                        }
                        aria-label="Teilnehmer reduzieren"
                      >
                        <svg
                          className="h-5 w-5"
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
                        className="block w-full [appearance:textfield] rounded-xl border-0 bg-white/80 px-16 py-3.5 text-center text-lg font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset focus:bg-white focus:ring-2 focus:ring-gray-700 focus:ring-inset [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                        required
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
                        className="absolute right-0 z-10 flex h-full w-14 items-center justify-center rounded-r-xl text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-gray-700 focus:outline-none focus:ring-inset disabled:cursor-not-allowed disabled:opacity-30"
                        disabled={
                          Number.parseInt(form.max_participants, 10) >= 50
                        }
                        aria-label="Teilnehmer erhöhen"
                      >
                        <svg
                          className="h-5 w-5"
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

                {/* Info Card */}
                <div className="relative overflow-hidden rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/80 to-slate-50/80 p-4 backdrop-blur-sm">
                  <div className="absolute top-0 right-0 h-24 w-24 rounded-full bg-gradient-to-br from-blue-100/10 to-indigo-100/10 blur-3xl"></div>
                  <div className="relative flex items-start gap-3">
                    <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-gradient-to-br from-gray-100 to-slate-100">
                      <svg
                        className="h-4 w-4 text-gray-600"
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
                    </div>
                    <div className="flex-1">
                      <p className="mb-1 text-sm font-medium text-gray-700">
                        Hinweis
                      </p>
                      <p className="text-sm text-gray-600">
                        Die Aktivität ist sofort für NFC-Terminals verfügbar.
                      </p>
                    </div>
                  </div>
                </div>
              </form>
            )}
          </div>
        </div>

        {/* Footer */}
        <div className="flex gap-3 border-t border-gray-100 bg-gray-50/50 p-4 md:p-6">
          <button
            type="button"
            onClick={handleClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            disabled={isSubmitting}
          >
            Abbrechen
          </button>

          <button
            type="submit"
            form="quick-create-form"
            disabled={
              isSubmitting || loading || !form.name.trim() || !form.category_id
            }
            className="flex-1 rounded-lg bg-gradient-to-br from-[#83CD2D] to-[#70B525] px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:from-[#73BD1D] hover:to-[#60A515] hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100"
          >
            {isSubmitting ? (
              <span className="flex items-center justify-center gap-2">
                {renderButtonSpinner()}
                Wird erstellt...
              </span>
            ) : (
              "Aktivität erstellen"
            )}
          </button>
        </div>
      </div>
    </div>
  );

  // Portal render
  if (typeof document !== "undefined") {
    return (
      <>
        {/* Render modal only when open */}
        {isOpen && createPortal(modalContent, document.body)}
        {/* Success toasts handled globally */}
      </>
    );
  }

  return null;
}
