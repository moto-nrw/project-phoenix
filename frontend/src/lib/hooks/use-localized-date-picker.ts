"use client";

import { useMemo } from "react";
import { useLocale, useTranslations } from "next-intl";

import type { DatePickerLabels } from "~/lib/date-picker-labels";
import { dateFnsLocaleFor } from "~/i18n/date-fns-locale";

/**
 * Locale, labels and placeholder for a kit date picker on a parent-facing
 * surface (parent portal, public enrollment form).
 *
 * The kit defaults to German because the staff and operator portals are
 * German-only. Anything a guardian sees runs in de/en/ru/sq, so it has to pass
 * its resolved locale — spread the result onto the picker:
 *
 *   const datePicker = useLocalizedDatePicker();
 *   <ISODatePicker {...datePicker} value={from} onChange={setFrom} />
 */
export function useLocalizedDatePicker(): {
  readonly locale: ReturnType<typeof dateFnsLocaleFor>;
  readonly labels: DatePickerLabels;
  readonly placeholder: string;
} {
  const t = useTranslations("datePicker");
  const appLocale = useLocale();

  return useMemo(
    () => ({
      locale: dateFnsLocaleFor(appLocale),
      labels: {
        clear: t("clear"),
        previousMonth: t("previousMonth"),
        nextMonth: t("nextMonth"),
        month: t("month"),
        year: t("year"),
      },
      placeholder: t("placeholder"),
    }),
    [appLocale, t],
  );
}
