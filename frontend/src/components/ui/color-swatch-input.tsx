"use client";

/**
 * Round colour preview that opens the native colour picker on click. The
 * native input is invisible but full-size on top, so the click target stays
 * accessible and the OS picker still opens (works on touch devices, no extra
 * dependency).
 */
export function ColorSwatchInput(props: {
  id?: string;
  /** Hex colour `#RRGGBB` shown in the swatch and preselected in the picker. */
  value: string;
  /** Receives the picked colour as upper-case hex. */
  onChange: (value: string) => void;
  onBlur?: () => void;
  label: string;
  disabled?: boolean;
}) {
  const { id, value, onChange, onBlur, label, disabled = false } = props;
  return (
    <label
      htmlFor={id}
      className={`relative inline-block h-9 w-9 shrink-0 rounded-full border border-gray-300 shadow-sm ring-1 ring-gray-200/50 transition ${
        disabled
          ? "cursor-not-allowed opacity-60"
          : "cursor-pointer hover:ring-gray-300"
      }`}
      style={{ backgroundColor: value }}
      aria-label={`${label} auswählen`}
    >
      <input
        id={id}
        type="color"
        value={value}
        onChange={(e) => onChange(e.target.value.toUpperCase())}
        onBlur={onBlur}
        disabled={disabled}
        className="absolute inset-0 h-full w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
        aria-label={label}
      />
    </label>
  );
}
