// Shared rendering of care-offering attributes, so the staff views and the
// Angebots-Katalog phrase the same fact identically (#2185).

const EURO_PRICE_FORMATTER = new Intl.NumberFormat("de-DE", {
  style: "currency",
  currency: "EUR",
});

/** Cents → "12,50 €". Null/undefined yields null, so callers can omit the badge. */
export function formatOfferingPrice(cents?: number | null): string | null {
  if (cents == null) return null;
  return EURO_PRICE_FORMATTER.format(cents / 100);
}
