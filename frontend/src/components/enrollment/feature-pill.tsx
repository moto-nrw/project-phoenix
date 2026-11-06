import { StatusBadge } from "~/components/ui/status-badge";
// Flat neutral pill for an offering attribute ("Mittagessen",
// "Ferienbetreuung", a price). Extracted from the care-offerings editor so the
// per-child views render the offering's attributes with exactly the badges
// staff already know from the Angebots-Katalog (#2185).
//
// Always gray: these are facts about the offering, not a status with a semantic
// tone, and a tinted pill per attribute would drown the row.
export function FeaturePill({ label }: Readonly<{ label: string }>) {
  return <StatusBadge tone="gray" compact label={label} />;
}
