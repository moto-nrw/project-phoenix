"use client";

// Die Frage nach dem Elternkonto am Ende der Ferienanmeldung (#6673,
// ADR 0216). Nur wer ohne Eltern-Sitzung anmeldet, sieht sie. Bei „Ja“
// schickt der Träger eine zweite E-Mail mit dem Link zum Passwort. Die
// Antwort verrät nicht, ob es zu der Adresse schon ein Konto gibt.

import { useTranslations } from "next-intl";
import type { Ref } from "react";
import { CustomFieldInput } from "~/components/enrollment/custom-field-input";
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
  // The card names the question like its neighbours name theirs; the group
  // inside carries no second frame, only the legend for screen readers.
  return (
    <SectionCard
      concept={parentsConcept}
      title={t("account.question")}
      description={t("account.benefit")}
    >
      <div className="sm:max-w-md">
        <CustomFieldInput
          field={{
            key: PARENT_ACCOUNT_FIELD,
            type: "boolean",
            label: t("account.question"),
            required: true,
            sort_order: 0,
          }}
          inputName={PARENT_ACCOUNT_FIELD}
          value={value}
          error={error}
          fieldsetRef={fieldsetRef}
          labelHidden
          tr={(key) =>
            t(key === "structured.yes" ? "account.yes" : "account.no")
          }
          onChange={(requested) => onChange(requested === true)}
        >
          {value === true ? (
            <p
              className="mt-3 text-sm leading-6 [overflow-wrap:anywhere] text-gray-700"
              aria-live="polite"
            >
              {t("account.mailNote", { email: email.trim() })}
            </p>
          ) : null}
        </CustomFieldInput>
      </div>
    </SectionCard>
  );
}
