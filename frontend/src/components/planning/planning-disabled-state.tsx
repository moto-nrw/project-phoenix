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
          keinen sichtbaren Titel. Die Erklärung steht als Unterzeile im Kopf,
          damit die Karte nicht nur den Titel trägt; der Leerzustand darunter
          nennt nur noch den abgeschalteten Bereich. */}
      <PageIntro kicker="Planung" title={pageTitle} description={description} />
      <div className={timetableSurface}>
        <EmptyState
          icon={<MotoConceptIcon concept="closingDays" size={42} />}
          title={heading}
        />
      </div>
    </div>
  );
}
