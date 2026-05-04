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
    <div className="relative">
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
        className="block w-auto max-w-xs min-w-[8rem] appearance-none rounded-lg border-0 bg-white py-2.5 pr-10 pl-3 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500"
      >
        {options.map((opt) => (
          <option key={toStr(opt.value)} value={toStr(opt.value)}>
            {opt.label}
          </option>
        ))}
      </select>
      <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
        <svg
          className="h-4 w-4 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </div>
    </div>
  );
}
