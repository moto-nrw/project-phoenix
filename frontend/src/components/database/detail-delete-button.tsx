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
      className="border-moto-red/20 bg-moto-red-soft text-moto-red-strong hover:bg-moto-red/10 flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm font-medium"
    >
      <Trash2 className="h-3.5 w-3.5" aria-hidden />
      {label}
    </button>
  );
}
