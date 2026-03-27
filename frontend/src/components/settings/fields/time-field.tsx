"use client";

interface TimeFieldProps {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
}

export function TimeField({
  value,
  onChange,
  disabled = false,
}: TimeFieldProps) {
  return (
    <input
      type="time"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      className="block w-36 rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset focus:ring-2 focus:ring-gray-900 focus:outline-none focus:ring-inset disabled:bg-gray-50 disabled:text-gray-500"
    />
  );
}
