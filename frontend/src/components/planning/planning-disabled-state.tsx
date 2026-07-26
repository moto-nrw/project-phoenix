import { CalendarOff } from "lucide-react";

import { timetableSurface } from "~/components/timetable/timetable-style";

interface PlanningDisabledStateProps {
  readonly pageTitle: string;
  readonly heading: string;
  readonly description: string;
  readonly testId: string;
}

export function PlanningDisabledState({
  pageTitle,
  heading,
  description,
  testId,
}: PlanningDisabledStateProps) {
  return (
    <div className="flex flex-col gap-4" data-testid={testId}>
      <h1 className="text-lg font-semibold text-gray-900">{pageTitle}</h1>
      <div className={`${timetableSurface} p-10 text-center`}>
        <CalendarOff className="mx-auto h-10 w-10 text-gray-300" aria-hidden />
        <h2 className="mt-4 text-base font-semibold text-gray-900">
          {heading}
        </h2>
        <p className="mx-auto mt-2 max-w-md text-sm leading-relaxed text-gray-600">
          {description}
        </p>
      </div>
    </div>
  );
}
