"use client";

import { useState, useEffect, useRef } from "react";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { createLogger } from "~/lib/logger";
import { getDefaultMaxLength } from "~/lib/constants/input-limits";
import useSWR from "swr";

const logger = createLogger({ component: "DatabaseForm" });

/** Privacy consent data structure */
interface PrivacyConsent {
  accepted: boolean;
  data_retention_days: number;
}

/** Gets default value for a form field based on its type */
export function getDefaultValueForField(field: FormField): unknown {
  switch (field.type) {
    case "checkbox":
      return false;
    case "multiselect":
      return [];
    case "number":
      if (field.name === "data_retention_days") return 30;
      return field.required ? 0 : "";
    default:
      return "";
  }
}

/** Checks if sections contain privacy consent fields */
export function hasPrivacyConsentFields(sections: FormSection[]): boolean {
  return sections.some((s) =>
    s.fields.some(
      (f) =>
        f.name === "privacy_consent_accepted" ||
        f.name === "data_retention_days",
    ),
  );
}

/** Extracts privacy consent from API response */
export function extractPrivacyConsent(
  responseData: unknown,
): PrivacyConsent | null {
  if (!responseData || typeof responseData !== "object") {
    return null;
  }

  if (!("data" in responseData)) {
    return null;
  }

  const consent = (responseData as { data: unknown }).data;
  if (
    consent &&
    typeof consent === "object" &&
    "accepted" in consent &&
    "data_retention_days" in consent
  ) {
    return consent as PrivacyConsent;
  }

  return null;
}

/** Fetches privacy consent for a student */
async function fetchPrivacyConsentForStudent(
  studentId: string,
): Promise<PrivacyConsent | null> {
  try {
    const response = await fetch(`/api/students/${studentId}/privacy-consent`);
    if (!response.ok) {
      return null;
    }
    const responseData = (await response.json()) as unknown;
    return extractPrivacyConsent(responseData);
  } catch (error) {
    logger.error("failed to fetch privacy consent", {
      error: error instanceof Error ? error.message : String(error),
    });
    return null;
  }
}

/** Applies initial data values to form, converting types as needed */
function applyInitialData<T>(
  formData: Record<string, unknown>,
  initialData: Partial<T>,
  sections: FormSection[],
): void {
  const allFields = sections.flatMap((s) => s.fields);

  for (const key of Object.keys(initialData)) {
    const value = initialData[key as keyof T];
    if (value === undefined || value === null) {
      continue;
    }

    // Convert string to number if field type requires it
    const field = allFields.find((f) => f.name === key);
    if (field?.type === "number" && typeof value === "string") {
      formData[key] = Number(value) || 0;
    } else {
      formData[key] = value;
    }
  }
}

/** Checks if a value is empty (undefined, null, or empty string) */
export function isEmptyValue(value: unknown): boolean {
  return value === undefined || value === null || value === "";
}

/** Validates a number field against min constraint */
export function validateNumberMin(
  value: unknown,
  min: number,
  label: string,
): string | null {
  const numValue = typeof value === "number" ? value : Number(value);
  if (Number.isNaN(numValue) || numValue < min) {
    return `${label} muss mindestens ${min} sein.`;
  }
  return null;
}

/** Validates a number field against max constraint */
export function validateNumberMax(
  value: unknown,
  max: number,
  label: string,
): string | null {
  const numValue = typeof value === "number" ? value : Number(value);
  if (Number.isNaN(numValue) || numValue > max) {
    return `${label} darf höchstens ${max} sein.`;
  }
  return null;
}

