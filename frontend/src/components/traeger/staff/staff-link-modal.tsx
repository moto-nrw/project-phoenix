"use client";

// Beschäftigungen zu einer Personalakte verknüpfen (#4705, ADR 0089
// Abschnitt 5). Der Dialog zeigt, was jede Schule über die Person weiß. Wo
// die Schulen sich unterscheiden, wählt die Personalverwaltung den Wert, der
// gilt; der andere Wert bleibt mit seiner Schule im Verlauf. Bank- und
// Steuerdaten erscheinen nur maskiert und ziehen in die Akte um.

import { useEffect, useRef, useState } from "react";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Loading } from "~/components/ui/loading";
import { Radio } from "~/components/ui/radio";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  linkStaff,
  previewStaffLink,
  staffName,
  type StaffLinkInput,
  type StaffLinkPreview,
  type StaffRecordDetail,
  type StaffRecordField,
} from "~/lib/traeger-staff-api";
import { STAFF_FIELD_LABELS, staffFieldValue } from "./staff-record-format";

const logger = createLogger({ component: "StaffLinkModal" });

/** Wo die Werte gleich sind, zeigt der Dialog sie nur zur Kontrolle. */
function agreedValue(preview: StaffLinkPreview, field: StaffRecordField) {
  const option = preview.fields.find((entry) => entry.field === field)
    ?.options[0];
  return option ? staffFieldValue(field, option.value) : "–";
}

