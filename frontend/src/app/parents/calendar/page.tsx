import { Suspense } from "react";
import { ParentCalendarPage } from "~/components/parent/calendar/parent-calendar-page";
import { ParentPageSkeleton } from "~/components/parent/parent-page";

export default function ParentsCalendarPage() {
  return (
    <Suspense fallback={<ParentPageSkeleton rows={2} />}>
      <ParentCalendarPage />
    </Suspense>
  );
}
