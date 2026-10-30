"use client";

// Zusammenfassung vor dem Absenden der Ferienanmeldung (#4661), mit dem
// Beitrag aus den Regeln des Trägers (#4664).

import { useLocale, useTranslations } from "next-intl";
import { type ReactNode, useCallback } from "react";
import {
  EnrollmentUploadScope,
  useEnrollmentUploadTarget,
} from "~/components/enrollment/enrollment-file-upload";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import type { NewUploadValue } from "~/lib/enrollment-uploads";
import {
  type HolidayRegistrationOffer,
  uploadHolidayProof,
} from "~/lib/holiday-registration-api";
import {
  bookedWeeks,
  type ContactDraft,
  hasSecondGuardian,
  isAnswered,
  OTHER_SCHOOL,
  type RegistrationDraft,
  visibleChildQuestions,
  visibleFamilyQuestions,
} from "./holiday-registration-model";
import { HolidayRegistrationContribution } from "./holiday-registration-contribution";
import { useWeekLabels } from "./holiday-registration-sections";

function contactLine(contact: ContactDraft): string {
  return [contact.name, contact.relationship, contact.phone]
    .map((part) => part.trim())
    .filter(Boolean)
    .join(" · ");
}

/** The answers to the Träger's questions as data fields (#5198). */
function AnswerFields({
  questions,
  answers,
}: Readonly<{
  questions: readonly HolidayFormField[];
  answers: Readonly<Record<string, unknown>>;
}>) {
  const t = useTranslations("holidayRegistration");
  const tr = useTranslations("enrollmentForm");
  const locale = useLocale();
  const text = (field: HolidayFormField): string => {
    const value = answers[field.key];
    if (!isAnswered(field, value)) return t("none");
    if (field.type === "boolean") {
      return value === true ? tr("structured.yes") : tr("structured.no");
    }
    const answer = String(value).trim();
    if (field.type === "select") {
      return (
        field.options?.find((option) => option.value === answer)?.label ??
        answer
      );
    }
    if (field.type === "date") return formatDate(answer, false, locale);
    return answer;
  };
  return questions.map((field) => (
    <DataField
      key={field.key}
      label={field.label}
      fullWidth={field.type === "textarea"}
    >
      {text(field)}
    </DataField>
  ));
}

export function HolidayRegistrationSummary({
  token,
  draft,
  offer,
  submitting,
  canSubmit,
  securityCheck,
  onBack,
  onSubmit,
  onBenefitsDeclaredChange,
  onBenefitsProofsChange,
}: Readonly<{
  token: string;
  draft: RegistrationDraft;
  offer: HolidayRegistrationOffer;
  submitting: boolean;
  /** The security check is solved; until then the send button waits. */
  canSubmit: boolean;
  securityCheck: ReactNode;
  onBack: () => void;
  onSubmit: () => void;
  onBenefitsDeclaredChange: (declared: boolean) => void;
  onBenefitsProofsChange: (proofs: NewUploadValue[]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const uploadText = useTranslations("enrollmentForm");
  const locale = useLocale();
  // A proof of benefits uploads through the public link (#5238); the send
  // button waits until every chosen file has arrived.
  const upload = useCallback(
    (file: File, signal: AbortSignal) =>
      uploadHolidayProof(token, file, signal),
    [token],
  );
  const { pendingUploads, uploadTarget } = useEnrollmentUploadTarget({
    tenantSlug: "",
    upload,
    locale,
    previewMode: false,
  });
  const labels = useWeekLabels();
  const none = t("none");
  const schoolName = (school: string, guestSchoolName: string) =>
    school === OTHER_SCHOOL
      ? `${t("guest")}${guestSchoolName.trim() ? ` · ${guestSchoolName.trim()}` : ""}`
      : (offer.schools.find((candidate) => candidate.id === school)?.name ??
        none);

  return (
    <div className="space-y-6">
      <SectionCard title={t("summaryTitle")} description={t("summaryText")} />

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
            <AnswerFields
              questions={visibleChildQuestions(draft, child, offer.form)}
              answers={child.answers}
            />
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
            {draft.pickupPersons.map(contactLine).join("; ") || t("noPickup")}
          </DataField>
          <AnswerFields
            questions={visibleFamilyQuestions(draft, offer.form)}
            answers={draft.answers}
          />
        </DataGrid>
      </SectionCard>

      {hasSecondGuardian(draft) ? (
        <SectionCard title={t("secondGuardianTitle")} headingLevel={3}>
          <DataGrid>
            <DataField
              label={t("name")}
            >{`${draft.secondGuardian.firstName.trim()} ${draft.secondGuardian.lastName.trim()}`}</DataField>
            <DataField label={t("email")}>
              {draft.secondGuardian.email.trim() || none}
            </DataField>
            <DataField label={t("phone")}>
              {draft.secondGuardian.phone.trim() || none}
            </DataField>
          </DataGrid>
        </SectionCard>
      ) : null}

      {(offer.form.legalBlocks ?? []).some((b) => b.enabled) ? (
        <SectionCard title={t("legalTitle")}>
          <DataGrid>
            {offer.form.legalBlocks
              ?.filter((b) => b.enabled)
              .map((block) => (
                <DataField key={block.key} label={block.title}>
                  {block.kind === "notice" && !block.required
                    ? block.label
                    : draft.legalAcceptances?.[block.key]
                      ? t("legalAccepted")
                      : t("legalNotAccepted")}
                </DataField>
              ))}
          </DataGrid>
        </SectionCard>
      ) : null}
      <EnrollmentUploadScope target={uploadTarget}>
        <HolidayRegistrationContribution
          token={token}
          draft={draft}
          offer={offer}
          onBenefitsDeclaredChange={onBenefitsDeclaredChange}
          onBenefitsProofsChange={onBenefitsProofsChange}
        />
      </EnrollmentUploadScope>

      {pendingUploads > 0 ? (
        <p className="text-sm text-gray-700" role="status">
          {uploadText("upload.pendingSubmit")}
        </p>
      ) : null}

      {/* The security check sits in the row of the send button: one step,
          no gap below the contribution. */}
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        {securityCheck}
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
            disabled={submitting || !canSubmit || pendingUploads > 0}
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
    </div>
  );
}
