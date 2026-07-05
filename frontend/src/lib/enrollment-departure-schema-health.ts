import type {
  FormField,
  FormFieldTarget,
  FormSchema,
} from "./enrollment-form-schema-api";

const MODERN_DEPARTURE_TARGET = "student.allowed_departure_modes" as const;

const LEGACY_DEPARTURE_TARGETS = [
  "student.departure",
  "student.bus_days",
  "student.bus",
  "student.pickup_status",
] as const satisfies readonly FormFieldTarget[];

const LEGACY_DEPARTURE_TARGET_SET = new Set<FormFieldTarget>(
  LEGACY_DEPARTURE_TARGETS,
);

const LEGACY_DEPARTURE_LABELS: Record<
  (typeof LEGACY_DEPARTURE_TARGETS)[number],
  string
> = {
  "student.departure": "Geh- und Abholregelung",
  "student.bus_days": "Buskind",
  "student.bus": "Buskind",
  "student.pickup_status": "Abholregelung",
};

export interface DepartureSchemaHealth {
  hasModernDepartureModes: boolean;
  legacyFields: FormField[];
  legacyLabels: string[];
  needsLegacyCleanup: boolean;
}

export function getDepartureSchemaHealth(
  schemaOrFields: FormSchema | readonly FormField[] | null | undefined,
): DepartureSchemaHealth {
  const fields: readonly FormField[] = isFieldArray(schemaOrFields)
    ? schemaOrFields
    : (schemaOrFields?.fields ?? []);
  const legacyFields = fields.filter((field) => isLegacyDepartureField(field));
  const legacyLabels = Array.from(
    new Set(
      legacyFields
        .map((field) => legacyDepartureTargetLabel(field.target))
        .filter((label): label is string => label !== null),
    ),
  );
  return {
    hasModernDepartureModes: fields.some(isModernDepartureField),
    legacyFields,
    legacyLabels,
    needsLegacyCleanup: legacyFields.length > 0,
  };
}

function isFieldArray(
  value: FormSchema | readonly FormField[] | null | undefined,
): value is readonly FormField[] {
  return Array.isArray(value);
}

export function isModernDepartureField(field: FormField): boolean {
  return (
    field.target === MODERN_DEPARTURE_TARGET &&
    field.type === "weekday_multi_mode"
  );
}

export function isLegacyDepartureField(field: FormField): boolean {
  return Boolean(field.target && LEGACY_DEPARTURE_TARGET_SET.has(field.target));
}

export function legacyDepartureTargetLabel(
  target: FormFieldTarget | undefined,
): string | null {
  if (!target || !LEGACY_DEPARTURE_TARGET_SET.has(target)) return null;
  return LEGACY_DEPARTURE_LABELS[
    target as (typeof LEGACY_DEPARTURE_TARGETS)[number]
  ];
}
