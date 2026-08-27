import { timetableSurface } from "~/components/timetable/timetable-style";
import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { PageIntro } from "~/components/ui/page-intro";

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
    <div className="w-full space-y-6" data-testid={testId}>
      {/* Auch der abgeschaltete Zustand beginnt mit der Kopfkarte, die jede
          Tenant-Seite trägt: sonst hätte ausgerechnet die Seite ohne Inhalt
          keinen sichtbaren Titel. Die Erklärung bleibt im Leerzustand
          darunter, damit der Kopf auf allen Planungsflächen gleich aussieht. */}
      <PageIntro kicker="Planung" title={pageTitle} />
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
