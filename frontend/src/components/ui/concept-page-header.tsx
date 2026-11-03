import type { ReactNode } from "react";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import type { MotoConcept } from "~/lib/moto-concepts";
import { cn } from "~/lib/utils";

interface ConceptPageHeaderProps {
  readonly title: string;
  readonly concept: MotoConcept;
  readonly eyebrow?: ReactNode;
  readonly subtitle?: ReactNode;
  readonly className?: string;
}

/**
 * Seitenkopf mit Konzeptsymbol für Seiten außerhalb von `TenantPage`
 * (Operator-Portal). Abschnittsköpfe sind `SectionCard` mit `concept`.
 */
export function ConceptPageHeader({
  title,
  concept,
  eyebrow,
  subtitle,
  className,
}: ConceptPageHeaderProps) {
  return (
    <div className={cn("flex min-w-0 items-start gap-3 sm:gap-4", className)}>
      <ConceptIconTile concept={concept} variant="page" />
      <div className="min-w-0">
        {eyebrow ? (
          <div className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
            {eyebrow}
          </div>
        ) : null}
        <h1 className="text-2xl font-bold text-balance text-gray-900 sm:text-3xl">
          {title}
        </h1>
        {subtitle ? (
          <div className="mt-1 text-sm leading-6 text-gray-600">{subtitle}</div>
        ) : null}
      </div>
    </div>
  );
}
