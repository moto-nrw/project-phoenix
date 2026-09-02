"use client";

import { useLocale, useTranslations } from "next-intl";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { StudentConsent } from "~/lib/student-helpers";

export function StudentConsentsReadOnly({
  consents,
}: Readonly<{ consents?: readonly StudentConsent[] }>) {
  const t = useTranslations("parentChild.consents");
  const locale = useLocale();
  const visibleConsents =
    consents?.filter((consent) => consent.state !== "not_recorded") ?? [];

  if (visibleConsents.length === 0) return null;

  return (
    <SectionCard
      id="student-consents"
      title={t("title")}
      description={t("staffDescription")}
    >
      <ul className="divide-y divide-gray-100">
        {visibleConsents.map((consent) => {
          const withdrawn = consent.state === "withdrawn";
          return (
            <li
              key={consent.key}
              className="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">
                  {t(`items.${consent.key}`)}
                </p>
                {consent.changed_at ? (
                  <p className="mt-1 text-sm text-gray-600">
                    {t(`dates.${withdrawn ? "withdrawn" : "granted"}`, {
                      date: formatBerlinDate(consent.changed_at, locale),
                    })}
                  </p>
                ) : null}
              </div>
              <div className="flex items-start sm:items-end">
                <StatusBadge
                  label={t(`states.${withdrawn ? "withdrawn" : "granted"}`)}
                  tone={withdrawn ? "red" : "green"}
                />
              </div>
            </li>
          );
        })}
      </ul>
    </SectionCard>
  );
}
