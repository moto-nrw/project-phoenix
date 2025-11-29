/**
 * Reusable form field components for student forms
 * Extracted to reduce code duplication between create and edit modals
 */

import type { Student } from "@/lib/api";

interface SelectOption {
  value: string;
  label: string;
}

/**
 * Personal Information Section
 */
export function PersonalInfoSection({
  formData,
  onChange,
  errors,
  groups = [],
  requiredFields = { firstName: true, lastName: true, schoolClass: true },
}: Readonly<{
  formData: Partial<Student>;
  onChange: (field: keyof Student, value: string | boolean | number | null) => void;
  errors: Record<string, string>;
  groups?: SelectOption[];
  requiredFields?: { firstName?: boolean; lastName?: boolean; schoolClass?: boolean };
}>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <svg
          className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
          />
        </svg>
        Persönliche Daten
      </h3>
      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
        <TextInput
          label="Vorname"
          value={formData.first_name ?? ""}
          onChange={(v) => onChange("first_name", v)}
          error={errors.first_name}
          required={requiredFields.firstName}
          placeholder="Max"
        />
        <TextInput
          label="Nachname"
          value={formData.second_name ?? ""}
          onChange={(v) => onChange("second_name", v)}
          error={errors.second_name}
          required={requiredFields.lastName}
          placeholder="Mustermann"
        />
        <TextInput
          label="Klasse"
          value={formData.school_class ?? ""}
          onChange={(v) => onChange("school_class", v)}
          error={errors.school_class}
          required={requiredFields.schoolClass}
          placeholder="5A"
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
  error,
  required = false,
  placeholder = "",
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  error?: string;
  required?: boolean;
  placeholder?: string;
}>) {
  return (
    <div>
      <label className="mb-1 block text-xs font-medium text-gray-700">
        {label} {required && <span className="text-red-500">*</span>}
      </label>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
          error
            ? "border-red-300 bg-red-50"
            : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
        }`}
        placeholder={placeholder}
      />
      {error && <p className="mt-1 text-xs text-red-600">{error}</p>}
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
  return (
    <div>
      <label className="mb-1 block text-xs font-medium text-gray-700">{label}</label>
      <div className="relative">
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="block w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
        >
          {options.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-gray-500">
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </div>
      </div>
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
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
}>) {
  return (
    <div>
      <label className="mb-1 block text-xs font-medium text-gray-700">{label}</label>
      <input
        type="date"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
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
}: Readonly<{
  label: string;
  value: string | undefined | null;
  onChange: (value: string) => void;
  placeholder?: string;
  rows?: number;
  iconColor?: string;
  iconPath: string;
}>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
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
      </h3>
      <textarea
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        rows={rows}
        className="block w-full resize-none rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] md:text-sm"
        placeholder={placeholder}
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
      iconColor="red-600"
      iconPath="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
    />
  );
}

/**
 * Supervisor Notes Section
 */
export function SupervisorNotesSection({
  value,
  onChange,
}: Readonly<{
  value: string | undefined | null;
  onChange: (value: string) => void;
}>) {
  return (
    <TextareaField
      label="Betreuernotizen"
      value={value}
      onChange={onChange}
      placeholder="Interne Notizen für Betreuer..."
      iconColor="amber-600"
      iconPath="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
    />
  );
}

/**
 * Additional Info Section
 */
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
      placeholder="Weitere Informationen über den Schüler..."
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
  errors,
}: Readonly<{
  formData: Partial<Student>;
  onChange: (field: keyof Student, value: string | boolean | number | null) => void;
  errors: Record<string, string>;
}>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <svg
          className="h-3.5 w-3.5 text-gray-600 md:h-4 md:w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
          />
        </svg>
        Datenschutz
      </h3>
      <div className="space-y-4">
        <label className="group flex cursor-pointer items-start gap-3">
          <input
            type="checkbox"
            checked={formData.privacy_consent_accepted ?? false}
            onChange={(e) => onChange("privacy_consent_accepted", e.target.checked)}
            className="mt-0.5 h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
          />
          <span className="text-sm text-gray-700 transition-colors group-hover:text-gray-900">
            Einwilligung zur Datenverarbeitung erteilt
          </span>
        </label>
        <div>
          <label htmlFor="data-retention-days" className="mb-1 block text-xs font-medium text-gray-700">
            Aufbewahrungsdauer (Tage)
          </label>
          <input
            id="data-retention-days"
            type="number"
            min="1"
            max="31"
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
            className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
              errors.data_retention_days
                ? "border-red-300 bg-red-50"
                : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
            }`}
            placeholder="30"
          />
          {errors.data_retention_days && (
            <p className="mt-1 text-xs text-red-600">{errors.data_retention_days}</p>
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
 * Bus Status Section
 */
export function BusStatusSection({
  value,
  onChange,
}: Readonly<{
  value: boolean | undefined | null;
  onChange: (value: boolean) => void;
}>) {
  return (
    <div className="rounded-xl border border-orange-200 bg-orange-50 p-4">
      <label htmlFor="bus-status" className="group flex cursor-pointer items-center gap-3">
        <input
          id="bus-status"
          type="checkbox"
          checked={value ?? false}
          onChange={(e) => onChange(e.target.checked)}
          className="h-4 w-4 rounded border-gray-300 text-orange-600 focus:ring-orange-600"
          aria-label="Fährt mit dem Bus"
        />
        <div className="flex items-center gap-2" aria-hidden="true">
          <svg
            className="h-5 w-5 text-orange-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
            />
          </svg>
          <span className="text-sm font-medium text-orange-900">Fährt mit dem Bus</span>
        </div>
      </label>
    </div>
  );
}

/**
 * Pickup Status Section
 */
export function PickupStatusSection({
  value,
  onChange,
}: Readonly<{
  value: string | undefined | null;
  onChange: (value: string | null) => void;
}>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
        <svg
          className="h-3.5 w-3.5 text-green-600 md:h-4 md:w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"
          />
        </svg>
        Abholstatus
      </h3>
      <div className="relative">
        <select
          value={value ?? ""}
          onChange={(e) => onChange(e.target.value || null)}
          className="block w-full appearance-none rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm transition-colors focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
        >
          <option value="">Nicht gesetzt</option>
          <option value="Geht alleine nach Hause">Geht alleine nach Hause</option>
          <option value="Wird abgeholt">Wird abgeholt</option>
        </select>
        <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-gray-500">
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </div>
      </div>
    </div>
  );
}
