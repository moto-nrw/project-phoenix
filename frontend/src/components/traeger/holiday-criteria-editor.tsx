"use client";

// Die Kriterien der Platzvergabe festlegen (#4700). Die Reihenfolge ist die
// Priorität: das oberste Kriterium zählt am meisten. Jede Frage beantworten
// Eltern mit Ja oder Nein, einmal für die Familie oder für jedes Kind.
// Entfernen nimmt ein Kriterium aus der Liste; schon gegebene Antworten und
// Entscheidungen bleiben gespeichert. Vorlagen sind nur Vorschläge.

import { ArrowDown, ArrowUp, Plus } from "lucide-react";
import { useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { EmptyState } from "~/components/ui/empty-state";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { IconTile } from "~/components/ui/icon-tile";
import { Input } from "~/components/ui/input";
import { InsetPanel } from "~/components/ui/inset-panel";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { SwitchRow } from "~/components/ui/switch";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type HolidayCriterion,
  type HolidayCriterionScope,
  MAX_CRITERION_NAME,
  MAX_CRITERION_QUESTION,
  MAX_HOLIDAY_CRITERIA,
  saveHolidayCriteria,
} from "~/lib/traeger-holiday-allocation-api";
import { criterionTone } from "./holiday-allocation-model";
import {
  CRITERION_TEMPLATES,
  type CriterionDraft,
  criteriaChanged,
  draftsFromCriteria,
  emptyCriterion,
  fromTemplate,
  inputsFromDrafts,
  moveCriterion,
  templateUsed,
  validateCriteria,
} from "./holiday-criteria-model";

const logger = createLogger({ component: "HolidayCriteriaEditor" });

const SCOPES: readonly { value: HolidayCriterionScope; label: string }[] = [
  { value: "family", label: "Familie" },
  { value: "child", label: "Jedes Kind" },
];

/** How the parents see the question: the text with its two answers. */
function ParentPreview({ draft }: Readonly<{ draft: CriterionDraft }>) {
  return (
    <InsetPanel dashed padding="sm">
      <SectionLabel as="p">So sehen Eltern die Frage</SectionLabel>
      <p className="mt-1 text-sm font-medium text-gray-900">
        {draft.question.trim() || "Hier steht Ihre Frage."}
      </p>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        <StatusBadge tone="outline" size="xs" label="Ja" />
        <StatusBadge tone="outline" size="xs" label="Nein" />
        <span className="text-xs text-gray-600">
          {draft.scope === "family"
            ? "Einmal für die Familie. Pflichtfrage."
            : "Für jedes Kind einzeln. Pflichtfrage."}
        </span>
      </div>
    </InsetPanel>
  );
}

