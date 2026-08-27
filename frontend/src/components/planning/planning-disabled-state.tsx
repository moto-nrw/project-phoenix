import { timetableSurface } from "~/components/timetable/timetable-style";
import { EmptyState } from "~/components/ui/empty-state";
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
      <div className={timetableSurface}>
        <EmptyState
          icon={<MotoConceptIcon concept="closingDays" size={42} />}
          title={heading}
          description={description}
        />
      </div>
    </div>
  );
}
