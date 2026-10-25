"use client";

// Zusammenfassung vor dem Absenden der Ferienanmeldung (#4661).

import { useLocale, useTranslations } from "next-intl";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import {
  bookedWeeks,
  type ContactDraft,
  OTHER_SCHOOL,
  type RegistrationDraft,
} from "./holiday-registration-model";
import { useWeekLabels } from "./holiday-registration-sections";

function contactLine(contact: ContactDraft): string {
  return [contact.name, contact.relationship, contact.phone]
    .map((part) => part.trim())
    .filter(Boolean)
    .join(" · ");
}

export function HolidayRegistrationSummary({
  draft,
  offer,
  submitting,
  onBack,
  onSubmit,
}: Readonly<{
  draft: RegistrationDraft;
  offer: HolidayRegistrationOffer;
  submitting: boolean;
  onBack: () => void;
  onSubmit: () => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  const labels = useWeekLabels();
  const none = t("none");
  const schoolName = (school: string, guestSchoolName: string) =>
    school === OTHER_SCHOOL
      ? `${t("guest")}${guestSchoolName.trim() ? ` · ${guestSchoolName.trim()}` : ""}`
      : (offer.schools.find((candidate) => candidate.id === school)?.name ??
        none);

  return (
    <div className="space-y-6">
      <SectionCard title={t("summaryTitle")} description={t("summaryText")}>
        <p className="text-sm text-gray-700">{t("summaryFee")}</p>
      </SectionCard>

      {draft.children.map((child, index) => (
        <SectionCard
          key={child.key}
          title={`${child.firstName.trim()} ${child.lastName.trim()}`}
          headingLevel={3}
        >
          <DataGrid>
            <DataField label={t("birthDate")}>
              {formatDate(child.birthDate, false, locale)}
            </DataField>
            <DataField label={t("school")}>
              {schoolName(child.school, child.guestSchoolName)}
            </DataField>
            {bookedWeeks(child, offer).map((booking) => {
              const week = offer.weeks.find((w) => w.id === booking.weekId);
              const model = week?.timeModels.find(
                (m) => m.id === booking.timeModelId,
              );
              if (!week || !model) return null;
              return (
                <DataField
                  key={`${index}-${week.id}`}
                  label={labels.range(week)}
                >
                  {`${labels.timeModel(model)} · ${week.hostSchoolName}`}
                </DataField>
              );
            })}
            <DataField label={t("allergies")}>
              {child.allergies.trim() || none}
            </DataField>
            <DataField label={t("medication")}>
              {child.medication.trim() || none}
            </DataField>
            <DataField label={t("healthNotes")}>
              {child.healthNotes.trim() || none}
            </DataField>
          </DataGrid>
        </SectionCard>
      ))}

      <SectionCard title={t("guardianTitle")} headingLevel={3}>
        <DataGrid>
          <DataField
            label={t("name")}
          >{`${draft.guardian.firstName.trim()} ${draft.guardian.lastName.trim()}`}</DataField>
          <DataField label={t("email")}>
            {draft.guardian.email.trim()}
          </DataField>
          <DataField label={t("phone")}>
            {draft.guardian.phone.trim()}
          </DataField>
          <DataField label={t("emergencyTitle")}>
            {draft.emergencyContacts.map(contactLine).join("; ")}
          </DataField>
          <DataField label={t("pickupTitle")}>
            {draft.pickupPersons.map(contactLine).join("; ") || none}
          </DataField>
        </DataGrid>
      </SectionCard>

      <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={submitting}
          onClick={onBack}
        >
          {t("back")}
        </Button>
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={submitting}
          onClick={onSubmit}
        >
          {submitting
            ? t("submitting")
            : offer.binding
              ? t("submitBinding")
              : t("submit")}
        </Button>
      </div>
    </div>
  );
}
