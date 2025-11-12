"use client";

import React from "react";

interface IOSToggleProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

export function IOSToggle({
  checked,
  onChange,
  disabled = false,
}: IOSToggleProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-7 w-12 items-center rounded-full transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 ${disabled ? "cursor-not-allowed opacity-50" : "cursor-pointer"} ${checked ? "bg-[#83CD2D]" : "bg-gray-300"} `}
    >
      <span
        className={`inline-block h-5 w-5 transform rounded-full bg-white shadow-md transition-transform duration-200 ease-in-out ${checked ? "translate-x-6" : "translate-x-1"} `}
      />
    </button>
  );
}
