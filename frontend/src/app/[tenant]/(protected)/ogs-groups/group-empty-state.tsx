import type { ReactNode } from "react";
import { Button } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { PageBodyEmpty } from "~/components/ui/page-body";
import { childrenConcept } from "~/lib/concepts/children";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";

/** Was eine Feriengruppe vor ihrer Woche über sich sagt (#6680). */
interface HolidayGroupNotice {
  readonly title: string;
  readonly description: string;
  readonly action?: ReactNode;
}

/**
 * Leerzustand der Gruppenseite. Eine gewöhnliche Gruppe ohne Kinder nennt
 * den nächsten Schritt: ein Kind in der Kindersuche zuordnen. Eine
 * Feriengruppe vor ihrer Woche ist absichtlich leer (#6680); sie sagt, wie
 * viele Kinder dazukommen und ab wann, und schickt niemanden in die
 * Kindersuche, denn die Feriengäste kommen von selbst.
 */
export function groupEmptyState({
  visible,
  groupName,
  severalGroups,
  holidayHint,
  onOpenSearch,
}: Readonly<{
  visible: boolean;
  groupName: string | undefined;
  severalGroups: boolean;
  holidayHint: HolidayGroupNotice | undefined;
  onOpenSearch: () => void;
}>): PageBodyEmpty {
  if (!visible) return null;
  if (holidayHint) {
    return {
      icon: <MotoConceptIcon concept={holidayWeeksConcept} size={48} />,
      title: holidayHint.title,
      description: holidayHint.description,
      action: holidayHint.action,
    };
  }
  return {
    icon: <MotoConceptIcon concept={childrenConcept} size={48} />,
    title: `Keine Kinder in ${groupName ?? "dieser Gruppe"}`,
    description: severalGroups
      ? "Dieser Gruppe ist noch kein Kind zugeordnet. Ordnen Sie ein Kind in der Kindersuche zu, oder wählen Sie oben eine andere Gruppe."
      : "Dieser Gruppe ist noch kein Kind zugeordnet. Ordnen Sie ein Kind in der Kindersuche dieser Gruppe zu.",
    action: (
      <Button type="button" variant="outline" size="md" onClick={onOpenSearch}>
        Zur Kindersuche
      </Button>
    ),
  };
}
