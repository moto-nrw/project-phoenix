// Entwurf, Prüfung und Versand der öffentlichen Ferienanmeldung (#4661).
// Die Feldnamen folgen dem Backend ("children.0.first_name"), damit Fehler
// vom Server am selben Feld landen wie die eigene Prüfung.

import { holidayCareDays } from "~/lib/holiday-days";
import { bookedWeeksOverlap } from "~/lib/holiday-week-groups";
import {
  buildFieldsByKey,
  type ConditionContext,
  isFieldVisible,
  visibleAnswerData,
} from "~/lib/enrollment-field-visibility";
import type {
  HolidayFormField,
  HolidayFormVersion,
} from "~/lib/holiday-form-field";
import { looksLikeEmail } from "~/lib/email-format";
import type { NewUploadValue } from "~/lib/enrollment-uploads";
import type {
  HolidayContactPayload,
  HolidayKnownChild,
  HolidayKnownFamily,
  HolidayQuoteChild,
  HolidayRegistrationOffer,
  HolidayRegistrationPayload,
} from "~/lib/holiday-registration-api";

/**
 * Version of the consent texts the translations of this form belong to
 * (#4722, #5626). The German texts come with the offer and bind; a
 * translation is shown below them only while the offer has this version.
 * Version 3 adds the custody and the name as the declaration. Version 4
 * names the retention of the Träger in days instead of three months (#6691).
 * Version 5 says that the Träger keeps the customer number of the Gemeinde
 * (Debitorennummer) for later registrations and when it goes (#6692).
 */
export const HOLIDAY_CONSENT_VERSION = 5;

/** Bounds the typed name like the name of the parent. */
export const DECLARATION_NAME_MAX_LENGTH = 100;

export const MAX_CHILDREN = 3;
// Mirrors HolidayRegistrationMaxEmergencyContacts / MaxPickupPersons in
// backend/modules/enrollment/internal/application/holiday_registrations.go.
// The offer does not expose these limits.
export const MAX_EMERGENCY_CONTACTS = 3;
export const MAX_PICKUP_PERSONS = 5;
export const MAX_CONTACTS = MAX_EMERGENCY_CONTACTS + MAX_PICKUP_PERSONS;

/** The school choice of a child who attends no school of the Träger. */
export const OTHER_SCHOOL = "other";

/** A parent's answer to a required health field (#6686). */
export type HolidayHealthChoice = "none" | "some";

/**
 * What a required health field stores when the child has none (#6686). The
 * Träger-Büro and the hosting OGS read German, so it stays German in every
 * language of the form.
 */
export const HOLIDAY_HEALTH_NONE = "Keine";

/** Whether the form requires allergies and medication (#6686). */
export function requiresHealth(form: HolidayFormVersion): boolean {
  return form.healthRequirement === "required";
}

/** The fixed health fields a required form asks with a choice. */
const REQUIRED_HEALTH_FIELDS = ["allergies", "medication"] as const;
export type RequiredHealthField = (typeof REQUIRED_HEALTH_FIELDS)[number];

function healthChoice(
  child: ChildDraft,
  field: RequiredHealthField,
): HolidayHealthChoice | undefined {
  return field === "allergies" ? child.allergiesChoice : child.medicationChoice;
}

/**
 * The fixed health fields of a child as the backend reads them: trimmed,
 * and for a required form „Keine“ or the text after the parent's choice.
 */
export function childHealthValues(
  child: ChildDraft,
  form: HolidayFormVersion,
): { allergies: string; medication: string; healthNotes: string } {
  const value = (field: RequiredHealthField) => {
    if (!requiresHealth(form)) return child[field].trim();
    const choice = healthChoice(child, field);
    if (choice === "none") return HOLIDAY_HEALTH_NONE;
    return choice === "some" ? child[field].trim() : "";
  };
  return {
    allergies: value("allergies"),
    medication: value("medication"),
    healthNotes: child.healthNotes.trim(),
  };
}

