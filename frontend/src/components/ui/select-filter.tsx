"use client";

interface SelectFilterProps {
  id: string;
  label: string;
  value: string | null;
  onChange: (value: string | null) => void;
  options: Array<{
    value: string;
    label: string;
  }>;
  placeholder?: string;
  className?: string;
  required?: boolean;
}

export function SelectFilter({
  id,
  label,
  value,
  onChange,
  options,
  placeholder = "Alle auswählen",
  className = "",
  required = false,
}: SelectFilterProps) {
  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newValue = e.target.value || null;
    onChange(newValue);
  };

  return (
    <div className={className}>
      <label 
        htmlFor={id} 
        className="mb-1 block text-xs md:text-sm font-medium text-gray-700"
      >
        {label}
        {required && "*"}
      </label>
      <select
        id={id}
        value={value ?? ""}
        onChange={handleChange}
        required={required}
        className="w-full rounded-lg border border-gray-300 px-3 py-2 text-xs md:text-sm transition-all duration-200 focus:ring-2 focus:ring-blue-500 focus:outline-none"
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </div>
  );
}