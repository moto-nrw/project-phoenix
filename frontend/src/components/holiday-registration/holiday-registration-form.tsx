"use client";

// Öffentliche Ferienanmeldung (#4661): bis zu drei Geschwister, Wochen mit
// Zeitmodell, Notfallkontakte, Abholberechtigte und Gesundheitsangaben. Vor
// dem Absenden steht eine Zusammenfassung, danach die Eingangsbestätigung.
// Eltern mit Elternkonto wählen ihre bekannten Kinder aus (#4662).

import { Turnstile } from "@marsidev/react-turnstile";
import { useTranslations } from "next-intl";
import { useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  type HolidayKnownFamily,
  type HolidayRegistrationOffer,
  submitHolidayRegistration,
} from "~/lib/holiday-registration-api";
import { createLogger } from "~/lib/logger";
import {
  emptyChild,
  type FieldErrorKey,
  hasHealthInfo,
  MAX_CHILDREN,
  MAX_EMERGENCY_CONTACTS,
  MAX_PICKUP_PERSONS,
  payloadFromDraft,
  type RegistrationDraft,
  registrationFromFamily,
  toggleKnownChild,
  validateRegistration,
} from "./holiday-registration-model";
import {
  ChildCard,
  ContactList,
  KnownChildPicker,
} from "./holiday-registration-sections";
import { HolidayRegistrationSummary } from "./holiday-registration-summary";

const logger = createLogger({ component: "HolidayRegistrationForm" });

type Step = "form" | "summary" | "done";

