interface StatsCardsProps {
  readonly total: number;
  readonly newCount: number;
  readonly existing: number;
  readonly errors: number;
}

// Icon component matching dashboard style
const Icon: React.FC<{ path: string; className?: string }> = ({
  path,
  className,
}) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

// Stat Card matching dashboard style
interface StatCardProps {
  readonly title: string;
  readonly value: number;
  readonly icon: string;
  readonly colorClass: string;
  readonly overlayClass: string;
}

const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  colorClass,
  overlayClass,
}) => (
  <div className="relative overflow-hidden rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_4px_20px_rgb(0,0,0,0.08)] backdrop-blur-md transition-all duration-300 hover:-translate-y-0.5 hover:shadow-[0_8px_30px_rgb(0,0,0,0.12)]">
    <div
      className={`pointer-events-none absolute inset-0 bg-gradient-to-br ${overlayClass} rounded-2xl opacity-[0.03]`}
    />
    <div className="pointer-events-none absolute inset-px rounded-2xl bg-gradient-to-br from-white/80 to-white/20" />

    <div className="relative p-4">
      <div className="flex items-start justify-between">
        <div className="space-y-0.5">
          <p className="text-xs font-medium text-gray-600">{title}</p>
          <p className="text-2xl font-bold text-gray-900">{value}</p>
        </div>
        <div
          className={`rounded-xl bg-gradient-to-br ${colorClass} p-2 text-white shadow-md`}
        >
          <Icon path={icon} className="h-4 w-4" />
        </div>
      </div>
    </div>
  </div>
);

export function StatsCards({
  total,
  newCount,
  existing,
  errors,
}: StatsCardsProps) {
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4 md:gap-4">
      {/* Gesamt - Gray/neutral */}
      <StatCard
        title="Gesamt"
        value={total}
        icon="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
        colorClass="from-gray-500 to-gray-600"
        overlayClass="from-gray-50/80 to-slate-100/80"
      />

      {/* Neu - Green (moto green) */}
      <StatCard
        title="Neu"
        value={newCount}
        icon="M12 4v16m8-8H4"
        colorClass="from-[#83CD2D] to-[#70b525]"
        overlayClass="from-green-50/80 to-lime-100/80"
      />

      {/* Vorhanden - Blue */}
      <StatCard
        title="Vorhanden"
        value={existing}
        icon="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
        colorClass="from-[#5080D8] to-[#4070c8]"
        overlayClass="from-blue-50/80 to-cyan-100/80"
      />

      {/* Fehler - Red */}
      <StatCard
        title="Fehler"
        value={errors}
        icon="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
        colorClass="from-[#FF3130] to-[#e02020]"
        overlayClass="from-red-50/80 to-rose-100/80"
      />
    </div>
  );
}
