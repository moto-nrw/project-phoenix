interface StatsCardsProps {
  readonly total: number;
  readonly newCount: number;
  readonly existing: number;
  readonly errors: number;
}

export function StatsCards({
  total,
  newCount,
  existing,
  errors,
}: StatsCardsProps) {
  return (
    <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
      <div className="rounded-xl border border-gray-100 bg-white p-4">
        <p className="text-2xl font-bold text-gray-900">{total}</p>
        <p className="text-xs text-gray-600">Gesamt</p>
      </div>
      <div className="rounded-xl border border-green-100 bg-green-50 p-4">
        <p className="text-2xl font-bold text-green-700">{newCount}</p>
        <p className="text-xs text-green-600">Neu</p>
      </div>
      <div className="rounded-xl border border-blue-100 bg-blue-50 p-4">
        <p className="text-2xl font-bold text-blue-700">{existing}</p>
        <p className="text-xs text-blue-600">Vorhanden</p>
      </div>
      <div className="rounded-xl border border-red-100 bg-red-50 p-4">
        <p className="text-2xl font-bold text-red-700">{errors}</p>
        <p className="text-xs text-red-600">Fehler</p>
      </div>
    </div>
  );
}
