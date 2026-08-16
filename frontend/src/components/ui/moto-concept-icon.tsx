import type { ComponentProps } from "react";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";

interface MotoConceptIconProps extends Omit<
  ComponentProps<typeof MotoDuotoneIcon>,
  "icon" | "tone"
> {
  readonly concept: MotoConceptKey;
}

export function MotoConceptIcon({ concept, ...props }: MotoConceptIconProps) {
  const definition = MOTO_CONCEPTS[concept];
  return (
    <MotoDuotoneIcon
      icon={definition.icon}
      tone={definition.tone}
      weight={definition.weight}
      {...props}
    />
  );
}