/** Validates a single form field and returns error message or null */
export function validateField(field: FormField, value: unknown): string | null {
  // Check required fields
  if (field.required && isEmptyValue(value)) {
    return `${field.label} ist erforderlich.`;
  }

  // Optional number fields stay valid while empty, but their constraints apply
  // as soon as the user enters a value.
  if (field.type === "number" && !isEmptyValue(value)) {
    const numericValue = typeof value === "number" ? value : Number(value);
    if (!Number.isInteger(numericValue)) {
      return `${field.label} muss eine ganze Zahl sein.`;
    }
    if (field.min !== undefined) {
      const minError = validateNumberMin(value, field.min, field.label);
      if (minError) return minError;
    }
    if (field.max !== undefined) {
      const maxError = validateNumberMax(value, field.max, field.label);
      if (maxError) return maxError;
    }
  }

  // Custom validation
  if (field.validation) {
    return field.validation(value) ?? null;
  }

  return null;
}

/** Validates all form fields and returns first error with field name, or null */
export function validateFormFields(
  sections: FormSection[],
  formData: Record<string, unknown>,
): { message: string; fieldName: string } | null {
  for (const section of sections) {
    for (const field of section.fields) {
      const error = validateField(field, formData[field.name]);
      if (error) return { message: error, fieldName: field.name };
    }
  }
  return null;
}

export interface FormField {
  name: string;
  label: string;
  type:
    | "text"
    | "email"
    | "select"
    | "multiselect"
    | "textarea"
    | "password"
    | "checkbox"
    | "custom"
    | "number"
    | "date";
  required?: boolean;
  placeholder?: string;
  options?:
    | Array<{ value: string; label: string }>
    | (() => Promise<Array<{ value: string; label: string }>>);
  validation?: (value: unknown) => string | null;
  component?: React.ComponentType<{
    value: unknown;
    onChange: (value: unknown) => void;
    label: string;
    required?: boolean;
    includeEmpty?: boolean;
    emptyLabel?: string;
  }>;
  helperText?: string;
  autoComplete?: string;
  colSpan?: 1 | 2;
  min?: number;
  max?: number;
  maxLength?: number;
  /** When true, the field is rendered as read-only and cannot be edited. */
  disabled?: boolean;
}

export interface FormSection {
  title: string;
  subtitle?: string;
  fields: FormField[];
  columns?: 1 | 2;
  /** Optional concept driving the section's header icon (gray tile, MotoConceptIcon). */
  concept?: MotoConceptKey;
}

/** Konzeptloser Abschnittstitel, gleiche Ebenenlogik wie ConceptSectionHeader. */
function SectionTitle({
  level,
  className,
  children,
}: Readonly<{
  level: 2 | 3 | 4;
  className?: string;
  children: React.ReactNode;
}>) {
  const Heading = `h${level}` as const;
  return <Heading className={className}>{children}</Heading>;
}

interface DatabaseFormProps<T = Record<string, unknown>> {
  readonly sections: FormSection[];
  readonly onSubmit: (data: T) => Promise<void>;
  readonly onCancel: () => void;
  readonly initialData?: Partial<T>;
  readonly isLoading?: boolean;
  readonly error?: string | null;
  readonly submitLabel: string;
  readonly stickyActions?: boolean; // Render sticky action bar like other entity forms
  /**
   * Ueberschriftenebene der Abschnittstitel. Default 2 fuer die
   * Master-Detail-Ansicht, wo das Formular direkt im Inhaltsbereich steht.
   * DatabaseFormModal setzt 4, weil Modal seinen Titel als h3 rendert und die
   * Abschnitte sonst ueber dem Dialog stehen, in dem sie liegen.
   */
  readonly sectionLevel?: 2 | 3 | 4;
}

