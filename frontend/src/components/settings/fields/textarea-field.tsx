"use client";

import { Textarea } from "~/components/ui/textarea";

interface TextareaFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  /** Example text of an empty field. */
  readonly placeholder?: string;
  /** Visible lines; long texts such as the AGB keep the default. */
  readonly rows?: number;
  /** id of the hint that explains why the value is refused. */
  readonly errorId?: string;
  /**
   * Plain text that prints as it is (a letter's sender or footer): the
   * field shows it in the letter's font instead of the Markdown font.
   */
  readonly plainText?: boolean;
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
  placeholder,
  rows = 10,
  errorId,
  plainText = false,
}: TextareaFieldProps) {
  return (
    <div className="w-full min-w-0 sm:w-96">
      <Textarea
        aria-label={ariaLabel}
        aria-invalid={errorId ? true : undefined}
        aria-describedby={errorId}
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onBlur={onBlur}
        disabled={disabled}
        rows={rows}
        className={
          plainText
            ? "resize-y py-2.5 leading-6"
            : "resize-y py-2.5 font-mono leading-6"
        }
      />
    </div>
  );
}
