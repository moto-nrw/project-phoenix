import type { ReactNode } from "react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { cn } from "~/lib/utils";

interface ConceptSectionHeaderProps {
  readonly title: string;
  readonly concept: MotoConceptKey;
  readonly subtitle?: ReactNode;
  readonly actions?: ReactNode;
  readonly className?: string;
}

interface ConceptPageHeaderProps {
  readonly title: string;
  readonly concept: MotoConceptKey;
  readonly eyebrow?: ReactNode;
  readonly subtitle?: ReactNode;
  readonly className?: string;
}

interface SectionHeaderProps {
  readonly title: string;
  readonly icon: ReactNode;
  readonly subtitle?: ReactNode;
  readonly actions?: ReactNode;
  readonly className?: string;
}

export function SectionHeader({
  title,
  icon,
  subtitle,
  actions,
  className,
}: SectionHeaderProps) {
  return (
    <div
      className={cn(
        "flex flex-wrap items-start justify-between gap-3",
        className,
      )}
    >
      <div className="flex min-w-0 items-start gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
          {icon}
        </div>
        <div className="min-w-0 pt-0.5">
          <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
            {title}
          </h2>
          {subtitle ? (
            <div className="mt-1 text-sm text-gray-600">{subtitle}</div>
          ) : null}
        </div>
      </div>
      {actions ? <div className="shrink-0">{actions}</div> : null}
    </div>
  );
}

export function ConceptSectionHeader({
  title,
  concept,
  subtitle,
  actions,
  className,
}: ConceptSectionHeaderProps) {
  return (
    <SectionHeader
      title={title}
      icon={<MotoConceptIcon concept={concept} size={22} />}
      subtitle={subtitle}
      actions={actions}
      className={className}
    />
  );
}

export function ConceptPageHeader({
  title,
  concept,
  eyebrow,
  subtitle,
  className,
}: ConceptPageHeaderProps) {
  return (
    <div className={cn("flex min-w-0 items-start gap-3 sm:gap-4", className)}>
      <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-12 sm:w-12">
        <MotoConceptIcon concept={concept} size={26} />
      </div>
      <div className="min-w-0">
        {eyebrow ? (
          <div className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {eyebrow}
          </div>
        ) : null}
        <h1 className="text-2xl font-semibold text-balance text-gray-900 sm:text-3xl">
          {title}
        </h1>
        {subtitle ? (
          <div className="mt-1 text-sm leading-6 text-gray-600">{subtitle}</div>
        ) : null}
      </div>
    </div>
  );
}
