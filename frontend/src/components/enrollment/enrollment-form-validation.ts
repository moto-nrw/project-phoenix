// The parent enrollment form's submit check. Pure: it reads the typed values
// and the per-child derivation and returns every problem of one attempt, so
// a missing core field and a missing care day are marked together.

import { isValidContactEmail, isValidContactPhone } from "~/lib/contact-format";
import {
  careOfferingDayConflicts,
  conflictOfferingList,
  type CareOfferingDayConflict,
} from "~/lib/care-offering-day-conflicts";
import type { MaterializedCareSelection } from "~/lib/care-offering-selection";
import {
  isFieldVisible,
  type ConditionContext,
} from "~/lib/enrollment-field-visibility";
import type {
  PublicFormSchema,
  PublicLegalBlock,
} from "~/lib/enrollment-form-schema-api";
import type {
  CareOfferingSelectionMode,
  PublicCareOffering,
} from "~/lib/enrollment-submission-api";
import { selectionGroupLabel } from "~/lib/enrollment-translations";
import type { SignatureValue } from "~/components/ui/signature-pad";
import { fileValueMissing } from "./enrollment-file-upload";
import type {
  ChildDerivation,
  ChildDraft,
} from "./enrollment-child-derivation";
import type { GuardianDraft } from "./enrollment-form-config";
import {
  contactListValueMissing,
  phoneListValueMissing,
} from "./enrollment-form-contact-inputs";
import {
  CONSENT_FIELD_PREFIX,
  formFieldName,
} from "./enrollment-form-field-names";
import {
  asScheduleObject,
  asStringMap,
  asWeekdayBooleanObject,
  asWeekdayMultiModeObject,
  DEPARTURE_COMPANION_KEY,
  departureMultiModeOptionsForField,
  WEEKDAYS,
  type EnrollmentFormTranslator,
  type FormField,
  type TranslationFn,
} from "./enrollment-form-shared";

export interface EnrollmentFormValues {
  guardian: GuardianDraft;
  additionalGuardians: readonly GuardianDraft[];
  customData: Record<string, unknown>;
  /** Show-if context of the request-level fields. */
  guardianContext: ConditionContext;
  children: readonly ChildDraft[];
  /** One entry per child, in the same order. */
  derivations: readonly ChildDerivation[];
  legalConsents: Record<string, boolean>;
  signature: SignatureValue | null;
}

export interface EnrollmentFormRules {
  schema: PublicFormSchema | null;
  /** The whole catalog as displayed; empty when the phase has none. */
  offerings: readonly PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  collectGradeLevel: boolean;
  schoolClassRequired: boolean;
  legalBlocks: readonly PublicLegalBlock[];
  signatureRequired: boolean;
  /** Reduced Halbjahreswechsel flow (#2251): only offerings are checked. */
  restrictToOfferings: boolean;
}

export interface EnrollmentFormProblems {
  /** Keyed by input name (formFieldName). */
  fieldErrors: Record<string, string>;
  /** Child index → the offering selection breaks the phase or a group rule. */
  childOfferingErrors: Record<number, boolean>;
  /** `${childIndex}_${offeringId}` → a parent_choice offering lacks a day. */
  offeringDayErrors: Record<string, boolean>;
  banner: string;
}

type CareSelectionRule = "at_least_one" | "exactly_one";

/** Whether a child breaks the phase-wide offering selection rule. */
export function violatesCareSelectionRule(
  child: ChildDraft,
  derivation: ChildDerivation,
  rule: CareSelectionRule,
): boolean {
  if (child.locked || !derivation.selectionModeApplies) return false;
  const count = derivation.choosableSelectedCount;
  return rule === "at_least_one" ? count === 0 : count !== 1;
}

/**
 * The children to mark after the server rejected the selection rule. The
 * response names no child, so the form applies its own rule.
 */
export function childrenViolatingCareSelectionRule(
  rule: CareSelectionRule,
  children: readonly ChildDraft[],
  derivations: readonly ChildDerivation[],
): Record<number, boolean> {
  const marks: Record<number, boolean> = {};
  children.forEach((child, i) => {
    const derivation = derivations[i];
    if (derivation && violatesCareSelectionRule(child, derivation, rule)) {
      marks[i] = true;
    }
  });
  return marks;
}

