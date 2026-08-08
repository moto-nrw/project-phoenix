// Flat neutral pill for an offering attribute ("Mittagessen",
// "Ferienbetreuung", a price). Extracted from the care-offerings editor so the
// per-child views render the offering's attributes with exactly the badges
// staff already know from the Angebots-Katalog (#2185).
//
// Deliberately not StatusBadge: these are facts about the offering, not a
// status with a semantic tone, and a dotted tinted pill per attribute would
// drown the row.
export function FeaturePill({ label }: Readonly<{ label: string }>) {
  return (
    <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
      {label}
    </span>
  );
}
