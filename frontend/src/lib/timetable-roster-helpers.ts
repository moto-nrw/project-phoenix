export function rosterPickupTimeLabel(
  pickupTime: string | null | undefined,
  pickupTimesLoaded: boolean | undefined,
  pickupTimesRedacted = false,
): string | null {
  if (pickupTimesRedacted) return null;
  if (pickupTimesLoaded === undefined) return null;
  if (!pickupTimesLoaded) return "Nicht geladen";
  return pickupTime ?? "—";
}
