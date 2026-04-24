"use client";

import { AlertCircle, ChevronRight } from "lucide-react";
import type { Student } from "~/lib/api";
import { cn } from "~/lib/utils";

interface StudentListItemProps {
  student: Student;
  isSelected: boolean;
  onSelect: () => void;
  hasArrival?: boolean;
  arrivalSummary?: string;
}

function formatName(student: Student): string {
  if (student.first_name && student.second_name) {
    return `${student.first_name} ${student.second_name}`;
  }
  return student.name || "Unbekannt";
}

export function StudentListItem({
  student,
  isSelected,
  onSelect,
  hasArrival = true,
  arrivalSummary,
}: StudentListItemProps) {
  const subtitleParts: string[] = [];
  if (student.group_name) {
    subtitleParts.push(student.group_name);
  }
  if (arrivalSummary) {
    subtitleParts.push(arrivalSummary);
  } else if (!hasArrival) {
    subtitleParts.push("keine Ankunft");
  }

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-current={isSelected ? "true" : undefined}
      className={cn(
        "flex w-full items-center gap-3 border-b border-gray-100 px-4 py-2.5 text-left transition-colors hover:bg-gray-50",
        isSelected && "bg-[#DCF5C1]/60 hover:bg-[#DCF5C1]/70",
      )}
    >
      <div className="min-w-0 flex-1">
        <div
          className={cn(
            "truncate text-sm text-gray-900",
            isSelected ? "font-semibold" : "font-medium",
          )}
        >
          {formatName(student)}
        </div>
        <div
          className={cn(
            "truncate text-xs",
            !hasArrival ? "font-medium text-[#F78C10]" : "text-gray-500",
          )}
        >
          {subtitleParts.join(" · ") || "—"}
        </div>
      </div>
      {!hasArrival ? (
        <AlertCircle
          className="h-4 w-4 shrink-0 text-[#F78C10]"
          aria-label="Ankunft fehlt"
        />
      ) : null}
      <ChevronRight
        className={cn(
          "h-4 w-4 shrink-0",
          isSelected ? "text-[#83CD2D]" : "text-gray-400",
        )}
        aria-hidden
      />
    </button>
  );
}
