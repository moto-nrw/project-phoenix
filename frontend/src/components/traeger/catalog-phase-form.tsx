"use client";

// Anmeldephase des Trägers anlegen oder ändern (#4704). Die Angaben bleiben
// ein Entwurf beim Träger; die Schulen sehen sie erst nach „Freigeben und
// übertragen“.

import { useRef, useState } from "react";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DateTimePicker } from "~/components/ui/date-time-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { FormFieldset } from "~/components/ui/form-fieldset";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  berlinTodayISO,
  fromDateTimeLocalValue,
  parseISODate,
  toDateTimeLocalValue,
} from "~/lib/date-helpers";
import type {
  CareOfferingSelectionMode,
  PhaseAudience,
  PhaseCareOverflowMode,
  PhaseKind,
} from "~/lib/enrollment-phase-api";
import {
  PHASE_AUDIENCE_LABELS,
  PHASE_CARE_SELECTION_LABELS,
  PHASE_KIND_LABELS,
  PHASE_OVERFLOW_LABELS,
} from "~/lib/enrollment-phase-labels";
import { createLogger } from "~/lib/logger";
import type {
  TraegerPhase,
  TraegerPhaseDetail,
  TraegerPhaseInput,
} from "~/lib/traeger-catalog-api";
import type { TraegerSchool } from "~/lib/traeger-api";

const logger = createLogger({ component: "CatalogPhaseForm" });

function options<T extends string>(labels: Record<T, string>) {
  return (Object.entries(labels) as [T, string][]).map(([value, label]) => ({
    value,
    label,
  }));
}

const KIND_OPTIONS = options(PHASE_KIND_LABELS);
const AUDIENCE_OPTIONS = options(PHASE_AUDIENCE_LABELS);
const OVERFLOW_OPTIONS = options(PHASE_OVERFLOW_LABELS);
const SELECTION_OPTIONS = options(PHASE_CARE_SELECTION_LABELS);

type SchoolScope = "all" | "chosen";

const SCOPE_ITEMS = [
  { value: "all" as const, label: "Alle Schulen (auch neue)" },
  { value: "chosen" as const, label: "Ausgewählte Schulen" },
];

/** The next school year, August to July, as the suggestion for a new phase. */
export function blankPhaseInput(today = berlinTodayISO()): TraegerPhaseInput {
  const now = parseISODate(today);
  const year = now.getMonth() >= 7 ? now.getFullYear() + 1 : now.getFullYear();
  return {
    name: "",
    kind: "school_year",
    serviceStartDate: `${year}-08-01`,
    serviceEndDate: `${year + 1}-07-31`,
    enrollmentOpenAt: null,
    enrollmentCloseAt: null,
    audience: "open",
    careOverflowMode: "waitlist",
    careOfferingSelectionMode: "optional",
    allSchools: true,
    schoolIds: [],
  };
}

function phaseInputOf(phase: TraegerPhase): TraegerPhaseInput {
  return {
    ...phase.draft,
    allSchools: phase.allSchools,
    schoolIds: phase.schools
      .filter((school) => school.selected)
      .map((school) => school.schoolId),
  };
}

type FieldErrors = Record<string, string>;

/** The checks the form makes before it sends; the backend checks again. */
export function validatePhaseInput(input: TraegerPhaseInput): FieldErrors {
  const errors: FieldErrors = {};
  if (!input.name.trim()) errors.name = "Bitte geben Sie einen Namen ein.";
  if (!input.serviceStartDate) {
    errors.service_start_date = "Bitte wählen Sie den ersten Tag.";
  }
  if (!input.serviceEndDate) {
    errors.service_end_date = "Bitte wählen Sie den letzten Tag.";
  } else if (
    input.serviceStartDate &&
    input.serviceEndDate < input.serviceStartDate
  ) {
    errors.service_end_date = "Das Ende liegt vor dem Beginn.";
  }
  if (
    input.enrollmentOpenAt &&
    input.enrollmentCloseAt &&
    input.enrollmentCloseAt <= input.enrollmentOpenAt
  ) {
    errors.enrollment_close_at = "Das Ende liegt vor dem Beginn.";
  }
  if (!input.allSchools && input.schoolIds.length === 0) {
    errors.school_ids = "Bitte wählen Sie mindestens eine Schule.";
  }
  return errors;
}

