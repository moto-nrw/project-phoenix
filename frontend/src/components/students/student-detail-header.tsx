"use client";

import { AlertTriangle } from "lucide-react";
import type { ReactNode } from "react";
import type { Student } from "~/lib/api";

interface StudentDetailHeaderProps {
  student: Student;
  warning?: string | null;
  actions?: ReactNode;
}

function getInitials(student: Student): string {
  const first = (student.first_name?.[0] ?? "").toUpperCase();
  const last = (student.second_name?.[0] ?? "").toUpperCase();
  return `${first}${last}` || "?";
}

function formatFullName(student: Student): string {
  if (student.first_name && student.second_name) {
    return `${student.first_name} ${student.second_name}`;
  }
  return student.name || "Unbekannt";
}

export function StudentDetailHeader({
  student,
  warning,
  actions,
}: StudentDetailHeaderProps) {
  const metaParts: string[] = [];
  if (student.school_class) {
    metaParts.push(student.school_class);
  }
  if (student.group_name) {
    metaParts.push(student.group_name);
  }

  return (
    <div className="flex items-center gap-4 px-6 py-5">
      <div
        aria-hidden
        className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-[#DCE6F8] text-base font-semibold text-[#5080D8]"
      >
        {getInitials(student)}
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate text-lg font-bold text-gray-900">
          {formatFullName(student)}
        </div>
        <div className="mt-0.5 truncate text-sm text-gray-500">
          {metaParts.join(" · ") || "Keine Klasse hinterlegt"}
        </div>
      </div>
      {warning ? (
        <div className="flex items-center gap-1.5 rounded-full bg-[#FFE8D0] px-3 py-1 text-xs font-semibold text-[#F78C10]">
          <AlertTriangle className="h-3 w-3" aria-hidden />
          {warning}
        </div>
      ) : null}
      {actions ? (
        <div className="flex items-center gap-2">{actions}</div>
      ) : null}
    </div>
  );
}
