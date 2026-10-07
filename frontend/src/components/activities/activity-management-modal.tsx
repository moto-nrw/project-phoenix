"use client";

import { useState, useEffect, useLayoutEffect, useRef } from "react";
import type { ChangeEvent, FormEvent } from "react";
import {
  updateActivity,
  deleteActivity,
  type Activity,
} from "~/lib/activity-api";
import {
  parseParticipantLimit,
  useActivityForm,
} from "~/hooks/useActivityForm";
import { createLogger } from "~/lib/logger";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Checkbox } from "~/components/ui/checkbox";
import { FormModal } from "~/components/ui/form-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { SpinnerIcon } from "~/components/ui/icons";
import { Minus, Plus, Trash2 } from "lucide-react";
import { InfoIcon } from "@phosphor-icons/react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";

const logger = createLogger({ component: "ActivityManagement" });

interface ActivityManagementModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess?: (message?: string) => void;
  readonly activity: Activity;
  readonly currentStaffId?: string | null;
  readonly readOnly?: boolean;
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
            className="hover:text-moto-red text-gray-400"
            disabled={isSubmitting || isDeleting}
            aria-label="Aktivität löschen"
          >
            <Trash2 className="h-5 w-5" strokeWidth={1.5} aria-hidden="true" />
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
                Speichern…
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
    loadError,
    handleInputChange,
    validateForm,
  } = useActivityForm(
    {
      name: activity.name,
      category_id: activity.ag_category_id || "",
      max_participants: activity.max_participant?.toString() ?? "",
    },
    isOpen,
  );

  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearFormErrors, fieldError } = formErrors;
  // Löschfehler bleiben im offenen Löschdialog.
  const deleteErrors = useApiFormError();
  const { clear: clearDeleteErrors } = deleteErrors;
  // „Wiederholen“ läuft mit dem aktuellen Formularstand.
  const retrySaveRef = useRef<() => void>(() => undefined);
  const retryDeleteRef = useRef<() => void>(() => undefined);
  const onInput = (event: ChangeEvent<HTMLInputElement>) => {
    handleInputChange(event);
    clearFormErrors();
  };

  // Reset form when activity changes
  useEffect(() => {
    if (isOpen) {
      setForm({
        name: activity.name,
        category_id: activity.ag_category_id || "",
        max_participants: activity.max_participant?.toString() ?? "",
      });
      clearFormErrors();
      clearDeleteErrors();
      setShowDeleteConfirm(false);
    }
  }, [isOpen, activity, setForm, clearFormErrors, clearDeleteErrors]);

  const save = async () => {
    formErrors.clear();
    const problem = validateForm();
    if (problem) {
      formErrors.invalid(problem.message, { [problem.field]: problem.message });
      return;
    }

    setIsSubmitting(true);

    try {
      // Prepare the update data
      const updateData = {
        name: form.name.trim(),
        category_id: Number.parseInt(form.category_id, 10),
        max_participants: parseParticipantLimit(form.max_participants),
        // Include existing values that might be required
        is_open: activity.is_open_ags || false,
        supervisor_ids: activity.supervisor_id
          ? [Number.parseInt(activity.supervisor_id, 10)]
          : [],
      };

      // Call the update API
      await updateActivity(activity.id, updateData);

      const successMessage = `Die Aktivität „${form.name.trim()}“ ist gespeichert.`;

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
      void formErrors.show(err, {
        object: "die Aktivität",
        retry: () => retrySaveRef.current(),
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    void save();
  };

  const handleDelete = async () => {
    setIsDeleting(true);
    deleteErrors.clear();

    try {
      await deleteActivity(activity.id);

      const successMessage = `Die Aktivität „${activity.name}“ ist gelöscht.`;

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
      void deleteErrors.show(err, {
        object: "die Aktivität",
        retry: () => retryDeleteRef.current(),
      });
    } finally {
      setIsDeleting(false);
    }
  };

  useLayoutEffect(() => {
    retrySaveRef.current = () => void save();
    retryDeleteRef.current = () => void handleDelete();
  });

  const footer = (
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
    <>
      <FormModal
        isOpen={isOpen}
        onClose={onClose}
        title={`Aktivität: ${activity.name}`}
        size="sm"
        mobilePosition="center"
        footer={footer}
      >
        {loading ? (
          <ModalLoadingMessage message="Kategorien werden geladen…" />
        ) : (
          <form
            ref={formRef}
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

            <LoadErrorAlert error={loadError} />
            <FormErrorAlert message={formErrors.error} />

            {/* Activity Name Card - Compact */}
            <div className="rounded-xl border border-gray-200/50 bg-gray-50 p-3 md:p-4">
              <div>
                <label
                  htmlFor="name"
                  className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
                >
                  <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gray-100">
                    <span className="text-[10px] font-bold text-gray-700">
                      1
                    </span>
                  </div>
                  Aktivitätsname
                </label>
                <input
                  id="name"
                  name="name"
                  value={form.name}
                  onChange={onInput}
                  aria-invalid={fieldError("name") ? true : undefined}
                  placeholder="z. B. Hausaufgaben, Malen, Basteln…"
                  className="focus:ring-moto-blue block w-full rounded-lg border-0 bg-white/80 px-3 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset placeholder:text-gray-400 focus:bg-white focus:ring-2 focus:ring-inset disabled:cursor-not-allowed disabled:bg-gray-50 md:py-2.5 md:text-sm"
                  required
                  disabled={readOnly}
                  maxLength={255}
                />
              </div>
            </div>

            {/* Category Card - Compact */}
            <div className="rounded-xl border border-gray-200/50 bg-gray-50 p-3 md:p-4">
              <div>
                <label
                  id="category_id-label"
                  htmlFor="category_id"
                  className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
                >
                  <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gray-100">
                    <span className="text-[10px] font-bold text-gray-700">
                      2
                    </span>
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
                  invalid={Boolean(fieldError("category_id"))}
                  options={[
                    { value: "", label: "Kategorie wählen…" },
                    ...categories.map((category) => ({
                      value: category.id,
                      label: category.name,
                    })),
                  ]}
                  placeholder="Kategorie wählen…"
                  required
                  disabled={readOnly}
                />
              </div>
            </div>

            {/* Participants Card - Compact */}
            <div className="rounded-xl border border-gray-200/50 bg-gray-50 p-3 md:p-4">
              <div>
                <label
                  htmlFor="max_participants"
                  className="mb-2 block flex items-center gap-1.5 text-xs font-semibold text-gray-700"
                >
                  <div className="flex h-4 w-4 flex-shrink-0 items-center justify-center rounded bg-gray-100">
                    <span className="text-[10px] font-bold text-gray-700">
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
                    className="focus:ring-moto-blue absolute left-0 z-10 flex h-full w-12 items-center justify-center rounded-l-lg text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:outline-none focus:ring-inset active:scale-95 disabled:cursor-not-allowed disabled:opacity-30 md:w-10"
                    disabled={
                      !form.max_participants ||
                      Number.parseInt(form.max_participants, 10) <= 1 ||
                      readOnly
                    }
                    aria-label="Teilnehmer reduzieren"
                  >
                    <Minus
                      className="h-5 w-5 md:h-4 md:w-4"
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
                    required={Boolean(form.max_participants)}
                    className="focus:ring-moto-blue block w-full [appearance:textfield] rounded-lg border-0 bg-white/80 px-14 py-3 text-center text-lg font-semibold text-gray-900 shadow-sm ring-1 ring-gray-200/50 backdrop-blur-sm transition-all duration-200 ring-inset focus:bg-white focus:ring-2 focus:ring-inset disabled:cursor-not-allowed disabled:bg-gray-50 md:px-12 md:py-2.5 md:text-base [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                    disabled={readOnly || !form.max_participants}
                  />

                  <button
                    type="button"
                    onClick={() => {
                      const current = Number.parseInt(
                        form.max_participants,
                        10,
                      );
                      if (Number.isFinite(current)) {
                        setForm((prev) => ({
                          ...prev,
                          max_participants: (current + 1).toString(),
                        }));
                      }
                    }}
                    className="focus:ring-moto-blue absolute right-0 z-10 flex h-full w-12 items-center justify-center rounded-r-lg text-gray-500 transition-all duration-200 hover:bg-white/50 hover:text-gray-700 focus:ring-2 focus:outline-none focus:ring-inset active:scale-95 disabled:cursor-not-allowed disabled:opacity-30 md:w-10"
                    disabled={readOnly || !form.max_participants}
                    aria-label="Teilnehmer erhöhen"
                  >
                    <Plus
                      className="h-5 w-5 md:h-4 md:w-4"
                      strokeWidth={2.5}
                      aria-hidden="true"
                    />
                  </button>
                </div>
                <label
                  htmlFor="activity-no-participant-limit"
                  className="mt-3 flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-700"
                >
                  <Checkbox
                    id="activity-no-participant-limit"
                    checked={!form.max_participants}
                    onChange={(event) =>
                      setForm((prev) => ({
                        ...prev,
                        max_participants: event.target.checked ? "" : "15",
                      }))
                    }
                    disabled={readOnly}
                  />
                  Keine Begrenzung
                </label>
              </div>
            </div>

            {/* Hinweiskarte. Die Löschabfrage läuft über das
              `ConfirmDeleteModal`, nicht über einen Wechsel dieser Karte
              und des Fußes (Bauart 2, Regel 6). */}
            <div className="rounded-lg border border-gray-200/30 bg-gray-50 p-3">
              <div className="flex items-center gap-2">
                <MotoDuotoneIcon
                  icon={InfoIcon}
                  tone="neutral"
                  size={14}
                  className="flex-shrink-0"
                />
                <p className="text-xs text-gray-600">
                  {readOnly
                    ? "Sie können nur Aktivitäten bearbeiten, die Sie selbst erstellt haben."
                    : "Änderungen werden sofort wirksam."}
                </p>
              </div>
            </div>
          </form>
        )}
      </FormModal>

      {/* Portalweit dasselbe Löschmuster. Für eine Aktivität reicht die
          zweistufige Rückfrage; die Texteingabe bleibt Fällen mit
          Datenverlust vorbehalten. */}
      <ConfirmDeleteModal
        isOpen={!readOnly && showDeleteConfirm}
        title="Aktivität löschen"
        description={`Möchten Sie die Aktivität „${activity.name}“ wirklich löschen? Das lässt sich nicht rückgängig machen.`}
        gate={{ mode: "twoStep" }}
        onConfirm={handleDelete}
        onClose={() => {
          clearDeleteErrors();
          setShowDeleteConfirm(false);
        }}
        loading={isDeleting}
        error={deleteErrors.error}
      />
    </>
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