export function CatalogPhaseForm({
  phase,
  schools,
  saveLabel,
  onSave,
  onSaved,
  onCancel,
}: Readonly<{
  /** The phase to change; without it the form creates one. */
  phase?: TraegerPhase;
  schools: readonly TraegerSchool[];
  saveLabel: string;
  onSave: (input: TraegerPhaseInput) => Promise<TraegerPhaseDetail>;
  onSaved: (phase: TraegerPhaseDetail) => void;
  onCancel: () => void;
}>) {
  const [draft, setDraft] = useState<TraegerPhaseInput>(() =>
    phase ? phaseInputOf(phase) : blankPhaseInput(),
  );
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);

  const set = <K extends keyof TraegerPhaseInput>(
    key: K,
    value: TraegerPhaseInput[K],
  ) => setDraft((current) => ({ ...current, [key]: value }));
  const fieldError = formErrors.fieldError;

  const toggleSchool = (schoolId: string, checked: boolean) =>
    set(
      "schoolIds",
      checked
        ? [...draft.schoolIds, schoolId]
        : draft.schoolIds.filter((id) => id !== schoolId),
    );

  const submit = async () => {
    formErrors.clear();
    const found = validatePhaseInput(draft);
    if (Object.keys(found).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", found);
      return;
    }
    setSaving(true);
    try {
      onSaved(await onSave(draft));
    } catch (err) {
      logger.error("traeger_catalog_phase_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Anmeldephase" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Anmeldephase"
      description="Diese Angaben bekommt jede Schule als eigene Anmeldephase. Die Schule kann sie danach ändern."
      footer={
        <EditActions
          onCancel={onCancel}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel={saveLabel}
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
        <FormErrorAlert message={formErrors.error} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            name="name"
            label="Name"
            required
            maxLength={200}
            value={draft.name}
            error={fieldError("name")}
            placeholder="Zum Beispiel: Schuljahr 2027/28"
            onChange={(event) => set("name", event.target.value)}
          />
          <CustomSelect
            id="catalog-phase-kind"
            name="kind"
            label="Art"
            value={draft.kind}
            options={KIND_OPTIONS}
            error={fieldError("kind")}
            onChange={(value) => set("kind", value as PhaseKind)}
          />
        </div>

        <FormFieldset legend="Leistungszeitraum" required>
          <div className="grid gap-3 sm:grid-cols-2">
            <ISODatePicker
              id="catalog-phase-start"
              name="service_start_date"
              label="Erster Tag"
              value={draft.serviceStartDate}
              error={fieldError("service_start_date")}
              calendarLayout="popover"
              hideClearButton
              required
              onChange={(next) => set("serviceStartDate", next)}
            />
            <ISODatePicker
              id="catalog-phase-end"
              name="service_end_date"
              label="Letzter Tag"
              min={draft.serviceStartDate || undefined}
              value={draft.serviceEndDate}
              error={fieldError("service_end_date")}
              calendarLayout="popover"
              hideClearButton
              required
              onChange={(next) => set("serviceEndDate", next)}
            />
          </div>
        </FormFieldset>

        <FormFieldset
          legend="Anmeldefenster (freiwillig)"
          hint="Leer heißt: Eltern können sich jederzeit anmelden."
        >
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <FieldLabel htmlFor="catalog-phase-open">Anmeldung ab</FieldLabel>
              <DateTimePicker
                id="catalog-phase-open"
                dateAriaLabel="Anmeldung ab"
                timeAriaLabel="Anmeldung ab, Uhrzeit"
                value={toDateTimeLocalValue(draft.enrollmentOpenAt)}
                onChange={(next) =>
                  set("enrollmentOpenAt", fromDateTimeLocalValue(next))
                }
              />
            </div>
            <div>
              <FieldLabel htmlFor="catalog-phase-close">
                Anmeldung bis
              </FieldLabel>
              <DateTimePicker
                id="catalog-phase-close"
                dateAriaLabel="Anmeldung bis"
                timeAriaLabel="Anmeldung bis, Uhrzeit"
                defaultTime="23:59"
                min={toDateTimeLocalValue(draft.enrollmentOpenAt) || undefined}
                value={toDateTimeLocalValue(draft.enrollmentCloseAt)}
                invalid={Boolean(fieldError("enrollment_close_at"))}
                onChange={(next) =>
                  set("enrollmentCloseAt", fromDateTimeLocalValue(next))
                }
              />
              <FieldErrorHint
                id="catalog-phase-close-error"
                message={fieldError("enrollment_close_at")}
              />
            </div>
          </div>
        </FormFieldset>

        <div className="grid gap-4 sm:grid-cols-3">
          <CustomSelect
            id="catalog-phase-audience"
            name="audience"
            label="Zielgruppe"
            hint="Wer sich anmelden darf."
            value={draft.audience}
            options={AUDIENCE_OPTIONS}
            error={fieldError("audience")}
            onChange={(value) => set("audience", value as PhaseAudience)}
          />
          <CustomSelect
            id="catalog-phase-selection"
            name="care_offering_selection_mode"
            label="Betreuungsauswahl"
            hint="Wie viele Angebote Eltern wählen müssen."
            value={draft.careOfferingSelectionMode}
            options={SELECTION_OPTIONS}
            error={fieldError("care_offering_selection_mode")}
            onChange={(value) =>
              set(
                "careOfferingSelectionMode",
                value as CareOfferingSelectionMode,
              )
            }
          />
          <CustomSelect
            id="catalog-phase-overflow"
            name="care_overflow_mode"
            label="Verhalten bei voller Betreuung"
            hint="Was passiert, wenn ein Angebot voll ist."
            value={draft.careOverflowMode}
            options={OVERFLOW_OPTIONS}
            error={fieldError("care_overflow_mode")}
            onChange={(value) =>
              set("careOverflowMode", value as PhaseCareOverflowMode)
            }
          />
        </div>

        <FormFieldset
          legend="Schulen"
          required
          error={fieldError("school_ids")}
          hint={
            draft.allSchools
              ? "Auch eine Schule, die später zu Ihrem Träger kommt, bekommt die Anmeldephase."
              : "Nur die gewählten Schulen bekommen die Anmeldephase."
          }
        >
          <div className="space-y-3">
            <SegmentedControl
              items={SCOPE_ITEMS}
              value={
                (draft.allSchools ? "all" : "chosen") satisfies SchoolScope
              }
              onChange={(scope) => set("allSchools", scope === "all")}
              ariaLabel="Für welche Schulen"
            />
            {draft.allSchools ? null : (
              <div className="grid gap-2 sm:grid-cols-2">
                {schools.map((school) => (
                  <CheckboxCard
                    key={school.id}
                    checked={draft.schoolIds.includes(school.id)}
                    onChange={(checked) => toggleSchool(school.id, checked)}
                    label={school.name}
                    hint={school.city || undefined}
                  />
                ))}
              </div>
            )}
          </div>
        </FormFieldset>
      </form>
    </SectionCard>
  );
}
