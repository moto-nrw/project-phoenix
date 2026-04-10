"use client";

import { useState } from "react";

interface PasswordFieldProps {
  readonly hasValue: boolean;
  readonly onChange: (value: unknown) => Promise<void>;
  readonly disabled?: boolean;
}

export function PasswordField({
  hasValue,
  onChange,
  disabled = false,
}: PasswordFieldProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [newValue, setNewValue] = useState("");

  if (!isEditing) {
    return (
      <div className="flex items-center gap-2">
        <span className="text-sm text-gray-500">
          {hasValue ? "••••••" : "Nicht gesetzt"}
        </span>
        <button
          type="button"
          onClick={() => setIsEditing(true)}
          disabled={disabled}
          className="text-sm font-medium text-gray-700 hover:text-gray-900 disabled:opacity-50"
        >
          {hasValue ? "Ändern" : "Setzen"}
        </button>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <input
        type="password"
        value={newValue}
        onChange={(e) => setNewValue(e.target.value)}
        placeholder="Neuen Wert eingeben"
        autoFocus
        className="block w-48 rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400"
      />
      <button
        type="button"
        onClick={async () => {
          if (newValue) {
            await onChange(newValue);
            setNewValue("");
            setIsEditing(false);
          }
        }}
        className="rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white transition-all hover:bg-gray-700"
      >
        Speichern
      </button>
      <button
        type="button"
        onClick={() => {
          setNewValue("");
          setIsEditing(false);
        }}
        className="text-sm text-gray-500 hover:text-gray-700"
      >
        Abbrechen
      </button>
    </div>
  );
}
