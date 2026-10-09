"use client";

import { useState, useEffect, useLayoutEffect, useRef } from "react";
import type { ChangeEvent, FormEvent } from "react";
import {
  parseParticipantLimit,
  useActivityForm,
} from "~/hooks/useActivityForm";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { apiErrorFromResponse, transportFetch } from "~/lib/api-error";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Checkbox } from "~/components/ui/checkbox";
import { FormModal } from "~/components/ui/form-modal";
import { SpinnerIcon } from "~/components/ui/icons";
import { createLogger } from "~/lib/logger";
import { Minus, Plus } from "lucide-react";
import { InfoIcon } from "@phosphor-icons/react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { SectionHeader } from "~/components/ui/concept-section-header";

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
    loadError,
    handleInputChange,
    validateForm,
  } = useActivityForm(defaultFormValues, isOpen);

  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearFormErrors, fieldError } = formErrors;
  // „Wiederholen“ legt die Aktivität mit dem aktuellen Formularstand an.
  const retrySubmitRef = useRef<() => void>(() => undefined);
  const onInput = (event: ChangeEvent<HTMLInputElement>) => {
    handleInputChange(event);
    clearFormErrors();
  };

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setForm(defaultFormValues);
      clearFormErrors();
    }
  }, [isOpen, setForm, clearFormErrors]);

  const submit = async () => {
    // Prevent double-submit: check synchronously at the very start
    if (isSubmitting || loading) {
      return;
    }
    setIsSubmitting(true);

    formErrors.clear();
    const problem = validateForm();
    if (problem) {
      formErrors.invalid(problem.message, { [problem.field]: problem.message });
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
      return;
    }

    try {
      // Prepare the request data
      const requestData = {
        name: form.name.trim(),
        category_id: Number.parseInt(form.category_id, 10),
        max_participants: parseParticipantLimit(form.max_participants),
      };

      // Call the quick-create API endpoint
      const response = await transportFetch("/api/activities/quick-create", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(requestData),
      });

      if (!response.ok) {
        throw await apiErrorFromResponse(
          response,
          `activity quick-create failed (${response.status})`,
        );
      }

      await response.json();

      toastSuccess(`Die Aktivität „${form.name.trim()}“ ist angelegt.`);

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
      void formErrors.show(err, {
        object: "die Aktivität",
        retry: () => retrySubmitRef.current(),
      });
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
    }
  };
  useLayoutEffect(() => {
    retrySubmitRef.current = () => void submit();
  });

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    void submit();
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
          ref={formRef}
          id="quick-create-form"
          onSubmit={handleSubmit}
          className="space-y-6"
        >
          <LoadErrorAlert error={loadError} />
          <FormErrorAlert message={formErrors.error} />

          {/* Activity Name Card */}
          <div className="rounded-2xl border border-gray-200/50 bg-gray-50 p-5">
            <div>
              <label
                htmlFor="name"
                className={`mb-3 block flex items-center gap-2 text-sm font-semibold ${fieldError("name") ? "text-moto-red" : "text-gray-700"}`}
              >
                <div className="flex h-5 w-5 items-center justify-center rounded bg-gray-100">
                  <span className="text-xs font-bold text-gray-700">1</span>
                </div>
                Aktivitätsname
              </label>
              <input
                id="name"
                name="name"
                value={form.name}
                onChange={onInput}
                aria-invalid={fieldError("name") ? true : undefined}
                placeholder="z.B. Hausaufgaben, Malen, Basteln..."
                className={`block w-full rounded-xl border-0 bg-white/80 px-4 py-3.5 text-base text-gray-900 shadow-sm ring-1 ${fieldError("name") ? "ring-moto-red/40" : "ring-gray-200/50"} backdrop-blur-sm transition-all duration-200 ring-inset placeholder:text-gray-400 focus:bg-white focus:ring-2 focus:ring-gray-700 focus:ring-inset`}
                required
                maxLength={255}
              />
            </div>
          </div>

          {/* Category Card */}
          <div className="rounded-2xl border border-gray-200/50 bg-gray-50 p-5">
            <div>
              <label
                id="category_id-label"
                htmlFor="category_id"
                className={`mb-3 block flex items-center gap-2 text-sm font-semibold ${fieldError("category_id") ? "text-moto-red" : "text-gray-700"}`}
              >
                <div className="flex h-5 w-5 items-center justify-center rounded bg-gray-100">
                  <span className="text-xs font-bold text-gray-700">2</span>
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
                  clearFormErrors();
                }}
                options={[
                  { value: "", label: "Kategorie wählen..." },
                  ...categories.map((category) => ({
                    value: category.id,
                    label: category.name,
                  })),
                ]}
                placeholder="Kategorie wählen..."
                invalid={Boolean(fieldError("category_id"))}
                required
              />
            </div>
          </div>

          {/* Participants Card */}
          <div className="rounded-2xl border border-gray-200/50 bg-gray-50 p-5">
            <div>
              <label
                htmlFor="max_participants"
                className={`mb-3 block flex items-center gap-2 text-sm font-semibold ${fieldError("max_participants") ? "text-moto-red" : "text-gray-700"}`}
              >
                <div className="flex h-5 w-5 items-center justify-center rounded bg-gray-100">
                  <span className="text-xs font-bold text-gray-700">3</span>
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
                  disabled={
                    !form.max_participants ||
                    Number.parseInt(form.max_participants, 10) <= 1
                  }
                  aria-label="Teilnehmer reduzieren"
                >
                  <Minus
                    className="h-5 w-5"
                    strokeWidth={2.5}
                    aria-hidden="true"
                  />
                </button>

                <input
                  id="max_participants"
                  name="max_participants"
                  type="number"
                  value={form.max_participants}
                  onChange={onInput}
                  aria-invalid={
                    fieldError("max_participants") ? true : undefined
                  }
                  min="1"
                  disabled={!form.max_participants}
                  required={Boolean(form.max_participants)}
                  className="block w-full [appearance:textfield] rounded-xl border-0 bg-white/80 px-16 py-3.5 text-center text-lg font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset focus:bg-white focus:ring-2 focus:ring-gray-700 focus:ring-inset [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                />

                <button
                  type="button"
                  onClick={() => {
                    const current = Number.parseInt(form.max_participants, 10);
                    if (Number.isFinite(current)) {
                      setForm((prev) => ({
                        ...prev,
                        max_participants: (current + 1).toString(),
                      }));
                    }
                  }}
                  className="absolute right-0 z-10 flex h-full w-14 items-center justify-center rounded-r-xl text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:ring-gray-700 focus:outline-none focus:ring-inset disabled:cursor-not-allowed disabled:opacity-30"
                  disabled={!form.max_participants}
                  aria-label="Teilnehmer erhöhen"
                >
                  <Plus
                    className="h-5 w-5"
                    strokeWidth={2.5}
                    aria-hidden="true"
                  />
                </button>
              </div>
              <label
                htmlFor="quick-create-no-participant-limit"
                className="mt-3 flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-700"
              >
                <Checkbox
                  id="quick-create-no-participant-limit"
                  checked={!form.max_participants}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      max_participants: event.target.checked ? "" : "15",
                    }))
                  }
                />
                Keine Begrenzung
              </label>
            </div>
          </div>

          {/* Info Card */}
          <div className="rounded-2xl border border-gray-200/50 bg-gray-50 p-4">
            <SectionHeader
              // FormModal rendert seinen Titel als h3.
              level={4}
              title="Hinweis"
              icon={
                <MotoDuotoneIcon icon={InfoIcon} tone="neutral" size={18} />
              }
              subtitle="Die Aktivität ist sofort für NFC-Terminals verfügbar."
            />
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
        <SpinnerIcon className="text-moto-blue h-12 w-12" />
        <p className="text-gray-600">{message}</p>
      </div>
    </div>
  );
}