export interface ChildDraft {
  readonly key: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly birthDate: string;
  /** A school ID of the Träger, OTHER_SCHOOL or "" before a choice. */
  readonly school: string;
  readonly guestSchoolName: string;
  readonly allergies: string;
  readonly medication: string;
  readonly healthNotes: string;
  /**
   * The answer to the required question „Allergien“ or „Medikamente“
   * (#6686): none sends „Keine“, some sends the text. Undefined before a
   * choice; an optional form ignores it and sends the text.
   */
  readonly allergiesChoice?: HolidayHealthChoice;
  readonly medicationChoice?: HolidayHealthChoice;
  /** Week ID → time model ID; a missing week is not booked. */
  readonly bookings: Readonly<Record<string, string>>;
  /**
   * Week ID → chosen days ("YYYY-MM-DD") of a week by days (#4701). Only
   * the days of a booked week count.
   */
  readonly days?: Readonly<Record<string, readonly string[]>>;
  /**
   * Week ID → the child eats lunch (#5631). Only a booked week that offers
   * lunch counts.
   */
  readonly lunch?: Readonly<Record<string, boolean>>;
  /** The child of the parent's account this draft started from (#4662). */
  readonly knownStudentId?: string;
  /**
   * The sealed statement of the known child's holiday care wish (#4703). It
   * counts only while name, date of birth and school stay as the school
   * knows them; the backend checks that.
   */
  readonly wishProof?: string;
  /** Answers to the Träger's questions per child, by key (#5198). */
  readonly answers: Readonly<Record<string, unknown>>;
  /** Yes/no to the criteria per child, by criterion ID (#4700). */
  readonly criteria?: Readonly<Record<string, boolean>>;
}

export interface ContactDraft {
  readonly emergency: boolean;
  readonly pickup: boolean;
  readonly key: string;
  readonly name: string;
  readonly phone: string;
  readonly relationship: string;
  /** The guardian of a known child this contact came from (#6682). */
  readonly knownGuardianId?: string;
  /**
   * All school records of that person: siblings at two schools bring one
   * person under two records (#6682).
   */
  readonly knownPersonIds?: readonly string[];
  /** What the prefill wrote, to keep the parent's own changes (#6682). */
  readonly prefilled?: Pick<
    ContactDraft,
    "name" | "phone" | "relationship" | "emergency" | "pickup"
  >;
}

export interface AddressDraft {
  readonly street: string;
  readonly postalCode: string;
  readonly city: string;
}

export interface RegistrationDraft {
  readonly guardian: {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
    readonly phone: string;
  };
  /** Postal address of the parent, when the form asks for it (#5625). */
  readonly guardianAddress: AddressDraft;
  readonly secondGuardian: RegistrationDraft["guardian"];
  /**
   * The custodian the prefill wrote into the second guardian (#6682); gone
   * once nothing of the prefill is left there.
   */
  readonly secondGuardianPrefill?: {
    readonly guardianId: string;
    readonly values: RegistrationDraft["guardian"];
  };
  readonly children: readonly ChildDraft[];
  readonly contacts: readonly ContactDraft[];
  /**
   * School records of the persons from the account the parent removed from
   * the contacts (#6682).
   */
  readonly dismissedContacts?: readonly string[];
  readonly legalAcceptances?: Readonly<Record<string, boolean>>;
  readonly privacyConsent: boolean;
  readonly healthConsent: boolean;
  readonly bindingConfirmed: boolean;
  /** The parent has the custody or the other custodian agrees (#5626). */
  readonly custodyConfirmed: boolean;
  /** The name the parent types to give the declarations (#5626). */
  readonly declarationName: string;
  /** The family receives benefits; the Träger checks it (#4664). */
  readonly benefitsDeclared: boolean;
  /**
   * The proofs of the benefits the parent uploaded (#5238); optional, the
   * Träger-Büro may still collect them later.
   */
  readonly benefitsProofs: readonly NewUploadValue[];
  /** Answers to the Träger's questions per family, by key (#5198). */
  readonly answers: Readonly<Record<string, unknown>>;
  /** Yes/no to the criteria of the family, by criterion ID (#4700). */
  readonly criteria?: Readonly<Record<string, boolean>>;
  /**
   * The answer to „Möchten Sie ein Elternkonto anlegen?“ (#6673); absent
   * until the parent chose. Only asked without a parent session.
   */
  readonly parentAccountRequested?: boolean;
  /**
   * The sealed proof of the signed-in parent's account from the prefill
   * (#6673, ADR 0216). With it the question is not asked.
   */
  readonly parentAccountProof?: string;
}

/** Hint of a refused field, translated, or undefined. */
export type FieldError = (name: string) => string | undefined;

