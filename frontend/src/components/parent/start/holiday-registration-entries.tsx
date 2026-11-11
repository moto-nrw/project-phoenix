"use client";

import { useTranslations } from "next-intl";
import { ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import {
  HolidayRegistrationStatusBadge,
  useEntryLine,
  useHolidayRegistrationEntries,
} from "~/components/parent/holiday-registration-entry-parts";
import { parentPath } from "~/lib/parent-url";

// Dasselbe Polster wie die Karten darüber und darunter (holiday-offers.tsx).
const CARD_PADDING = "compact:p-5 max-sm:p-5";

/** So viele Anmeldungen zeigt die Startseite; der Rest steht auf der Seite. */
const SHOWN = 3;

/**
 * „Ihre Ferienanmeldungen“ auf der Startseite (#6673): der Stand der
 * eigenen Anmeldungen auf einen Blick und der Weg zur Seite. Die Zeilen
 * sind Anzeige, nur der Knopf führt weiter. Ohne Anmeldung, beim Laden und
 * bei einem Fehler rendert sie nichts: Die Seite selbst zeigt den Fehler.
 */
export function HolidayRegistrationEntriesCard() {
  const t = useTranslations("parentHolidayRegistrations");
  const { data } = useHolidayRegistrationEntries();
  const line = useEntryLine();
  if (!data || data.length === 0) return null;
  const shown = data.slice(0, SHOWN);
  const more = data.length - shown.length;
  return (
    <SectionCard
      testId="parent-holiday-registration-entries"
      title={t("card.title")}
      className={CARD_PADDING}
      inlineActions
      action={
        <ButtonLink
          href={parentPath("/parents/ferienanmeldungen")}
          variant="outline"
          size="compact"
          aria-label={t("card.showAllLabel")}
        >
          {t("card.showAll")}
        </ButtonLink>
      }
    >
      <ul className="divide-y divide-gray-100">
        {shown.map((entry) => (
          <li
            key={entry.id}
            className="flex flex-wrap items-start justify-between gap-x-3 gap-y-1 py-3 first:pt-0 last:pb-0"
          >
            <div className="min-w-0">
              <p className="text-sm font-semibold text-gray-900">
                {entry.periodName}
              </p>
              <p className="text-sm leading-6 text-gray-600">{line(entry)}</p>
            </div>
            <HolidayRegistrationStatusBadge status={entry.status} />
          </li>
        ))}
      </ul>
      {more > 0 ? (
        <p className="mt-3 text-sm text-gray-600">
          {t("card.more", { count: more })}
        </p>
      ) : null}
    </SectionCard>
  );
}
