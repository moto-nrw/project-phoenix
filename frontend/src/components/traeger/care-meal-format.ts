// Texte und Zahlen der Mensa-Abrechnung (#4686): Abrechnungsart, Preis je
// Monat und die Stände einer Schule.

import type { CareMealMode, CareMealTariff } from "~/lib/traeger-care-meal-api";
import { formatMonth } from "./care-billing-format";

export const MEAL_MODE_LABELS: Record<CareMealMode, string> = {
  per_meal: "Je Essen",
  flat: "Pauschale",
};

/** The prices of a school, the earliest first. */
export function mealStands(
  tariffs: readonly CareMealTariff[],
  schoolId: string,
): CareMealTariff[] {
  return tariffs
    .filter((tariff) => tariff.schoolId === schoolId)
    .sort((a, b) => a.validFrom.localeCompare(b.validFrom));
}

export type MealStandStatus = "current" | "planned" | "earlier";

export const MEAL_STAND_LABELS: Record<MealStandStatus, string> = {
  current: "Gilt jetzt",
  planned: "Geplant",
  earlier: "Früher",
};

/** Whether a price holds in the current month, starts later or ended. */
export function mealStandStatus(
  tariff: CareMealTariff,
  currentMonth: string,
): MealStandStatus {
  if (tariff.validFrom > currentMonth) return "planned";
  if (tariff.validUntil && tariff.validUntil < currentMonth) return "earlier";
  return "current";
}

/**
 * The line of a Mensa position as MK-001 writes it: „2 Mittagessen im
 * Januar 2026“, or the flat amount of the month.
 */
export function mealPositionLabel(
  mealMonth: string,
  mode: CareMealMode,
  quantity: number,
): string {
  if (mode === "flat") return `Mensa-Pauschale ${formatMonth(mealMonth)}`;
  return `${quantity} Mittagessen im ${formatMonth(mealMonth)}`;
}