/** The reason a field is refused, as a key of `holidayRegistration.errors`. */
export type FieldErrorKey =
  | "required"
  | "email"
  | "phone"
  | "school"
  | "weeks"
  | "weeksOverlap"
  | "days"
  | "dayLimit"
  | "privacy"
  | "health"
  | "healthRequired"
  | "binding"
  | "custody"
  | "declarationName"
  | "yesNo"
  | "choose"
  | "contactRole"
  | "emergencyRequired"
  | "emergencyLimit"
  | "pickupLimit"
  | "contactLimit"
  | "postalCode";

let keyCounter = 0;
const nextKey = (prefix: string) => `${prefix}-${++keyCounter}`;

export function emptyChild(): ChildDraft {
  return {
    key: nextKey("child"),
    firstName: "",
    lastName: "",
    birthDate: "",
    school: "",
    guestSchoolName: "",
    allergies: "",
    medication: "",
    healthNotes: "",
    bookings: {},
    answers: {},
  };
}

export function emptyContact(): ContactDraft {
  return {
    key: nextKey("contact"),
    name: "",
    phone: "",
    relationship: "",
    emergency: true,
    pickup: false,
  };
}

export function emptyRegistration(): RegistrationDraft {
  return {
    guardian: { firstName: "", lastName: "", email: "", phone: "" },
    guardianAddress: { street: "", postalCode: "", city: "" },
    secondGuardian: { firstName: "", lastName: "", email: "", phone: "" },
    children: [emptyChild()],
    contacts: [emptyContact()],
    privacyConsent: false,
    healthConsent: false,
    bindingConfirmed: false,
    custodyConfirmed: false,
    declarationName: "",
    benefitsDeclared: false,
    benefitsProofs: [],
    answers: {},
  };
}

/**
 * A registration of a parent with an account (#4662): the parent fields and
 * the postal address (#6682) come from what the school knows. Without a
 * family the registration starts empty.
 */
export function registrationFromFamily(
  family: HolidayKnownFamily | null,
): RegistrationDraft {
  const draft = emptyRegistration();
  if (!family) return draft;
  const { address, ...guardian } = family.guardian;
  return {
    ...draft,
    guardian,
    ...(address ? { guardianAddress: { ...address } } : {}),
    ...(family.accountProof ? { parentAccountProof: family.accountProof } : {}),
  };
}

function childFromKnown(
  known: HolidayKnownChild,
  offer: HolidayRegistrationOffer,
): ChildDraft {
  return {
    ...emptyChild(),
    firstName: known.firstName,
    lastName: known.lastName,
    birthDate: known.birthDate,
    school: offer.schools.some((school) => school.id === known.schoolId)
      ? known.schoolId
      : "",
    knownStudentId: known.studentId,
    ...(known.holidayWishProof ? { wishProof: known.holidayWishProof } : {}),
  };
}

/**
 * A known child the period refuses (#4703): only children with a holiday
 * care wish may register, and the school knows the parents said no.
 */
export function knownChildRefused(
  known: HolidayKnownChild,
  offer: HolidayRegistrationOffer,
): boolean {
  return offer.holidayWishMode === "wish_only" && known.holidayWish === "no";
}

function isUntouched(child: ChildDraft): boolean {
  const blank = emptyChild();
  return (
    child.knownStudentId === undefined &&
    child.firstName === blank.firstName &&
    child.lastName === blank.lastName &&
    child.birthDate === blank.birthDate &&
    child.school === blank.school &&
    child.guestSchoolName === blank.guestSchoolName &&
    child.allergies === blank.allergies &&
    child.medication === blank.medication &&
    child.healthNotes === blank.healthNotes &&
    child.allergiesChoice === undefined &&
    child.medicationChoice === undefined &&
    Object.keys(child.bookings).length === 0 &&
    Object.keys(child.answers).length === 0
  );
}

/** Whether one more known child fits next to the children already filled. */
export function hasRoomForKnownChild(draft: RegistrationDraft): boolean {
  return (
    draft.children.filter((child) => !isUntouched(child)).length < MAX_CHILDREN
  );
}

/**
 * Picks or drops a known child. A picked child takes the place of an
 * untouched empty child, otherwise it is added while there is room. The
 * values are a copy: changing them changes only this registration.
 */
