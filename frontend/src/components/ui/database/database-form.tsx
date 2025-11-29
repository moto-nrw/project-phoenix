"use client";

import { useState, useEffect, useRef } from "react";
import type { DatabaseTheme } from "./themes";
import { getThemeClassNames } from "./themes";
import { getAccentRing, getAccentText } from "./accents";

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
}

export interface FormSection {
  title: string;
  subtitle?: string;
  fields: FormField[];
  columns?: 1 | 2;
  backgroundColor?: string; // Override theme background
  iconPath?: string; // Optional small header icon (heroicons path)
}

export interface DatabaseFormProps<T = Record<string, unknown>> {
  theme: DatabaseTheme;
  sections: FormSection[];
  onSubmit: (data: T) => Promise<void>;
  onCancel: () => void;
  initialData?: Partial<T>;
  isLoading?: boolean;
  error?: string | null;
  submitLabel: string;
  submitButtonGradient?: string; // Override default gradient
  stickyActions?: boolean; // Render sticky action bar like other entity forms
}

export function DatabaseForm<T = Record<string, unknown>>({
  theme,
  sections,
  onSubmit,
  onCancel,
  initialData,
  isLoading,
  error: externalError,
  submitLabel,
  submitButtonGradient,
  stickyActions = false,
}: DatabaseFormProps<T>) {
  const [formData, setFormData] = useState<Record<string, unknown>>({});
  const [error, setError] = useState<string | null>(null);
  const [asyncOptions, setAsyncOptions] = useState<
    Record<string, Array<{ value: string; label: string }>>
  >({});
  const [loadingOptions, setLoadingOptions] = useState<Record<string, boolean>>(
    {},
  );
  const loadedFieldsRef = useRef<Set<string>>(new Set());
  const themeClasses = getThemeClassNames(theme);
  const accentTextClass = getAccentText(theme.accent);

  // Initialize form data from sections
  useEffect(() => {
    const initializeFormData = async () => {
      const initialFormData: Record<string, unknown> = {};

      // Set defaults from sections
      sections.forEach((section) => {
        section.fields.forEach((field) => {
          if (field.type === "checkbox") {
            initialFormData[field.name] = false;
          } else if (field.type === "multiselect") {
            initialFormData[field.name] = [];
          } else if (field.type === "number") {
            // Special handling for data_retention_days
            if (field.name === "data_retention_days") {
              initialFormData[field.name] = 30;
            } else {
              initialFormData[field.name] = 0;
            }
          } else {
            initialFormData[field.name] = "";
          }
        });
      });

      // Override with initial data if provided
      if (initialData) {
        Object.keys(initialData).forEach((key) => {
          const value = initialData[key as keyof T];
          if (value !== undefined && value !== null) {
            // Ensure numbers are properly typed
            const field = sections
              .flatMap((s) => s.fields)
              .find((f) => f.name === key);
            if (field?.type === "number" && typeof value === "string") {
              initialFormData[key] =
                parseInt(value as unknown as string, 10) || 0;
            } else {
              initialFormData[key] = value;
            }
          }
        });

        // Special handling for students - fetch privacy consent if editing
        if (
          initialData &&
          "id" in initialData &&
          typeof initialData.id === "string" &&
          sections.some((s) =>
            s.fields.some(
              (f) =>
                f.name === "privacy_consent_accepted" ||
                f.name === "data_retention_days",
            ),
          )
        ) {
          try {
            const response = await fetch(
              `/api/students/${initialData.id}/privacy-consent`,
            );
            if (response.ok) {
              const responseData = (await response.json()) as unknown;
              console.log("Privacy consent response:", responseData);

              // The route handler should return the consent data directly in the data field
              if (
                responseData &&
                typeof responseData === "object" &&
                "data" in responseData
              ) {
                const consent = (responseData as { data: unknown }).data;
                if (
                  consent &&
                  typeof consent === "object" &&
                  "accepted" in consent &&
                  "data_retention_days" in consent
                ) {
                  const typedConsent = consent as {
                    accepted: boolean;
                    data_retention_days: number;
                  };
                  initialFormData.privacy_consent_accepted =
                    typedConsent.accepted;
                  initialFormData.data_retention_days =
                    typedConsent.data_retention_days;
                  console.log("Set privacy consent fields:", {
                    privacy_consent_accepted: typedConsent.accepted,
                    data_retention_days: typedConsent.data_retention_days,
                  });
                }
              }
            }
          } catch (error) {
            console.error("Error fetching privacy consent:", error);
          }
        }
      }

      setFormData(initialFormData);
    };

    void initializeFormData();
  }, [initialData, sections]);

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
              console.error(`Error loading options for ${field.name}:`, error);
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
        const numValue = parseInt(value, 10);
        setFormData((prev) => ({
          ...prev,
          [name]: isNaN(numValue) ? "" : numValue,
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
    setError(null);

    // Validate required fields
    for (const section of sections) {
      for (const field of section.fields) {
        const value = formData[field.name];

        // Check required fields
        if (field.required) {
          if (value === undefined || value === null || value === "") {
            setError(`${field.label} ist erforderlich.`);
            return;
          }
          // For number fields, also check for valid positive number if min is set
          if (field.type === "number" && field.min !== undefined) {
            const numValue =
              typeof value === "number" ? value : parseInt(value as string, 10);
            if (isNaN(numValue) || numValue < field.min) {
              setError(`${field.label} muss mindestens ${field.min} sein.`);
              return;
            }
          }
        }

        // Custom validation
        if (field.validation) {
          const validationError = field.validation(formData[field.name]);
          if (validationError) {
            setError(validationError);
            return;
          }
        }
      }
    }

    try {
      await onSubmit(formData as T);
    } catch (err) {
      console.error("Error submitting form:", err);
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Speichern der Daten. Bitte versuchen Sie es später erneut.",
      );
    }
  };

  const renderField = (field: FormField, _sectionBackground: string) => {
    // Determine focus ring color based on theme accent for consistency across neutral backgrounds
    const focusRingColor = getAccentRing(theme.accent);

    const baseInputClasses = `w-full rounded-lg border border-gray-300 px-3 py-2 md:px-4 md:py-2 text-sm transition-all duration-200 focus:ring-2 ${focusRingColor} focus:outline-none`;

    switch (field.type) {
      case "custom":
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

      case "checkbox":
        return (
          <div className="flex items-center">
            <input
              type="checkbox"
              id={field.name}
              name={field.name}
              checked={Boolean(formData[field.name])}
              onChange={handleChange}
              className={`h-4 w-4 rounded border-gray-300 text-${theme.accent}-600 focus:ring-${theme.accent}-500`}
            />
            <label
              htmlFor={field.name}
              className="ml-2 block text-xs text-gray-700 md:text-sm"
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
            <label
              htmlFor={field.name}
              className="mb-1.5 block text-xs font-medium text-gray-700"
            >
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
              rows={3}
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      case "select":
        const selectOptions = Array.isArray(field.options)
          ? field.options
          : (asyncOptions[field.name] ?? []);

        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1.5 block text-xs font-medium text-gray-700"
            >
              {field.label}
              {field.required && "*"}
            </label>
            <div className="relative">
              <select
                id={field.name}
                name={field.name}
                value={(formData[field.name] as string) ?? ""}
                onChange={handleChange}
                required={field.required}
                className={`${baseInputClasses} appearance-none pr-10`}
                disabled={loadingOptions[field.name]}
              >
                <option value="">
                  {loadingOptions[field.name]
                    ? "Optionen werden geladen..."
                    : (field.placeholder ?? "Bitte wählen")}
                </option>
                {selectOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
              {/* Dropdown chevron */}
              <svg
                className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
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
            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      case "multiselect":
        const multiselectOptions = Array.isArray(field.options)
          ? field.options
          : (asyncOptions[field.name] ?? []);
        const selectedValues = Array.isArray(formData[field.name])
          ? (formData[field.name] as string[])
          : [];

        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1.5 block text-xs font-medium text-gray-700"
            >
              {field.label}
              {field.required && "*"}
            </label>

            {/* Selected items as tags */}
            {selectedValues.length > 0 && (
              <div className="mb-2 flex flex-wrap gap-1.5">
                {selectedValues.map((value) => {
                  const option = multiselectOptions.find(
                    (opt) => opt.value === value,
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
                        onClick={() => {
                          setFormData((prev) => ({
                            ...prev,
                            [field.name]: selectedValues.filter(
                              (v) => v !== value,
                            ),
                          }));
                        }}
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

            {/* Dropdown for adding new selections */}
            <div className="relative">
              <select
                id={field.name}
                value=""
                onChange={(e) => {
                  const value = e.target.value;
                  if (value && !selectedValues.includes(value)) {
                    setFormData((prev) => ({
                      ...prev,
                      [field.name]: [...selectedValues, value],
                    }));
                  }
                }}
                className={`${baseInputClasses} appearance-none pr-10`}
                disabled={loadingOptions[field.name]}
              >
                <option value="">
                  {loadingOptions[field.name]
                    ? "Optionen werden geladen..."
                    : (field.placeholder ?? "Weitere hinzufügen...")}
                </option>
                {multiselectOptions
                  .filter((option) => !selectedValues.includes(option.value))
                  .map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
              </select>
              <svg
                className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
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

            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      case "number":
        // Handle both number and empty string values
        const numberValue = formData[field.name] as
          | string
          | number
          | undefined
          | null;
        const displayValue =
          numberValue === "" ||
          numberValue === undefined ||
          numberValue === null
            ? ""
            : String(numberValue);

        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1.5 block text-xs font-medium text-gray-700"
            >
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
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );

      default:
        return (
          <div>
            <label
              htmlFor={field.name}
              className="mb-1.5 block text-xs font-medium text-gray-700"
            >
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
              className={baseInputClasses}
            />
            {field.helperText && (
              <p className="mt-1 text-xs text-gray-500">{field.helperText}</p>
            )}
          </div>
        );
    }
  };

  // Determine button gradient
  const buttonGradient =
    submitButtonGradient ?? `from-${theme.primary} to-${theme.secondary}`;
  const buttonHoverGradient = submitButtonGradient
    ? submitButtonGradient.replace("500", "600").replace("600", "700")
    : `from-${theme.primary.replace("500", "600")} to-${theme.secondary.replace("600", "700")}`;

  return (
    <>
      {(error ?? externalError) && (
        <div className="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-800 md:mb-6 md:p-4 md:text-base">
          <p>{error ?? externalError}</p>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {sections.map((section, sectionIndex) => {
          // Use custom background or theme background
          const bgClass = section.backgroundColor ?? themeClasses.background;
          const textClass = "text-gray-900";

          return (
            <div
              key={`section-${sectionIndex}`}
              className={`mb-6 rounded-lg md:mb-8 ${bgClass} p-3 md:p-4`}
            >
              <h2
                className={`mb-2.5 text-xs font-semibold md:mb-3 md:text-sm ${textClass} flex items-center gap-2`}
              >
                {section.iconPath && (
                  <svg
                    className={`h-3.5 w-3.5 md:h-4 md:w-4 ${accentTextClass}`}
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d={section.iconPath}
                    />
                  </svg>
                )}
                <span>{section.title}</span>
              </h2>
              {section.subtitle && (
                <p className="mb-2.5 text-xs text-gray-600 md:mb-3">
                  {section.subtitle}
                </p>
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
                    {renderField(field, bgClass)}
                  </div>
                ))}
              </div>
            </div>
          );
        })}

        {/* Form actions */}
        {stickyActions ? (
          <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 pt-3 pb-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:pt-4 md:pb-4">
            <button
              type="button"
              onClick={onCancel}
              className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
              disabled={isLoading}
            >
              Abbrechen
            </button>
            <button
              type="submit"
              className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:px-4 md:text-sm md:hover:scale-105"
              disabled={isLoading}
            >
              {isLoading ? "Wird gespeichert..." : submitLabel}
            </button>
          </div>
        ) : (
          <div className="flex justify-end pt-6 pb-2">
            <button
              type="button"
              onClick={onCancel}
              className="mr-2 rounded-lg px-3 py-2 text-sm text-gray-700 shadow-sm transition-colors hover:bg-gray-100 md:px-4 md:text-base"
              disabled={isLoading}
            >
              Abbrechen
            </button>
            <button
              type="submit"
              className={`rounded-lg bg-gradient-to-r ${buttonGradient} px-4 py-2 text-sm text-white transition-all duration-200 md:px-6 md:text-base hover:${buttonHoverGradient} hover:shadow-lg`}
              disabled={isLoading}
            >
              {isLoading ? "Wird gespeichert..." : submitLabel}
            </button>
          </div>
        )}
      </form>
    </>
  );
}
