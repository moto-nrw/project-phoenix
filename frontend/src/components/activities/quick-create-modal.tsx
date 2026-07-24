"use client";

import { useState, useEffect, useRef } from "react";
import type { FormEvent } from "react";
import { getDbOperationMessage } from "~/lib/use-notification";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { useActivityForm } from "~/hooks/useActivityForm";
import { useToast } from "~/contexts/ToastContext";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { SpinnerIcon } from "~/components/ui/icons";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "QuickCreateActivityModal" });

interface QuickCreateActivityModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: () => void;
}

const defaultFormValues = {
  name: "",
  category_id: "",
  max_participants: "15",
};

export function QuickCreateActivityModal({
  isOpen,
  onClose,
  onSuccess,
}: QuickCreateActivityModalProps) {
  const { success: toastSuccess } = useToast();
  const [isSubmitting, setIsSubmitting] = useState(false);
  // Track mount state to avoid setState on unmounted component
  const isMountedRef = useRef(true);

  // Track unmount
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

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
  } = useActivityForm(defaultFormValues, isOpen);

  const errorRef = useScrollToError(error);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setForm(defaultFormValues);
      setError(null);
      setErrorFieldName(null);
    }
  }, [isOpen, setForm, setError]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();

    // Prevent double-submit: check synchronously at the very start
    if (isSubmitting || loading) {
      return;
    }
    setIsSubmitting(true);

    setErrorFieldName(null);
    const validationError = validateForm();
    if (validationError) {
      setError(validationError);
      // Map validation error to field name
      if (validationError.includes("name")) setErrorFieldName("name");
      else if (validationError.includes("category"))
        setErrorFieldName("category_id");
      else if (validationError.includes("participants"))
        setErrorFieldName("max_participants");
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
      return;
    }

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
      onClose();
    } catch (err) {
      logger.error("activity creation failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        getApiErrorMessage(
          err,
          "erstellen",
          "Aktivitäten",
          "Failed to create activity",
        ),
      );
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
    }
  };

  const footer = (
    <>
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onClose}
        className="flex-1"
        disabled={isSubmitting}
      >
        Abbrechen
      </Button>

      <Button
        type="submit"
        form="quick-create-form"
        size="md"
        className="flex-1"
        disabled={
          isSubmitting || loading || !form.name.trim() || !form.category_id
        }
      >
        {isSubmitting ? (
          <span className="flex items-center justify-center gap-2">
            <SpinnerIcon />
            Wird erstellt...
          </span>
        ) : (
          "Aktivität erstellen"
        )}
      </Button>
    </>
  );

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Aktivität erstellen"
      size="sm"
      mobilePosition="center"
      footer={footer}
    >
      {loading ? (
        <ModalLoadingMessage message="Kategorien werden geladen..." />
      ) : (
        <form
          id="quick-create-form"
          onSubmit={handleSubmit}
          className="space-y-6"
        >
          {error && (
            <div ref={errorRef}>
              <Alert type="error" message={error} />
            </div>
          )}

          {/* Activity Name Card */}
          <div className="relative overflow-hidden rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5">
            <div className="absolute top-2 right-2 h-16 w-16 rounded-full bg-gray-100/20 blur-2xl"></div>
            <div className="absolute bottom-2 left-2 h-12 w-12 rounded-full bg-slate-100/20 blur-xl"></div>
            <div className="relative">
              <label
                htmlFor="name"
                className={`mb-3 block flex items-center gap-2 text-sm font-semibold ${errorFieldName === "name" ? "text-red-600" : "text-gray-700"}`}
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
                className={`block w-full rounded-xl border-0 bg-white/80 px-4 py-3.5 text-base text-gray-900 shadow-sm ring-1 ${errorFieldName === "name" ? "ring-red-400" : "ring-gray-200/50"} backdrop-blur-sm transition-all duration-200 ring-inset placeholder:text-gray-400 focus:bg-white focus:ring-2 focus:ring-gray-700 focus:ring-inset`}
                required
                maxLength={255}
              />
            </div>
          </div>

          {/* Category Card — no overflow-hidden on the card itself: it would clip the CustomSelect menu */}
          <div className="relative rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5">
            <div className="pointer-events-none absolute inset-0 overflow-hidden rounded-2xl">
              <div className="absolute top-2 left-2 h-14 w-14 rounded-full bg-gray-100/20 blur-2xl"></div>
            </div>
            <div className="relative">
              <label
                htmlFor="category_id"
                className={`mb-3 block flex items-center gap-2 text-sm font-semibold ${errorFieldName === "category_id" ? "text-red-600" : "text-gray-700"}`}
              >
                <div className="flex h-5 w-5 items-center justify-center rounded bg-gradient-to-br from-gray-600 to-gray-700">
                  <span className="text-xs font-bold text-white">2</span>
                </div>
                Kategorie
              </label>
              <CustomSelect
                id="category_id"
                name="category_id"
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
                invalid={errorFieldName === "category_id"}
                required
              />
            </div>
          </div>

          {/* Participants Card */}
          <div className="relative overflow-hidden rounded-2xl border border-gray-200/50 bg-gradient-to-br from-gray-50/50 to-slate-50/50 p-5">
            <div className="absolute right-2 bottom-2 h-20 w-20 rounded-full bg-gray-100/20 blur-2xl"></div>
            <div className="relative">
              <label
                htmlFor="max_participants"
                className={`mb-3 block flex items-center gap-2 text-sm font-semibold ${errorFieldName === "max_participants" ? "text-red-600" : "text-gray-700"}`}
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
                    const current = Number.parseInt(form.max_participants, 10);
                    if (current > 1) {
                      setForm((prev) => ({
                        ...prev,
                        max_participants: (current - 1).toString(),
                      }));
                    }
                  }}
                  className="absolute left-0 z-10 flex h-full w-14 items-center justify-center rounded-l-xl text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-gray-700 focus:outline-none focus:ring-inset disabled:cursor-not-allowed disabled:opacity-30"
                  disabled={Number.parseInt(form.max_participants, 10) <= 1}
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
                    const current = Number.parseInt(form.max_participants, 10);
                    if (current < 50) {
                      setForm((prev) => ({
                        ...prev,
                        max_participants: (current + 1).toString(),
                      }));
                    }
                  }}
                  className="absolute right-0 z-10 flex h-full w-14 items-center justify-center rounded-r-xl text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-gray-700 focus:outline-none focus:ring-inset disabled:cursor-not-allowed disabled:opacity-30"
                  disabled={Number.parseInt(form.max_participants, 10) >= 50}
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