export function toggleKnownChild(
  draft: RegistrationDraft,
  known: HolidayKnownChild,
  offer: HolidayRegistrationOffer,
): RegistrationDraft {
  if (
    draft.children.some((child) => child.knownStudentId === known.studentId)
  ) {
    const children = draft.children.filter(
      (child) => child.knownStudentId !== known.studentId,
    );
    return {
      ...draft,
      children: children.length > 0 ? children : [emptyChild()],
    };
  }
  if (!hasRoomForKnownChild(draft) || knownChildRefused(known, offer)) {
    return draft;
  }
  const children = draft.children.filter((child) => !isUntouched(child));
  return { ...draft, children: [...children, childFromKnown(known, offer)] };
}

/** The answers a family-level question is evaluated against. */
export function familyContext(
  draft: RegistrationDraft,
  form: HolidayFormVersion,
): ConditionContext {
  return {
    guardianAnswers: { ...draft.answers },
    fieldsByKey: buildFieldsByKey(form.fields),
  };
}

/** The answers a question of one child is evaluated against. */
export function childContext(
  draft: RegistrationDraft,
  child: ChildDraft,
  form: HolidayFormVersion,
): ConditionContext {
  return { ...familyContext(draft, form), childAnswers: { ...child.answers } };
}

/** Whether a question carries an answer: a yes or no, or some text. */
export function isAnswered(field: HolidayFormField, value: unknown): boolean {
  if (field.type === "boolean") return typeof value === "boolean";
  return typeof value === "string" && value.trim() !== "";
}

/** The visible questions of one scope that collect an answer. */
function visibleQuestions(
  form: HolidayFormVersion,
  child: boolean,
  ctx: ConditionContext,
): HolidayFormField[] {
  return form.fields.filter(
    (field) =>
      Boolean(field.applies_to_child) === child &&
      field.type !== "information" &&
      isFieldVisible(field, ctx),
  );
}

function answersHealth(
  form: HolidayFormVersion,
  child: boolean,
  answers: Readonly<Record<string, unknown>>,
  ctx: ConditionContext,
): boolean {
  return visibleQuestions(form, child, ctx).some(
    (field) => field.health && isAnswered(field, answers[field.key]),
  );
}

function childHasHealthInfo(
  draft: RegistrationDraft,
  child: ChildDraft,
  form: HolidayFormVersion,
): boolean {
  return (
    Object.values(childHealthValues(child, form)).some(
      (value) => value !== "",
    ) ||
    answersHealth(form, true, child.answers, childContext(draft, child, form))
  );
}

/**
 * Whether the registration carries health information: the fixed health
 * fields or an answer to a question marked as health information (#5198).
 */
export function hasHealthInfo(
  draft: RegistrationDraft,
  form: HolidayFormVersion,
): boolean {
  return (
    draft.children.some((child) => childHasHealthInfo(draft, child, form)) ||
    answersHealth(form, false, draft.answers, familyContext(draft, form))
  );
}

/** The answer of a question as the backend reads it; text is trimmed. */
function sentAnswers(
  form: HolidayFormVersion,
  child: boolean,
  answers: Readonly<Record<string, unknown>>,
  ctx: ConditionContext,
): Record<string, unknown> {
  const visible = visibleAnswerData(form.fields, child, { ...answers }, ctx);
  const out: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(visible)) {
    if (typeof value === "string") {
      if (value.trim() !== "") out[key] = value.trim();
    } else if (typeof value === "boolean") {
      out[key] = value;
    }
  }
  return out;
}

/** The visible questions of the family, for the summary. */
export function visibleFamilyQuestions(
  draft: RegistrationDraft,
  form: HolidayFormVersion,
): HolidayFormField[] {
  return visibleQuestions(form, false, familyContext(draft, form));
}

/** The visible questions of one child, for the summary. */
export function visibleChildQuestions(
  draft: RegistrationDraft,
  child: ChildDraft,
  form: HolidayFormVersion,
): HolidayFormField[] {
  return visibleQuestions(form, true, childContext(draft, child, form));
}

/** The family answers that are sent: visible questions, answered ones only. */
function familyAnswersToSend(
  draft: RegistrationDraft,
  form: HolidayFormVersion,
): Record<string, unknown> {
  return sentAnswers(form, false, draft.answers, familyContext(draft, form));
}

/** The answers of one child that are sent. */
function childAnswersToSend(
  draft: RegistrationDraft,
  child: ChildDraft,
  form: HolidayFormVersion,
): Record<string, unknown> {
  return sentAnswers(
    form,
    true,
    child.answers,
    childContext(draft, child, form),
  );
}

