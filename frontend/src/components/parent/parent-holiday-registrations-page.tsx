"use client";

/**
 * Ferienanmeldungen (#6673, ADR 0216): the holiday registrations linked to
 * the account, newest first, with their state and the place of every week.
 * The page only shows; the Träger decides. A registration the Träger
 * confirmed points to the documents, where the confirmation and the invoice
 * arrive.
 */

import { useLocale, useTranslations } from "next-intl";
import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import {
  HOLIDAY_PLACEMENT_BADGES,
  HOLIDAY_REGISTRATION_STATUS_TONES,
} from "~/components/holiday-registration/holiday-registration-badges";
import { StatusBadge } from "~/components/ui/status-badge";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { formatDate } from "~/lib/date-helpers";
import { useSwrResourceLoadError } from "~/lib/hooks/use-swr-load-error";
import type { HolidayRegistrationEntry } from "~/lib/parent-holiday-registration-entries";
import { parentPath } from "~/lib/parent-url";
import {
  useEntryLine,
  useHolidayRegistrationEntries,
} from "./holiday-registration-entry-parts";

export function ParentHolidayRegistrationsPage() {
  const t = useTranslations("parentHolidayRegistrations");
  const entries = useHolidayRegistrationEntries();
  const { error } = useSwrResourceLoadError([entries], t("errorObject"), () =>
    entries.mutate(),
  );
  const loading = !entries.data && !entries.error;

  return (
    <ParentPage
      header={
        <ParentPageHeader title={t("title")} description={t("description")} />
      }
      loading={loading}
      error={loading ? null : error}
      empty={
        entries.data?.length === 0
          ? {
              icon: <MotoConceptIcon concept={holidayWeeksConcept} />,
              title: t("emptyTitle"),
              description: t("emptyDescription"),
            }
          : null
      }
    >
      {entries.data && entries.data.length > 0 ? (
        <ul className="space-y-5">
          {entries.data.map((entry) => (
            <li key={entry.id}>
              <HolidayRegistrationCard entry={entry} />
            </li>
          ))}
        </ul>
      ) : null}
    </ParentPage>
  );
}

function HolidayRegistrationCard({
  entry,
}: Readonly<{ entry: HolidayRegistrationEntry }>) {
  const t = useTranslations("parentHolidayRegistrations");
  const tr = useTranslations("holidayRegistration");
  const locale = useLocale();
  const line = useEntryLine();
  const active = entry.status !== "cancelled";
  const weeks = entry.children.flatMap((child) => child.weeks);
  const waitlisted =
    active && weeks.some((week) => week.placement === "waitlisted");
  const pending = active && weeks.some((week) => week.placement === "pending");

  return (
    <SectionCard
      concept={holidayWeeksConcept}
      title={entry.periodName}
      titleBadge={
        <StatusBadge
          label={t(`status.${entry.status}`)}
          tone={HOLIDAY_REGISTRATION_STATUS_TONES[entry.status]}
        />
      }
      description={line(entry)}
      namedRegion
    >
      <div className="space-y-5">
        <div className="space-y-1 text-sm leading-6 text-gray-700">
          <p className="text-gray-600">
            {t("submittedOn", {
              date: formatDate(entry.submittedAt, false, locale),
            })}
          </p>
          <p>{t(`statusNote.${entry.status}`)}</p>
        </div>

        {entry.children.length > 0 ? (
          <ul className="space-y-4">
            {entry.children.map((child, index) => (
              <li key={`${child.firstName}-${index}`} className="space-y-2">
                <SectionLabel as="h3">{child.firstName}</SectionLabel>
                <ul className="divide-y divide-gray-100">
                  {child.weeks.map((week) => (
                    <li
                      key={`${week.firstDay}-${week.lastDay}`}
                      className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 py-2 text-sm text-gray-700 first:pt-0 last:pb-0"
                    >
                      <span className="min-w-0">
                        {[
                          tr("weekRange", {
                            starts: formatDate(week.firstDay, false, locale),
                            ends: formatDate(week.lastDay, false, locale),
                          }),
                          week.timeModelName,
                        ]
                          .filter(Boolean)
                          .join(" · ")}
                      </span>
                      {active ? (
                        <StatusBadge
                          label={tr(
                            HOLIDAY_PLACEMENT_BADGES[week.placement].key,
                          )}
                          tone={HOLIDAY_PLACEMENT_BADGES[week.placement].tone}
                          size="xs"
                        />
                      ) : null}
                    </li>
                  ))}
                </ul>
              </li>
            ))}
          </ul>
        ) : null}

        {pending ? <Alert type="info" message={tr("pendingNote")} /> : null}
        {waitlisted ? <Alert type="info" message={tr("waitlistNote")} /> : null}

        {entry.status === "confirmed" ? (
          <div className="flex flex-col gap-3 border-t border-gray-100 pt-4 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm leading-6 text-gray-700">
              {t("documentsHint")}
            </p>
            <ButtonLink
              href={parentPath("/parents/documents")}
              variant="outline"
              size="md"
              className="shrink-0 max-sm:w-full"
              tooltip={t("toDocumentsTooltip")}
            >
              {t("toDocuments")}
            </ButtonLink>
          </div>
        ) : null}
      </div>
    </SectionCard>
  );
}
