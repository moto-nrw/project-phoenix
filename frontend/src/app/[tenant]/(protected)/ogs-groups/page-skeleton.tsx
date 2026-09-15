"use client";

import { StudentCardPageSkeleton } from "~/components/students/student-card-skeleton";

// Page-shell skeleton for the OGS-groups gate/Suspense states.
export function OgsGroupsPageSkeleton() {
  return (
    <StudentCardPageSkeleton
      label="Gruppe wird geladen"
      testId="ogs-groups-skeleton"
      title="Meine Gruppe"
    />
  );
}
