"use client";

import { StudentCardPageSkeleton } from "~/components/students/student-card-skeleton";

// Page-shell skeleton for the student-search gate/Suspense states.
export function StudentSearchPageSkeleton() {
  return (
    <StudentCardPageSkeleton
      label="Alle Kinder werden geladen"
      testId="students-search-skeleton"
      kicker="Kinder"
      title="Alle Kinder"
      description="Alle betreuten Kinder suchen, filtern und ihren aktuellen Aufenthaltsort sehen."
    />
  );
}