/**
 * Whether a visible departure field allows the accompanied mode on a care
 * day; only then is the "mit wem" note (#1694) asked for and sent. A schema
 * may carry several departure fields (only keys are unique), so every one
 * counts. The backend's companionNoteAllowed applies the same rule.
 */
export function departureAllowsCompanion(
  fields: readonly FormField[],
  answers: Record<string, unknown>,
  careDays: readonly string[],
  isVisible: (field: FormField) => boolean,
): boolean {
  return fields.some((field) => {
    if (
      field.target !== "student.allowed_departure_modes" ||
      !field.applies_to_child ||
      !isVisible(field)
    ) {
      return false;
    }
    const modes = asWeekdayMultiModeObject(
      answers[field.key],
      careDays,
      departureMultiModeOptionsForField(field),
    );
    return careDays.some((day) => (modes[day] ?? []).includes("accompanied"));
  });
}

export function companionNote(child: ChildDraft): string | undefined {
  return (child.custom[DEPARTURE_COMPANION_KEY] as string | undefined)?.trim();
}

export function validateEnrollmentForm(
  values: EnrollmentFormValues,
  rules: EnrollmentFormRules,
  tr: EnrollmentFormTranslator,
  locale: string,
): EnrollmentFormProblems | null {
  const fieldErrors = rules.restrictToOfferings
    ? {}
    : dataFieldErrors(values, rules, tr);
  if (rules.signatureRequired && values.signature === null) {
    fieldErrors[formFieldName.signature] = tr("signature.required");
  }
  const offering = offeringProblems(values, rules, tr, locale);
  const fieldErrorKeys = Object.keys(fieldErrors);
  const dayErrorCount = Object.keys(offering.dayErrors).length;
  const hasOfferingIssue =
    offering.missingCare.length > 0 ||
    offering.exactlyOneCare.length > 0 ||
    offering.groupRule.length > 0 ||
    dayErrorCount > 0;
  if (fieldErrorKeys.length === 0 && !hasOfferingIssue) return null;

  const invalidCare = [
    ...offering.missingCare,
    ...offering.exactlyOneCare,
    ...offering.groupRule,
  ];
  // One field: its own message. Only consents, only one kind of offering
  // problem: that summary. Anything else: the generic one.
  let banner = tr("fixMarked");
  if (fieldErrorKeys.length === 1 && !hasOfferingIssue) {
    banner = Object.values(fieldErrors)[0] ?? banner;
  } else if (
    !hasOfferingIssue &&
    fieldErrorKeys.every((key) => key.startsWith(CONSENT_FIELD_PREFIX))
  ) {
    banner = tr("consentSummary");
  } else if (fieldErrorKeys.length === 0) {
    if (offering.missingCare.length > 0) {
      banner = tr("errors.careMissing");
    } else if (offering.exactlyOneCare.length > 0) {
      banner = tr("errors.careExactlyOne");
    } else if (dayErrorCount > 0 && offering.firstDayError) {
      banner = offering.firstDayError;
    } else if (offering.groupRule.length > 0 && offering.firstGroupRule) {
      banner = offering.firstGroupRule;
    }
  }
  return {
    fieldErrors,
    childOfferingErrors: Object.fromEntries(invalidCare.map((i) => [i, true])),
    offeringDayErrors: offering.dayErrors,
    banner,
  };
}

