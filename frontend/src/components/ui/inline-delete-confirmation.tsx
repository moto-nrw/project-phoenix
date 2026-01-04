"use client";

import type { ReactNode } from "react";

interface InlineDeleteConfirmationProps {
  /** Title shown at the top, e.g. "Raum löschen?" */
  readonly title: string;
  /** Main confirmation message content */
  readonly children: ReactNode;
  /** Called when user clicks "Abbrechen" */
  readonly onCancel: () => void;
  /** Called when user clicks "Löschen" */
  readonly onConfirm: () => void;
}

/**
 * Inline delete confirmation view that replaces the modal content.
 * Used by modals that show confirmation inline rather than in a separate overlay.
 * Preserves the exact UX pattern from room/student/teacher detail modals.
 */
export function InlineDeleteConfirmation({
  title,
  children,
  onCancel,
  onConfirm,
}: Readonly<InlineDeleteConfirmationProps>) {
  return (
    <div className="space-y-6">
      {/* Warning Icon */}
      <div className="flex justify-center">
        <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
          <svg
            className="h-8 w-8 text-red-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
            />
          </svg>
        </div>
      </div>

      {/* Confirmation Message */}
      <div className="space-y-3 text-center">
        <h3 className="text-xl font-bold text-gray-900">{title}</h3>
        {children}
        <p className="text-sm font-medium text-red-600">
          Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </div>

      {/* Action Buttons */}
      <div className="flex gap-3 border-t border-gray-100 pt-4">
        <button
          type="button"
          onClick={onCancel}
          className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
        >
          Abbrechen
        </button>
        <button
          type="button"
          onClick={onConfirm}
          className="flex-1 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-red-700 hover:shadow-lg active:scale-100"
        >
          <span className="flex items-center justify-center gap-2">
            <svg
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
            Löschen
          </span>
        </button>
      </div>
    </div>
  );
}