/**
 * The criteria of one scope as yes/no questions (#4700): required, keyed by
 * the criterion's ID, so the answer fields of the form can show them.
 */
export function criterionQuestions(
  offer: HolidayRegistrationOffer,
  scope: "family" | "child",
): HolidayFormField[] {
  if (offer.allocationMode !== "criteria") return [];
  return (offer.criteria ?? [])
    .filter((criterion) => criterion.scope === scope)
    .map((criterion, index) => ({
      sort_order: index + 1,
      key: criterion.id,
      label: criterion.question,
      type: "boolean",
      required: true,
    }));
}

/** Every criterion of the scope needs a yes or a no. */
function requireCriteria(
  errors: Record<string, FieldErrorKey>,
  prefix: string,
  questions: readonly HolidayFormField[],
  answers: Readonly<Record<string, boolean>> | undefined,
) {
  for (const field of questions) {
    if (typeof answers?.[field.key] !== "boolean") {
      errors[`${prefix}criteria.${field.key}`] = "yesNo";
    }
  }
}

/** The answers the backend reads: only the current criteria of the scope. */
function criteriaToSend(
  questions: readonly HolidayFormField[],
  answers: Readonly<Record<string, boolean>> | undefined,
): Record<string, boolean> {
  return Object.fromEntries(
    questions
      .filter((field) => typeof answers?.[field.key] === "boolean")
      .map((field) => [field.key, answers?.[field.key] === true]),
  );
}

function requireAnswers(
  errors: Record<string, FieldErrorKey>,
  prefix: string,
  questions: readonly HolidayFormField[],
  answers: Readonly<Record<string, unknown>>,
) {
  for (const field of questions) {
    if (!field.required || isAnswered(field, answers[field.key])) continue;
    errors[`${prefix}custom_data.${field.key}`] =
      field.type === "boolean"
        ? "yesNo"
        : field.type === "select"
          ? "choose"
          : "required";
  }
}

/**
 * The bookings of a child in the order of the period's weeks; a week by days
 * carries its chosen care days in order (#4701). `lunch` is true only in a
 * week that offers lunch (#5631).
 */
export function bookedWeeks(
  child: ChildDraft,
  offer: HolidayRegistrationOffer,
): {
  weekId: string;
  timeModelId: string;
  days: readonly string[];
  lunch: boolean;
}[] {
  return offer.weeks.flatMap((week) => {
    const timeModelId = child.bookings[week.id];
    if (!timeModelId) return [];
    const chosen = child.days?.[week.id] ?? [];
    const days =
      week.bookingUnit === "day"
        ? (week.careDays ?? []).filter((day) => chosen.includes(day))
        : [];
    const lunch =
      week.lunchPriceCents != null && (child.lunch?.[week.id] ?? false);
    return [{ weekId: week.id, timeModelId, days, lunch }];
  });
}

/**
 * The lunches of a booking with lunch and what they cost (#5631): one lunch
 * per chosen day of a week by days, the lunches of the week otherwise. Null
 * for a week without lunch.
 */
export function weekLunch(
  week: HolidayRegistrationOffer["weeks"][number],
  chosenDays: readonly string[],
): { readonly lunches: number; readonly cents: number } | null {
  if (week.lunchPriceCents == null) return null;
  const lunches =
    week.bookingUnit === "day"
      ? (week.careDays ?? []).filter((day) => chosenDays.includes(day)).length
      : (week.lunchDays ?? 0);
  return { lunches, cents: lunches * week.lunchPriceCents };
}

/**
 * The care days a child books, as the backend counts them against the bound
 * of the period (#4701): the chosen days of a week by days, every care day
 * (Monday to Friday) of a whole week.
 */
export function bookedDayCount(
  child: ChildDraft,
  offer: HolidayRegistrationOffer,
): number {
  return bookedWeeks(child, offer).reduce((count, booking) => {
    const week = offer.weeks.find((w) => w.id === booking.weekId);
    if (!week) return count;
    return (
      count +
      (week.bookingUnit === "day"
        ? booking.days.length
        : holidayCareDays(week.startsOn, week.endsOn).length)
    );
  }, 0);
}

/** Matches the canonical optional contact formats of Enrollment intake. */
const SECOND_EMAIL = /^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$/;
const PHONE = /^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$/;

