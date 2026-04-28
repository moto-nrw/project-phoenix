"use client";

import { Trash2 } from "lucide-react";

interface DetailDeleteButtonProps {
  onClick: () => void;
  label?: string;
}

export function DetailDeleteButton({
  onClick,
  label = "Löschen",
}: DetailDeleteButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-1.5 rounded-md border border-red-200 bg-red-50 px-3 py-1.5 text-sm font-medium text-red-700 hover:bg-red-100"
    >
      <Trash2 className="h-3.5 w-3.5" aria-hidden />
      {label}
    </button>
  );
}
