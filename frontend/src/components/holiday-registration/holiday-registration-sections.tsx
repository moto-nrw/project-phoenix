"use client";

// Die Abschnitte der Ferienanmeldung (#4661): ein Kind mit Schule, Wochen und
// Gesundheit, die Listen der Notfallkontakte und Abholberechtigten und die
// bekannten Kinder eines Elternkontos (#4662).

import { useLocale, useTranslations } from "next-intl";
import { useMemo } from "react";
import { DateOfBirthPicker } from "~/components/enrollment/date-of-birth-picker";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { Textarea } from "~/components/ui/textarea";
import { formatDate } from "~/lib/date-helpers";
import {
  type ConditionContext,
  isFieldVisible,
} from "~/lib/enrollment-field-visibility";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import type {
  HolidayKnownChild,
  HolidayRegistrationOffer,
  HolidayRegistrationTimeModel,
  HolidayRegistrationWeek,
} from "~/lib/holiday-registration-api";
import {
  type ChildDraft,
  type ContactDraft,
  emptyContact,
  type FieldError,
  hasRoomForKnownChild,
  OTHER_SCHOOL,
  type RegistrationDraft,
} from "./holiday-registration-model";
import { HolidayQuestions } from "./holiday-registration-questions";

export function useWeekLabels() {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  return useMemo(() => {
    const euro = new Intl.NumberFormat(locale, {
      style: "currency",
      currency: "EUR",
    });
    return {
      range: (week: HolidayRegistrationWeek) =>
        t("weekRange", {
          starts: formatDate(week.startsOn, false, locale),
          ends: formatDate(week.endsOn, false, locale),
        }),
      timeModel: (model: HolidayRegistrationTimeModel) =>
        t("timeModel", {
          name: model.name,
          startsAt: model.startsAt,
          endsAt: model.endsAt,
        }),
      // The price per week at the choice: the base line the summary shows
      // under "Beitrag", before any sibling discount or exemption (#5237).
      // It follows the name, so a narrow field cuts the times, not the price.
      pricedTimeModel: (model: HolidayRegistrationTimeModel) =>
        t("timeModelPrice", {
          name: model.name,
          price: euro.format(model.priceCents / 100),
          startsAt: model.startsAt,
          endsAt: model.endsAt,
        }),
    };
  }, [locale, t]);
}

