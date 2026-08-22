import type { ComponentProps } from "react";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import type { MotoDuotoneTone } from "~/lib/location-helper";

interface MotoConceptIconProps extends Omit<
  ComponentProps<typeof MotoDuotoneIcon>,
  "icon" | "tone"
> {
  readonly concept: MotoConceptKey;
  /** Optional context tone when one surface deliberately unifies icon color. */
  readonly tone?: MotoDuotoneTone;
}

export function MotoConceptIcon({
  concept,
  tone,
  ...props
}: MotoConceptIconProps) {
  const definition = MOTO_CONCEPTS[concept];
  return (
    <MotoDuotoneIcon
      icon={definition.icon}
      tone={tone ?? definition.tone}
      weight={definition.weight}
      {...props}
    />
  );
}
