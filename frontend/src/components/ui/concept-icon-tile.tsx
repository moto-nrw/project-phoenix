import type { ComponentProps } from "react";

import { IconTile } from "~/components/ui/icon-tile";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConcept } from "~/lib/moto-concepts";
import type { MotoDuotoneTone } from "~/lib/location-helper";

interface ConceptIconTileProps extends Omit<
  ComponentProps<typeof IconTile>,
  "icon"
> {
  readonly concept: MotoConcept;
  /** Abweichender Symbolton, wenn eine Fläche die Symbole bewusst angleicht. */
  readonly iconTone?: MotoDuotoneTone;
}

/**
 * `IconTile` mit dem Symbol eines fachlichen Konzepts (siehe `SectionCard`
 * und den Seitenkopf). Größe, Form, Fläche und Lage-Klassen gehen
 * unverändert an `IconTile`.
 */
export function ConceptIconTile({
  concept,
  iconTone,
  ...tile
}: ConceptIconTileProps) {
  return (
    <IconTile
      {...tile}
      icon={<MotoConceptIcon concept={concept} tone={iconTone} />}
    />
  );
}
