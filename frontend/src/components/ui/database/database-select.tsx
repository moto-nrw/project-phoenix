"use client";

import {
  useState,
  useEffect,
  useCallback,
  useLayoutEffect,
  useRef,
} from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { apiErrorFromResponse, unavailableApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DatabaseSelect" });

interface SelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
}

/**
 * Static options, or options loaded on mount. A failed load goes to the
 * owner's shared load path (`useApiLoadError`, #2517): the kit may not import
 * contexts, so the owner receives the error with a retry through
 * `onLoadError` and hands the catalog text back as `loadError`. While it is
 * set the select gives way to that text, never to an empty list. The owner's
 * retry clears `loadError` and then calls the handed-in retry, which loads
 * again.
 */
type DatabaseSelectOptionsSource =
  | {
      readonly options?: ReadonlyArray<SelectOption>;
      readonly loadOptions?: undefined;
      readonly loadError?: undefined;
      readonly onLoadError?: undefined;
    }
  | {
      readonly options?: undefined;
      readonly loadOptions: () => Promise<ReadonlyArray<SelectOption>>;
      readonly loadError: FormErrorInput;
      readonly onLoadError: (error: unknown, retry: () => void) => void;
    };

type DatabaseSelectProps = DatabaseSelectOptionsSource & {
  // Core props
  readonly id?: string;
  readonly name: string;
  readonly label?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;

  // UI props
  readonly placeholder?: string;
  readonly emptyOptionLabel?: string;
  readonly required?: boolean;
  readonly disabled?: boolean;
  readonly loading?: boolean;
  readonly error?: string;
  readonly helperText?: string;
  readonly className?: string;
  readonly includeEmpty?: boolean;
};

export function DatabaseSelect({
  id,
  name,
  label,
  value,
  onChange,
  options: staticOptions,
  loadOptions,
  loadError = null,
  onLoadError,
  placeholder = "Bitte wählen",
  emptyOptionLabel,
  required = false,
  disabled = false,
  loading: externalLoading = false,
  error: externalError,
  helperText,
  className = "",
  includeEmpty = true,
}: DatabaseSelectProps) {
  const [options, setOptions] = useState<readonly SelectOption[]>(
    staticOptions ?? [],
  );
  const [loading, setLoading] = useState(false);
  // Bumped by the owner's „Wiederholen“ after a failed load.
  const [reload, setReload] = useState(0);
  // The loader runs in an effect; it reports to the owner's current handler.
  const onLoadErrorRef = useRef(onLoadError);
  useLayoutEffect(() => {
    onLoadErrorRef.current = onLoadError;
  });

  // Load async options if loadOptions is provided
  useEffect(() => {
    if (!loadOptions || staticOptions) return;

    let cancelled = false;
    const fetchOptions = async () => {
      try {
        setLoading(true);
        const loadedOptions = await loadOptions();
        if (!cancelled) setOptions(loadedOptions);
      } catch (err) {
        logger.warn("failed to load options", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          onLoadErrorRef.current?.(err, () => setReload((n) => n + 1));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void fetchOptions();
    return () => {
      cancelled = true;
    };
  }, [loadOptions, staticOptions, reload]);

  // Update options if staticOptions change
  useEffect(() => {
    if (staticOptions) {
      setOptions(staticOptions);
    }
  }, [staticOptions]);

  const isLoading = loading || externalLoading;
  const displayError = externalError;

  const selectOptions = [
    ...(includeEmpty
      ? [
          {
            value: "",
            label: isLoading ? "Lädt..." : (emptyOptionLabel ?? placeholder),
          },
        ]
      : []),
    ...options.map((option) => ({
      value: option.value,
      label: option.label,
      disabled: option.disabled,
    })),
    ...(!isLoading && options.length === 0 && !includeEmpty
      ? [{ value: "", label: "Keine Optionen verfügbar", disabled: true }]
      : []),
  ];

  return (
    <div className="w-full">
      {label && (
        <label
          htmlFor={id ?? name}
          className="mb-1 block text-xs font-medium text-gray-700 md:text-sm"
        >
          {label}
          {required && "*"}
        </label>
      )}

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : (
        <CustomSelect
          ariaLabel={label ?? name}
          id={id ?? name}
          name={name}
          value={value}
          options={selectOptions}
          onChange={onChange}
          disabled={disabled || isLoading}
          required={required}
          invalid={Boolean(displayError)}
          placeholder={
            isLoading ? "Lädt..." : (emptyOptionLabel ?? placeholder)
          }
          className={`${isLoading ? "cursor-wait opacity-50" : ""} ${className}`}
        />
      )}

      {/* Helper text or error message */}
      {displayError && (
        <p className="text-moto-red mt-1 text-xs md:text-sm">{displayError}</p>
      )}
      {!displayError && helperText && (
        <p className="mt-1 text-xs text-gray-500 md:text-sm">{helperText}</p>
      )}
    </div>
  );
}

/**
 * Specialized select components using DatabaseSelect as base
 */

// Example: EntitySelect for loading entities from API
type EntitySelectProps = Omit<
  DatabaseSelectProps,
  "loadOptions" | "options" | "loadError" | "onLoadError"
> & {
  readonly entityType: "groups" | "rooms" | "teachers" | "activities";
  readonly filters?: Record<string, unknown>;
  /** The owner's load path, see `DatabaseSelectOptionsSource`. */
  readonly loadError: FormErrorInput;
  readonly onLoadError: (error: unknown, retry: () => void) => void;
};

function EntitySelect({ entityType, filters, ...props }: EntitySelectProps) {
  const loadOptions = useCallback(async () => {
    const params = new URLSearchParams();
    if (filters) {
      Object.entries(filters).forEach(([key, value]) => {
        if (value !== null && value !== undefined) {
          // Convert value to string safely
          let stringValue: string;
          if (typeof value === "string") {
            stringValue = value;
          } else if (typeof value === "number" || typeof value === "boolean") {
            stringValue = String(value);
          } else {
            // For objects and other types, use JSON.stringify
            stringValue = JSON.stringify(value);
          }
          params.append(key, stringValue);
        }
      });
    }

    const queryString = params.toString();
    const url = queryString
      ? `/api/${entityType}?${queryString}`
      : `/api/${entityType}`;
    // A request that never reached the API becomes general.unavailable.
    const response = await fetch(url).catch((error: unknown) => {
      throw unavailableApiError(error);
    });

    if (!response.ok) {
      throw await apiErrorFromResponse(
        response,
        `Failed to load ${entityType}`,
      );
    }

    const data = (await response.json()) as
      | Array<{ id: string | number; name: string }>
      | { data: Array<{ id: string | number; name: string }> };

    // Handle different response formats
    const items = Array.isArray(data) ? data : data.data;

    return items.map((item) => ({
      value: String(item.id),
      label: item.name,
    }));
  }, [entityType, filters]);

  return <DatabaseSelect {...props} loadOptions={loadOptions} />;
}

/**
 * Convenience components for specific entity types
 */

export function GroupSelect(
  props: Readonly<Omit<EntitySelectProps, "entityType">>,
) {
  return (
    <EntitySelect
      {...props}
      entityType="groups"
      label={props.label ?? "Gruppe"}
    />
  );
}
