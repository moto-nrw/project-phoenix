"use client";

// Die Erklärung der Ferienanmeldung (#5626): die festen Einverständnisse,
// das Sorgerecht und der Name, mit dem die Eltern die Erklärungen abgeben.
// Es gilt der deutsche Text aus dem Angebot; in einer anderen Sprache steht
// die Übersetzung als Lesehilfe darunter.

import { useLocale, useTranslations } from "next-intl";
import { ConsentCheckbox } from "~/components/ui/consent-checkbox";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import type { HolidayConsentTexts } from "~/lib/holiday-registration-api";
import {
  DECLARATION_NAME_MAX_LENGTH,
  type FieldError,
  HOLIDAY_CONSENT_VERSION,
  type RegistrationDraft,
} from "./holiday-registration-model";

type DeclarationDraft = Pick<
  RegistrationDraft,
  | "privacyConsent"
  | "healthConsent"
  | "bindingConfirmed"
  | "custodyConfirmed"
  | "declarationName"
>;

/**
 * A binding German text and, in another language, its translation. The
 * translation belongs to HOLIDAY_CONSENT_VERSION; for other texts it stays
 * away rather than translate something else.
 */
function DeclarationText({
  text,
  translation,
  version,
}: Readonly<{ text: string; translation: string; version: number }>) {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  if (locale === "de" || version !== HOLIDAY_CONSENT_VERSION) {
    return <span lang="de">{text}</span>;
  }
  return (
    <>
      <span lang="de" className="block">
        {text}
      </span>
      <span lang={locale} className="mt-1 block text-sm text-gray-600">
        {t("translationNote")} {translation}
      </span>
    </>
  );
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
  const text = (german: string, translation: string) => (
    <DeclarationText
      text={german}
      translation={translation}
      version={texts.version}
    />
  );
  return (
    <>
      <SectionCard title={t("consentTitle")}>
        <div className="space-y-3">
          <ConsentCheckbox
            required
            name="privacy_consent"
            label={text(texts.privacyConsent, t("privacyConsent"))}
            checked={draft.privacyConsent}
            error={fieldError("privacy_consent")}
            onChange={(privacyConsent) => onChange({ privacyConsent })}
          />
          {hasHealth ? (
            <ConsentCheckbox
              required
              name="health_consent"
              label={text(texts.healthConsent, t("healthConsent"))}
              checked={draft.healthConsent}
              error={fieldError("health_consent")}
              onChange={(healthConsent) => onChange({ healthConsent })}
            />
          ) : null}
          {binding ? (
            <ConsentCheckbox
              required
              name="binding_confirmed"
              label={text(texts.bindingConfirmed, t("bindingConfirmed"))}
              checked={draft.bindingConfirmed}
              error={fieldError("binding_confirmed")}
              onChange={(bindingConfirmed) => onChange({ bindingConfirmed })}
            />
          ) : null}
          <ConsentCheckbox
            required
            name="custody_confirmed"
            label={text(texts.custodyConfirmed, t("custodyConfirmed"))}
            checked={draft.custodyConfirmed}
            error={fieldError("custody_confirmed")}
            onChange={(custodyConfirmed) => onChange({ custodyConfirmed })}
          />
        </div>
      </SectionCard>
      <SectionCard
        title={t("declarationTitle")}
        description={text(texts.declarationName, t("declarationText"))}
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