function CriterionRow({
  draft,
  index,
  count,
  disabled,
  fieldError,
  onChange,
  onMove,
  onRemove,
}: Readonly<{
  draft: CriterionDraft;
  index: number;
  count: number;
  disabled: boolean;
  fieldError: (name: string) => string | undefined;
  onChange: (next: CriterionDraft) => void;
  onMove: (delta: -1 | 1) => void;
  onRemove: () => void;
}>) {
  const position = index + 1;
  const label = draft.name.trim() || `Kriterium ${position}`;
  const scopeId = `criterion-${draft.key}-scope`;
  const scopeError = fieldError(`criteria.${index}.scope`);
  return (
    <InsetPanel as="li" tone="white" framed padding="md" className="space-y-4">
      <div className="flex flex-wrap items-center gap-3">
        <IconTile
          icon={
            <span className="text-sm font-semibold tabular-nums">
              {position}
            </span>
          }
          size="xs"
          shape="round"
          tone={draft.active ? "inverse" : "neutral"}
        />
        <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
          <h3 className="text-sm font-semibold wrap-anywhere text-gray-900">
            <span className="sr-only">Priorität {position}: </span>
            {label}
          </h3>
          {draft.active ? (
            <StatusBadge
              tone={criterionTone(position)}
              size="xs"
              label={`Priorität ${position}`}
            />
          ) : (
            <StatusBadge tone="gray" size="xs" label="Ausgeschaltet" />
          )}
        </div>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon"
            disabled={disabled || index === 0}
            disabledReason={
              index === 0
                ? "Dieses Kriterium hat schon die höchste Priorität."
                : undefined
            }
            tooltip="Schiebt das Kriterium eine Stelle höher, es zählt dann mehr."
            aria-label={`„${label}“ nach oben, höhere Priorität`}
            onClick={() => onMove(-1)}
          >
            <ArrowUp className="h-4 w-4" aria-hidden="true" />
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            disabled={disabled || index === count - 1}
            disabledReason={
              index === count - 1
                ? "Dieses Kriterium hat schon die niedrigste Priorität."
                : undefined
            }
            tooltip="Schiebt das Kriterium eine Stelle tiefer, es zählt dann weniger."
            aria-label={`„${label}“ nach unten, niedrigere Priorität`}
            onClick={() => onMove(1)}
          >
            <ArrowDown className="h-4 w-4" aria-hidden="true" />
          </Button>
          <OverflowMenu
            ariaLabel={`Weitere Aktionen für „${label}“`}
            disabled={disabled}
            items={[
              {
                label: "Entfernen",
                description: draft.answered
                  ? "Gegebene Antworten bleiben gespeichert."
                  : "Nimmt das Kriterium aus der Liste.",
                destructive: true,
                onClick: onRemove,
              },
            ]}
          />
        </div>
      </div>

      <SwitchRow
        framed={false}
        checked={draft.active}
        disabled={disabled}
        ariaLabel={`„${label}“ eingeschaltet`}
        onCheckedChange={(active) => onChange({ ...draft, active })}
        label={
          draft.active
            ? "Eingeschaltet: Eltern sehen die Frage."
            : "Ausgeschaltet: Eltern sehen die Frage nicht. Es zählt nicht."
        }
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_auto]">
        <Input
          controlSize="compact"
          name={`criteria.${index}.name`}
          label="Name"
          hint="Sehen nur Sie und Ihr Büro."
          required
          maxLength={MAX_CRITERION_NAME}
          value={draft.name}
          disabled={disabled}
          error={fieldError(`criteria.${index}.name`)}
          onChange={(event) => onChange({ ...draft, name: event.target.value })}
        />
        <div className="space-y-1">
          <FieldLabel id={scopeId}>Gilt für</FieldLabel>
          {/* Named like the server's field, so a refused scope focuses here. */}
          <input
            type="hidden"
            name={`criteria.${index}.scope`}
            value={draft.scope}
          />
          <SegmentedControl
            items={SCOPES.map((scope) => ({
              ...scope,
              disabled: disabled || draft.answered,
            }))}
            value={draft.scope}
            onChange={(scope) => onChange({ ...draft, scope })}
            ariaLabel={`Gilt für: ${label}`}
            ariaDescribedBy={scopeError ? `${scopeId}-error` : undefined}
          />
          <FieldErrorHint id={`${scopeId}-error`} message={scopeError} />
          {draft.answered ? (
            <p className="text-xs text-gray-600">
              Eltern haben schon geantwortet. Darum bleibt das so.
            </p>
          ) : null}
        </div>
      </div>
      <Textarea
        name={`criteria.${index}.question`}
        label="Frage an die Eltern"
        hint="Eine Frage, die man mit Ja oder Nein beantwortet."
        required
        rows={2}
        autoGrow
        maxLength={MAX_CRITERION_QUESTION}
        value={draft.question}
        disabled={disabled}
        error={fieldError(`criteria.${index}.question`)}
        onChange={(event) =>
          onChange({ ...draft, question: event.target.value })
        }
      />
      <ParentPreview draft={draft} />
    </InsetPanel>
  );
}

