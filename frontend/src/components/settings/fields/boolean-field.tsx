"use client";

interface BooleanFieldProps {
  readonly value: boolean;
  readonly onChange: (value: boolean) => void;
  readonly disabled?: boolean;
  readonly ariaLabel?: string;
}

export function BooleanField({
  value,
  onChange,
  disabled = false,
  ariaLabel,
}: BooleanFieldProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={value}
      aria-label={ariaLabel}
      onClick={() => onChange(!value)}
      disabled={disabled}
      className={`relative inline-flex h-7 w-12 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 ${
        value ? "bg-gray-900" : "bg-gray-200"
      }`}
    >
      <span
        className={`pointer-events-none inline-block h-6 w-6 rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out ${
          value ? "translate-x-5" : "translate-x-0"
        }`}
      />
    </button>
  );
}
