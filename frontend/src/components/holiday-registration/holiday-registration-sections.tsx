"use client";

// Die Abschnitte der Ferienanmeldung (#4661): ein Kind mit Schule, Wochen und
// Gesundheit, die Listen der Notfallkontakte und Abholberechtigten und die
// bekannten Kinder eines Elternkontos (#4662).

import { useLocale, useTranslations } from "next-intl";
import { useMemo } from "react";
import { DateOfBirthPicker } from "~/components/enrollment/date-of-birth-picker";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { Alert } from "~/components/ui/alert";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { WeekdayToggleGroup } from "~/components/ui/weekday-toggle-group";
import { formatDate } from "~/lib/date-helpers";
import {
  holidayCareDays,
  holidayDayLabel,
  holidayDayList,
  holidayWeekdayShort,
} from "~/lib/holiday-days";
import {
  type ConditionContext,
  isFieldVisible,
} from "~/lib/enrollment-field-visibility";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import {
  groupParallelWeeks,
  weekGroupBooking,
  withWeekGroupBooking,
} from "~/lib/holiday-week-groups";
import type {
  HolidayKnownChild,
  HolidayRegistrationOffer,
  HolidayRegistrationTimeModel,
  HolidayRegistrationWeek,
} from "~/lib/holiday-registration-api";
import {
  type AddressDraft,
  bookedDayCount,
  bookedWeeks,
  type ChildDraft,
  type ContactDraft,
  emptyContact,
  type FieldError,
  hasRoomForKnownChild,
  knownChildRefused,
  OTHER_SCHOOL,
  type RegistrationDraft,
} from "./holiday-registration-model";
import { GuardianAddressFields } from "./holiday-guardian-address-fields";
import { HolidayQuestions } from "./holiday-registration-questions";
import { useCareTermTranslations } from "~/lib/care-facility-term";

export function useWeekLabels() {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  return useMemo(() => {
    const euro = new Intl.NumberFormat(locale, {
      style: "currency",
      currency: "EUR",
    });
    return {
      locale,
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
      pricedTimeModel: (model: HolidayRegistrationTimeModel, perDay = false) =>
        t(perDay ? "timeModelDayPrice" : "timeModelPrice", {
          name: model.name,
          price: euro.format(model.priceCents / 100),
          startsAt: model.startsAt,
          endsAt: model.endsAt,
        }),
    };
  }, [locale, t]);
}

/**
 * One choice for the weeks on the same days (#5689): a single week offers its
 * time models, parallel weeks offer those of all their schools, so a child
 * takes part in only one of them.
 */
