"use client";

import { Lock } from "lucide-react";
import { fieldTypeLabels } from "~/components/enrollment/form-field-builder";
import { StatusBadge } from "~/components/ui/status-badge";
import { SectionCard } from "~/components/ui/section-card";
import { Switch } from "~/components/ui/switch";
import type {
  CoreRequirementKey,
  CoreRequirements,
  FormFieldType,
} from "~/lib/enrollment-form-schema-api";

export interface CoreField {
  readonly key: string;
  readonly label: string;
  /** The label in the builder's form preview, without the group. */
  readonly previewLabel: string;
  readonly type: FormFieldType;
  readonly required: boolean;
  readonly group: "guardian" | "child";
  readonly requirementKey?: CoreRequirementKey;
  readonly requirementLabel?: string;
  readonly requirementHint?: string;
}

export const CORE_FIELDS: ReadonlyArray<CoreField> = [
  {
    key: "guardian_first_name",
    label: "Vorname (Elternteil)",
    previewLabel: "Vorname",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_last_name",
    label: "Nachname (Elternteil)",
    previewLabel: "Nachname",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_email",
    label: "E-Mail (Elternteil)",
    previewLabel: "E-Mail",
    type: "text",
    required: true,
    group: "guardian",
  },
  {
    key: "guardian_phone",
    label: "Telefonnummer (Elternteil)",
    previewLabel: "Telefonnummer",
    type: "text",
    required: false,
    group: "guardian",
    requirementKey: "guardian_phone",
    requirementLabel: "Telefonnummer verpflichtend",
    requirementHint: "Eltern müssen eine gültige Telefonnummer angeben.",
  },
  {
    key: "first_name",
    label: "Vorname (Kind)",
    previewLabel: "Vorname",
    type: "text",
    required: true,
    group: "child",
  },
  {
    key: "last_name",
    label: "Nachname (Kind)",
    previewLabel: "Nachname",
    type: "text",
    required: true,
    group: "child",
  },
  {
    key: "date_of_birth",
    label: "Geburtsdatum (Kind)",
    previewLabel: "Geburtsdatum",
    type: "date",
    required: true,
    group: "child",
  },
  {
    key: "target_grade_level",
    label: "Klassenstufe (Kind)",
    previewLabel: "Klassenstufe",
    type: "number",
    required: true,
    group: "child",
  },
];

export function CoreFieldsSection({
  coreRequirements,
  onRequirementChange,
  disabled,
}: Readonly<{
  coreRequirements: CoreRequirements;
  onRequirementChange: (key: CoreRequirementKey, required: boolean) => void;
  disabled: boolean;
}>) {
  const guardian = CORE_FIELDS.filter((f) => f.group === "guardian");
  const child = CORE_FIELDS.filter((f) => f.group === "child");
  const groups = [
    { title: "Eltern", fields: guardian },
    { title: "Kind", fields: child },
  ] satisfies ReadonlyArray<{ title: string; fields: CoreField[] }>;

  return (
    <SectionCard>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-base font-semibold text-gray-900">
            Pflichtstatus der Basisfelder
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Fest gesetzte Felder bleiben immer Pflicht. Optionale Basisfelder
            können Sie hier verpflichtend machen.
          </p>
        </div>
        <span className="w-fit">
          <StatusBadge tone="gray" icon={Lock} label="Basisfelder" />
        </span>
      </div>

      <div className="mt-4 overflow-hidden rounded-xl border border-gray-200">
        {groups.map((group) => (
          <CoreFieldGroup
            key={group.title}
            title={group.title}
            fields={group.fields}
            coreRequirements={coreRequirements}
            onRequirementChange={onRequirementChange}
            disabled={disabled}
          />
        ))}
      </div>
    </SectionCard>
  );
}

function CoreFieldGroup({
  title,
  fields,
  coreRequirements,
  onRequirementChange,
  disabled,
}: Readonly<{
  title: string;
  fields: CoreField[];
  coreRequirements: CoreRequirements;
  onRequirementChange: (key: CoreRequirementKey, required: boolean) => void;
  disabled: boolean;
}>) {
  return (
    <div className="border-b border-gray-100 last:border-b-0">
      <div className="bg-gray-50/80 px-4 py-2">
        <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
      </div>
      <ul className="divide-y divide-gray-100">
        {fields.map((field) => (
          <CoreFieldRow
            key={field.key}
            field={field}
            required={Boolean(
              field.requirementKey
                ? coreRequirements[field.requirementKey]
                : field.required,
            )}
            onRequirementChange={onRequirementChange}
            disabled={disabled}
          />
        ))}
      </ul>
    </div>
  );
}

function CoreFieldRow({
  field,
  required,
  onRequirementChange,
  disabled,
}: {
  readonly field: CoreField;
  readonly required: boolean;
  readonly onRequirementChange: (
    key: CoreRequirementKey,
    required: boolean,
  ) => void;
  readonly disabled: boolean;
}) {
  return (
    <li className="grid gap-3 bg-white px-4 py-3 sm:grid-cols-[minmax(0,1fr)_220px] sm:items-center">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium text-gray-900">{field.label}</p>
          <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs text-gray-500">
            {fieldTypeLabels[field.type]}
          </span>
        </div>
        <p className="mt-0.5 text-xs leading-5 text-gray-500">
          {field.requirementKey
            ? required
              ? (field.requirementHint ?? "Diese Angabe ist verpflichtend.")
              : "Kann verpflichtend gemacht werden."
            : "Immer erforderlich und deshalb nicht änderbar."}
        </p>
      </div>
      {field.requirementKey ? (
        <div className="flex items-center justify-between gap-3 sm:justify-end">
          <span className="text-xs font-medium text-gray-600">
            {required ? "Pflicht" : "Optional"}
          </span>
          <Switch
            checked={required}
            onCheckedChange={(checked) =>
              onRequirementChange(field.requirementKey!, checked)
            }
            disabled={disabled}
            ariaLabel={field.requirementLabel ?? `${field.label} Pflichtfeld`}
          />
        </div>
      ) : (
        <div className="flex justify-start sm:justify-end">
          <StatusBadge tone="gray" icon={Lock} label="Immer Pflicht" />
        </div>
      )}
    </li>
  );
}
