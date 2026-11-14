"use client";

import { useContext, useMemo } from "react";
import { Check, Lock, Plus } from "lucide-react";
import {
  FieldErrorContext,
  fieldErrorProps,
} from "~/components/enrollment/form-field-builder";
import type { ReservedTarget } from "~/components/enrollment/form-template-draft";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { IconTile } from "~/components/ui/icon-tile";
import { Input } from "~/components/ui/input";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Modal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import { Switch } from "~/components/ui/switch";
import {
  RESERVED_TARGETS,
  type FormField,
  type FormSchema,
} from "~/lib/enrollment-form-schema-api";

export type PendingNavigation = "overview" | "new" | "preview";

// The fixed suggestions in picker order. The legacy targets student.bus,
// student.bus_days, student.pickup_status and student.departure are
// superseded by student.allowed_departure_modes (#1610): saved fields keep
// them, new fields cannot pick them.
const TARGET_PICKER_ORDER = [
  "schedule.pickup",
  "schedule.arrival",
  "student.allowed_departure_modes",
  "student.health_info",
  "student.extra_info",
  "student.contacts",
] as const satisfies readonly ReservedTarget[];

const targetSuggestionDescriptions: Record<
  (typeof TARGET_PICKER_ORDER)[number],
  string
> = {
  "student.health_info":
    "Für Allergien, Medikamente oder andere Gesundheitsangaben.",
  "student.extra_info":
    "Für wichtige Hinweise, die im Alltag der Betreuung sichtbar sein sollen.",
  "student.allowed_departure_modes":
    "Für Betreuungstage festlegen, welche Heimwege erlaubt sind: zu Fuß, Bus, Abholung oder mit anderem Kind. Mit Betreuungsangeboten sehen Eltern nur die gewählten Betreuungstage; als Pflichtfrage ist pro Betreuungstag mindestens ein Heimweg nötig.",
  "schedule.pickup": "Für regelmäßige Abholzeiten je Wochentag.",
  "schedule.arrival": "Für regelmäßige Ankunftszeiten je Wochentag.",
  "student.contacts":
    "Für weitere Kontakte, Abholberechtigte oder Notfallkontakte.",
};

export function UnsavedChangesDialog({
  pendingNavigation,
  saving,
  saveValidationMessage,
  pendingUploadMessage,
  onCancel,
  onDiscard,
  onSave,
}: Readonly<{
  pendingNavigation: PendingNavigation | null;
  saving: boolean;
  saveValidationMessage: string | null;
  pendingUploadMessage: string | null;
  onCancel: () => void;
  onDiscard: () => void;
  onSave: () => void;
}>) {
  if (!pendingNavigation) return null;

  const isPreview = pendingNavigation === "preview";
  const saveBlocked = Boolean(saveValidationMessage || pendingUploadMessage);
  const footer = (
    <>
      <Button
        type="button"
        onClick={onCancel}
        disabled={saving}
        noTooltip="Abbrechen schließt nur den Dialog und bleibt im Editor."
        variant="outline"
        size="md"
        className="inline-flex items-center justify-center"
      >
        Abbrechen
      </Button>
      {!isPreview ? (
        <Button
          type="button"
          onClick={onDiscard}
          disabled={saving}
          tooltip={
            pendingNavigation === "new"
              ? "Verwirft Ihre Änderungen sofort und beginnt eine leere neue Vorlage."
              : "Verwirft Ihre Änderungen sofort und öffnet die Übersicht."
          }
          variant="outline"
          size="md"
          className="inline-flex items-center justify-center"
        >
          Verwerfen
        </Button>
      ) : null}
      <Button
        type="button"
        onClick={onSave}
        disabled={saving || saveBlocked}
        disabledReason={
          saving
            ? undefined
            : (saveValidationMessage ?? pendingUploadMessage ?? undefined)
        }
        tooltip={
          isPreview
            ? "Speichert die Vorlage und öffnet die Vorschau in einem neuen Tab."
            : pendingNavigation === "new"
              ? "Speichert die Vorlage und beginnt danach eine leere neue Vorlage."
              : "Speichert die Vorlage und öffnet danach die Übersicht."
        }
        variant="primary"
        size="md"
        className="inline-flex items-center justify-center"
      >
        {saving
          ? "Speichert…"
          : isPreview
            ? "Speichern und Vorschau öffnen"
            : "Speichern und fortfahren"}
      </Button>
    </>
  );

  return (
    <Modal
      isOpen
      onClose={onCancel}
      title="Änderungen speichern?"
      isDismissDisabled={saving}
      footer={footer}
    >
      <p className="text-sm leading-6 text-gray-600">
        {isPreview
          ? "Für die externe Vorschau muss die Vorlage zuerst gespeichert werden."
          : "Sie haben Änderungen an dieser Vorlage. Speichern Sie sie, bevor Sie den Bereich verlassen, oder verwerfen Sie sie bewusst."}
      </p>
      {saveValidationMessage || pendingUploadMessage ? (
        <div className="mt-4 space-y-3">
          {saveValidationMessage ? (
            <FormErrorAlert message={saveValidationMessage} />
          ) : null}
          {pendingUploadMessage ? (
            <Alert type="warning" message={pendingUploadMessage} />
          ) : null}
        </div>
      ) : null}
    </Modal>
  );
}

export function FormBuilderIntro() {
  return (
    <header>
      <h2 className="text-xl font-semibold text-gray-900">
        Formularvorlage bearbeiten
      </h2>
      <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
        Legen Sie fest, welche optionalen Basisfelder verpflichtend sind, und
        ergänzen Sie bei Bedarf Zusatzfragen. Die Vorlage wird später in einer
        Anmeldephase ausgewählt.
      </p>
    </header>
  );
}