function dataFieldErrors(
  values: EnrollmentFormValues,
  rules: EnrollmentFormRules,
  tr: EnrollmentFormTranslator,
): Record<string, string> {
  const errors: Record<string, string> = {};
  const fields = rules.schema?.fields ?? [];
  const { guardian } = values;
  if (!guardian.first_name.trim()) {
    errors[formFieldName.guardian("first_name")] = tr("errors.firstName");
  }
  if (!guardian.last_name.trim()) {
    errors[formFieldName.guardian("last_name")] = tr("errors.lastName");
  }
  const email = guardian.email.trim();
  if (!email) {
    errors[formFieldName.guardian("email")] = tr("errors.email");
  } else if (!isValidContactEmail(email)) {
    errors[formFieldName.guardian("email")] = tr("errors.validEmail");
  }
  // An optional phone still has to be valid: the backend enforces the same
  // rule, and a typo would make the request impossible to approve.
  const phone = guardian.phone.trim();
  if (rules.schema?.core_requirements?.guardian_phone === true && !phone) {
    errors[formFieldName.guardian("phone")] = tr("errors.phone");
  } else if (phone && !isValidContactPhone(phone)) {
    errors[formFieldName.guardian("phone")] = tr("errors.validPhone");
  }
  // A co-guardian card with any value needs both names; an empty card is
  // dropped at submit.
  values.additionalGuardians.forEach((g, i) => {
    const first = g.first_name.trim();
    const last = g.last_name.trim();
    const gEmail = g.email.trim();
    const gPhone = g.phone.trim();
    if (!first && !last && !gEmail && !gPhone) return;
    if (!first) {
      errors[formFieldName.additionalGuardian(i, "first_name")] =
        tr("errors.firstName");
    }
    if (!last) {
      errors[formFieldName.additionalGuardian(i, "last_name")] =
        tr("errors.lastName");
    }
    if (gEmail && !isValidContactEmail(gEmail)) {
      errors[formFieldName.additionalGuardian(i, "email")] =
        tr("errors.validEmail");
    }
    if (gPhone && !isValidContactPhone(gPhone)) {
      errors[formFieldName.additionalGuardian(i, "phone")] =
        tr("errors.validPhone");
    }
  });
  // A field hidden by its show-if condition never blocks the submit.
  for (const field of fields) {
    if (field.applies_to_child || !requiresAnswer(field)) continue;
    if (!isFieldVisible(field, values.guardianContext)) continue;
    if (customValueMissing(field, values.customData[field.key])) {
      errors[formFieldName.custom(field.key)] = requiredMessageForField(
        field,
        tr,
      );
    }
  }
  values.children.forEach((child, i) => {
    const derivation = values.derivations[i];
    if (child.locked || !derivation) return;
    Object.assign(errors, childFieldErrors(child, i, derivation, rules, tr));
  });
  for (const block of rules.legalBlocks) {
    if (block.required && !values.legalConsents[block.key]) {
      errors[formFieldName.consent(block.key)] = tr("errors.consentRequired");
    }
  }
  return errors;
}

function childFieldErrors(
  child: ChildDraft,
  i: number,
  derivation: ChildDerivation,
  rules: EnrollmentFormRules,
  tr: EnrollmentFormTranslator,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (!child.first_name.trim()) {
    errors[formFieldName.child(i, "first_name")] = tr("errors.firstName");
  }
  if (!child.last_name.trim()) {
    errors[formFieldName.child(i, "last_name")] = tr("errors.lastName");
  }
  if (!child.date_of_birth) {
    errors[formFieldName.child(i, "date_of_birth")] = tr("errors.dob");
  }
  if (rules.collectGradeLevel && !child.target_grade_level) {
    errors[formFieldName.child(i, "target_grade_level")] = tr("errors.grade");
  }
  // A grade the phase offers no class for stays "Klasse offen", even when
  // the phase requires classes; the backend gate is the same (#1833).
  if (
    rules.schoolClassRequired &&
    derivation.collectsSchoolClass &&
    !child.target_school_class.trim()
  ) {
    errors[formFieldName.child(i, "target_school_class")] =
      tr("errors.schoolClass");
  }
  const fields = rules.schema?.fields ?? [];
  const visible = (field: FormField) =>
    isFieldVisible(field, derivation.conditionContext);
  for (const field of fields) {
    if (!field.applies_to_child || !requiresAnswer(field)) continue;
    if (!visible(field)) continue;
    if (
      customValueMissing(
        field,
        child.custom[field.key],
        derivation.careDays,
        derivation.careDaysConstrained,
      )
    ) {
      errors[formFieldName.childCustom(i, field.key)] = requiredMessageForField(
        field,
        tr,
        derivation.careDaysConstrained,
      );
    }
  }
  if (
    departureAllowsCompanion(
      fields,
      child.custom,
      derivation.careDays,
      visible,
    ) &&
    !companionNote(child)
  ) {
    errors[formFieldName.childCompanionNote(i)] = tr(
      "structured.departureCompanionRequired",
    );
  }
  return errors;
}

