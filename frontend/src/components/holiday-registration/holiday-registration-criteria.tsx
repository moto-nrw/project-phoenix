"use client";

// Fragen für die Platzvergabe in der Ferienanmeldung (#4700): vergibt der
// Träger die Plätze nach Kriterien, beantworten Eltern jede Frage mit Ja oder
// Nein, Fragen zur Familie einmal, Fragen zum Kind für jedes Kind. Eltern
// sehen nur die Fragen, nie die Angaben anderer Familien.

import { useTranslations } from "next-intl";
import { CustomFieldInput } from "~/components/enrollment/custom-field-input";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { useCareTermTranslations } from "~/lib/care-facility-term";
import type { HolidayRegistrationOffer } from "~/lib/holiday-registration-api";
import {
  criterionQuestions,
  type FieldError,
  type RegistrationDraft,
} from "./holiday-registration-model";

export function HolidayRegistrationCriteria({
  offer,
  draft,
  fieldError,
  onChange,
}: Readonly<{
  offer: HolidayRegistrationOffer;
  draft: RegistrationDraft;
  fieldError: FieldError;
  onChange: (next: RegistrationDraft) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const tr = useCareTermTranslations("enrollmentForm");
  const family = criterionQuestions(offer, "family");
  const child = criterionQuestions(offer, "child");
  if (family.length === 0 && child.length === 0) return null;
  return (
    <SectionCard title={t("criteriaTitle")} description={t("criteriaText")}>
      <div className="space-y-6">
        {family.length > 0 ? (
          <div className="space-y-4">
            <SectionLabel as="h3">{t("criteriaFamily")}</SectionLabel>
            {family.map((field) => {
              const name = `criteria.${field.key}`;
              return (
                <CustomFieldInput
                  key={field.key}
                  field={field}
                  inputName={name}
                  controlId={`holiday-${name.replaceAll(".", "-")}`}
                  value={draft.criteria?.[field.key]}
                  error={fieldError(name)}
                  tr={tr}
                  onChange={(value) =>
                    onChange({
                      ...draft,
                      criteria: {
                        ...draft.criteria,
                        [field.key]: value === true,
                      },
                    })
                  }
                />
              );
            })}
          </div>
        ) : null}
        {child.length > 0
          ? draft.children.map((entry, index) => {
              const childName = `${entry.firstName} ${entry.lastName}`.trim();
              return (
                <div key={entry.key} className="space-y-4">
                  <SectionLabel as="h3">
                    {childName
                      ? t("criteriaChild", { name: childName })
                      : t("criteriaChildNumber", { number: index + 1 })}
                  </SectionLabel>
                  {child.map((field) => {
                    const name = `children.${index}.criteria.${field.key}`;
                    return (
                      <CustomFieldInput
                        key={field.key}
                        field={field}
                        inputName={name}
                        controlId={`holiday-${name.replaceAll(".", "-")}`}
                        value={entry.criteria?.[field.key]}
                        error={fieldError(name)}
                        tr={tr}
                        onChange={(value) =>
                          onChange({
                            ...draft,
                            children: draft.children.map((current, i) =>
                              i === index
                                ? {
                                    ...current,
                                    criteria: {
                                      ...current.criteria,
                                      [field.key]: value === true,
                                    },
                                  }
                                : current,
                            ),
                          })
                        }
                      />
                    );
                  })}
                </div>
              );
            })
          : null}
      </div>
    </SectionCard>
  );
}