export function HolidayRegistrationForm({
  token,
  offer,
  family = null,
}: Readonly<{
  token: string;
  offer: HolidayRegistrationOffer;
  /** What the schools know of a parent with an account (#4662). */
  family?: HolidayKnownFamily | null;
}>) {
  const t = useTranslations("holidayRegistration");
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  const [draft, setDraft] = useState<RegistrationDraft>(() =>
    registrationFromFamily(family),
  );
  const [step, setStep] = useState<Step>("form");
  const [captchaToken, setCaptchaToken] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const fieldError = (name: string) => errorPath.fieldError(name) ?? undefined;

  const toSummary = () => {
    const errors = validateRegistration(draft, offer);
    if (Object.keys(errors).length > 0) {
      errorPath.invalid(
        t("checkFields"),
        Object.fromEntries(
          Object.entries(errors).map(
            ([field, key]: [string, FieldErrorKey]) => [
              field,
              t(`errors.${key}`),
            ],
          ),
        ),
      );
      return;
    }
    errorPath.clear();
    setStep("summary");
    window.scrollTo({ top: 0 });
  };

  const submit = async () => {
    setSubmitting(true);
    try {
      await submitHolidayRegistration(
        token,
        payloadFromDraft(draft, offer, captchaToken),
      );
      setStep("done");
      window.scrollTo({ top: 0 });
    } catch (err) {
      logger.error("holiday_registration_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The answer may name a field; the form shows it where it is.
      setStep("form");
      setCaptchaToken("");
      void errorPath.show(err, { object: t("errorObject") });
    } finally {
      setSubmitting(false);
    }
  };

  if (step === "done") {
    return (
      <SectionCard title={t("doneTitle")}>
        <div className="space-y-2 text-sm text-gray-700">
          <p>{t("doneText", { email: draft.guardian.email.trim() })}</p>
          <p>{t("doneNext")}</p>
        </div>
      </SectionCard>
    );
  }

  if (step === "summary") {
    // The security check sits next to the send button: it stays mounted
    // while the parent reads the summary, so Turnstile renews its token
    // instead of a stale one from the form step being sent.
    return (
      <HolidayRegistrationSummary
        token={token}
        draft={draft}
        offer={offer}
        submitting={submitting}
        canSubmit={captchaToken !== ""}
        securityCheck={
          offer.captchaSiteKey ? (
            <Turnstile
              siteKey={offer.captchaSiteKey}
              options={{ theme: "light" }}
              onSuccess={setCaptchaToken}
              onExpire={() => setCaptchaToken("")}
              onError={() => setCaptchaToken("")}
            />
          ) : (
            <p className="text-sm text-gray-700">{t("captchaMissing")}</p>
          )
        }
        onBack={() => {
          setCaptchaToken("");
          setStep("form");
        }}
        onSubmit={() => void submit()}
        onBenefitsDeclaredChange={(benefitsDeclared) =>
          setDraft((current) => ({ ...current, benefitsDeclared }))
        }
      />
    );
  }

  const setGuardian = (patch: Partial<RegistrationDraft["guardian"]>) =>
    setDraft({ ...draft, guardian: { ...draft.guardian, ...patch } });

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-6"
      onSubmit={(event) => {
        event.preventDefault();
        toSummary();
      }}
    >
      <FormErrorAlert message={errorPath.error} />

      <SectionCard title={t("childrenTitle")} description={t("childrenText")}>
        <div className="space-y-4">
          {family && family.children.length > 0 ? (
            <KnownChildPicker
              known={family.children}
              draft={draft}
              offer={offer}
              onToggle={(known) =>
                setDraft(toggleKnownChild(draft, known, offer))
              }
            />
          ) : null}
          {draft.children.map((child, index) => (
            <ChildCard
              key={child.key}
              index={index}
              child={child}
              offer={offer}
              removable={draft.children.length > 1}
              fieldError={fieldError}
              onChange={(next) =>
                setDraft({
                  ...draft,
                  children: draft.children.map((current, i) =>
                    i === index ? next : current,
                  ),
                })
              }
              onRemove={() =>
                setDraft({
                  ...draft,
                  children: draft.children.filter((_, i) => i !== index),
                })
              }
            />
          ))}
          {draft.children.length < MAX_CHILDREN ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() =>
                setDraft({
                  ...draft,
                  children: [...draft.children, emptyChild()],
                })
              }
            >
              {t("addChild")}
            </Button>
          ) : null}
        </div>
      </SectionCard>

      <SectionCard title={t("guardianTitle")} description={t("guardianText")}>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Input
            name="guardian.first_name"
            label={t("firstName")}
            autoComplete="given-name"
            maxLength={100}
            value={draft.guardian.firstName}
            error={fieldError("guardian.first_name")}
            onChange={(event) => setGuardian({ firstName: event.target.value })}
          />
          <Input
            name="guardian.last_name"
            label={t("lastName")}
            autoComplete="family-name"
            maxLength={100}
            value={draft.guardian.lastName}
            error={fieldError("guardian.last_name")}
            onChange={(event) => setGuardian({ lastName: event.target.value })}
          />
          <Input
            name="guardian.email"
            type="email"
            label={t("email")}
            autoComplete="email"
            maxLength={254}
            value={draft.guardian.email}
            error={fieldError("guardian.email")}
            onChange={(event) => setGuardian({ email: event.target.value })}
          />
          <Input
            name="guardian.phone"
            type="tel"
            label={t("phone")}
            autoComplete="tel"
            maxLength={40}
            value={draft.guardian.phone}
            error={fieldError("guardian.phone")}
            onChange={(event) => setGuardian({ phone: event.target.value })}
          />
        </div>
      </SectionCard>

      <ContactList
        kind="emergency_contacts"
        contacts={draft.emergencyContacts}
        max={MAX_EMERGENCY_CONTACTS}
        fieldError={fieldError}
        onChange={(emergencyContacts) =>
          setDraft({ ...draft, emergencyContacts })
        }
      />
      <ContactList
        kind="pickup_persons"
        contacts={draft.pickupPersons}
        max={MAX_PICKUP_PERSONS}
        fieldError={fieldError}
        onChange={(pickupPersons) => setDraft({ ...draft, pickupPersons })}
      />

      <SectionCard title={t("consentTitle")}>
        <div className="space-y-3">
          <ConsentCheckbox
            name="privacy_consent"
            label={t("privacyConsent")}
            checked={draft.privacyConsent}
            error={fieldError("privacy_consent")}
            onChange={(privacyConsent) =>
              setDraft({ ...draft, privacyConsent })
            }
          />
          {hasHealthInfo(draft) ? (
            <ConsentCheckbox
              name="health_consent"
              label={t("healthConsent")}
              checked={draft.healthConsent}
              error={fieldError("health_consent")}
              onChange={(healthConsent) =>
                setDraft({ ...draft, healthConsent })
              }
            />
          ) : null}
          {offer.binding ? (
            <ConsentCheckbox
              name="binding_confirmed"
              label={t("bindingConfirmed")}
              checked={draft.bindingConfirmed}
              error={fieldError("binding_confirmed")}
              onChange={(bindingConfirmed) =>
                setDraft({ ...draft, bindingConfirmed })
              }
            />
          ) : null}
        </div>
      </SectionCard>

      <Button type="submit" variant="primary" size="lg" className="w-full">
        {t("toSummary")}
      </Button>
    </form>
  );
}

function ConsentCheckbox({
  name,
  label,
  checked,
  error,
  onChange,
}: Readonly<{
  name: string;
  label: string;
  checked: boolean;
  error: string | undefined;
  onChange: (checked: boolean) => void;
}>) {
  return (
    <div>
      <label htmlFor={name} className="flex items-start gap-3">
        <Checkbox
          id={name}
          name={name}
          checked={checked}
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? `${name}-error` : undefined}
          onChange={(event) => onChange(event.target.checked)}
        />
        <span className="text-sm text-gray-900">{label}</span>
      </label>
      <FieldErrorHint id={`${name}-error`} message={error} />
    </div>
  );
}