/** Whether the form takes this as the second custodian's phone. */
export function looksLikePhone(phone: string): boolean {
  return PHONE.test(phone.trim());
}

export function hasSecondGuardian(draft: RegistrationDraft): boolean {
  return Object.values(draft.secondGuardian).some(
    (value) => value.trim() !== "",
  );
}

/** A German postal code; the backend checks the same (#5625). */
const POSTAL_CODE = /^[0-9]{5}$/;

/** Whether the form asks for the address of the parent (#5625). */
export function asksForAddress(form: HolidayFormVersion): boolean {
  return (form.addressRequirement ?? "off") !== "off";
}

/** Whether the parent filled in some part of the address. */
export function hasAddress(draft: RegistrationDraft): boolean {
  return Object.values(draft.guardianAddress).some(
    (value) => value.trim() !== "",
  );
}

/**
 * Checks the address against the form: all three parts or none, all three
 * when it is required, and a postal code of five digits.
 */
function validateAddress(
  errors: Record<string, FieldErrorKey>,
  draft: RegistrationDraft,
  form: HolidayFormVersion,
) {
  if (!asksForAddress(form)) return;
  if (form.addressRequirement !== "required" && !hasAddress(draft)) return;
  const address = draft.guardianAddress;
  requireText(errors, "guardian_address.street", address.street);
  const postalCode = address.postalCode.trim();
  if (postalCode === "") errors["guardian_address.postal_code"] = "required";
  else if (!POSTAL_CODE.test(postalCode))
    errors["guardian_address.postal_code"] = "postalCode";
  requireText(errors, "guardian_address.city", address.city);
}

function requireText(
  errors: Record<string, FieldErrorKey>,
  field: string,
  value: string,
) {
  if (value.trim() === "") errors[field] = "required";
}

function validateContacts(
  errors: Record<string, FieldErrorKey>,
  contacts: readonly ContactDraft[],
) {
  let emergencyCount = 0;
  let pickupCount = 0;
  contacts.forEach((contact, index) => {
    const prefix = `contacts.${index}`;
    requireText(errors, `${prefix}.name`, contact.name);
    if (contact.emergency) {
      requireText(errors, `${prefix}.phone`, contact.phone);
      if (++emergencyCount > MAX_EMERGENCY_CONTACTS)
        errors[`${prefix}.emergency`] = "emergencyLimit";
    }
    if (contact.pickup && ++pickupCount > MAX_PICKUP_PERSONS)
      errors[`${prefix}.pickup`] = "pickupLimit";
    if (!contact.emergency && !contact.pickup)
      errors[`${prefix}.emergency`] = "contactRole";
    if (index >= MAX_CONTACTS) errors[`${prefix}.name`] = "contactLimit";
  });
  if (emergencyCount === 0)
    errors["contacts.0.emergency"] = "emergencyRequired";
}

/** Map the two payload lists back to the submitted person's form fields. */
export function contactFieldFromPayload(
  field: string,
  contacts: readonly ContactDraft[],
): string {
  const match =
    /^(emergency_contacts|pickup_persons)(?:\.(\d+)(?:\.(.+))?)?$/.exec(field);
  if (!match) return field;
  const role = match[1] === "emergency_contacts" ? "emergency" : "pickup";
  if (match[2] === undefined) return `contacts.0.${role}`;
  const indices = contacts.flatMap((contact, index) =>
    contact[role] ? [index] : [],
  );
  const index = indices[Number(match[2])];
  return index === undefined ? field : `contacts.${index}.${match[3] ?? role}`;
}

/**
 * Checks a registration before the summary; empty means complete. The
 * errors follow the order of the form, so the first one is the field the
 * parent reaches first: the form focuses it after a failed check.
 */
