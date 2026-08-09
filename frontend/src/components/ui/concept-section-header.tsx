import type { ReactNode } from "react";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { cn } from "~/lib/utils";

/**
 * Ueberschriftenebene des Abschnitts. Default 2, weil der haeufigste Fall ein
 * Abschnitt direkt unter dem h1 der Seite ist. Muss dort mitgegeben werden, wo
 * die Umgebung eine andere Ebene vorgibt: in einem Dialog steht der Titel als
 * h3 (form-modal, modal), und Karten innerhalb eines Abschnitts bringen
 * eigene Unterebenen mit. Eine feste h2 erzeugt sonst Ebenenspruenge, die
 * Screenreader als fehlende Zwischenebene vorlesen.
 */
type SectionHeadingLevel = 2 | 3 | 4;

interface ConceptSectionHeaderProps {
  readonly title: string;
  readonly concept: MotoConceptKey;
  readonly subtitle?: ReactNode;
  readonly actions?: ReactNode;
  readonly className?: string;
  readonly level?: SectionHeadingLevel;
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
  readonly level?: SectionHeadingLevel;
}

interface SectionHeaderShellProps {
  readonly iconTile: ReactNode;
  readonly title: string;
  readonly subtitle?: ReactNode;
  readonly actions?: ReactNode;
  readonly className?: string;
  readonly level?: SectionHeadingLevel;
}

function SectionHeaderShell({
  iconTile,
  title,
  subtitle,
  actions,
  className,
  level = 2,
}: SectionHeaderShellProps) {
  const Heading = `h${level}` as const;
  return (
    <div
      className={cn(
        "flex flex-wrap items-start justify-between gap-3",
        className,
      )}
    >
      <div className="flex min-w-0 items-start gap-3">
        {iconTile}
        <div className="min-w-0 pt-0.5">
          <Heading className="text-base font-semibold text-gray-900 sm:text-lg">
            {title}
          </Heading>
          {subtitle ? (
            <div className="mt-1 text-sm text-gray-600">{subtitle}</div>
          ) : null}
        </div>
      </div>
      {actions ? <div className="shrink-0">{actions}</div> : null}
    </div>
  );
}

export function SectionHeader({
  title,
  icon,
  subtitle,
  actions,
  className,
  level,
}: SectionHeaderProps) {
  return (
    <SectionHeaderShell
      iconTile={
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
          {icon}
        </div>
      }
      title={title}
      subtitle={subtitle}
      actions={actions}
      className={className}
      level={level}
    />
  );
}

export function ConceptSectionHeader({
  title,
  concept,
  subtitle,
  actions,
  className,
  level,
}: ConceptSectionHeaderProps) {
  return (
    <SectionHeaderShell
      iconTile={<ConceptIconTile concept={concept} variant="section" />}
      title={title}
      subtitle={subtitle}
      actions={actions}
      className={className}
      level={level}
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
      <ConceptIconTile concept={concept} variant="page" />
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
