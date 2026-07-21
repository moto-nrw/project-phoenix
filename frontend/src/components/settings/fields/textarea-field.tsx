"use client";

interface TextareaFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
}

/**
 * Multi-line text editor for long/rich settings (e.g. the per-tenant
 * AGB and Datenschutz documents). Authors write Markdown here; the
 * consuming surface (the public enrollment form) renders it. Unlike the
 * single-line TextField it spans the full row width and grows to a
 * sensible height so a full legal page is editable without a cramped
 * box. Auto-save is debounced by the parent SettingsField; Enter inserts
 * a newline (no blur-on-Enter) since multi-line content is expected.
 */
export function TextareaField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  onBlur,
  disabled = false,
}: TextareaFieldProps) {
  return (
    <textarea
      aria-label={ariaLabel}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      disabled={disabled}
      rows={10}
      className="block w-full min-w-0 resize-y rounded-lg border-0 bg-white px-3 py-2.5 font-mono text-sm leading-6 text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 sm:w-96"
    />
  );
}
