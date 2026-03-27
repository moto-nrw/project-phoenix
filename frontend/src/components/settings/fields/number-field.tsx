"use client";

interface NumberFieldProps {
  readonly value: number;
  readonly onChange: (value: number) => void;
  readonly disabled?: boolean;
  readonly min?: number;
  readonly max?: number;
}

export function NumberField({
  value,
  onChange,
  disabled = false,
  min,
  max,
}: NumberFieldProps) {
  return (
    <input
      type="number"
      value={value}
      onChange={(e) => {
        const num = parseFloat(e.target.value);
        if (!isNaN(num)) {
          onChange(num);
        }
      }}
      min={min}
      max={max}
      disabled={disabled}
      className="block w-32 rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:ring-2 focus:ring-gray-900 focus:outline-none focus:ring-inset disabled:bg-gray-50 disabled:text-gray-500"
    />
  );
}
