"use client";

import { ArrowRight } from "lucide-react";
import { SectionHeader } from "~/components/ui/concept-section-header";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TileCard } from "~/components/ui/tile-card";
import { cn } from "~/lib/utils";

/**
 * Der Einstieg zur Seite „Unterwegs“ (Kinder ohne Raumzuweisung). Steht auf
 * „Räume“ und, für alle mit Zuweisungsrecht, auf „Alle Kinder“ (#5026).
 */
export function TransitAssignmentCard({
  count,
  href,
  className,
}: {
  readonly count: number;
  readonly href: string;
  /** Abstand zur Umgebung, etwa unter den Hinweisen auf „Alle Kinder“. */
  readonly className?: string;
}) {
  return (
    <TileCard
      href={href}
      className={cn("flex items-center justify-between gap-4", className)}
    >
      <SectionHeader
        className="min-w-0 flex-1"
        title="Unterwegs"
        icon={<MotoConceptIcon concept="transit" size={22} />}
        subtitle={
          <>
            <span className="font-medium text-gray-900">{count}</span>{" "}
            {count === 1 ? "Kind" : "Kinder"} ohne Raumzuweisung
          </>
        }
        actions={
          <span className="moto-content-surface inline-flex shrink-0 items-center gap-2 rounded-full border px-3 py-2 text-sm font-medium text-gray-700 transition-colors group-hover:border-gray-300 group-hover:bg-gray-50">
            Zuweisen
            <ArrowRight className="h-4 w-4 text-gray-500" aria-hidden="true" />
          </span>
        }
      />
    </TileCard>
  );
}
