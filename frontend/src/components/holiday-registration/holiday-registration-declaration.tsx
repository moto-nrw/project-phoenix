"use client";

// Die Erklärung der Ferienanmeldung (#5626): die festen Einverständnisse,
// das Sorgerecht und der Name, mit dem die Eltern die Erklärungen abgeben.
// Es gilt der deutsche Text aus dem Angebot; in einer anderen Sprache steht
// die Übersetzung als Lesehilfe darunter.

import { useLocale, useTranslations } from "next-intl";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import type { HolidayConsentTexts } from "~/lib/holiday-consent-texts";
import {
  DECLARATION_NAME_MAX_LENGTH,
  type FieldError,
  HOLIDAY_CONSENT_VERSION,
  type RegistrationDraft,
} from "./holiday-registration-model";

type TranslatedKey =
  | "privacyConsent"
  | "healthConsent"
  | "bindingConfirmed"
  | "custodyConfirmed"
  | "declarationText";

type DeclarationDraft = Pick<
  RegistrationDraft,
  | "privacyConsent"
  | "healthConsent"
  | "bindingConfirmed"
  | "custodyConfirmed"
  | "declarationName"
>;

/**
 * Whether the translations of this locale fit the texts of the offer. They
 * belong to HOLIDAY_CONSENT_VERSION; for other texts they stay away rather
 * than translate something else.
 */
function useReadingAid(version: number): string | null {
  const locale = useLocale();
  return locale !== "de" && version === HOLIDAY_CONSENT_VERSION ? locale : null;
}

export function HolidayRegistrationDeclaration({
  texts,
  draft,
  binding,
  hasHealth,
  fieldError,
  onChange,
}: Readonly<{
  texts: HolidayConsentTexts;
  draft: DeclarationDraft;
  binding: boolean;
  hasHealth: boolean;
  fieldError: FieldError;
  onChange: (patch: Partial<DeclarationDraft>) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const readingAid = useReadingAid(texts.version);
  // The German text binds; in another language the translation stands below
  // it as a reading aid.
  const german = (text: string) => <span lang="de">{text}</span>;
  const translation = (key: TranslatedKey) =>
    readingAid ? (
      <span lang={readingAid}>
        {t("translationNote")} {t(key)}
      </span>
    ) : undefined;
  return (
    <>
      <SectionCard title={t("consentTitle")}>
        <div className="space-y-3">
          <ConsentCheckbox
            required
            name="privacy_consent"
            label={german(texts.privacyConsent)}
            description={translation("privacyConsent")}
            checked={draft.privacyConsent}
            error={fieldError("privacy_consent")}
            onChange={(privacyConsent) => onChange({ privacyConsent })}
          />
          {hasHealth ? (
            <ConsentCheckbox
              required
              name="health_consent"
              label={german(texts.healthConsent)}
              description={translation("healthConsent")}
              checked={draft.healthConsent}
              error={fieldError("health_consent")}
              onChange={(healthConsent) => onChange({ healthConsent })}
            />
          ) : null}
          {binding ? (
            <ConsentCheckbox
              required
              name="binding_confirmed"
              label={german(texts.bindingConfirmed)}
              description={translation("bindingConfirmed")}
              checked={draft.bindingConfirmed}
              error={fieldError("binding_confirmed")}
              onChange={(bindingConfirmed) => onChange({ bindingConfirmed })}
            />
          ) : null}
          <ConsentCheckbox
            required
            name="custody_confirmed"
            label={german(texts.custodyConfirmed)}
            description={translation("custodyConfirmed")}
            checked={draft.custodyConfirmed}
            error={fieldError("custody_confirmed")}
            onChange={(custodyConfirmed) => onChange({ custodyConfirmed })}
          />
        </div>
      </SectionCard>
      <SectionCard
        title={t("declarationTitle")}
        description={
          <>
            {german(texts.declarationName)}
            {readingAid ? (
              <span className="mt-1 block">
                {translation("declarationText")}
              </span>
            ) : null}
          </>
        }
      >
        <Input
          name="declaration_name"
          required
          label={t("declarationName")}
          autoComplete="name"
          maxLength={DECLARATION_NAME_MAX_LENGTH}
          value={draft.declarationName}
          error={fieldError("declaration_name")}
          onChange={(event) =>
            onChange({ declarationName: event.target.value })
          }
        />
      </SectionCard>
    </>
  );
}