export function BuilderTemplateSummary({
  name,
  isCreating,
  saving,
  currentSchema,
  onNameChange,
  fields,
}: Readonly<{
  name: string;
  isCreating: boolean;
  saving: boolean;
  currentSchema: FormSchema | null;
  onNameChange: (value: string) => void;
  fields: FormField[];
}>) {
  const nameError = fieldErrorProps(useContext(FieldErrorContext), "name");
  const requiredCount = fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    <SectionCard>
      <div className="grid gap-4 md:grid-cols-[minmax(0,1fr)_280px]">
        <div className="min-w-0">
          <h2 className="text-base font-semibold text-gray-900">
            {isCreating ? "Neue Formularvorlage" : name}
          </h2>
          <p className="mt-1 max-w-2xl text-sm text-gray-600">
            {isCreating
              ? "Geben Sie der Vorlage einen eindeutigen Namen. Danach können Sie Pflichtangaben und Zusatzfragen festlegen."
              : "Der Name lässt sich hier ändern und gilt für alle Versionen. Bestehende Anmeldungen bleiben nachvollziehbar."}
          </p>
        </div>

        <div className="space-y-3">
          <Input
            id={nameError.controlId}
            name={nameError.controlProps.name}
            type="text"
            label="Name der Vorlage"
            controlSize="compact"
            value={name}
            onChange={(event) => onNameChange(event.target.value)}
            placeholder="z. B. Ferienbetreuung Sommer 2026"
            disabled={saving}
            error={nameError.message}
          />
          {!isCreating ? (
            <div className="flex flex-wrap content-start gap-2 text-xs text-gray-600">
              <StatusBadge
                tone="gray"
                label={`${fields.length} Zusatzfragen`}
              />
              <StatusBadge
                tone="gray"
                label={`${requiredCount} Pflicht-Zusatzfragen`}
              />
              <StatusBadge tone="gray" label={`${childFieldCount} pro Kind`} />
            </div>
          ) : null}
        </div>
      </div>

      {currentSchema ? (
        <div className="mt-4 flex flex-wrap gap-2 text-xs text-gray-600">
          <StatusBadge
            tone="gray"
            label={new Date(currentSchema.created_at).toLocaleString("de-DE", {
              timeZone: "Europe/Berlin",
            })}
          />
        </div>
      ) : null}
    </SectionCard>
  );
}

// SignatureRequirementSection switches the parent's signature on for the
// whole form (#4786): the form then ends with a signature field after the
// legal texts, and every later edit by the parents asks for it again.
export function SignatureRequirementSection({
  required,
  onChange,
  disabled,
}: Readonly<{
  required: boolean;
  onChange: (checked: boolean) => void;
  disabled: boolean;
}>) {
  return (
    <SectionCard>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-gray-900">
            Unterschrift
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            {required
              ? "Eltern unterschreiben am Ende des Formulars mit einer einfachen elektronischen Signatur mit gezeichneter Unterschrift. Ohne Unterschrift lässt sich die Anmeldung nicht absenden. Auch jede spätere Änderung durch die Eltern braucht eine neue Unterschrift."
              : "Eltern senden die Anmeldung ohne Unterschrift ab."}
          </p>
        </div>
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <span className="text-xs font-medium text-gray-600">
            {required ? "Erforderlich" : "Aus"}
          </span>
          <Switch
            checked={required}
            onCheckedChange={onChange}
            disabled={disabled}
            ariaLabel="Unterschrift erforderlich"
          />
        </div>
      </div>
    </SectionCard>
  );
}

export function TargetSuggestions({
  fields,
  onAdd,
  disabled,
}: Readonly<{
  fields: FormField[];
  onAdd: (target: ReservedTarget) => void;
  disabled: boolean;
}>) {
  const selectedTargets = useMemo(
    () => new Set(fields.map((field) => field.target).filter(Boolean)),
    [fields],
  );

  return (
    <InsetPanel as="section" framed padding="md">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h3 className="text-sm font-semibold text-gray-900">
            Stammdaten-Vorschläge
          </h3>
          <p className="mt-1 max-w-2xl text-xs leading-5 text-gray-600">
            Diese Fragen sind mit vorhandenen Stammdaten verbunden. Sie können
            sie hinzufügen oder entfernen; Label und Typ sind fest vorgegeben.
          </p>
        </div>
        <span className="w-fit">
          <StatusBadge tone="outline" icon={Lock} label="Stammdaten" />
        </span>
      </div>

      <div className="mt-4 grid gap-2 md:grid-cols-2">
        {TARGET_PICKER_ORDER.map((target) => {
          const selected = selectedTargets.has(target);
          return (
            <ChoiceTile
              key={target}
              as="button"
              selected={selected}
              disabled={disabled || selected}
              onClick={() => onAdd(target)}
              className="min-h-24 items-start p-3 shadow-sm"
            >
              <IconTile
                size="xs"
                tone={selected ? "neutral" : "inverse"}
                className="mt-0.5"
                icon={
                  selected ? (
                    <Check aria-hidden="true" />
                  ) : (
                    <Plus aria-hidden="true" />
                  )
                }
              />
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-gray-900">
                  {RESERVED_TARGETS[target].label}
                </span>
                <span className="mt-1 block text-xs leading-5 text-gray-500">
                  {targetSuggestionDescriptions[target]}
                </span>
                <span className="mt-2 block">
                  <StatusBadge
                    tone="gray"
                    compact
                    label={selected ? "Ist drin" : "Hinzufügen"}
                  />
                </span>
              </span>
            </ChoiceTile>
          );
        })}
      </div>
    </InsetPanel>
  );
}