export function ChildCard({
  index,
  child,
  offer,
  removable,
  questions,
  questionContext,
  fieldError,
  onChange,
  onRemove,
}: Readonly<{
  index: number;
  child: ChildDraft;
  offer: HolidayRegistrationOffer;
  removable: boolean;
  /** The Träger's questions per child (#5198). */
  questions: readonly HolidayFormField[];
  /** The answers the conditions of these questions read. */
  questionContext: ConditionContext;
  fieldError: FieldError;
  onChange: (child: ChildDraft) => void;
  onRemove: () => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const dateOfBirth = useTranslations("enrollmentForm");
  const labels = useWeekLabels();
  const prefix = `children.${index}`;
  const guest = child.school === OTHER_SCHOOL;
  const set = (patch: Partial<ChildDraft>) => onChange({ ...child, ...patch });
  // The server names a single booking ("children.0.bookings.1") when a week
  // or time model went away after the page loaded; every booking control of
  // the child carries that hint.
  const bookingsError =
    fieldError(`${prefix}.bookings`) ??
    offer.weeks
      .map((_, booking) => fieldError(`${prefix}.bookings.${booking}`))
      .find(Boolean);
  const birthDateError = fieldError(`${prefix}.birth_date`);

  return (
    <SectionCard
      title={t("childTitle", { number: index + 1 })}
      headingLevel={3}
      action={
        removable ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onRemove}
          >
            {t("removeChild")}
          </Button>
        ) : undefined
      }
    >
      <div className="space-y-5">
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Input
            name={`${prefix}.first_name`}
            label={t("firstName")}
            required
            autoComplete="off"
            maxLength={100}
            value={child.firstName}
            error={fieldError(`${prefix}.first_name`)}
            onChange={(event) => set({ firstName: event.target.value })}
          />
          <Input
            name={`${prefix}.last_name`}
            label={t("lastName")}
            required
            autoComplete="off"
            maxLength={100}
            value={child.lastName}
            error={fieldError(`${prefix}.last_name`)}
            onChange={(event) => set({ lastName: event.target.value })}
          />
        </div>
        <div>
          <FieldLabel required>{t("birthDate")}</FieldLabel>
          <DateOfBirthPicker
            value={child.birthDate}
            onChange={(birthDate) => set({ birthDate })}
            error={birthDateError}
            tr={dateOfBirth}
            name={`${prefix}.birth_date`}
            errorId={`${prefix}-birth-date-error`}
          />
        </div>
        <div className="space-y-2">
          <FieldLabel id={`${prefix}-school-label`} required>
            {t("school")}
          </FieldLabel>
          <CustomSelect
            name={`${prefix}.home_school_id`}
            ariaLabelledBy={`${prefix}-school-label`}
            value={child.school}
            placeholder={t("schoolPlaceholder")}
            invalid={Boolean(fieldError(`${prefix}.home_school_id`))}
            ariaDescribedBy={`${prefix}-school-error`}
            options={[
              ...offer.schools.map((school) => ({
                value: school.id,
                label: school.name,
              })),
              { value: OTHER_SCHOOL, label: t("otherSchool") },
            ]}
            onChange={(school) => set({ school })}
          />
          <FieldErrorHint
            id={`${prefix}-school-error`}
            message={fieldError(`${prefix}.home_school_id`)}
          />
          {guest ? (
            <>
              <p className="text-sm text-gray-600">{t("guestHint")}</p>
              <Input
                name={`${prefix}.guest_school_name`}
                label={t("guestSchoolName")}
                maxLength={120}
                value={child.guestSchoolName}
                onChange={(event) =>
                  set({ guestSchoolName: event.target.value })
                }
              />
            </>
          ) : null}
        </div>

        <fieldset className="space-y-3">
          <legend className="text-sm font-semibold text-gray-900">
            {t("weeksTitle")}
            <RequiredMark />
          </legend>
          <p className="text-sm text-gray-600">
            {`${t("weeksText")} ${t("weeksPriceText")}`}
          </p>
          {offer.weeks.map((week) => (
            <div
              key={week.id}
              className="grid grid-cols-1 items-center gap-2 sm:grid-cols-2"
            >
              <span
                id={`${prefix}-week-${week.id}`}
                className="text-sm text-gray-900"
              >
                {labels.range(week)}
                <span className="block text-xs text-gray-500">
                  {week.hostSchoolName}
                </span>
              </span>
              <CustomSelect
                name={`${prefix}.bookings`}
                ariaLabelledBy={`${prefix}-week-${week.id}`}
                value={child.bookings[week.id] ?? ""}
                invalid={Boolean(bookingsError)}
                ariaDescribedBy={
                  bookingsError ? `${prefix}-bookings-error` : undefined
                }
                options={[
                  { value: "", label: t("notBooked") },
                  ...week.timeModels.map((model) => ({
                    value: model.id,
                    label: labels.pricedTimeModel(model),
                  })),
                ]}
                onChange={(timeModelId) => {
                  const bookings = { ...child.bookings };
                  if (timeModelId) bookings[week.id] = timeModelId;
                  else delete bookings[week.id];
                  set({ bookings });
                }}
              />
            </div>
          ))}
          <FieldErrorHint
            id={`${prefix}-bookings-error`}
            message={bookingsError}
          />
        </fieldset>

        <fieldset className="space-y-3">
          <legend className="text-sm font-semibold text-gray-900">
            {t("healthTitle")}
          </legend>
          <p className="text-sm text-gray-600">{t("healthText")}</p>
          <Input
            name={`${prefix}.allergies`}
            label={t("allergies")}
            maxLength={1000}
            value={child.allergies}
            onChange={(event) => set({ allergies: event.target.value })}
          />
          <Input
            name={`${prefix}.medication`}
            label={t("medication")}
            maxLength={1000}
            value={child.medication}
            onChange={(event) => set({ medication: event.target.value })}
          />
          <Textarea
            name={`${prefix}.health_notes`}
            label={t("healthNotes")}
            rows={2}
            maxLength={2000}
            value={child.healthNotes}
            onChange={(event) => set({ healthNotes: event.target.value })}
          />
        </fieldset>

        {questions.some((field) => isFieldVisible(field, questionContext)) ? (
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-900">
              {t("childQuestionsTitle")}
            </legend>
            <HolidayQuestions
              fields={questions}
              answers={child.answers}
              context={questionContext}
              prefix={`${prefix}.`}
              fieldError={fieldError}
              onChange={(key, value) =>
                set({ answers: { ...child.answers, [key]: value } })
              }
            />
          </fieldset>
        ) : null}
      </div>
    </SectionCard>
  );
}