function offeringProblems(
  values: EnrollmentFormValues,
  rules: EnrollmentFormRules,
  tr: EnrollmentFormTranslator,
  locale: string,
) {
  const missingCare: number[] = [];
  const exactlyOneCare: number[] = [];
  const groupRule: number[] = [];
  const dayErrors: Record<string, boolean> = {};
  let firstDayError: string | null = null;
  let firstGroupRule: string | null = null;
  values.children.forEach((child, i) => {
    const derivation = values.derivations[i];
    if (child.locked || !derivation) return;
    const mode = rules.careOfferingSelectionMode;
    if (
      mode !== "optional" &&
      violatesCareSelectionRule(child, derivation, mode)
    ) {
      (mode === "at_least_one" ? missingCare : exactlyOneCare).push(i);
    }
    for (const id of derivation.care.offeringIds) {
      const offering = derivation.offerings.find((o) => o.id === id);
      if (offering?.days_of_week_mode !== "parent_choice") continue;
      const picked = derivation.care.offeringDays[id];
      if (!picked || picked.size === 0) {
        dayErrors[`${i}_${id}`] = true;
        firstDayError ??= tr("errors.dayMissing", {
          child: i + 1,
          offering: offering.name,
        });
      }
    }
    const message = offeringGroupRuleError(
      derivation.care,
      derivation.offerings,
      tr,
      locale,
    );
    if (message) {
      groupRule.push(i);
      firstGroupRule ??= `${tr("structured.child")} ${i + 1}: ${message}`;
    }
  });
  return {
    missingCare,
    exactlyOneCare,
    groupRule,
    dayErrors,
    firstDayError,
    firstGroupRule,
  };
}

// Validates one child's selection against every selection_group rule; the
// backend re-checks it in validateOfferingGroupRules.
function offeringGroupRuleError(
  care: Pick<MaterializedCareSelection, "offeringIds" | "offeringDays">,
  offerings: readonly PublicCareOffering[],
  tr: EnrollmentFormTranslator,
  locale: string,
): string | null {
  const [dayConflict] = careOfferingDayConflicts(
    offerings,
    care.offeringIds,
    care.offeringDays,
  );
  if (dayConflict) {
    return dayConflictMessage(dayConflict, offerings, tr, locale);
  }
  const ruleByGroup = new Map<string, string>();
  for (const o of offerings) {
    const group = o.selection_group?.trim() ?? "";
    const rule = o.selection_rule ?? "optional";
    if (group === "" || rule === "optional") continue;
    ruleByGroup.set(group, rule);
  }
  for (const [groupKey, rule] of ruleByGroup) {
    const count = offerings.filter(
      (o) =>
        (o.selection_group?.trim() ?? "") === groupKey &&
        care.offeringIds.has(o.id),
    ).length;
    const group = selectionGroupLabel(offerings, groupKey, locale);
    if (rule === "exactly_one" && count !== 1) {
      return tr("errors.groupExactlyOne", { group });
    }
    if (rule === "at_least_one" && count < 1) {
      return tr("errors.groupAtLeastOne", { group });
    }
    if (rule === "at_most_one" && count > 1) {
      return tr("errors.groupAtMostOne", { group });
    }
  }
  return null;
}

// Names the day and the offerings of one conflict, so the family sees what
// to change without the form changing it for them (#4514).
export function dayConflictMessage(
  conflict: CareOfferingDayConflict,
  offerings: readonly PublicCareOffering[],
  tr: EnrollmentFormTranslator,
  locale: string,
): string {
  const weekdays = asStringMap(tr.raw("weekdays"));
  const names = conflict.offeringIds.map(
    (id) => offerings.find((o) => o.id === id)?.name ?? id,
  );
  return tr("errors.groupDayConflict", {
    day: weekdays[conflict.day] ?? conflict.day,
    offerings: conflictOfferingList(names, locale),
  });
}

