"use client";

// Die Frage nach dem Elternkonto am Ende der Ferienanmeldung (#6673,
// ADR 0216). Nur wer ohne Eltern-Sitzung anmeldet, sieht sie. Bei „Ja“
// schickt der Träger eine zweite E-Mail mit dem Link zum Passwort. Die
// Antwort verrät nicht, ob es zu der Adresse schon ein Konto gibt.

import { useTranslations } from "next-intl";
import { useId, type Ref } from "react";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Radio } from "~/components/ui/radio";
import { SectionCard } from "~/components/ui/section-card";
import { parentsConcept } from "~/lib/concepts/parents";

/** The name of the radio group; a field error and the focus find it. */
const PARENT_ACCOUNT_FIELD = "parent_account_requested";

export function HolidayRegistrationAccountQuestion({
  value,
  email,
  error,
  fieldsetRef,
  onChange,
}: Readonly<{
  /** The answer; undefined until the parent chose. */
  value: boolean | undefined;
  /** The address the parent entered; the mail with the link goes there. */
  email: string;
  error?: string;
  fieldsetRef?: Ref<HTMLFieldSetElement>;
  onChange: (requested: boolean) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const errorId = useId();
  const options = [
    { label: t("account.yes"), value: "yes", requested: true },
    { label: t("account.no"), value: "no", requested: false },
  ] as const;
  // The card names the question like its neighbours name theirs; the group
  // inside carries no second frame, only the legend for screen readers.
  return (
    <SectionCard
      concept={parentsConcept}
      title={t("account.question")}
      description={t("account.benefit")}
    >
      <fieldset
        ref={fieldsetRef}
        tabIndex={-1}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        className="min-w-0 outline-none"
      >
        <legend className="sr-only">{t("account.question")}</legend>
        <div className="grid grid-cols-2 gap-2 sm:max-w-md">
          {options.map((option) => (
            <ChoiceTile
              key={option.value}
              selected={value === option.requested}
              className="min-h-11 justify-center py-0"
            >
              <Radio
                name={PARENT_ACCOUNT_FIELD}
                value={option.value}
                required
                checked={value === option.requested}
                onChange={() => onChange(option.requested)}
              />
              {option.label}
            </ChoiceTile>
          ))}
        </div>
        <FieldErrorHint id={errorId} message={error} />
        {value === true ? (
          <p
            className="mt-3 text-sm leading-6 [overflow-wrap:anywhere] text-gray-700"
            aria-live="polite"
          >
            {t("account.mailNote", { email: email.trim() })}
          </p>
        ) : null}
      </fieldset>
    </SectionCard>
  );
}
