"use client";

/**
 * MaterializeButton — triggers POST /api/timetable/materialize for the
 * currently visible week. Sole admin action that is read-write in this
 * read-only MVP. Confirms before running because re-materialising
 * overwrites all `planned` non-spontaneous instances in the window
 * (active/completed/cancelled survive — see backend ReplanWeek docs).
 */

import { useState } from "react";
import { CalendarPlus } from "lucide-react";

import { Button } from "~/components/ui/button";

interface MaterializeButtonProps {
  onMaterialize: () => Promise<void>;
  weekLabel: string;
  disabled?: boolean;
  variant?: "primary" | "secondary";
}

export function MaterializeButton({
  onMaterialize,
  weekLabel,
  disabled = false,
  variant = "secondary",
}: MaterializeButtonProps) {
  const [isPending, setIsPending] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);

  const handleClick = () => {
    if (!showConfirm) {
      setShowConfirm(true);
      return;
    }
    setIsPending(true);
    void onMaterialize().finally(() => {
      setIsPending(false);
      setShowConfirm(false);
    });
  };

  return (
    <div className="flex items-center gap-2">
      {showConfirm && !isPending && (
        <span className="text-xs text-slate-600">
          Geplante Termine in <span className="font-semibold">{weekLabel}</span>
          wird aus den Vorlagen als konkrete Termine angelegt.
        </span>
      )}
      <Button
        variant={variant === "primary" ? "success" : "outline"}
        size="sm"
        onClick={handleClick}
        disabled={disabled}
        isLoading={isPending}
        loadingText="Plane …"
        className="!shadow-sm"
        type="button"
      >
        <span className="inline-flex items-center gap-2">
          <CalendarPlus className="h-4 w-4" />
          {showConfirm
            ? "Planen bestätigen"
            : variant === "primary"
              ? "Woche aus Vorlagen planen"
              : "Aus Vorlagen planen"}
        </span>
      </Button>
      {showConfirm && !isPending && (
        <button
          type="button"
          onClick={() => setShowConfirm(false)}
          className="text-xs font-medium text-slate-500 hover:text-slate-700"
        >
          Abbrechen
        </button>
      )}
    </div>
  );
}
