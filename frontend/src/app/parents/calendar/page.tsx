import { Suspense } from "react";
import { ParentCalendarPage } from "~/components/parent/calendar/parent-calendar-page";

export default function ParentsCalendarPage() {
  return (
    <Suspense fallback={null}>
      <ParentCalendarPage />
    </Suspense>
  );
}
