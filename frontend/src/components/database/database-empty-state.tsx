"use client";

import type { ReactNode } from "react";
import { EmptyState } from "~/components/ui/empty-state";

interface DatabaseEmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
}

/**
 * Leerzustand der Datenverwaltung: der geteilte `EmptyState` aus dem UI-Kit,
 * vertikal in der Master-Detail-Fläche zentriert. Die Props bleiben bewusst
 * unverändert, damit die acht Seiten der Datenverwaltung ihn weiterhin gleich
 * aufrufen.
 */
export function DatabaseEmptyState({
  icon,
  title,
  description,
}: Readonly<DatabaseEmptyStateProps>) {
  return (
    <div className="flex min-h-[300px] items-center justify-center">
      <EmptyState icon={icon} title={title} description={description} />
    </div>
  );
}
