"use client";
import { HolidayRegistrationLegal } from "./holiday-registration-legal";

// Öffentliche Ferienanmeldung (#4661): bis zu drei Geschwister, Wochen mit
// Zeitmodell, Notfallkontakte, Abholberechtigte und Gesundheitsangaben. Vor
// dem Absenden steht eine Zusammenfassung, danach die Eingangsbestätigung.
// Eltern mit Elternkonto wählen ihre bekannten Kinder aus (#4662).

import { Turnstile } from "@marsidev/react-turnstile";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  type HolidayKnownFamily,
  type HolidayReceiptPlacement,
  type HolidayRegistrationOffer,
  submitHolidayRegistration,
} from "~/lib/holiday-registration-api";
import { HolidayRegistrationPlacements } from "./holiday-registration-placements";
import { createLogger } from "~/lib/logger";
import { isFieldVisible } from "~/lib/enrollment-field-visibility";
import {
  asksForAddress,
  childContext,
  contactFieldFromPayload,
  emptyChild,
  familyContext,
  type FieldErrorKey,
  hasHealthInfo,
  MAX_CHILDREN,
  MAX_CONTACTS,
  payloadFromDraft,
  type RegistrationDraft,
  registrationFromFamily,
  toggleKnownChild,
  validateRegistration,
} from "./holiday-registration-model";
import {
  ChildCard,
  ContactList,
  GuardianAddressCard,
  KnownChildPicker,
  SecondGuardianCard,
} from "./holiday-registration-sections";
import { HolidayQuestions } from "./holiday-registration-questions";
import { HolidayRegistrationCriteria } from "./holiday-registration-criteria";
import { HolidayRegistrationDeclaration } from "./holiday-registration-declaration";
import { HolidayRegistrationSummary } from "./holiday-registration-summary";
import { HolidayRegistrationAccountQuestion } from "./holiday-registration-account-question";
import { parentPath } from "~/lib/parent-url";
import { useInvalidateMatching } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";

/** The code of a refusal because the Träger changed the form (#5198). */
const FORM_CHANGED = "enrollment.holiday_registration_form_changed";
/** The code of a proof whose upload no longer counts (#5238). */
const UPLOAD_EXPIRED = "enrollment.upload_expired";

const logger = createLogger({ component: "HolidayRegistrationForm" });

export type HolidayRegistrationStep = "form" | "summary" | "done";

/**
 * How the registration meets the parent account (#6673): "ask" puts the
 * question at the end of the summary, "signed-in" sends the account's proof
 * and leads back to the parents app after sending.
 */
export type HolidayRegistrationParentAccount = "ask" | "signed-in";

