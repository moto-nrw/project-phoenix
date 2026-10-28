/**
 * Reusable form field components for student forms
 * Extracted to reduce code duplication between create and edit modals
 */

import { useId } from "react";
import { Bus } from "lucide-react";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type { Student } from "@/lib/api";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { ISODatePicker } from "~/components/ui/date-picker";
import { todayISO } from "~/lib/date-helpers";
import {
  type BusDays,
  DEPARTURE_WEEKDAYS,
  type AllowedDepartureModes,
  type DepartureMode,
  formatAllowedDepartureDays,
  normalizeAllowedDepartureModes,
} from "~/lib/student-helpers";
import {
  ParentVisibleBadge,
  ParentVisibilityLegend,
} from "~/components/ui/parent-visible-badge";
import { PARENT_VISIBLE_HINTS } from "~/lib/parent-visible-fields";
import { CompanionPicker } from "./companion-picker";
import type {
  CompanionExtensionConfirmation,
  CompanionWeekday,
  StudentCompanion,
} from "~/lib/student-companion-api";

interface SelectOption {
  value: string;
  label: string;
}

const EMPTY_GROUPS: SelectOption[] = [];
const DEFAULT_REQUIRED_FIELDS = {
  firstName: true,
  lastName: true,
  schoolClass: true,
} as const;

/**
 * Personal Information Section
 */
export function PersonalInfoSection({
  formData,
  onChange,
  fieldError,
  groups = EMPTY_GROUPS,
  requiredFields = DEFAULT_REQUIRED_FIELDS,
}: Readonly<{
  formData: Partial<Student>;
  onChange: (
    field: keyof Student,
    value: string | boolean | number | BusDays | null,
  ) => void;
  /** Hint per API field name, from the form's shared error path (#2513). */
  fieldError: (name: string) => string | undefined;
  groups?: SelectOption[];
  requiredFields?: {
    firstName?: boolean;
    lastName?: boolean;
    schoolClass?: boolean;
  };
}>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.children.icon}
          tone={MOTO_CONCEPTS.children.tone}
          size={18}
        />
        Persönliche Daten
      </h3>
      <ParentVisibilityLegend className="mb-3 md:mb-4" />
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
        <TextInput
          label="Vorname"
          value={formData.first_name ?? ""}
          onChange={(v) => onChange("first_name", v)}
          name="first_name"
          error={fieldError("first_name")}
          required={requiredFields.firstName}
          placeholder="Max"
          parentVisibleHint={PARENT_VISIBLE_HINTS.name}
        />
        <TextInput
          label="Nachname"
          value={formData.second_name ?? ""}
          onChange={(v) => onChange("second_name", v)}
          name="last_name"
          error={fieldError("last_name")}
          required={requiredFields.lastName}
          placeholder="Mustermann"
          parentVisibleHint={PARENT_VISIBLE_HINTS.name}
        />
        <TextInput
          label="Klasse"
          value={formData.school_class ?? ""}
          onChange={(v) => onChange("school_class", v)}
          name="school_class"
          error={fieldError("school_class")}
          required={requiredFields.schoolClass}
          placeholder="5A"
          parentVisibleHint={PARENT_VISIBLE_HINTS.schoolClass}
        />
        <SelectInput
          label="Gruppe"
          value={formData.group_id ?? ""}
          onChange={(v) => onChange("group_id", v || null)}
          options={[{ value: "", label: "Keine Gruppe" }, ...groups]}
        />
        <DateInput
          label="Geburtstag"
          value={formData.birthday ?? ""}
          onChange={(v) => onChange("birthday", v)}
          parentVisibleHint={PARENT_VISIBLE_HINTS.birthday}
        />
        <TextInput
          label="Straße und Hausnummer"
          value={formData.address_street ?? ""}
          onChange={(v) => onChange("address_street", v)}
          placeholder="Musterstraße 12"
        />
        <TextInput
          label="PLZ"
          value={formData.address_postal_code ?? ""}
          onChange={(v) => onChange("address_postal_code", v)}
          placeholder="50667"
        />
        <TextInput
          label="Ort"
          value={formData.address_city ?? ""}
          onChange={(v) => onChange("address_city", v)}
          placeholder="Köln"
        />
      </div>
    </div>
  );
}

