import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";

export function ParentNavIcon({
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
