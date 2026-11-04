"use client";

import { Trash2 } from "lucide-react";
import { Button } from "~/components/ui/button";

interface DetailDeleteButtonProps {
  onClick: () => void;
  label?: string;
}

export function DetailDeleteButton({
  onClick,
  label = "Löschen",
}: DetailDeleteButtonProps) {
  return (
    <Button
      type="button"
      variant="outline_danger"
      size="md"
      onClick={onClick}
      className="gap-1.5"
    >
      <Trash2 className="h-3.5 w-3.5" aria-hidden />
      {label}
    </Button>
  );
}
