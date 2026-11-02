import type { ComponentProps } from "react";
import type { MotoConcept } from "~/lib/moto-concepts";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import type { MotoDuotoneTone } from "~/lib/location-helper";

interface MotoConceptIconProps extends Omit<
  ComponentProps<typeof MotoDuotoneIcon>,
  "icon" | "tone"
> {
  /**
   * The concept to show. Import it from its own module
   * (`~/lib/concepts/<key>`) so the page carries only this glyph.
   */
  readonly concept: MotoConcept;
  /** Optional context tone when one surface deliberately unifies icon color. */
  readonly tone?: MotoDuotoneTone;
}

export function MotoConceptIcon({
  concept,
  tone,
  ...props
}: MotoConceptIconProps) {
  return (
    <MotoDuotoneIcon
      icon={concept.icon}
      tone={tone ?? concept.tone}
      weight={concept.weight}
      {...props}
    />
  );
}
