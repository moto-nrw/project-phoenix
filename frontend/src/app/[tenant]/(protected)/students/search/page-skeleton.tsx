"use client";

import { StudentCardPageSkeleton } from "~/components/students/student-card-skeleton";

// Page-shell skeleton for the student-search gate/Suspense states.
export function StudentSearchPageSkeleton() {
  return (
    <StudentCardPageSkeleton
      label="Kinder werden geladen"
      testId="students-search-skeleton"
      title="Kinder"
    />
  );
}