export function validateRegistration(
  draft: RegistrationDraft,
  offer: HolidayRegistrationOffer,
): Record<string, FieldErrorKey> {
  const errors: Record<string, FieldErrorKey> = {};
  draft.children.forEach((child, index) => {
    const prefix = `children.${index}`;
    requireText(errors, `${prefix}.first_name`, child.firstName);
    requireText(errors, `${prefix}.last_name`, child.lastName);
    requireText(errors, `${prefix}.birth_date`, child.birthDate);
    if (child.school === "") errors[`${prefix}.home_school_id`] = "school";
    const booked = bookedWeeks(child, offer);
    if (booked.length === 0) {
      errors[`${prefix}.bookings`] = "weeks";
    } else if (
      bookedWeeksOverlap(
        offer.weeks,
        booked.map((booking) => booking.weekId),
      )
    ) {
      errors[`${prefix}.bookings`] = "weeksOverlap";
    } else if (
      booked.some(
        (booking) =>
          booking.days.length === 0 &&
          offer.weeks.find((w) => w.id === booking.weekId)?.bookingUnit ===
            "day",
      )
    ) {
      errors[`${prefix}.bookings`] = "days";
    } else if (
      offer.maxDaysPerChild != null &&
      bookedDayCount(child, offer) > offer.maxDaysPerChild
    ) {
      errors[`${prefix}.bookings`] = "dayLimit";
    }
    if (requiresHealth(offer.form)) {
      for (const field of REQUIRED_HEALTH_FIELDS) {
        const choice = healthChoice(child, field);
        if (choice === undefined) errors[`${prefix}.${field}`] = "yesNo";
        else if (choice === "some") {
          requireText(errors, `${prefix}.${field}`, child[field]);
        }
      }
    }
    requireAnswers(
      errors,
      `${prefix}.`,
      visibleQuestions(
        offer.form,
        true,
        childContext(draft, child, offer.form),
      ),
      child.answers,
    );
  });
  requireText(errors, "guardian.first_name", draft.guardian.firstName);
  requireText(errors, "guardian.last_name", draft.guardian.lastName);
  if (!looksLikeEmail(draft.guardian.email)) {
    errors["guardian.email"] =
      draft.guardian.email.trim() === "" ? "required" : "email";
  }
  requireText(errors, "guardian.phone", draft.guardian.phone);
  validateAddress(errors, draft, offer.form);
  if (hasSecondGuardian(draft)) {
    const second = draft.secondGuardian;
    requireText(errors, "second_guardian.first_name", second.firstName);
    requireText(errors, "second_guardian.last_name", second.lastName);
    if (second.email.trim() && !SECOND_EMAIL.test(second.email.trim())) {
      errors["second_guardian.email"] = "email";
    }
    if (second.phone.trim() && !looksLikePhone(second.phone)) {
      errors["second_guardian.phone"] = "phone";
    }
  }
  // The questions per family follow the parent in the form.
  requireAnswers(
    errors,
    "",
    visibleQuestions(offer.form, false, familyContext(draft, offer.form)),
    draft.answers,
  );
  // The criteria follow the questions in the form (#4700).
  requireCriteria(
    errors,
    "",
    criterionQuestions(offer, "family"),
    draft.criteria,
  );
  const childCriteria = criterionQuestions(offer, "child");
  draft.children.forEach((child, index) => {
    requireCriteria(
      errors,
      `children.${index}.`,
      childCriteria,
      child.criteria,
    );
  });
  validateContacts(errors, draft.contacts);
  if (!draft.privacyConsent) errors.privacy_consent = "privacy";
  if (hasHealthInfo(draft, offer.form) && !draft.healthConsent) {
    // A required form gives no way out by deleting the answers (#6686).
    errors.health_consent = requiresHealth(offer.form)
      ? "healthRequired"
      : "health";
  }
  if (offer.binding && !draft.bindingConfirmed) {
    errors.binding_confirmed = "binding";
  }
  if (!draft.custodyConfirmed) errors.custody_confirmed = "custody";
  for (const block of offer.form.legalBlocks ?? []) {
    if (block.enabled && block.required && !draft.legalAcceptances?.[block.key])
      errors[`legal_acceptances.${block.key}`] = "required";
  }
  if (draft.declarationName.trim() === "")
    errors.declaration_name = "declarationName";
  return errors;
}

function healthPayload(values: ReturnType<typeof childHealthValues>) {
  return {
    allergies: values.allergies,
    medication: values.medication,
    health_notes: values.healthNotes,
  };
}

function contactPayload(contact: ContactDraft): HolidayContactPayload {
  return {
    name: contact.name.trim(),
    phone: contact.phone.trim(),
    relationship: contact.relationship.trim(),
  };
}

/** The bookings of a child as the backend reads them. */
export function quoteBookings(
  child: ChildDraft,
  offer: HolidayRegistrationOffer,
): HolidayQuoteChild {
  return bookedWeeks(child, offer).map((booking) => ({
    week_id: Number(booking.weekId),
    time_model_id: Number(booking.timeModelId),
    ...(booking.days.length > 0 ? { days: booking.days } : {}),
    ...(booking.lunch ? { lunch: true } : {}),
  }));
}

