"use client";

import React from "react";

interface SimpleToggleProps {
  label: string;
  description?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

export function SimpleToggle({
  label,
  description,
  checked,
  onChange,
  disabled = false,
}: SimpleToggleProps) {
  return (
    <div className="flex items-center justify-between rounded-2xl border border-gray-100 bg-white p-4">
      <div className="flex-1 pr-4">
        <h4 className="font-medium text-gray-900">{label}</h4>
        {description && (
          <p className="mt-1 text-sm text-gray-600">{description}</p>
        )}
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        disabled={disabled}
        onClick={() => onChange(!checked)}
        className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 focus:ring-2 focus:ring-[#5080D8] focus:ring-offset-2 focus:outline-none ${checked ? "bg-[#83CD2D]" : "bg-gray-200"} ${disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer"} `}
      >
        <span
          className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-sm transition-transform duration-200 ${checked ? "translate-x-6" : "translate-x-1"} `}
        />
      </button>
    </div>
  );
}
