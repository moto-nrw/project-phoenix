"use client";

interface TextFieldProps {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
}

export function TextField({
  value,
  onChange,
  disabled = false,
}: TextFieldProps) {
  return (
    <input
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      className="block w-full max-w-xs rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:ring-2 focus:ring-gray-900 focus:outline-none focus:ring-inset disabled:bg-gray-50 disabled:text-gray-500"
    />
  );
}
