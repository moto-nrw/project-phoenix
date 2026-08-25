import type { ReactNode } from "react";
import { AlertTriangle, Check, Plus } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

interface StatsCardsProps {
  readonly total: number;
  readonly newCount: number;
  readonly existing: number;
  /** Title of the "existing" card; update mode calls it "Wird aktualisiert". */
  readonly existingTitle?: string;
  readonly errors: number;
}

interface StatCardProps {
  readonly title: string;
  readonly value: number;
  readonly icon: ReactNode;
}

function StatCard({ title, value, icon }: StatCardProps) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div className="space-y-0.5">
          <p className="text-xs font-medium text-gray-600">{title}</p>
          <p className="text-2xl font-bold text-gray-900">{value}</p>
        </div>
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
          {icon}
        </div>
      </div>
    </div>
  );
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
        title="Gesamt"
        value={total}
        icon={<MotoConceptIcon concept="database" size={20} />}
      />
      <StatCard
        title="Neu"
        value={newCount}
        icon={<Plus className="h-5 w-5 text-gray-700" aria-hidden="true" />}
      />
      <StatCard
        title={existingTitle}
        value={existing}
        icon={<Check className="h-5 w-5 text-gray-700" aria-hidden="true" />}
      />
      <StatCard
        title="Fehler"
        value={errors}
        icon={
          <AlertTriangle className="text-moto-red h-5 w-5" aria-hidden="true" />
        }
      />
    </div>
  );
}