/**
 * Text Input Field
 */
function TextInput({
  label,
  value,
  onChange,
  name,
  error,
  required = false,
  placeholder = "",
  parentVisibleHint,
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** API field name, so a field error from the shared error path finds it. */
  name?: string;
  error?: string;
  required?: boolean;
  placeholder?: string;
  /** Set when the parent portal mirrors this field (see parent-visible-fields). */
  parentVisibleHint?: string;
}>) {
  const inputId = useId();

  return (
    <div>
      <div className="mb-1 flex items-center gap-1">
        <label
          htmlFor={inputId}
          className="block text-xs font-medium text-gray-700"
        >
          {label}
        </label>
        {required && <RequiredMark />}
        {parentVisibleHint && (
          <ParentVisibleBadge compact hint={parentVisibleHint} />
        )}
      </div>
      <Input
        id={inputId}
        name={name}
        type="text"
        controlSize="compact"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-required={required || undefined}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? `${inputId}-error` : undefined}
        placeholder={placeholder}
        maxLength={255}
      />
      {error && (
        <p id={`${inputId}-error`} className="text-moto-red mt-1 text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

/**
 * Select Input Field
 */
function SelectInput({
  label,
  value,
  onChange,
  options,
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: SelectOption[];
}>) {
  const selectId = useId();
  return (
    <div>
      <label
        id={`${selectId}-label`}
        htmlFor={selectId}
        className="mb-1 block text-xs font-medium text-gray-700"
      >
        {label}
      </label>
      <CustomSelect
        id={selectId}
        ariaLabelledBy={`${selectId}-label`}
        value={value}
        options={options}
        onChange={onChange}
      />
    </div>
  );
}

/**
 * Date Input Field
 */
function DateInput({
  label,
  value,
  onChange,
  parentVisibleHint,
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** Set when the parent portal mirrors this field (see parent-visible-fields). */
  parentVisibleHint?: string;
}>) {
  const inputId = useId();
  return (
    <div>
      <div className="mb-1 flex items-center gap-1">
        <label
          htmlFor={inputId}
          className="block text-xs font-medium text-gray-700"
        >
          {label}
        </label>
        {parentVisibleHint && (
          <ParentVisibleBadge compact hint={parentVisibleHint} />
        )}
      </div>
      <ISODatePicker
        id={inputId}
        value={value}
        onChange={onChange}
        // A birthday is years back, so month arrows alone are unusable; and it
        // can never be in the future.
        monthYearNavigation
        max={todayISO()}
        calendarLayout="popover"
      />
    </div>
  );
}

/**
 * Textarea Field
 */
function TextareaField({
  label,
  value,
  onChange,
  placeholder = "",
  rows = 3,
  iconColor = "blue-600",
  iconPath,
  parentVisibleHint,
}: Readonly<{
  label: string;
  value: string | undefined | null;
  onChange: (value: string) => void;
  placeholder?: string;
  rows?: number;
  iconColor?: string;
  iconPath: string;
  /** Set when the parent portal mirrors this field (see parent-visible-fields). */
  parentVisibleHint?: string;
}>) {
  return (
    <div className="bg-moto-blue-soft/30 rounded-xl border border-gray-100 p-3 md:p-4">
      <h3 className="mb-3 flex flex-wrap items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <svg
          className={`h-3.5 w-3.5 text-${iconColor} md:h-4 md:w-4`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d={iconPath}
          />
        </svg>
        {label}
        {parentVisibleHint && <ParentVisibleBadge hint={parentVisibleHint} />}
      </h3>
      <textarea
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        rows={rows}
        className="moto-content-surface focus:border-moto-blue focus:ring-moto-blue block w-full resize-none rounded-lg border px-3 py-2 text-xs transition-colors focus:ring-1 md:text-sm"
        placeholder={placeholder}
        maxLength={2000}
      />
    </div>
  );
}

/**
 * Health Information Section
 */
export function HealthInfoSection({
  value,
  onChange,
}: Readonly<{
  value: string | undefined | null;
  onChange: (value: string) => void;
}>) {
  return (
    <TextareaField
      label="Gesundheitsinformationen"
      value={value}
      onChange={onChange}
      placeholder="Allergien, Medikamente, gesundheitliche Einschränkungen..."
      parentVisibleHint={PARENT_VISIBLE_HINTS.healthInfo}
      iconColor="red-600"
      iconPath="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
    />
  );
}

export function AdditionalInfoSection({
  value,
  onChange,
}: Readonly<{
  value: string | undefined | null;
  onChange: (value: string) => void;
}>) {
  return (
    <TextareaField
      label="Elternnotizen"
      value={value}
      onChange={onChange}
      placeholder="Weitere Informationen über das Kind..."
      iconColor="blue-600"
      iconPath="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
    />
  );
}

/**
 * Privacy & Data Retention Section
 */
export function PrivacyConsentSection({
  formData,
  onChange,
  fieldError,
}: Readonly<{
  formData: Partial<Student>;
  onChange: (
    field: keyof Student,
    value: string | boolean | number | BusDays | null,
  ) => void;
  /** Hint per API field name, from the form's shared error path (#2513). */
  fieldError: (name: string) => string | undefined;
}>) {
  const retentionError = fieldError("data_retention_days");
  return (
    <div className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.permissions.icon}
          tone={MOTO_CONCEPTS.permissions.tone}
          size={18}
        />
        Datenschutz
      </h3>
      <div className="space-y-4">
        <label
          htmlFor="privacy-consent-accepted"
          className="group flex cursor-pointer items-start gap-3"
        >
          <Checkbox
            id="privacy-consent-accepted"
            checked={formData.privacy_consent_accepted ?? false}
            onChange={(e) =>
              onChange("privacy_consent_accepted", e.target.checked)
            }
          />
          <span className="text-sm text-gray-700 transition-colors group-hover:text-gray-900">
            Einwilligung zur Datenverarbeitung erteilt
          </span>
        </label>
        <div>
          <label
            htmlFor="data-retention-days"
            className="mb-1 block text-xs font-medium text-gray-700"
          >
            Aufbewahrungsdauer (Tage)
          </label>
          <Input
            id="data-retention-days"
            controlSize="compact"
            name="data_retention_days"
            type="number"
            min="1"
            max="31"
            aria-invalid={retentionError ? true : undefined}
            aria-describedby={
              retentionError ? "data-retention-days-error" : undefined
            }
            value={formData.data_retention_days ?? ""}
            onChange={(e) => {
              const inputValue = e.target.value;
              if (inputValue === "") {
                onChange("data_retention_days", null);
              } else {
                const v = Number.parseInt(inputValue, 10);
                onChange("data_retention_days", Number.isNaN(v) ? null : v);
              }
            }}
            placeholder="30"
          />
          {retentionError && (
            <p
              id="data-retention-days-error"
              className="text-moto-red mt-1 text-xs"
            >
              {retentionError}
            </p>
          )}
          <p className="mt-1 text-xs text-gray-500">
            Daten werden nach dieser Zeit automatisch gelöscht (1-31 Tage)
          </p>
        </div>
      </div>
    </div>
  );
}

/**
 * Departure Section — per-weekday allowed way-home selector. Multiple options
 * per day are valid because the OGS can allow parents to keep flexible pickup,
 * bus and walking arrangements.
 */
const DEPARTURE_OPTIONS: ReadonlyArray<{
  value: DepartureMode;
  short: string;
}> = [
  { value: "alone", short: "Zu Fuß" },
  { value: "bus", short: "Bus" },
  { value: "pickup", short: "Abgeholt" },
  { value: "accompanied", short: "Anderes Kind" },
];

// Stable empty default so the prop keeps referential equality across renders.
const EMPTY_COMPANIONS: StudentCompanion[] = [];

/** Weekdays whose allowed modes include "Anderes Kind" — the only days a
 *  Laufgemeinschaft may be recorded for. */
function accompaniedWeekdays(
  allowed: AllowedDepartureModes,
): CompanionWeekday[] {
  return DEPARTURE_WEEKDAYS.filter((day) =>
    allowed[day.key]?.includes("accompanied"),
  ).map((day) => day.key as CompanionWeekday);
}

export function DepartureSection({
  days,
  onChange,
  companionNote,
  onCompanionNoteChange,
  companionNoteError,
  companions,
  onCompanionsChange,
  companionStudentId,
  onCompanionExtensionConfirmed,
}: Readonly<{
  days?: AllowedDepartureModes | null;
  onChange: (value: AllowedDepartureModes) => void;
  /** Free-text "mit wem" for the accompanied mode (#1694). Optional: when the
   *  handler is omitted the note input is not rendered. */
  companionNote?: string | null;
  onCompanionNoteChange?: (value: string) => void;
  /** Validation message shown under the companion note (required when an
   *  accompanied day is selected). */
  companionNoteError?: string;
  /** Linked children (Laufgemeinschaft). Rendered only when the handler is
   *  supplied AND a day allows the accompanied mode — the link exists only as
   *  the detail of that mode. */
  companions?: StudentCompanion[];
  onCompanionsChange?: (next: StudentCompanion[]) => void;
  /** The child being edited, filtered out of the companion search. */
  companionStudentId?: string;
  /** The user confirmed widening a linked child's own departure plan, for the
   *  named child and weekdays. */
  onCompanionExtensionConfirmed?: (
    confirmation: CompanionExtensionConfirmation,
  ) => void;
}>) {
  const normalized = normalizeAllowedDepartureModes(days);
  const anySelected = DEPARTURE_WEEKDAYS.some(
    (day) => (normalized[day.key]?.length ?? 0) > 0,
  );
  const accompaniedSelected = DEPARTURE_WEEKDAYS.some((day) =>
    normalized[day.key]?.includes("accompanied"),
  );
  const companionNoteId = useId();
  return (
    <div className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4">
      <div className="mb-3 flex items-start justify-between gap-3 md:mb-4">
        <h3 className="flex min-w-0 flex-wrap items-center gap-2 text-xs font-semibold text-gray-900 md:text-sm">
          <Bus
            className="h-3.5 w-3.5 shrink-0 md:h-4 md:w-4"
            style={{ color: GROUP_ROOM_SHADES.text }}
          />
          Erlaubte Heimwege
          <ParentVisibleBadge hint={PARENT_VISIBLE_HINTS.departure} />
        </h3>
        {anySelected && (
          <span className="bg-moto-green-soft text-moto-green-strong hidden shrink-0 rounded-full px-2.5 py-1 text-xs font-medium sm:inline">
            {formatAllowedDepartureDays(normalized)}
          </span>
        )}
      </div>
      <p className="mb-3 text-xs text-gray-500">
        Pro Wochentag alle Heimwege auswählen, die für das Kind erlaubt sind.
      </p>
      <div className="space-y-2">
        {DEPARTURE_WEEKDAYS.map((day) => {
          return (
            <div key={day.key} className="flex items-center gap-2">
              <span className="w-16 shrink-0 text-xs font-semibold text-gray-700 md:w-20">
                {day.label}
              </span>
              <div className="grid flex-1 grid-cols-2 gap-1">
                {DEPARTURE_OPTIONS.map((opt) => {
                  const active =
                    normalized[day.key]?.includes(opt.value) ?? false;
                  return (
                    <label
                      key={opt.value}
                      className={`flex h-8 cursor-pointer items-center gap-2 rounded-lg border px-2.5 text-xs font-semibold transition-colors ${
                        active
                          ? "border-gray-900 bg-white text-gray-900"
                          : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                      }`}
                    >
                      <Checkbox
                        checked={active}
                        aria-label={`${day.label}: ${opt.short}`}
                        onChange={() => {
                          const current = new Set(normalized[day.key] ?? []);
                          if (current.has(opt.value)) current.delete(opt.value);
                          else current.add(opt.value);
                          const ordered = DEPARTURE_OPTIONS.map(
                            (option) => option.value,
                          ).filter((mode) => current.has(mode));
                          const next = { ...normalized };
                          if (ordered.length === 0) delete next[day.key];
                          else next[day.key] = ordered;
                          onChange(next);
                          // Drop the orphaned companion note when the last
                          // accompanied day is removed, so a now-hidden stale
                          // note is never submitted unchanged (#1694).
                          const stillAccompanied = DEPARTURE_WEEKDAYS.some(
                            (d) => next[d.key]?.includes("accompanied"),
                          );
                          if (
                            !stillAccompanied &&
                            onCompanionNoteChange &&
                            (companionNote ?? "") !== ""
                          ) {
                            onCompanionNoteChange("");
                          }
                          // Same for the links, and for a second reason: the
                          // picker is hidden from here on, so a list left
                          // behind would travel with the save while the user
                          // can no longer see or edit it. The backend refuses
                          // a non-empty list on a plan that allows the mode on
                          // no day at all — an error about a control that is
                          // not on screen. Clearing it here submits what the
                          // plan already says: no Laufgemeinschaft.
                          if (
                            !stillAccompanied &&
                            onCompanionsChange &&
                            (companions?.length ?? 0) > 0
                          ) {
                            onCompanionsChange([]);
                          }
                        }}
                      />
                      {opt.short}
                    </label>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
      {onCompanionsChange && accompaniedSelected && (
        <div className="mt-3">
          <CompanionPicker
            value={companions ?? EMPTY_COMPANIONS}
            onChange={onCompanionsChange}
            allowedDays={accompaniedWeekdays(normalized)}
            excludeStudentId={companionStudentId}
            onExtensionConfirmed={onCompanionExtensionConfirmed}
          />
        </div>
      )}
      {onCompanionNoteChange && accompaniedSelected && (
        <div className="mt-3">
          <label
            htmlFor={companionNoteId}
            className="mb-1 block text-xs font-medium text-gray-700"
          >
            {onCompanionsChange
              ? "Oder mit welcher Person?"
              : "Mit welchem Kind?"}
          </label>
          <Input
            id={companionNoteId}
            controlSize="compact"
            name="departure_companion_note"
            type="text"
            value={companionNote ?? ""}
            onChange={(e) => onCompanionNoteChange(e.target.value)}
            placeholder="z. B. Geschwisterkind, Freund, Name"
            maxLength={255}
            aria-invalid={companionNoteError ? true : undefined}
            aria-describedby={
              companionNoteError ? `${companionNoteId}-error` : undefined
            }
          />
          {companionNoteError ? (
            <p
              id={`${companionNoteId}-error`}
              className="text-moto-red mt-1 text-xs"
            >
              {companionNoteError}
            </p>
          ) : (
            <p className="mt-1 text-xs text-gray-500">
              {onCompanionsChange
                ? "Nur nötig, wenn das Kind mit einer Person geht, die kein Kind der Schule ist (z. B. Nachbarin)."
                : "Mit welchem Kind das Kind an den Tagen „Mit anderem Kind“ nach Hause geht."}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