function WeekChoice({
  group,
  prefix,
  value,
  days,
  daysName,
  daysError,
  invalid,
  onChange,
  onDaysChange,
}: Readonly<{
  group: readonly HolidayRegistrationWeek[];
  prefix: string;
  value: string;
  /** The chosen days of the booked week, when it is booked by days. */
  days: readonly string[];
  daysName?: string;
  daysError?: string;
  invalid: boolean;
  onChange: (timeModelId: string) => void;
  onDaysChange: (weekId: string, days: readonly string[]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const labels = useWeekLabels();
  const first = group[0];
  if (!first) return null;
  const parallel = group.length > 1;
  const optionLabel = (
    week: HolidayRegistrationWeek,
    model: HolidayRegistrationTimeModel,
  ) => {
    const priced = labels.pricedTimeModel(model, week.bookingUnit === "day");
    return parallel ? `${week.hostSchoolName}: ${priced}` : priced;
  };
  const labelId = `${prefix}-week-${first.id}`;
  // A week with a day without care at its host school takes no booking
  // (#4702); the server refuses it as well. A week by days stays bookable:
  // that day is simply not offered (#4701).
  const bookable = group.filter(
    (week) => week.bookingUnit === "day" || !week.noCareDays?.length,
  );
  const blocked = group.filter(
    (week) => week.bookingUnit !== "day" && (week.noCareDays?.length ?? 0) > 0,
  );
  const blockedHints = blocked.map((week) => {
    const day = week.noCareDays![0]!;
    const hint = t("weekWithoutCare", {
      date: formatDate(day.date, false, labels.locale),
      name: day.name,
    });
    return parallel ? `${week.hostSchoolName}: ${hint}` : hint;
  });
  const booked = group.find((week) =>
    week.timeModels.some((model) => model.id === value),
  );
  return (
    <div className="space-y-3">
      <div className="grid grid-cols-1 items-center gap-2 sm:grid-cols-2">
        <span id={labelId} className="text-sm text-gray-900">
          {labels.range(first)}
          <span className="block text-xs text-gray-500">
            {parallel ? t("parallelWeeksHint") : first.hostSchoolName}
          </span>
          {blockedHints.map((hint) => (
            <span key={hint} className="mt-1 block text-xs text-gray-600">
              {hint}
            </span>
          ))}
        </span>
        <CustomSelect
          name={`${prefix}.bookings`}
          wrapLabels
          ariaLabelledBy={labelId}
          value={value}
          invalid={invalid}
          disabled={bookable.length === 0 && !value}
          ariaDescribedBy={invalid ? `${prefix}-bookings-error` : undefined}
          options={[
            { value: "", label: t("notBooked") },
            ...bookable.flatMap((week) =>
              week.timeModels.map((model) => ({
                value: model.id,
                label: optionLabel(week, model),
              })),
            ),
          ]}
          onChange={onChange}
        />
      </div>
      {booked?.bookingUnit === "day" ? (
        <DayChoice
          week={booked}
          label={labels.range(booked)}
          days={days}
          name={daysName}
          error={daysError}
          onChange={(next) => onDaysChange(booked.id, next)}
        />
      ) : null}
    </div>
  );
}

/**
 * The days of a week by days (#4701): one toggle per care day. A full day
 * stays selectable; the line under the days says that a booking with it
 * waits as a whole.
 */
function DayChoice({
  week,
  label,
  days,
  name,
  error,
  onChange,
}: Readonly<{
  week: HolidayRegistrationWeek;
  label: string;
  days: readonly string[];
  name?: string;
  error?: string;
  onChange: (days: readonly string[]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const locale = useLocale();
  const fullDays = week.fullDays ?? [];
  // Every day from Monday to Friday stays in the row; a day without care
  // (#6447) is shown struck through and cannot be chosen.
  const bookable = week.careDays ?? [];
  const weekDays = [
    ...new Set([...holidayCareDays(week.startsOn, week.endsOn), ...bookable]),
  ].sort();
  const chosenFull = days.filter((day) => fullDays.includes(day));
  const hint = [
    t("daysHint"),
    ...(week.noCareDays ?? []).map((day) =>
      t("dayWithoutCare", {
        date: holidayDayLabel(day.date, locale),
        name: day.name,
      }),
    ),
    fullDays.length > 0
      ? t("fullDaysHint", { days: holidayDayList(fullDays, locale) })
      : "",
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <InsetPanel padding="sm" className="space-y-2 sm:ml-auto sm:w-1/2">
      <WeekdayToggleGroup
        name={name}
        error={error}
        ariaLabel={t("daysLabel", { week: label })}
        days={weekDays.map((day) => ({
          value: day,
          label: holidayWeekdayShort(day, locale),
          disabled: !bookable.includes(day),
          ariaLabel: !bookable.includes(day)
            ? t("noCareDayAria", { day: holidayDayLabel(day, locale) })
            : fullDays.includes(day)
              ? t("fullDayAria", { day: holidayDayLabel(day, locale) })
              : holidayDayLabel(day, locale),
        }))}
        value={days}
        onToggle={(day) =>
          onChange(
            days.includes(day)
              ? days.filter((chosen) => chosen !== day)
              : [...days, day].sort(),
          )
        }
        hint={hint}
      />
      {chosenFull.length > 0 ? (
        <Alert
          type="warning"
          size="compact"
          message={t("fullDaysChosen", {
            count: chosenFull.length,
            days: holidayDayList(chosenFull, locale),
          })}
        />
      ) : null}
    </InsetPanel>
  );
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
  const dateOfBirth = useCareTermTranslations("enrollmentForm");
  const prefix = `children.${index}`;
  const guest = child.school === OTHER_SCHOOL;
  const set = (patch: Partial<ChildDraft>) => onChange({ ...child, ...patch });
  const bookings = bookedWeeks(child, offer);
  // The server names a single booking ("children.0.bookings.1") when a week
  // or time model went away after the page loaded; every booking control of
  // the child carries that hint.
  const bookingsError =
    fieldError(`${prefix}.bookings`) ??
    offer.weeks
      .map((_, booking) => fieldError(`${prefix}.bookings.${booking}`))
      .find(Boolean);
  const birthDateError = fieldError(`${prefix}.birth_date`);
  const dayCount = bookedDayCount(child, offer);
  const overDayLimit =
    offer.maxDaysPerChild != null && dayCount > offer.maxDaysPerChild;

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
            {[
              t("weeksText"),
              t("weeksPriceText"),
              offer.weeks.some((week) => week.bookingUnit === "day")
                ? t("weeksDayPriceText")
                : "",
            ]
              .filter(Boolean)
              .join(" ")}
          </p>
          {offer.maxDaysPerChild == null ? null : (
            <Alert
              type={overDayLimit ? "warning" : "info"}
              size="compact"
              message={`${t("dayLimitHint", { limit: offer.maxDaysPerChild })} ${t(overDayLimit ? "dayLimitOver" : "dayLimitCount", { count: dayCount })}`}
            />
          )}
          {groupParallelWeeks(offer.weeks).map((group) => {
            const value = weekGroupBooking(child.bookings, group);
            const bookedWeek = group.find((week) =>
              week.timeModels.some((model) => model.id === value),
            );
            const bookingIndex = bookings.findIndex(
              (booking) => booking.weekId === bookedWeek?.id,
            );
            const daysName =
              bookingIndex >= 0
                ? `${prefix}.bookings.${bookingIndex}.days`
                : undefined;
            return (
              <WeekChoice
                key={group[0]?.id}
                group={group}
                prefix={prefix}
                value={value}
                days={bookedWeek ? (child.days?.[bookedWeek.id] ?? []) : []}
                daysName={daysName}
                daysError={daysName ? fieldError(daysName) : undefined}
                onDaysChange={(weekId, days) =>
                  set({ days: { ...child.days, [weekId]: days } })
                }
                invalid={Boolean(bookingsError)}
                onChange={(timeModelId) =>
                  set({
                    bookings: withWeekGroupBooking(
                      child.bookings,
                      group,
                      timeModelId,
                    ),
                  })
                }
              />
            );
          })}
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
  contacts,
  max,
  fieldError,
  onChange,
}: Readonly<{
  contacts: readonly ContactDraft[];
  max: number;
  fieldError: FieldError;
  onChange: (contacts: readonly ContactDraft[]) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  const setContact = (index: number, contact: ContactDraft) =>
    onChange(contacts.map((current, i) => (i === index ? contact : current)));

  return (
    <SectionCard title={t("contactsTitle")} description={t("contactsText")}>
      <div className="space-y-4">
        {contacts.map((contact, index) => (
          <fieldset
            key={contact.key}
            className="grid grid-cols-1 gap-3 sm:grid-cols-3"
          >
            <legend className="sr-only">
              {t("contactTitle", { number: index + 1 })}
            </legend>
            <Input
              name={`contacts.${index}.name`}
              label={t("name")}
              required
              maxLength={120}
              value={contact.name}
              error={fieldError(`contacts.${index}.name`)}
              onChange={(event) =>
                setContact(index, { ...contact, name: event.target.value })
              }
            />
            <Input
              name={`contacts.${index}.phone`}
              type="tel"
              label={contact.emergency ? t("phone") : t("phoneOptional")}
              required={contact.emergency}
              maxLength={40}
              value={contact.phone}
              error={fieldError(`contacts.${index}.phone`)}
              onChange={(event) =>
                setContact(index, { ...contact, phone: event.target.value })
              }
            />
            <div className="flex items-end gap-2">
              <div className="min-w-0 flex-1">
                <Input
                  name={`contacts.${index}.relationship`}
                  label={t("relationship")}
                  maxLength={60}
                  error={fieldError(`contacts.${index}.relationship`)}
                  value={contact.relationship}
                  onChange={(event) =>
                    setContact(index, {
                      ...contact,
                      relationship: event.target.value,
                    })
                  }
                />
              </div>
              {contacts.length === 1 ? null : (
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
            <div className="space-y-2 sm:col-span-3">
              {(["emergency", "pickup"] as const).map((role) => {
                const name = `contacts.${index}.${role}`;
                const error = fieldError(name);
                const hintId = `${contact.key}-${role}-error`;
                return (
                  <div key={role}>
                    <label className="flex items-center gap-2 text-sm text-gray-700">
                      <Checkbox
                        name={name}
                        checked={contact[role]}
                        aria-invalid={error ? true : undefined}
                        aria-describedby={error ? hintId : undefined}
                        onChange={(event) =>
                          setContact(index, {
                            ...contact,
                            [role]: event.target.checked,
                          })
                        }
                      />
                      {t(role === "emergency" ? "emergencyRole" : "pickupRole")}
                    </label>
                    <FieldErrorHint id={hintId} message={error} />
                  </div>
                );
              })}
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
            {t("addPickup")}
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
          const refused = knownChildRefused(child, offer);
          const disabled = !selected && (full || refused);
          const wish = knownChildWish(child, offer);
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
                {wish ? (
                  <span className="mt-1.5 flex flex-col items-start gap-1">
                    <StatusBadge tone={wish.tone} label={t(wish.label)} />
                    {wish.hint ? (
                      <span className="text-xs font-normal text-gray-600">
                        {t(wish.hint)}
                      </span>
                    ) : null}
                  </span>
                ) : null}
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

/**
 * The postal address of the parent who registers (#5625), when the form of
 * the period asks for it. Optional takes all three parts or none.
 */
export function GuardianAddressCard({
  address,
  required,
  fieldError,
  onChange,
}: Readonly<{
  address: AddressDraft;
  required: boolean;
  fieldError: FieldError;
  onChange: (address: AddressDraft) => void;
}>) {
  const t = useTranslations("holidayRegistration");
  return (
    <SectionCard
      title={required ? t("addressTitle") : t("addressTitleOptional")}
      description={required ? t("addressText") : t("addressOptionalText")}
    >
      <GuardianAddressFields
        address={address}
        required={required}
        labels={{
          street: t("street"),
          postalCode: t("postalCode"),
          city: t("city"),
        }}
        fieldError={fieldError}
        onChange={onChange}
      />
    </SectionCard>
  );
}

type WishLabel =
  "wishKnownYes" | "wishKnownNo" | "wishKnownUnknown" | "wishRefused";

/**
 * What the form tells the parent about a known child's holiday care wish
 * (#4703), only while the period takes it into account. The wish comes from
 * the parent's own OGS registration; the school knows nobody else's.
 */
function knownChildWish(
  child: HolidayKnownChild,
  offer: HolidayRegistrationOffer,
): {
  tone: "green" | "gray" | "warning";
  label: WishLabel;
  hint?: "wishHintNo" | "wishHintRefused" | "wishHintUnknown";
} | null {
  if (offer.holidayWishMode === "off") return null;
  if (child.holidayWish === "yes")
    return { tone: "green", label: "wishKnownYes" };
  if (child.holidayWish === "no") {
    return knownChildRefused(child, offer)
      ? { tone: "warning", label: "wishRefused", hint: "wishHintRefused" }
      : { tone: "gray", label: "wishKnownNo", hint: "wishHintNo" };
  }
  return { tone: "gray", label: "wishKnownUnknown", hint: "wishHintUnknown" };
}
