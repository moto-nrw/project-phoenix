import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";

/**
 * Das Navigations-Icon der Portal-Hüllen.
 *
 * Aktiv wird ein Ziel farbig (Duotone im Ton seines Konzepts), inaktiv bleibt
 * es eine schlichte Kontur. Diese Unterscheidung ist der Grund, warum es
 * neben `MotoConceptIcon` steht: dort ist ein Icon immer farbig, hier trägt
 * die Farbe die Aussage "hier bin ich gerade".
 *
 * Eltern-Portal (#1671) und Schul-Portal (#2207) rendern beide daraus.
 */
export function MotoNavIcon({
  concept,
  iconConcept,
  active,
  className,
}: Readonly<{
  concept: MotoConceptKey;
  iconConcept?: MotoConceptKey;
  active: boolean;
  className?: string;
}>) {
  const definition = MOTO_CONCEPTS[concept];
  const Icon = MOTO_CONCEPTS[iconConcept ?? concept].icon;

  if (active) {
    return (
      <MotoDuotoneIcon
        icon={Icon}
        tone={definition.tone}
        size={22}
        className={className}
      />
    );
  }

  return (
    <Icon size={22} weight="regular" className={className} aria-hidden="true" />
  );
}