export function StaffLinkModal({
  request,
  onClose,
  onLinked,
}: Readonly<{
  /** Was verknüpft werden soll; `null` schließt den Dialog. */
  request: Omit<StaffLinkInput, "choices"> | null;
  onClose: () => void;
  onLinked: (record: StaffRecordDetail) => void;
}>) {
  const isOpen = request !== null;
  const [preview, setPreview] = useState<StaffLinkPreview | null>(null);
  const [choices, setChoices] = useState<
    Partial<Record<StaffRecordField, string>>
  >({});
  const save = useFormModalPending(isOpen);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors, show: showError } = formErrors;

  useEffect(() => {
    if (!request) return;
    let current = true;
    setPreview(null);
    setChoices({});
    clearErrors();
    previewStaffLink({ ...request, choices: {} })
      .then((result) => {
        if (current) setPreview(result);
      })
      .catch((err: unknown) => {
        logger.error("traeger_staff_link_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (current) void showError(err, { object: "die Vorschau" });
      });
    return () => {
      current = false;
    };
  }, [request, clearErrors, showError]);

  const conflicts = preview?.fields.filter((field) => field.conflict) ?? [];
  const missing = conflicts.filter((field) => !choices[field.field]);
  const joinsRecord = request?.recordId != null;

  const submit = async () => {
    if (!request || !preview) return;
    formErrors.clear();
    if (missing.length > 0) {
      formErrors.invalid(
        "Bitte wählen Sie bei jedem Feld, welcher Wert gilt.",
        Object.fromEntries(
          missing.map((field) => [
            `choice-${field.field}`,
            "Bitte wählen Sie einen Wert.",
          ]),
        ),
      );
      return;
    }
    await save.run(async (op) => {
      try {
        const record = await linkStaff({ ...request, choices });
        if (op.isCurrent()) onLinked(record);
      } catch (err) {
        logger.error("traeger_staff_link_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(err, { object: "die Personalakte" });
      }
    });
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      isBackdropDismissDisabled
      title={
        joinsRecord ? "Zur Personalakte hinzufügen" : "Personalakte anlegen"
      }
      titleLevel={2}
      size="lg"
      error={formErrors.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={save.pending}
          disabled={!preview}
          disabledReason="Die Vorschau lädt noch."
          saveLabel={joinsRecord ? "Hinzufügen" : "Akte anlegen"}
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-6"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        {preview ? (
          <>
            <section className="space-y-2">
              <SectionLabel as="h3">
                {preview.record
                  ? `Akte von ${staffName(preview.record)}`
                  : "Diese Beschäftigungen werden eine Akte"}
              </SectionLabel>
              <ul className="grid gap-2 sm:grid-cols-2">
                {preview.members.map((member) => (
                  <li key={`${member.schoolId}-${member.staffId}`}>
                    <InsetPanel tone="white" padding="md" framed>
                      <p className="font-medium text-gray-900">
                        {staffName(member)}
                      </p>
                      <p className="text-sm text-gray-600">
                        {member.schoolName}
                      </p>
                      <p className="mt-1 text-sm text-gray-500 tabular-nums">
                        {member.personnelNumber
                          ? `Personalnummer ${member.personnelNumber}`
                          : "Ohne Personalnummer"}
                      </p>
                    </InsetPanel>
                  </li>
                ))}
              </ul>
            </section>

            {conflicts.length > 0 ? (
              <section className="space-y-4">
                <div className="space-y-1">
                  <SectionLabel as="h3">Welcher Wert gilt?</SectionLabel>
                  <p className="text-sm text-gray-600">
                    Die Schulen haben hier verschiedene Werte. Der andere Wert
                    bleibt mit seiner Schule im Verlauf der Akte.
                  </p>
                </div>
                {conflicts.map((field) => {
                  const labelId = `staff-choice-${field.field}-label`;
                  return (
                    <div key={field.field} className="space-y-2">
                      <FieldLabel id={labelId} required>
                        {STAFF_FIELD_LABELS[field.field]}
                      </FieldLabel>
                      <div
                        role="radiogroup"
                        aria-labelledby={labelId}
                        className="grid gap-2 sm:grid-cols-2"
                      >
                        {field.options.map((option) => (
                          <ChoiceTile
                            key={option.source}
                            selected={choices[field.field] === option.source}
                            className="items-start gap-3 px-4 py-3"
                          >
                            <Radio
                              name={`staff-choice-${field.field}`}
                              value={option.source}
                              checked={choices[field.field] === option.source}
                              onChange={() =>
                                setChoices((current) => ({
                                  ...current,
                                  [field.field]: option.source,
                                }))
                              }
                              className="mt-0.5"
                            />
                            <span className="flex min-w-0 flex-col gap-0.5">
                              <span className="font-medium wrap-anywhere text-gray-900">
                                {staffFieldValue(field.field, option.value)}
                              </span>
                              <span className="text-sm font-normal text-gray-600">
                                {option.source === "record"
                                  ? "Bisher in der Akte"
                                  : option.schoolName}
                              </span>
                            </span>
                          </ChoiceTile>
                        ))}
                      </div>
                      <FieldErrorHint
                        id={`staff-choice-${field.field}-error`}
                        message={formErrors.fieldError(`choice-${field.field}`)}
                      />
                    </div>
                  );
                })}
              </section>
            ) : null}

            <section className="space-y-2">
              <SectionLabel as="h3">Das steht danach in der Akte</SectionLabel>
              <InsetPanel padding="md">
                <DataGrid columns={2}>
                  {preview.fields
                    .filter((field) => !field.conflict)
                    .map((field) => (
                      <DataField
                        key={field.field}
                        label={STAFF_FIELD_LABELS[field.field]}
                      >
                        {agreedValue(preview, field.field)}
                      </DataField>
                    ))}
                </DataGrid>
              </InsetPanel>
              {preview.fields.some(
                (field) =>
                  ["iban", "tax_id", "social_security_number"].includes(
                    field.field,
                  ) && field.options.length > 0,
              ) ? (
                <p className="flex flex-wrap items-center gap-2 text-sm text-gray-600">
                  <StatusBadge label="Bank und Steuer" tone="blue" size="xs" />
                  Die Schulen geben ihre Bank- und Steuerdaten an die Akte ab.
                  Danach führt sie nur noch der Träger.
                </p>
              ) : null}
            </section>
          </>
        ) : formErrors.error ? null : (
          <Loading message="Vorschau wird geladen…" compact />
        )}
      </form>
    </FormModal>
  );
}
