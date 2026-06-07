"use client";

import { Lock } from "lucide-react";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";
import type { Student } from "~/lib/api";

interface StudentGuardiansTabProps {
  student: Student;
  onChanged?: () => void;
}

export function StudentGuardiansTab({
  student,
  onChanged,
}: StudentGuardiansTabProps) {
  const canWrite = student.has_write_access !== false;

  // Admin context: write access is assumed. If the current session lacks
  // full access to this student's data (GDPR scoping), fall back to a hint.
  if (student.has_full_access === false) {
    return (
      <div className="flex items-start gap-3 rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600">
        <Lock className="mt-0.5 h-4 w-4 shrink-0 text-gray-400" aria-hidden />
        <div>
          <div className="font-semibold text-gray-900">
            Kein vollständiger Zugriff
          </div>
          <div className="mt-0.5 text-xs">
            Erziehungsberechtigte sind nur für Mitarbeitende mit Vollzugriff auf
            diesen Schüler sichtbar. Bitte bei der Schulleitung nachfragen.
          </div>
        </div>
      </div>
    );
  }

  return (
    <StudentGuardianManager
      key={student.id}
      studentId={student.id}
      readOnly={!canWrite}
      onUpdate={canWrite ? onChanged : undefined}
    />
  );
}
