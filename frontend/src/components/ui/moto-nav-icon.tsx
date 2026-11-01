import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { NavSceneIcon } from "~/components/ui/nav-scene-icon/nav-scene-icon";
import type { MotoConcept } from "~/lib/moto-concepts";

/**
 * Das Navigations-Icon der Portal-Hüllen.
 *
 * Aktiv wird ein Ziel farbig (Duotone im Ton seines Konzepts), inaktiv bleibt
 * es eine schlichte Kontur. Diese Unterscheidung ist der Grund, warum es
 * neben `MotoConceptIcon` steht: dort ist ein Icon immer farbig, hier trägt
 * die Farbe die Aussage "hier bin ich gerade".
 *
 * Eltern-Portal (#1671) und Schul-Portal (#2207) rendern beide daraus.
 *
 * `scene` zeichnet das Icon als Szene, die beim Überfahren der Zeile spielt
 * (`nav-scene-icon/README.md`). Jede Seitenleiste setzt es.
 */
export function MotoNavIcon({
  concept,
  iconConcept,
  active,
  scene = false,
  className,
}: Readonly<{
  concept: MotoConcept;
  iconConcept?: MotoConcept;
  active: boolean;
  scene?: boolean;
  className?: string;
}>) {
  const Icon = (iconConcept ?? concept).icon;

  if (scene) {
    return (
      <NavSceneIcon
        icon={Icon}
        tone={concept.tone}
        active={active}
        size={22}
        className={className}
      />
    );
  }

  if (active) {
    return (
      <MotoDuotoneIcon
        icon={Icon}
        tone={concept.tone}
        size={22}
        className={className}
      />
    );
  }

  return (
    <Icon size={22} weight="regular" className={className} aria-hidden="true" />
  );
}
