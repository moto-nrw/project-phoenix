"use client";

import { StudentCardPageSkeleton } from "~/components/students/student-card-skeleton";

// Page-shell skeleton for the student-search gate/Suspense states.
export function StudentSearchPageSkeleton() {
  return (
    <StudentCardPageSkeleton
      label="Kindersuche wird geladen"
      testId="students-search-skeleton"
    />
  );
}
