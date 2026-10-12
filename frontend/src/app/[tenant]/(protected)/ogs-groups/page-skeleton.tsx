"use client";

import { StudentCardPageSkeleton } from "~/components/students/student-card-skeleton";
import { OGS_GROUP_SECTION_LABELS } from "~/lib/ogs-group-sections";

// Page-shell skeleton for the OGS-groups gate/Suspense states.
export function OgsGroupsPageSkeleton() {
  return (
    <StudentCardPageSkeleton
      label="Gruppe wird geladen"
      testId="ogs-groups-skeleton"
      title={OGS_GROUP_SECTION_LABELS.personal}
    />
  );
}
