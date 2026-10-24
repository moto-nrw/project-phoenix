"use client";

import { useTranslations } from "next-intl";

import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import {
  MEAL_ALLERGENS,
  mealAllergenLetter,
  type MealAllergen,
} from "~/lib/meal-allergens";
import { cn } from "~/lib/utils";

/**
 * The allergen marks of one dish: the letters for the eye, the full names
 * for screen readers. Renders nothing for an unmarked dish.
 */
export function MealAllergenMarks({
  allergens,
  className,
}: {
  /** Missing on payloads that predate the marking: shown as unmarked. */
  readonly allergens: readonly MealAllergen[] | undefined;
  readonly className?: string;
}) {
  const t = useTranslations("mealAllergens");
  if (!allergens?.length) return null;
  const names = allergens.map((code) => t(`names.${code}`)).join(", ");
  return (
    <p className={cn("text-xs leading-snug text-gray-500", className)}>
      <span className="sr-only">{t("marksLabel", { names })}</span>
      <span aria-hidden="true" title={names}>
        {t("marksShort", {
          letters: allergens.map(mealAllergenLetter).join(", "),
        })}
      </span>
    </p>
  );
}

/**
 * The legend of the letters used this week, with the line that says where
 * the information comes from. Read-only: it carries no affordance. The staff
 * plan leaves the source line out: the staff enter the information.
 */
export function MealAllergenLegend({
  allergens,
  showSource = true,
  className,
}: {
  readonly allergens: readonly MealAllergen[];
  readonly showSource?: boolean;
  readonly className?: string;
}) {
  const t = useTranslations("mealAllergens");
  return (
    <div className={cn("text-sm leading-relaxed text-gray-600", className)}>
      {allergens.length > 0 ? (
        <ul
          aria-label={t("legendTitle")}
          className="flex flex-wrap gap-x-4 gap-y-1"
        >
          {allergens.map((code) => (
            <li key={code}>
              <span className="font-semibold text-gray-900">
                {mealAllergenLetter(code)}
              </span>{" "}
              {t(`names.${code}`)}
            </li>
          ))}
        </ul>
      ) : (
        <p>{t("noneThisWeek")}</p>
      )}
      {showSource ? (
        <p className="mt-1.5 text-gray-500">{t("source")}</p>
      ) : null}
    </div>
  );
}

/**
 * The multiple choice of a dish's allergens on the staff meal plan. The
 * narrow day column shows only the letters; the button's name carries the
 * full names for screen readers.
 */
export function MealAllergenSelect({
  value,
  onChange,
  menuAlign,
  className,
}: {
  readonly value: readonly MealAllergen[];
  readonly onChange: (codes: string[]) => void;
  readonly menuAlign?: "start" | "end";
  readonly className?: string;
}) {
  const t = useTranslations("mealAllergens");
  const options = MEAL_ALLERGENS.map(({ code, letter }) => ({
    value: code,
    label: `${t(`names.${code}`)} (${letter})`,
  }));
  const names = value.map((code) => t(`names.${code}`)).join(", ");
  return (
    <MultiCheckboxSelect
      value={value}
      options={options}
      onChange={onChange}
      ariaLabel={names ? `${t("legendTitle")}: ${names}` : t("choose")}
      emptyLabel={t("legendTitle")}
      summaryLabel={(selected) =>
        selected
          .map((option) => mealAllergenLetter(option.value as MealAllergen))
          .join(", ")
      }
      className={cn("h-8 text-xs", className)}
      menuClassName="w-72"
      menuAlign={menuAlign}
    />
  );
}
