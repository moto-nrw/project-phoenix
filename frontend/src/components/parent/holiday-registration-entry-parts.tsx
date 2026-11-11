"use client";

// Teile der eigenen Ferienanmeldungen (#6673), geteilt von der Seite
// „Ferienanmeldungen“ und der Karte auf der Startseite.

import { useLocale, useTranslations } from "next-intl";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { PARENT_RESOURCE_OPTIONS } from "~/lib/hooks/use-parent-resources";
import { listHolidayRegistrationEntries } from "~/lib/parent-api";
import type {
  HolidayRegistrationEntry,
  HolidayRegistrationEntryStatus,
} from "~/lib/parent-holiday-registration-entries";
import { useSWRAuth } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";

const STATUS_TONES = {
  received: "blue",
  confirmed: "green",
  cancelled: "gray",
} as const satisfies Record<HolidayRegistrationEntryStatus, StatusBadgeTone>;

/** The state of a registration as one plain word. */
export function HolidayRegistrationStatusBadge({
  status,
}: Readonly<{ status: HolidayRegistrationEntryStatus }>) {
  const t = useTranslations("parentHolidayRegistrations");
  return (
    <StatusBadge label={t(`status.${status}`)} tone={STATUS_TONES[status]} />
  );
}

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
