"use client";

// Die festen Gesundheitsangaben eines Kindes in der Ferienanmeldung (#4722,
// #6686). Ein freiwilliges Formular zeigt drei offene Felder. Verlangt der
// Träger Allergien und Medikamente, beantworten Eltern jede der beiden
// Fragen mit „Nein, keine“ oder „Ja“ und schreiben bei „Ja“ dazu, was ihr
// Kind hat. So steht in der Anmeldung nie ein leeres Feld, das „keine“ oder
// „vergessen“ bedeuten kann. Weitere Hinweise bleiben freiwillig.

import { useTranslations } from "next-intl";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { Textarea } from "~/components/ui/textarea";
import type { HolidayFormVersion } from "~/lib/holiday-form-field";
import {
  type ChildDraft,
  type FieldError,
  type HolidayHealthChoice,
  type RequiredHealthField,
  requiresHealth,
} from "./holiday-registration-model";

/** The draft field and the message keys of each required question. */
const HEALTH_QUESTION_KEYS = {
  allergies: {
    choice: "allergiesChoice",
    question: "allergiesQuestion",
    detail: "allergiesDetail",
  },
  medication: {
    choice: "medicationChoice",
    question: "medicationQuestion",
    detail: "medicationDetail",
  },
} as const;

/** One required health field: „Nein, keine“ or „Ja“ with the text. */
function RequiredHealthQuestion({
  prefix,
  field,
  child,
  fieldError,
  set,
}: Readonly<{
  prefix: string;
  field: RequiredHealthField;
  child: ChildDraft;
  fieldError: FieldError;
  set: (patch: Partial<ChildDraft>) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const keys = HEALTH_QUESTION_KEYS[field];
  const choice = child[keys.choice];
  const name = `${prefix}.${field}`;
  const error = fieldError(name);
  const errorId = `${prefix}-${field}-error`;
  // The request path names the control the parent must fix: the choice
  // until „Ja“ is chosen, then the text. The form focuses it after a failed
  // check, and a refusal of the server lands there as well.
  const radioName = choice === "some" ? `${name}_choice` : name;
  const options: readonly { value: HolidayHealthChoice; label: string }[] = [
    { value: "none", label: t("healthNone") },
    { value: "some", label: t("healthSome") },
  ];
  return (
    <div className="space-y-3">
      <fieldset
        className="min-w-0"
        aria-invalid={error && choice !== "some" ? "true" : undefined}
        aria-describedby={error && choice !== "some" ? errorId : undefined}
      >
        <legend className="mb-2 block text-sm font-medium text-gray-700">
          {t(keys.question)}
          <RequiredMark />
        </legend>
        <div className="grid grid-cols-2 gap-2">
          {options.map((option) => (
            <ChoiceTile
              key={option.value}
              selected={choice === option.value}
              className="h-10 justify-center py-0"
            >
              <Radio
                name={radioName}
                value={option.value}
                aria-required="true"
                checked={choice === option.value}
                onChange={() => set({ [keys.choice]: option.value })}
              />
              {option.label}
            </ChoiceTile>
          ))}
        </div>
        {choice === "some" ? null : (
          <FieldErrorHint id={errorId} message={error} />
        )}
      </fieldset>
      {choice === "some" ? (
        <Input
          name={name}
          label={t(keys.detail)}
          aria-required="true"
          maxLength={1000}
          value={child[field]}
          error={error}
          onChange={(event) => set({ [field]: event.target.value })}
        />
      ) : null}
    </div>
  );
}

/** The fixed health fields of one child, as the form version asks them. */
export function ChildHealthFields({
  form,
  prefix,
  child,
  fieldError,
  set,
}: Readonly<{
  form: HolidayFormVersion;
  prefix: string;
  child: ChildDraft;
  fieldError: FieldError;
  set: (patch: Partial<ChildDraft>) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const required = requiresHealth(form);
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-semibold text-gray-900">
        {t(required ? "healthTitleRequired" : "healthTitle")}
      </legend>
      <p className="text-sm text-gray-600">{t("healthText")}</p>
      {required ? (
        <>
          <RequiredHealthQuestion
            prefix={prefix}
            field="allergies"
            child={child}
            fieldError={fieldError}
            set={set}
          />
          <RequiredHealthQuestion
            prefix={prefix}
            field="medication"
            child={child}
            fieldError={fieldError}
            set={set}
          />
        </>
      ) : (
        <>
          <Input
            name={`${prefix}.allergies`}
            label={t("allergies")}
            maxLength={1000}
            value={child.allergies}
            error={fieldError(`${prefix}.allergies`)}
            onChange={(event) => set({ allergies: event.target.value })}
          />
          <Input
            name={`${prefix}.medication`}
            label={t("medication")}
            maxLength={1000}
            value={child.medication}
            error={fieldError(`${prefix}.medication`)}
            onChange={(event) => set({ medication: event.target.value })}
          />
        </>
      )}
      <Textarea
        name={`${prefix}.health_notes`}
        label={t(required ? "healthNotesOptional" : "healthNotes")}
        rows={2}
        maxLength={2000}
        value={child.healthNotes}
        error={fieldError(`${prefix}.health_notes`)}
        onChange={(event) => set({ healthNotes: event.target.value })}
      />
    </fieldset>
  );
}