export function HolidayRegistrationForm({
  token,
  offer,
  family = null,
  parentAccount,
  onFormChanged,
  onStepChange,
}: Readonly<{
  token: string;
  offer: HolidayRegistrationOffer;
  /** What the schools know of a parent with an account (#4662). */
  family?: HolidayKnownFamily | null;
  /** Without a value the form neither asks nor links back (#6673). */
  parentAccount?: HolidayRegistrationParentAccount;
  /**
   * Loads the offer again after the Träger changed the form (#5198). The
   * answers stay: they are kept by the key of their question.
   */
  onFormChanged?: () => Promise<void>;
  /** Tells the page which step shows; only the form step has fields (#5334). */
  onStepChange?: (step: HolidayRegistrationStep) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  const [draft, setDraft] = useState<RegistrationDraft>(() =>
    registrationFromFamily(family),
  );
  const [step, setStep] = useState<HolidayRegistrationStep>("form");
  const [captchaToken, setCaptchaToken] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [placements, setPlacements] = useState<
    readonly HolidayReceiptPlacement[]
  >([]);
  const fieldError = (name: string) => errorPath.fieldError(name) ?? undefined;
  const accountFieldsetRef = useRef<HTMLFieldSetElement>(null);
  const [accountError, setAccountError] = useState<string | undefined>();
  const asksForAccount = parentAccount === "ask";
  const invalidate = useInvalidateMatching();

  useEffect(() => {
    onStepChange?.(step);
  }, [onStepChange, step]);

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
    // The question has no preselection: without an answer the send waits
    // and the group names what is missing (#6673).
    if (asksForAccount && draft.parentAccountRequested === undefined) {
      setAccountError(t("account.required"));
      accountFieldsetRef.current
        ?.querySelector<HTMLInputElement>("input[type=radio]")
        ?.focus();
      return;
    }
    setSubmitting(true);
    try {
      const placed = await submitHolidayRegistration(
        token,
        payloadFromDraft(draft, offer, captchaToken),
      );
      if (parentAccount === "signed-in") {
        await invalidate(
          (key) => key === parentKeys.holidayRegistrationEntries(),
          { clear: true },
        );
      }
      setPlacements(placed ?? []);
      setStep("done");
      window.scrollTo({ top: 0 });
    } catch (err) {
      logger.error("holiday_registration_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The answer may name a field; the form shows it where it is.
      setStep("form");
      setCaptchaToken("");
      if (err instanceof ApiError && err.code === FORM_CHANGED) {
        // The privacy text may name another retention now (#6691): the
        // parent agrees to the text the reloaded form shows, not the old one.
        setDraft((current) => ({
          ...current,
          legalAcceptances: {},
          privacyConsent: false,
        }));
        await onFormChanged?.();
      } else if (
        err instanceof ApiError &&
        err.errors?.some(({ field }) =>
          /^(children\.\d+\.)?criteria(?:\.|$)/.test(field),
        )
      ) {
        // The catalog can change without a new form version. Reload its
        // questions so parents can answer them, keeping the entire draft.
        await onFormChanged?.();
      }
      // The parent uploads the proof again; the old token claims nothing.
      if (err instanceof ApiError && err.code === UPLOAD_EXPIRED) {
        setDraft((current) => ({ ...current, benefitsProofs: [] }));
      }
      if (err instanceof ApiError && err.errors) {
        err.errors = err.errors.map((error) => ({
          ...error,
          field: contactFieldFromPayload(error.field, draft.contacts),
        }));
      }
      void errorPath.show(err, { object: t("errorObject") });
    } finally {
      setSubmitting(false);
    }
  };

  if (step === "done") {
    return (
      <SectionCard title={t("doneTitle")}>
        <div className="space-y-5">
          <div className="space-y-2 text-sm text-gray-700">
            <p>{t("doneText", { email: draft.guardian.email.trim() })}</p>
            <p>{t("doneNext")}</p>
          </div>
          <HolidayRegistrationPlacements
            childDrafts={draft.children}
            weeks={offer.weeks}
            placements={placements}
          />
          {asksForAccount && draft.parentAccountRequested === true ? (
            // The second mail goes out for every address; the text does
            // not say whether an account already exists (ADR 0216).
            <Alert
              type="info"
              announce="off"
              title={t("doneAccountTitle")}
              message={t("doneAccountText", {
                email: draft.guardian.email.trim(),
              })}
            />
          ) : null}
          {parentAccount === "signed-in" ? (
            <ButtonLink
              href={parentPath("/parents/ferienanmeldungen")}
              variant="primary"
              size="md"
              className="max-sm:w-full"
            >
              {t("doneToRegistrations")}
            </ButtonLink>
          ) : null}
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
        accountQuestion={
          asksForAccount ? (
            <HolidayRegistrationAccountQuestion
              value={draft.parentAccountRequested}
              email={draft.guardian.email}
              error={accountError}
              fieldsetRef={accountFieldsetRef}
              onChange={(parentAccountRequested) => {
                setAccountError(undefined);
                setDraft((current) => ({ ...current, parentAccountRequested }));
              }}
            />
          ) : null
        }
        onBack={() => {
          setCaptchaToken("");
          setAccountError(undefined);
          setStep("form");
        }}
        onSubmit={() => void submit()}
        onBenefitsDeclaredChange={(benefitsDeclared) =>
          setDraft((current) => ({ ...current, benefitsDeclared }))
        }
        onBenefitsProofsChange={(benefitsProofs) =>
          setDraft((current) => ({ ...current, benefitsProofs }))
        }
      />
    );
  }

  const familyQuestions = offer.form.fields.filter(
    (field) => !field.applies_to_child,
  );
  const childQuestions = offer.form.fields.filter(
    (field) => field.applies_to_child,
  );
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
          {offer.holidayWishMode === "off" ? null : (
            <Alert
              type="info"
              announce="off"
              title={t("wishRuleTitle")}
              message={t(
                offer.holidayWishMode === "wish_only"
                  ? "wishRuleWishOnly"
                  : "wishRulePriority",
              )}
            />
          )}
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
              questions={childQuestions}
              questionContext={childContext(draft, child, offer.form)}
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
            required
            label={t("firstName")}
            autoComplete="given-name"
            maxLength={100}
            value={draft.guardian.firstName}
            error={fieldError("guardian.first_name")}
            onChange={(event) => setGuardian({ firstName: event.target.value })}
          />
          <Input
            name="guardian.last_name"
            required
            label={t("lastName")}
            autoComplete="family-name"
            maxLength={100}
            value={draft.guardian.lastName}
            error={fieldError("guardian.last_name")}
            onChange={(event) => setGuardian({ lastName: event.target.value })}
          />
          <Input
            name="guardian.email"
            required
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
            required
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

      {asksForAddress(offer.form) ? (
        <GuardianAddressCard
          address={draft.guardianAddress}
          required={offer.form.addressRequirement === "required"}
          fieldError={fieldError}
          onChange={(guardianAddress) =>
            setDraft({ ...draft, guardianAddress })
          }
        />
      ) : null}

      <SecondGuardianCard
        guardian={draft.secondGuardian}
        fieldError={fieldError}
        onChange={(secondGuardian) => setDraft({ ...draft, secondGuardian })}
      />
      {familyQuestions.some((field) =>
        isFieldVisible(field, familyContext(draft, offer.form)),
      ) ? (
        <SectionCard title={t("familyQuestionsTitle")}>
          <HolidayQuestions
            fields={familyQuestions}
            answers={draft.answers}
            context={familyContext(draft, offer.form)}
            prefix=""
            fieldError={fieldError}
            onChange={(key, value) =>
              setDraft({
                ...draft,
                answers: { ...draft.answers, [key]: value },
              })
            }
          />
        </SectionCard>
      ) : null}

      <HolidayRegistrationCriteria
        offer={offer}
        draft={draft}
        fieldError={fieldError}
        onChange={setDraft}
      />

      <ContactList
        contacts={draft.contacts}
        max={MAX_CONTACTS}
        fieldError={fieldError}
        onChange={(contacts) => setDraft({ ...draft, contacts })}
      />

      <HolidayRegistrationLegal
        token={token}
        blocks={offer.form.legalBlocks ?? []}
        accepted={draft.legalAcceptances ?? {}}
        fieldError={fieldError}
        onChange={(key, checked) =>
          setDraft({
            ...draft,
            legalAcceptances: { ...draft.legalAcceptances, [key]: checked },
          })
        }
      />
      <HolidayRegistrationDeclaration
        texts={offer.consentTexts}
        draft={draft}
        binding={offer.binding}
        hasHealth={hasHealthInfo(draft, offer.form)}
        fieldError={fieldError}
        onChange={(patch) => setDraft({ ...draft, ...patch })}
      />

      <Button type="submit" variant="primary" size="md" className="w-full">
        {t("toSummary")}
      </Button>
    </form>
  );
}
