import type { Weekday } from "./traeger-care-contracts-api";

const WEEKDAY_ORDER: readonly Weekday[] = [
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
];

/**
 * The weekdays a contract lists, as its PDF prints them: Monday to Friday
 * always, Saturday and Sunday only when booked or named in `include` (a
 * weekend day another version books). A listed day without services has no
 * care.
 */
export function contractWeekdays(
  services: readonly { readonly weekday: Weekday }[],
  include: readonly Weekday[] = [],
): Weekday[] {
  return WEEKDAY_ORDER.filter(
    (weekday, index) =>
      index < 5 ||
      include.includes(weekday) ||
      services.some((service) => service.weekday === weekday),
  );
}
