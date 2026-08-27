import { timetableSurface } from "~/components/timetable/timetable-style";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

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
    <div className="w-full space-y-4" data-testid={testId}>
      {/* Gleiche Titelbehandlung wie in der PlanningContextBar: mobil sichtbar,
          ab md trägt die App-Kopfzeile den Seitennamen. */}
      <h1 className="truncate text-xs font-medium tracking-wide text-gray-500 uppercase md:sr-only">
        {pageTitle}
      </h1>
      <div className={`${timetableSurface} p-10 text-center`}>
        <MotoConceptIcon concept="closingDays" size={42} className="mx-auto" />
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
