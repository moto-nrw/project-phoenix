// Entwurf, Prüfung und Versand der öffentlichen Ferienanmeldung (#4661).
// Die Feldnamen folgen dem Backend ("children.0.first_name"), damit Fehler
// vom Server am selben Feld landen wie die eigene Prüfung.

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
import type {
  HolidayContactPayload,
  HolidayKnownChild,
  HolidayKnownFamily,
  HolidayQuoteChild,
  HolidayRegistrationOffer,
  HolidayRegistrationPayload,
} from "~/lib/holiday-registration-api";

export const MAX_CHILDREN = 3;
export const MAX_EMERGENCY_CONTACTS = 3;
export const MAX_PICKUP_PERSONS = 5;

/** The school choice of a child who attends no school of the Träger. */
export const OTHER_SCHOOL = "other";

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
  /** Week ID → time model ID; a missing week is not booked. */
  readonly bookings: Readonly<Record<string, string>>;
  /** The child of the parent's account this draft started from (#4662). */
  readonly knownStudentId?: string;
  /** Answers to the Träger's questions per child, by key (#5198). */
  readonly answers: Readonly<Record<string, unknown>>;
}

export interface ContactDraft {
  readonly key: string;
  readonly name: string;
  readonly phone: string;
  readonly relationship: string;
}

export interface RegistrationDraft {
  readonly guardian: {
    readonly firstName: string;
    readonly lastName: string;
    readonly email: string;
    readonly phone: string;
  };
  readonly children: readonly ChildDraft[];
  readonly emergencyContacts: readonly ContactDraft[];
  readonly pickupPersons: readonly ContactDraft[];
  readonly privacyConsent: boolean;
  readonly healthConsent: boolean;
  readonly bindingConfirmed: boolean;
  /** The family receives benefits; the Träger checks it (#4664). */
  readonly benefitsDeclared: boolean;
  /** Answers to the Träger's questions per family, by key (#5198). */
  readonly answers: Readonly<Record<string, unknown>>;
}

/** The reason a field is refused, as a key of `holidayRegistration.errors`. */
export type FieldErrorKey =
  | "required"
  | "email"
  | "school"
  | "weeks"
  | "privacy"
  | "health"
  | "binding"
  | "yesNo"
  | "choose";

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
  return { key: nextKey("contact"), name: "", phone: "", relationship: "" };
}

export function emptyRegistration(): RegistrationDraft {
  return {
    guardian: { firstName: "", lastName: "", email: "", phone: "" },
    children: [emptyChild()],
    emergencyContacts: [emptyContact()],
    pickupPersons: [],
    privacyConsent: false,
    healthConsent: false,
    bindingConfirmed: false,
    benefitsDeclared: false,
    answers: {},
  };
}

/**
 * A registration of a parent with an account (#4662): the parent fields come
 * from what the school knows. Without a family the registration starts empty.
 */
export function registrationFromFamily(
  family: HolidayKnownFamily | null,
): RegistrationDraft {
  const draft = emptyRegistration();
  return family ? { ...draft, guardian: { ...family.guardian } } : draft;
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
  };
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
  if (!hasRoomForKnownChild(draft)) return draft;
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
    [child.allergies, child.medication, child.healthNotes].some(
      (value) => value.trim() !== "",
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
export function familyAnswersToSend(
  draft: RegistrationDraft,
  form: HolidayFormVersion,
): Record<string, unknown> {
  return sentAnswers(form, false, draft.answers, familyContext(draft, form));
}

/** The answers of one child that are sent. */
export function childAnswersToSend(
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

/** The bookings of a child in the order of the period's weeks. */
export function bookedWeeks(
  child: ChildDraft,
  offer: HolidayRegistrationOffer,
): { weekId: string; timeModelId: string }[] {
  return offer.weeks.flatMap((week) => {
    const timeModelId = child.bookings[week.id];
    return timeModelId ? [{ weekId: week.id, timeModelId }] : [];
  });
}

const EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function requireText(
  errors: Record<string, FieldErrorKey>,
  field: string,
  value: string,
) {
  if (value.trim() === "") errors[field] = "required";
}

function validateContacts(
  errors: Record<string, FieldErrorKey>,
  prefix: string,
  contacts: readonly ContactDraft[],
  phoneRequired: boolean,
) {
  contacts.forEach((contact, index) => {
    requireText(errors, `${prefix}.${index}.name`, contact.name);
    if (phoneRequired) {
      requireText(errors, `${prefix}.${index}.phone`, contact.phone);
    }
  });
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
    if (bookedWeeks(child, offer).length === 0) {
      errors[`${prefix}.bookings`] = "weeks";
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
  if (!EMAIL.test(draft.guardian.email.trim())) {
    errors["guardian.email"] =
      draft.guardian.email.trim() === "" ? "required" : "email";
  }
  requireText(errors, "guardian.phone", draft.guardian.phone);
  // The questions per family follow the parent in the form.
  requireAnswers(
    errors,
    "",
    visibleQuestions(offer.form, false, familyContext(draft, offer.form)),
    draft.answers,
  );
  validateContacts(errors, "emergency_contacts", draft.emergencyContacts, true);
  validateContacts(errors, "pickup_persons", draft.pickupPersons, false);
  if (!draft.privacyConsent) errors.privacy_consent = "privacy";
  if (hasHealthInfo(draft, offer.form) && !draft.healthConsent) {
    errors.health_consent = "health";
  }
  if (offer.binding && !draft.bindingConfirmed) {
    errors.binding_confirmed = "binding";
  }
  return errors;
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
  }));
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
    children: draft.children.map((child) => {
      const guest = child.school === OTHER_SCHOOL;
      return {
        first_name: child.firstName.trim(),
        last_name: child.lastName.trim(),
        birth_date: child.birthDate,
        home_school_id: guest ? 0 : Number(child.school),
        guest_school_name: guest ? child.guestSchoolName.trim() : "",
        allergies: child.allergies.trim(),
        medication: child.medication.trim(),
        health_notes: child.healthNotes.trim(),
        bookings: quoteBookings(child, offer),
        custom_data: childAnswersToSend(draft, child, offer.form),
      };
    }),
    emergency_contacts: draft.emergencyContacts.map(contactPayload),
    pickup_persons: draft.pickupPersons.map(contactPayload),
    privacy_consent: draft.privacyConsent,
    health_consent: hasHealthInfo(draft, offer.form) && draft.healthConsent,
    binding_confirmed: offer.binding && draft.bindingConfirmed,
    benefits_declared: draft.benefitsDeclared,
    captcha_token: captchaToken,
    form_version: offer.form.version,
    custom_data: familyAnswersToSend(draft, offer.form),
  };
}
