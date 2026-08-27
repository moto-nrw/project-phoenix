export function rosterPickupTimeLabel(
  pickupTime: string | null | undefined,
  pickupTimesLoaded: boolean | undefined,
): string | null {
  if (pickupTimesLoaded === undefined) return null;
  if (!pickupTimesLoaded) return "Nicht geladen";
  return pickupTime ?? "—";
}
