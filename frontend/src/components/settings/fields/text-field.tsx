"use client";

import { Input } from "~/components/ui/input";

interface TextFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  /** The id of the hint that names what is wrong with the value. */
  readonly errorId?: string;
}

export function TextField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  onBlur,
  disabled = false,
  errorId,
}: TextFieldProps) {
  return (
    <Input
      aria-label={ariaLabel}
      controlSize="compact"
      type="text"
      // A text setting had no length limit before the kit field (#5217);
      // keep URLs and longer texts typeable instead of the kit's 255.
      maxLength={2000}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onBlur={onBlur}
      onKeyDown={(e) => {
        if (e.key === "Enter") {
          e.currentTarget.blur();
        }
      }}
      disabled={disabled}
      aria-invalid={errorId ? true : undefined}
      aria-describedby={errorId}
      className="max-w-xs"
    />
  );
}
