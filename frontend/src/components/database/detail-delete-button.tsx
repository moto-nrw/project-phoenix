"use client";

import { Trash2 } from "lucide-react";
import { Button } from "~/components/ui/button";

interface DetailDeleteButtonProps {
  onClick: () => void;
  label?: string;
  /** What the click deletes and whether moto asks first (#6643). */
  tooltip?: string;
}

export function DetailDeleteButton({
  onClick,
  label = "Löschen",
  tooltip,
}: DetailDeleteButtonProps) {
  return (
    <Button
      type="button"
      variant="outline_danger"
      size="md"
      onClick={onClick}
      className="gap-1.5"
      tooltip={tooltip}
    >
      <Trash2 className="h-3.5 w-3.5" aria-hidden />
      {label}
    </Button>
  );
}
