"use client";

import { useLocale, useTranslations } from "next-intl";
import {
  CollectionGrid,
  CollectionGridTile,
} from "~/components/ui/collection-grid";
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
      {/* Raster statt Zeilen (#4365): drei bis sechs kurze Einträge füllen
          die Breite, der Status steht direkt neben seinem Titel. */}
      <CollectionGrid as="ul">
        {visibleConsents.map((consent) => {
          const withdrawn = consent.state === "withdrawn";
          return (
            <CollectionGridTile
              key={consent.key}
              className="flex items-start justify-between gap-3"
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
              <StatusBadge
                label={t(`states.${withdrawn ? "withdrawn" : "granted"}`)}
                tone={withdrawn ? "red" : "green"}
              />
            </CollectionGridTile>
          );
        })}
      </CollectionGrid>
    </SectionCard>
  );
}
