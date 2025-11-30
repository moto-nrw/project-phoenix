"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import { ConfirmationModal } from "~/components/ui/modal";

interface DetailModalActionsProps {
  onEdit: () => void;
  onDelete: () => void;
  entityName: string;
  entityType: string; // e.g. "Gruppe", "Aktivität", "Raum", "Gerät"
  /** Custom confirmation message content (optional) */
  confirmationContent?: ReactNode;
  /**
   * Optional custom click handler for delete button.
   * When provided, the component will NOT render its own ConfirmationModal.
   * Use this for inline confirmation patterns where the parent handles confirmation.
   */
  onDeleteClick?: () => void;
}

// German article lookup for entity types
const GERMAN_ARTICLES: Record<string, string> = {
  Gerät: "das",
  // All others use "die"
};

export function DetailModalActions({
  onEdit,
  onDelete,
  entityName,
  entityType,
  confirmationContent,
  onDeleteClick,
}: DetailModalActionsProps) {
  const [confirmOpen, setConfirmOpen] = useState(false);

  // Get the correct German article ("die" or "das")
  const article = GERMAN_ARTICLES[entityType] ?? "die";

  // Use custom handler if provided, otherwise open internal ConfirmationModal
  const handleDeleteClick = onDeleteClick ?? (() => setConfirmOpen(true));

  return (
    <>
      <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex flex-wrap gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
        <button
          type="button"
          onClick={handleDeleteClick}
          className="rounded-lg border border-red-300 px-3 py-2 text-xs font-medium text-red-700 transition-all duration-200 hover:border-red-400 hover:bg-red-50 hover:shadow-md active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
        >
          <span className="flex items-center gap-2">
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
        <button
          type="button"
          onClick={onEdit}
          className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
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
                d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
              />
            </svg>
            Bearbeiten
          </span>
        </button>
      </div>

      {/* Only render ConfirmationModal when using internal confirmation (no custom onDeleteClick) */}
      {!onDeleteClick && (
        <ConfirmationModal
          isOpen={confirmOpen}
          onClose={() => setConfirmOpen(false)}
          onConfirm={() => {
            setConfirmOpen(false);
            onDelete();
          }}
          title={`${entityType} löschen?`}
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          {confirmationContent ?? (
            <p className="text-sm text-gray-700">
              Möchten Sie {article} {entityType}{" "}
              <span className="font-medium">{entityName}</span> wirklich
              löschen? Diese Aktion kann nicht rückgängig gemacht werden.
            </p>
          )}
        </ConfirmationModal>
      )}
    </>
  );
}