/**
 * Pickup fields to mark after the server rejected a pickup time outside a
 * fixed list; the response names no field. Times the loaded list already
 * rejects come first. When there are none, the list changed after loading,
 * so every answered fixed-list field is marked.
 */
export function pickupTimeMarks(
  schema: PublicFormSchema | null,
  children: readonly ChildDraft[],
  customData: Record<string, unknown>,
): string[] {
  const offList: string[] = [];
  const answered: string[] = [];
  for (const f of schema?.fields ?? []) {
    const allowed = f.allowed_times;
    if (
      f.target !== "schedule.pickup" ||
      !allowed?.length ||
      f.type !== "weekday_schedule"
    ) {
      continue;
    }
    const mark = (answer: unknown, name: string): void => {
      const times = Object.values(asScheduleObject(answer)).filter(Boolean);
      if (times.length === 0) return;
      answered.push(name);
      if (times.some((time) => !allowed.includes(time))) offList.push(name);
    };
    if (f.applies_to_child) {
      children.forEach((c, i) =>
        mark(c.custom[f.key], formFieldName.childCustom(i, f.key)),
      );
    } else {
      mark(customData[f.key], formFieldName.custom(f.key));
    }
  }
  return offList.length > 0 ? offList : answered;
}

function requiresAnswer(field: FormField): boolean {
  return field.type !== "information" && field.required === true;
}

function customValueMissing(
  field: FormField,
  value: unknown,
  relevantDays?: readonly string[],
  careConstrained = false,
): boolean {
  if (field.type === "boolean") {
    return typeof value !== "boolean";
  }
  if (field.type === "phone_list") {
    return phoneListValueMissing(value);
  }
  if (field.type === "contact_list") {
    return contactListValueMissing(value);
  }
  if (field.type === "file") {
    return fileValueMissing(value);
  }
  if (field.type === "weekday_schedule") {
    const days = relevantDays ?? WEEKDAYS;
    if (days.length === 0) return false;
    const schedule = asScheduleObject(value);
    // Shown days that are the child's care days each need a time. With all
    // weekdays shown, one time is enough: a child may attend some days only.
    if (careConstrained) {
      return days.some((day) => (schedule[day] ?? "").trim() === "");
    }
    return !days.some((day) => (schedule[day] ?? "").trim() !== "");
  }
  if (field.type === "weekday_boolean") {
    // An empty pickup selection is the valid "Geht alleine nach Hause"
    // answer; only an untouched field is missing. Other weekday_boolean
    // fields (Buskind) need a day.
    if (field.target === "student.pickup_status") {
      return value === undefined || value === null || typeof value !== "object";
    }
    const days = asWeekdayBooleanObject(value);
    return !Object.values(days).some(Boolean);
  }
  if (field.type === "weekday_mode") {
    // Like pickup: an empty plan is an answer, an untouched field is not.
    return value === undefined || value === null || typeof value !== "object";
  }
  if (field.type === "weekday_multi_mode") {
    const days = relevantDays ?? WEEKDAYS;
    if (days.length === 0) return false;
    const modes = asWeekdayMultiModeObject(
      value,
      days,
      departureMultiModeOptionsForField(field),
    );
    return days.some((day) => (modes[day]?.length ?? 0) === 0);
  }
  return typeof value !== "string" || value.trim() === "";
}

function requiredMessageForField(
  field: FormField,
  tr: TranslationFn,
  careConstrained = false,
): string {
  switch (field.type) {
    case "boolean":
      return tr("errors.yesNo");
    case "phone_list":
      return tr("errors.phoneList");
    case "contact_list":
      return tr("errors.contactList");
    case "weekday_schedule":
      return tr(
        careConstrained ? "errors.scheduleEveryDay" : "errors.schedule",
      );
    case "weekday_boolean":
      return tr(
        field.target === "student.pickup_status"
          ? "errors.pickupConfirm"
          : "errors.weekdayRequired",
      );
    case "weekday_mode":
      return tr("errors.weekdayModeConfirm");
    case "weekday_multi_mode":
      return tr("errors.weekdayMultiModeRequired");
    case "select":
      return tr("errors.select");
    case "file":
      return tr("errors.file");
    default:
      return tr("errors.required");
  }
}
