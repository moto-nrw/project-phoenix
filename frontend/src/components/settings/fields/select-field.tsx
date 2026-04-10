"use client";

interface SelectOption {
  readonly label: string;
  readonly value: unknown;
}

interface SelectFieldProps {
  readonly value: unknown;
  readonly onChange: (value: unknown) => void;
  readonly options: SelectOption[];
  readonly disabled?: boolean;
}

function toStr(v: unknown): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  if (typeof v === "number" || typeof v === "boolean") return v.toString();
  return JSON.stringify(v);
}

export function SelectField({
  value,
  onChange,
  options,
  disabled = false,
}: SelectFieldProps) {
  return (
    <select
      value={toStr(value)}
      onChange={(e) => {
        const selected = options.find(
          (opt) => toStr(opt.value) === e.target.value,
        );
        if (selected) {
          onChange(selected.value);
        }
      }}
      disabled={disabled}
      className="block w-48 rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
    >
      {options.map((opt) => (
        <option key={toStr(opt.value)} value={toStr(opt.value)}>
          {opt.label}
        </option>
      ))}
    </select>
  );
}