export function DatabaseForm<T = Record<string, unknown>>({
  sections,
  onSubmit,
  onCancel,
  initialData,
  isLoading,
  error: externalError,
  submitLabel,
  stickyActions = false,
  sectionLevel = 2,
}: DatabaseFormProps<T>) {
  const privacyStudentId =
    initialData &&
    "id" in initialData &&
    typeof initialData.id === "string" &&
    hasPrivacyConsentFields(sections)
      ? initialData.id
      : null;
  const { data: privacyConsent } = useSWR(
    privacyStudentId
      ? `/api/students/${privacyStudentId}/privacy-consent`
      : null,
    () => fetchPrivacyConsentForStudent(privacyStudentId!),
  );
  const [formData, setFormData] = useState<Record<string, unknown>>({});
  const [error, setError] = useState<string | null>(null);
  const [errorFieldName, setErrorFieldName] = useState<string | null>(null);
  const errorRef = useScrollToError(error);
  // Local submitting state to prevent double-clicks (set synchronously before async onSubmit)
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [asyncOptions, setAsyncOptions] = useState<
    Record<string, Array<{ value: string; label: string }>>
  >({});
  const [loadingOptions, setLoadingOptions] = useState<Record<string, boolean>>(
    {},
  );
  const loadedFieldsRef = useRef<Set<string>>(new Set());
  const dirtyPrivacyFieldsRef = useRef<Set<string>>(new Set());
  // Track mount state to avoid setState on unmounted component
  const isMountedRef = useRef(true);

  // Track unmount
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  // Initialize form data from sections
  useEffect(() => {
    const initialFormData: Record<string, unknown> = {};

    // Set defaults from sections using helper
    for (const section of sections) {
      for (const field of section.fields) {
        initialFormData[field.name] = getDefaultValueForField(field);
      }
    }

    if (initialData) {
      applyInitialData(initialFormData, initialData, sections);
    }

    dirtyPrivacyFieldsRef.current.clear();
    setFormData(initialFormData);
  }, [initialData, sections]);

  // Apply separately fetched consent without resetting unrelated form edits.
  // Preserve consent fields too once the user has changed them locally.
  useEffect(() => {
    if (!privacyConsent) return;

    setFormData((currentFormData) => {
      const nextFormData = { ...currentFormData };
      if (!dirtyPrivacyFieldsRef.current.has("privacy_consent_accepted")) {
        nextFormData.privacy_consent_accepted = privacyConsent.accepted;
      }
      if (!dirtyPrivacyFieldsRef.current.has("data_retention_days")) {
        nextFormData.data_retention_days = privacyConsent.data_retention_days;
      }
      return nextFormData;
    });
  }, [privacyConsent]);

  // Load async options for select fields
  useEffect(() => {
    const loadAsyncOptions = async () => {
      for (const section of sections) {
        for (const field of section.fields) {
          if (
            (field.type === "select" || field.type === "multiselect") &&
            typeof field.options === "function"
          ) {
            // Skip if already loaded
            if (loadedFieldsRef.current.has(field.name)) {
              continue;
            }

            loadedFieldsRef.current.add(field.name);
            setLoadingOptions((prev) => ({ ...prev, [field.name]: true }));
            try {
              const options = await field.options();
              setAsyncOptions((prev) => ({ ...prev, [field.name]: options }));
            } catch (error) {
              logger.error("failed to load field options", {
                field: field.name,
                error: error instanceof Error ? error.message : String(error),
              });
              setAsyncOptions((prev) => ({ ...prev, [field.name]: [] }));
            } finally {
              setLoadingOptions((prev) => ({ ...prev, [field.name]: false }));
            }
          }
        }
      }
    };

    void loadAsyncOptions();
  }, [sections]);

  const handleChange = (
    e: React.ChangeEvent<
      HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
    >,
  ) => {
    const { name, value, type } = e.target as HTMLInputElement;
    if (name === "privacy_consent_accepted" || name === "data_retention_days") {
      dirtyPrivacyFieldsRef.current.add(name);
    }

    if (type === "checkbox") {
      const { checked } = e.target as HTMLInputElement;
      setFormData((prev) => ({
        ...prev,
        [name]: checked,
      }));
    } else if (type === "number") {
      // Allow empty string during editing for better UX
      // Will be converted to number on submit
      if (value === "") {
        setFormData((prev) => ({
          ...prev,
          [name]: "",
        }));
      } else {
        const numValue = Number(value);
        setFormData((prev) => ({
          ...prev,
          [name]: Number.isNaN(numValue) ? "" : numValue,
        }));
      }
    } else {
      setFormData((prev) => ({
        ...prev,
        [name]: value,
      }));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // Prevent double-submit: check and set synchronously before any async work
    if (isSubmitting || isLoading) {
      return;
    }
    setIsSubmitting(true);
    setError(null);
    setErrorFieldName(null);

    // Validate all form fields
    const validationResult = validateFormFields(sections, formData);
    if (validationResult) {
      setError(validationResult.message);
      setErrorFieldName(validationResult.fieldName);
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
      return;
    }

    const submitData = { ...formData };
    // The student proxy writes the Datenschutz pair BEFORE the student PUT, so
    // whatever this payload carries is stored even when the rest of the save is
    // refused. While editing, the two fields are echoes of the separately
    // fetched server consent unless the user touched them — resubmitting that
    // copy would overwrite a change somebody else made since it was fetched.
    // Both go or both stay: the proxy's consent PUT upserts the pair, so a lone
    // field resets the other to its default. On create there is nothing to echo
    // (no privacyStudentId), and the values travel as before.
    if (
      privacyStudentId &&
      !dirtyPrivacyFieldsRef.current.has("privacy_consent_accepted") &&
      !dirtyPrivacyFieldsRef.current.has("data_retention_days")
    ) {
      delete submitData.privacy_consent_accepted;
      delete submitData.data_retention_days;
    }

    try {
      await onSubmit(submitData as T);
    } catch (err) {
      logger.error("failed to submit form", {
        error: err instanceof Error ? err.message : String(err),
      });
      const errorMessage =
        err instanceof Error
          ? err.message
          : "Fehler beim Speichern der Daten. Bitte versuchen Sie es später erneut.";
      setError(errorMessage);
    } finally {
      if (isMountedRef.current) {
        setIsSubmitting(false);
      }
    }
  };

  // Handler for removing a value from a multiselect field
  const handleMultiselectRemove = (
    fieldName: string,
    currentValues: string[],
    valueToRemove: string,
  ) => {
    setFormData((prev) => ({
      ...prev,
      [fieldName]: currentValues.filter((v) => v !== valueToRemove),
    }));
  };

  const renderSelectField = (
    field: FormField,
    hasError: boolean,
    labelClasses: string,
  ) => {
    const selectOptions = Array.isArray(field.options)
      ? field.options
      : (asyncOptions[field.name] ?? []);
    const hasEmptyOption = selectOptions.some((option) => option.value === "");

    return (
      <div>
        <label htmlFor={field.name} className={labelClasses}>
          {field.label}
          {field.required && "*"}
        </label>
        <CustomSelect
          id={field.name}
          name={field.name}
          ariaLabel={field.label}
          value={(formData[field.name] as string) ?? ""}
          options={[
            ...(!hasEmptyOption
              ? [
                  {
                    value: "",
                    label: loadingOptions[field.name]
                      ? "Optionen werden geladen..."
                      : (field.placeholder ?? "Bitte wählen"),
                  },
                ]
              : []),
            ...selectOptions,
          ]}
          onChange={(next) => {
            if (field.name === "data_retention_days") {
              dirtyPrivacyFieldsRef.current.add(field.name);
            }
            setFormData((prev) => ({ ...prev, [field.name]: next }));
          }}
          required={field.required}
          invalid={hasError}
          disabled={loadingOptions[field.name]}
        />
        {field.helperText && (
          <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
        )}
      </div>
    );
  };

  const renderMultiselectField = (
    field: FormField,
    hasError: boolean,
    labelClasses: string,
  ) => {
    const multiselectOptions = Array.isArray(field.options)
      ? field.options
      : (asyncOptions[field.name] ?? []);
    const selectedValues = Array.isArray(formData[field.name])
      ? (formData[field.name] as string[])
      : [];

    return (
      <div>
        <label htmlFor={field.name} className={labelClasses}>
          {field.label}
          {field.required && "*"}
        </label>

        {selectedValues.length > 0 && (
          <div className="mb-2 flex flex-wrap gap-1.5">
            {selectedValues.map((value) => {
              const option = multiselectOptions.find(
                (item) => item.value === value,
              );
              if (!option) return null;

              return (
                <span
                  key={value}
                  className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-medium text-blue-800"
                >
                  {option.label}
                  <button
                    type="button"
                    onClick={() =>
                      handleMultiselectRemove(field.name, selectedValues, value)
                    }
                    className="ml-1 inline-flex h-3.5 w-3.5 items-center justify-center rounded-full bg-blue-200 text-blue-600 hover:bg-blue-300 hover:text-blue-700"
                    aria-label={`Remove ${option.label}`}
                  >
                    ×
                  </button>
                </span>
              );
            })}
          </div>
        )}

        <CustomSelect
          id={field.name}
          ariaLabel={field.label}
          value=""
          placeholder={
            loadingOptions[field.name]
              ? "Optionen werden geladen..."
              : (field.placeholder ?? "Weitere hinzufügen...")
          }
          options={multiselectOptions
            .filter((option) => !selectedValues.includes(option.value))
            .map((option) => ({
              value: option.value,
              label: option.label,
            }))}
          onChange={(next) => {
            if (next && !selectedValues.includes(next)) {
              setFormData((prev) => ({
                ...prev,
                [field.name]: [...selectedValues, next],
              }));
            }
          }}
          invalid={hasError}
          disabled={loadingOptions[field.name]}
        />

        {field.helperText && (
          <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
        )}
      </div>
    );
  };

  const renderField = (field: FormField) => {
    const hasError = field.name === errorFieldName;

    const baseInputClasses = `w-full rounded-lg border ${hasError ? "border-moto-red/40" : "border-gray-300"} px-3 py-2 md:px-4 md:py-2 text-sm transition-all duration-200 focus:ring-2 focus:ring-moto-blue focus:outline-none`;
    const labelClasses = `mb-1.5 block text-xs font-medium ${hasError ? "text-red-600" : "text-gray-700"}`;

    switch (field.type) {
      case "custom": {
        if (!field.component) return null;
        const Component = field.component;
        return (
          <Component
            value={formData[field.name]}
            onChange={(value: unknown) => {
              setFormData((prev) => ({
                ...prev,
                [field.name]: value,
              }));
            }}
            label={field.label}
            required={field.required}
            includeEmpty={true}
            emptyLabel={field.placeholder}
          />
        );
      }

      case "checkbox":
        return (
          <div className="flex items-center">
            <input
              type="checkbox"
              id={field.name}
              name={field.name}
              checked={Boolean(formData[field.name])}
              onChange={handleChange}
              className="text-moto-blue focus:ring-moto-blue h-4 w-4 rounded border-gray-300"
            />
            <label
              htmlFor={field.name}
              className={`ml-2 block text-xs md:text-sm ${hasError ? "text-red-600" : "text-gray-700"}`}
            >
              {field.label}
            </label>
            {field.helperText && (
              <p className="ml-2 text-xs text-gray-500 md:text-sm">
                {field.helperText}
              </p>
            )}
          </div>
        );

      case "textarea":
        return (
          <div>
            <label htmlFor={field.name} className={labelClasses}>
              {field.label}
              {field.required && "*"}
            </label>
            <textarea
              id={field.name}
              name={field.name}
              value={(formData[field.name] as string) ?? ""}
              onChange={handleChange}
              required={field.required}
              placeholder={field.placeholder}
              maxLength={field.maxLength ?? getDefaultMaxLength(field.type)}
              rows={3}
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      case "select":
        return renderSelectField(field, hasError, labelClasses);

      case "multiselect":
        return renderMultiselectField(field, hasError, labelClasses);

      case "number": {
        // Handle both number and empty string values
        const numberValue = formData[field.name] as
          string | number | undefined | null;
        const displayValue =
          numberValue === "" ||
          numberValue === undefined ||
          numberValue === null
            ? ""
            : String(numberValue);

        return (
          <div>
            <label htmlFor={field.name} className={labelClasses}>
              {field.label}
              {field.required && "*"}
            </label>
            <input
              type="number"
              id={field.name}
              name={field.name}
              value={displayValue}
              onChange={handleChange}
              required={field.required}
              placeholder={field.placeholder}
              min={field.min}
              max={field.max}
              aria-invalid={hasError}
              aria-describedby={
                field.helperText ? `${field.name}-helper` : undefined
              }
              className={baseInputClasses}
            />
            {field.helperText && (
              <p
                id={`${field.name}-helper`}
                className="mt-1 text-xs text-gray-500"
              >
                {field.helperText}
              </p>
            )}
          </div>
        );
      }

      default:
        return (
          <div>
            <label htmlFor={field.name} className={labelClasses}>
              {field.label}
              {field.required && "*"}
            </label>
            <input
              type={field.type}
              id={field.name}
              name={field.name}
              value={(formData[field.name] as string) ?? ""}
              onChange={handleChange}
              required={field.required}
              placeholder={field.placeholder}
              autoComplete={field.autoComplete}
              maxLength={field.maxLength ?? getDefaultMaxLength(field.type)}
              disabled={field.disabled}
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );
    }
  };

  return (
    <>
      {(error ?? externalError) && (
        <div ref={errorRef} className="mb-4 md:mb-6">
          <Alert type="error" message={error ?? externalError ?? ""} />
        </div>
      )}

      <form onSubmit={handleSubmit} noValidate className="space-y-6">
        {sections.map((section) => {
          return (
            <div
              key={section.title}
              className="mb-6 rounded-lg bg-gray-50 p-3 md:mb-8 md:p-4"
            >
              {section.concept ? (
                <ConceptSectionHeader
                  className="mb-2.5 md:mb-3"
                  level={sectionLevel}
                  title={section.title}
                  concept={section.concept}
                  subtitle={section.subtitle}
                />
              ) : (
                <>
                  <SectionTitle
                    level={sectionLevel}
                    className="mb-2.5 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm"
                  >
                    {section.title}
                  </SectionTitle>
                  {section.subtitle && (
                    <p className="mb-2.5 text-xs text-gray-600 md:mb-3">
                      {section.subtitle}
                    </p>
                  )}
                </>
              )}
              <div
                className={`grid grid-cols-1 gap-3 md:gap-4 ${section.columns === 2 ? "md:grid-cols-2" : ""}`}
              >
                {section.fields.map((field) => (
                  <div
                    key={field.name}
                    className={
                      field.colSpan === 2 && section.columns === 2
                        ? "md:col-span-2"
                        : ""
                    }
                  >
                    {renderField(field)}
                  </div>
                ))}
              </div>
            </div>
          );
        })}

        {/* Form actions */}
        {(() => {
          // Combined busy state: parent loading OR local submitting
          const isBusy = isLoading === true || isSubmitting;
          return stickyActions ? (
            <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 pt-3 pb-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:pt-4 md:pb-4">
              <button
                type="button"
                onClick={onCancel}
                className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
                disabled={isBusy}
              >
                Abbrechen
              </button>
              <button
                type="submit"
                className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
                disabled={isBusy}
              >
                {isBusy ? "Wird gespeichert..." : submitLabel}
              </button>
            </div>
          ) : (
            <div className="flex justify-end pt-6 pb-2">
              <button
                type="button"
                onClick={onCancel}
                className="mr-2 rounded-lg px-3 py-2 text-sm text-gray-700 shadow-sm transition-colors hover:bg-gray-100 md:px-4 md:text-base"
                disabled={isBusy}
              >
                Abbrechen
              </button>
              <button
                type="submit"
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg md:px-6 md:text-base"
                disabled={isBusy}
              >
                {isBusy ? "Wird gespeichert..." : submitLabel}
              </button>
            </div>
          );
        })()}
      </form>
    </>
  );
}
