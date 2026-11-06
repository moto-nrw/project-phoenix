import { AlertTriangle, Check, Plus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { databaseConcept } from "~/lib/concepts/database";
import { StatCard } from "~/components/ui/stat-card";

interface StatsCardsProps {
  readonly total: number;
  readonly newCount: number;
  readonly existing: number;
  /** Title of the "existing" card; update mode calls it "Wird aktualisiert". */
  readonly existingTitle?: string;
  readonly errors: number;
}

export function StatsCards({
  total,
  newCount,
  existing,
  existingTitle = "Vorhanden",
  errors,
}: StatsCardsProps) {
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4">
      <StatCard
        tone="neutral"
        label="Gesamt"
        value={total}
        icon={<MotoConceptIcon concept={databaseConcept} size={20} />}
      />
      <StatCard
        tone="neutral"
        label="Neu"
        value={newCount}
        icon={<Plus className="h-5 w-5 text-gray-700" aria-hidden="true" />}
      />
      <StatCard
        tone="neutral"
        label={existingTitle}
        value={existing}
        icon={<Check className="h-5 w-5 text-gray-700" aria-hidden="true" />}
      />
      <StatCard
        tone="neutral"
        label="Fehler"
        value={errors}
        icon={
          <AlertTriangle className="text-moto-red h-5 w-5" aria-hidden="true" />
        }
      />
    </div>
  );
}