export function HolidayCriteriaEditor({
  criteria,
  onSaved,
}: Readonly<{
  criteria: readonly HolidayCriterion[];
  onSaved: (criteria: HolidayCriterion[]) => void;
}>) {
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const toast = useToast();
  const [drafts, setDrafts] = useState<CriterionDraft[]>(() =>
    draftsFromCriteria(criteria),
  );
  const [saving, setSaving] = useState(false);
  const changed = criteriaChanged(drafts, criteria);
  const full = drafts.length >= MAX_HOLIDAY_CRITERIA;
  const fieldError = (name: string) => formErrors.fieldError(name) ?? undefined;
  const unusedTemplates = CRITERION_TEMPLATES.filter(
    (template) => !templateUsed(drafts, template.name),
  );

  const setDraft = (index: number, next: CriterionDraft) =>
    setDrafts((current) =>
      current.map((draft, i) => (i === index ? next : draft)),
    );

  const save = async () => {
    const errors = validateCriteria(drafts);
    if (Object.keys(errors).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", errors);
      return;
    }
    formErrors.clear();
    setSaving(true);
    try {
      const saved = await saveHolidayCriteria(inputsFromDrafts(drafts));
      setDrafts(draftsFromCriteria(saved));
      toast.success("Die Kriterien sind gespeichert.");
      onSaved(saved);
    } catch (err) {
      logger.error("holiday_criteria_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Kriterien" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      ref={formRef}
      noValidate
      aria-label="Kriterien der Platzvergabe"
      className="flex flex-col gap-6"
      onSubmit={(event) => {
        event.preventDefault();
        void save();
      }}
    >
      <SectionCard
        title="Kriterien"
        description="Die Reihenfolge ist die Priorität. Das oberste Kriterium zählt am meisten."
        footer={
          <EditActions
            onCancel={() => {
              formErrors.clear();
              setDrafts(draftsFromCriteria(criteria));
            }}
            cancelDisabled={saving || !changed}
            cancelLabel="Verwerfen"
            saving={saving}
            disabled={!changed}
            saveType="submit"
            saveLabel="Kriterien speichern"
            stretchOnPhone
          />
        }
      >
        <div className="space-y-4">
          <FormErrorAlert message={formErrors.error} />
          <Alert
            type="info"
            message="Fragen Sie nur, was Sie für die Vergabe wirklich brauchen. Fragen Sie nicht nach Gesundheit oder Förderbedarf. Eltern antworten mit Ja oder Nein. Das Büro kann nach einem Nachweis fragen."
          />
          {drafts.length === 0 ? (
            <EmptyState
              variant="dashed"
              title="Noch kein Kriterium"
              description="Wählen Sie unten eine Vorlage oder legen Sie ein eigenes Kriterium an."
            />
          ) : (
            <ol className="space-y-3" aria-label="Kriterien nach Priorität">
              {drafts.map((draft, index) => (
                <CriterionRow
                  key={draft.key}
                  draft={draft}
                  index={index}
                  count={drafts.length}
                  disabled={saving}
                  fieldError={fieldError}
                  onChange={(next) => setDraft(index, next)}
                  onMove={(delta) =>
                    setDrafts((current) => moveCriterion(current, index, delta))
                  }
                  onRemove={() =>
                    setDrafts((current) =>
                      current.filter((_, i) => i !== index),
                    )
                  }
                />
              ))}
            </ol>
          )}
          <FieldErrorHint
            id="holiday-criteria-error"
            message={fieldError("criteria")}
          />
          <div className="space-y-2 border-t border-gray-200 pt-4">
            <SectionLabel as="h3">Kriterium hinzufügen</SectionLabel>
            {full ? (
              <p className="text-sm text-gray-600">
                Es gehen höchstens {MAX_HOLIDAY_CRITERIA} Kriterien.
              </p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {unusedTemplates.map((template) => (
                  <Button
                    key={template.name}
                    type="button"
                    variant="outline"
                    size="compact"
                    className="gap-1.5"
                    disabled={saving}
                    aria-label={`Vorlage „${template.name}“ hinzufügen`}
                    tooltip="Fügt das Kriterium unten an, wirksam erst nach dem Speichern."
                    onClick={() =>
                      setDrafts((current) => [
                        ...current,
                        fromTemplate(template),
                      ])
                    }
                  >
                    <Plus className="h-4 w-4" aria-hidden="true" />
                    {template.name}
                  </Button>
                ))}
                <Button
                  type="button"
                  variant="dashed"
                  size="compact"
                  className="gap-1.5"
                  disabled={saving}
                  tooltip="Fügt ein leeres Kriterium an, das Sie selbst benennen."
                  onClick={() =>
                    setDrafts((current) => [...current, emptyCriterion()])
                  }
                >
                  <Plus className="h-4 w-4" aria-hidden="true" />
                  Eigenes Kriterium
                </Button>
              </div>
            )}
          </div>
        </div>
      </SectionCard>
    </form>
  );
}
