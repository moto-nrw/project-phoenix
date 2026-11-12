"use client";

// Teile der eigenen Ferienanmeldungen (#6673), geteilt von der Seite
// „Ferienanmeldungen“ und der Karte auf der Startseite.

import { useLocale, useTranslations } from "next-intl";
import { formatDate } from "~/lib/date-helpers";
import { PARENT_RESOURCE_OPTIONS } from "~/lib/hooks/use-parent-resources";
import { listHolidayRegistrationEntries } from "~/lib/parent-api";
import type { HolidayRegistrationEntry } from "~/lib/parent-holiday-registration-entries";
import { useSWRAuth } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";

/** The registrations of the account, shared by the page and the start card. */
export function useHolidayRegistrationEntries() {
  return useSWRAuth(
    parentKeys.holidayRegistrationEntries(),
    listHolidayRegistrationEntries,
    PARENT_RESOURCE_OPTIONS,
  );
}

/** "Träger · 04.01.2027 bis 08.01.2027", without the dates when unknown. */
export function useEntryLine() {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  return (entry: HolidayRegistrationEntry) =>
    entry.firstDay && entry.lastDay
      ? `${entry.traegerName} · ${t("weekRange", {
          starts: formatDate(entry.firstDay, false, locale),
          ends: formatDate(entry.lastDay, false, locale),
        })}`
      : entry.traegerName;
}
