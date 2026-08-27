import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TenantPage } from "~/components/ui/tenant-page";

interface PlanningDisabledStateProps {
  readonly pageTitle: string;
  readonly heading: string;
  readonly description: string;
  readonly testId: string;
}

/**
 * Abgeschalteter Planungsbereich. Auch dieser Zustand rendert das
 * Seitengerüst: Kopfkarte mit dem Seitentitel, darunter der Leerzustand des
 * Gerüsts. Die Erklärung steht im Leerzustand, nicht als Erklärsatz in der
 * Statuszeile — dort gehören nur Zahlen hin.
 */
export function PlanningDisabledState({
  pageTitle,
  heading,
  description,
  testId,
}: PlanningDisabledStateProps) {
  return (
    <TenantPage
      title={pageTitle}
      testId={testId}
      empty={{
        title: heading,
        description,
        icon: <MotoConceptIcon concept="closingDays" size={42} />,
      }}
    />
  );
}