export function ContactList({
  kind,
  contacts,
  max,
  fieldError,
  onChange,
}: Readonly<{
  kind: "emergency_contacts" | "pickup_persons";
  contacts: readonly ContactDraft[];
  max: number;
  fieldError: FieldError;
  onChange: (contacts: readonly ContactDraft[]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const emergency = kind === "emergency_contacts";
  const setContact = (index: number, contact: ContactDraft) =>
    onChange(contacts.map((current, i) => (i === index ? contact : current)));

  return (
    <SectionCard
      title={emergency ? t("emergencyTitle") : t("pickupTitle")}
      description={emergency ? t("emergencyText") : t("pickupText")}
    >
      <div className="space-y-4">
        {contacts.length === 0 ? (
          <p className="text-sm text-gray-600">{t("noPickup")}</p>
        ) : null}
        {contacts.map((contact, index) => (
          <fieldset
            key={contact.key}
            className="grid grid-cols-1 gap-3 sm:grid-cols-3"
          >
            <legend className="sr-only">
              {t("contactTitle", { number: index + 1 })}
            </legend>
            <Input
              name={`${kind}.${index}.name`}
              label={t("name")}
              required
              maxLength={120}
              value={contact.name}
              error={fieldError(`${kind}.${index}.name`)}
              onChange={(event) =>
                setContact(index, { ...contact, name: event.target.value })
              }
            />
            <Input
              name={`${kind}.${index}.phone`}
              type="tel"
              label={emergency ? t("phone") : t("phoneOptional")}
              required={emergency}
              maxLength={40}
              value={contact.phone}
              error={fieldError(`${kind}.${index}.phone`)}
              onChange={(event) =>
                setContact(index, { ...contact, phone: event.target.value })
              }
            />
            <div className="flex items-end gap-2">
              <div className="min-w-0 flex-1">
                <Input
                  name={`${kind}.${index}.relationship`}
                  label={t("relationship")}
                  maxLength={60}
                  value={contact.relationship}
                  onChange={(event) =>
                    setContact(index, {
                      ...contact,
                      relationship: event.target.value,
                    })
                  }
                />
              </div>
              {emergency && contacts.length === 1 ? null : (
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  onClick={() =>
                    onChange(contacts.filter((_, i) => i !== index))
                  }
                >
                  {t("remove")}
                </Button>
              )}
            </div>
          </fieldset>
        ))}
        {contacts.length < max ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => onChange([...contacts, emptyContact()])}
          >
            {emergency ? t("addContact") : t("addPickup")}
          </Button>
        ) : null}
      </div>
    </SectionCard>
  );
}

/**
 * The children the schools of the Träger know of a parent with an account
 * (#4662). Picking one fills a child of the form; the parent can still change
 * every value there, and that changes only the registration.
 */
export function KnownChildPicker({
  known,
  draft,
  offer,
  onToggle,
}: Readonly<{
  known: readonly HolidayKnownChild[];
  draft: RegistrationDraft;
  offer: HolidayRegistrationOffer;
  onToggle: (child: HolidayKnownChild) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  const picked = new Set(draft.children.map((child) => child.knownStudentId));
  const full = !hasRoomForKnownChild(draft);
  const schoolName = (id: string) =>
    offer.schools.find((school) => school.id === id)?.name ?? "";

  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-semibold text-gray-900">
        {t("knownChildrenTitle")}
      </legend>
      <p className="text-sm text-gray-600">{t("knownChildrenText")}</p>
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
        {known.map((child) => {
          const selected = picked.has(child.studentId);
          const disabled = !selected && full;
          return (
            <ChoiceTile
              key={child.studentId}
              selected={selected}
              disabled={disabled}
              tone="green"
              className="items-start"
            >
              <Checkbox
                checked={selected}
                disabled={disabled}
                onChange={() => onToggle(child)}
              />
              <span className="flex flex-col">
                <span className="text-gray-900">
                  {child.firstName} {child.lastName}
                </span>
                <span className="font-normal text-gray-600">
                  {child.birthDate
                    ? t("knownChildMeta", {
                        school: schoolName(child.schoolId),
                        birthDate: formatDate(child.birthDate, false, locale),
                      })
                    : schoolName(child.schoolId)}
                </span>
              </span>
            </ChoiceTile>
          );
        })}
      </div>
      {full ? (
        <p className="text-sm text-gray-600">{t("knownChildrenFull")}</p>
      ) : null}
    </fieldset>
  );
}

/** Exactly one optional co-guardian; the registering parent's data stays separate. */
export function SecondGuardianCard({
  guardian,
  fieldError,
  onChange,
}: Readonly<{
  guardian: RegistrationDraft["guardian"];
  fieldError: FieldError;
  onChange: (guardian: RegistrationDraft["guardian"]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const namesRequired = Object.values(guardian).some(
    (value) => value.trim() !== "",
  );
  return (
    <SectionCard
      title={t("secondGuardianTitle")}
      description={t("secondGuardianText")}
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <Input
          name="second_guardian.first_name"
          label={t("firstName")}
          required={namesRequired}
          autoComplete="section-second-guardian given-name"
          maxLength={100}
          value={guardian.firstName}
          error={fieldError("second_guardian.first_name")}
          onChange={(event) =>
            onChange({ ...guardian, firstName: event.target.value })
          }
        />
        <Input
          name="second_guardian.last_name"
          label={t("lastName")}
          required={namesRequired}
          autoComplete="section-second-guardian family-name"
          maxLength={100}
          value={guardian.lastName}
          error={fieldError("second_guardian.last_name")}
          onChange={(event) =>
            onChange({ ...guardian, lastName: event.target.value })
          }
        />
        <Input
          name="second_guardian.email"
          type="email"
          label={t("emailOptional")}
          autoComplete="section-second-guardian email"
          maxLength={254}
          value={guardian.email}
          error={fieldError("second_guardian.email")}
          onChange={(event) =>
            onChange({ ...guardian, email: event.target.value })
          }
        />
        <Input
          name="second_guardian.phone"
          type="tel"
          label={t("phoneOptional")}
          autoComplete="section-second-guardian tel"
          maxLength={40}
          value={guardian.phone}
          error={fieldError("second_guardian.phone")}
          onChange={(event) =>
            onChange({ ...guardian, phone: event.target.value })
          }
        />
      </div>
    </SectionCard>
  );
}