/**
 * The place of every child among the children by age, 1 for the oldest, for
 * Träger that rank siblings by age (#5664). The date of birth stays in the
 * form: the contribution needs only the order. A child without a date gets 0;
 * children of the same age share a place.
 */
export function quoteAgeRanks(children: readonly ChildDraft[]): number[] {
  const dates = [
    ...new Set(children.map((child) => child.birthDate).filter(Boolean)),
  ].sort((a, b) => a.localeCompare(b));
  return children.map((child) => dates.indexOf(child.birthDate) + 1);
}

/** The registration as the backend reads it. */
export function payloadFromDraft(
  draft: RegistrationDraft,
  offer: HolidayRegistrationOffer,
  captchaToken: string,
): HolidayRegistrationPayload {
  return {
    guardian: {
      first_name: draft.guardian.firstName.trim(),
      last_name: draft.guardian.lastName.trim(),
      email: draft.guardian.email.trim(),
      phone: draft.guardian.phone.trim(),
    },
    ...(asksForAddress(offer.form) && hasAddress(draft)
      ? {
          guardian_address: {
            street: draft.guardianAddress.street.trim(),
            postal_code: draft.guardianAddress.postalCode.trim(),
            city: draft.guardianAddress.city.trim(),
          },
        }
      : {}),
    ...(hasSecondGuardian(draft)
      ? {
          second_guardian: {
            first_name: draft.secondGuardian.firstName.trim(),
            last_name: draft.secondGuardian.lastName.trim(),
            email: draft.secondGuardian.email.trim().toLowerCase(),
            phone: draft.secondGuardian.phone.trim(),
          },
        }
      : {}),
    children: draft.children.map((child) => {
      const guest = child.school === OTHER_SCHOOL;
      return {
        first_name: child.firstName.trim(),
        last_name: child.lastName.trim(),
        birth_date: child.birthDate,
        home_school_id: guest ? 0 : Number(child.school),
        guest_school_name: guest ? child.guestSchoolName.trim() : "",
        ...healthPayload(childHealthValues(child, offer.form)),
        bookings: quoteBookings(child, offer),
        custom_data: childAnswersToSend(draft, child, offer.form),
        ...(child.wishProof && !guest ? { wish_proof: child.wishProof } : {}),
        ...(offer.allocationMode === "criteria"
          ? {
              criteria: criteriaToSend(
                criterionQuestions(offer, "child"),
                child.criteria,
              ),
            }
          : {}),
      };
    }),
    emergency_contacts: draft.contacts
      .filter((contact) => contact.emergency)
      .map(contactPayload),
    pickup_persons: draft.contacts
      .filter((contact) => contact.pickup)
      .map(contactPayload),
    legal_acceptances: Object.fromEntries(
      (offer.form.legalBlocks ?? [])
        .filter((b) => b.enabled)
        .map((b) => [b.key, draft.legalAcceptances?.[b.key] === true]),
    ),
    privacy_consent: draft.privacyConsent,
    health_consent: hasHealthInfo(draft, offer.form) && draft.healthConsent,
    binding_confirmed: offer.binding && draft.bindingConfirmed,
    custody_confirmed: draft.custodyConfirmed,
    declaration_name: draft.declarationName.trim(),
    benefits_declared: draft.benefitsDeclared,
    benefits_proof_tokens: draft.benefitsDeclared
      ? draft.benefitsProofs.map((proof) => proof.token)
      : [],
    captcha_token: captchaToken,
    form_version: offer.form.version,
    // The version of the texts the form showed, as the offer named them.
    consent_version: offer.consentTexts.version,
    // The retention its privacy text named; the server refuses another one.
    retention_days: offer.consentTexts.retentionDays,
    custom_data: familyAnswersToSend(draft, offer.form),
    ...(offer.allocationMode === "criteria"
      ? {
          criteria: criteriaToSend(
            criterionQuestions(offer, "family"),
            draft.criteria,
          ),
        }
      : {}),
    // The account of a signed-in parent travels as its proof; anyone else
    // answered the question at the end of the form (#6673).
    parent_account_requested:
      !draft.parentAccountProof && draft.parentAccountRequested === true,
    ...(draft.parentAccountProof
      ? { parent_account_proof: draft.parentAccountProof }
      : {}),
  };
}
