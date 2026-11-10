// Builds the submit payload of the parent enrollment form from values that
// passed validateEnrollmentForm. Pure: no checks, only mapping.

import { visibleAnswerData } from "~/lib/enrollment-field-visibility";
import {
  holidayCareWishAsked,
  type PublicFormSchema,
  type PublicLegalBlock,
} from "~/lib/enrollment-form-schema-api";
import type {
  SubmitChildPayload,
  SubmitEnrollmentPayload,
  SubmitGuardianPayload,
} from "~/lib/enrollment-submission-api";
import { signatureRequestBody } from "~/lib/signature";
import type {
  ChildDerivation,
  ChildDraft,
} from "./enrollment-child-derivation";
import {
  asScheduleObject,
  asWeekdayMultiModeObject,
  DEPARTURE_COMPANION_KEY,
  departureMultiModeOptionsForField,
  WEEKDAYS,
  type FormField,
} from "./enrollment-form-shared";
import {
  companionNote,
  departureAllowsCompanion,
  type EnrollmentFormValues,
} from "./enrollment-form-validation";

export interface EnrollmentPayloadRules {
  phaseID: string;
  schema: PublicFormSchema | null;
  collectGradeLevel: boolean;
  careOfferingsEnabled: boolean;
  legalBlocks: readonly PublicLegalBlock[];
  signatureRequired: boolean;
  /** Hidden sections travel unchanged from the draft (#2251). */
  restrictToOfferings: boolean;
  captchaToken: string | undefined;
  lateInviteToken: string | undefined;
}

export function buildEnrollmentPayload(
  values: EnrollmentFormValues,
  rules: EnrollmentPayloadRules,
): SubmitEnrollmentPayload {
  const fields = rules.schema?.fields ?? [];
  const consentFlags: Record<string, unknown> = {};
  for (const block of rules.legalBlocks) {
    consentFlags[block.key] =
      block.kind === "notice" ? true : values.legalConsents[block.key] === true;
  }
  // Validation guarantees both names on every co-guardian card that is not
  // empty; empty cards are dropped.
  const additionalGuardians: SubmitGuardianPayload[] =
    values.additionalGuardians
      .map((g) => ({
        first_name: g.first_name.trim(),
        last_name: g.last_name.trim(),
        email: g.email.trim().toLowerCase() || undefined,
        phone: g.phone.trim() || undefined,
      }))
      .filter(
        (g) => g.first_name !== "" || g.last_name !== "" || g.email || g.phone,
      );
  const lateInviteToken = rules.lateInviteToken?.trim();
  return {
    phase_id: Number(rules.phaseID),
    guardian_first_name: values.guardian.first_name.trim(),
    guardian_last_name: values.guardian.last_name.trim(),
    guardian_email: values.guardian.email.trim().toLowerCase(),
    guardian_phone: values.guardian.phone.trim() || undefined,
    additional_guardians:
      additionalGuardians.length > 0 ? additionalGuardians : undefined,
    consent_flags: consentFlags,
    form_schema_id: rules.schema?.id ? Number(rules.schema.id) : undefined,
    custom_data: rules.restrictToOfferings
      ? { ...values.customData }
      : visibleAnswerData(
          fields,
          false,
          values.customData,
          values.guardianContext,
        ),
    children: values.children.map((child, i) =>
      childPayload(child, values.derivations[i]!, fields, rules),
    ),
    captcha_token: rules.captchaToken,
    late_invite_token: lateInviteToken || undefined,
    signature:
      rules.signatureRequired && values.signature
        ? signatureRequestBody(values.signature)
        : undefined,
  };
}

function childPayload(
  child: ChildDraft,
  derivation: ChildDerivation,
  fields: readonly FormField[],
  rules: EnrollmentPayloadRules,
): SubmitChildPayload {
  if (child.lockedPayload) return child.lockedPayload;
  const bookable = new Set(derivation.offerings.map((o) => o.id));
  const offeringDays: NonNullable<SubmitChildPayload["offering_days"]> = [];
  for (const id of child.offering_ids) {
    const offering = derivation.offerings.find((o) => o.id === id);
    if (offering?.days_of_week_mode !== "parent_choice") continue;
    const picked = child.offering_days[id];
    if (!picked || picked.size === 0) continue;
    offeringDays.push({
      offering_id: Number(id),
      // The catalog's day order, so the payload does not follow click order.
      selected_days: offering.available_days.filter((d) => picked.has(d)),
    });
  }
  // Answers to fields the parent could not see never reach the backend.
  const customData = rules.restrictToOfferings
    ? { ...child.custom }
    : pruneWeekdayAnswers(
        visibleAnswerData(
          fields,
          true,
          child.custom,
          derivation.conditionContext,
        ),
        fields,
        derivation.careDays,
      );
  // The "mit wem" note rides on a reserved key, which visibleAnswerData
  // drops; it goes back in only while a sent departure plan allows it.
  const note = companionNote(child);
  if (
    note &&
    departureAllowsCompanion(
      fields,
      customData,
      rules.restrictToOfferings ? WEEKDAYS : derivation.careDays,
      () => true,
    )
  ) {
    customData[DEPARTURE_COMPANION_KEY] = note;
  }
  const schoolClass = child.target_school_class.trim();
  return {
    ...(child.id ? { id: child.id } : {}),
    first_name: child.first_name.trim(),
    last_name: child.last_name.trim(),
    date_of_birth: child.date_of_birth,
    target_grade_level: rules.collectGradeLevel
      ? Number(child.target_grade_level)
      : undefined,
    // The backend re-validates and normalises the class (#1833/#1663).
    target_school_class:
      derivation.collectsSchoolClass && schoolClass ? schoolClass : undefined,
    holiday_care_wish: holidayCareWishAsked(rules.schema)
      ? child.holiday_care_wish
      : undefined,
    custom_data: customData,
    offering_ids: rules.careOfferingsEnabled
      ? Array.from(child.offering_ids)
          .filter((id) => bookable.has(id))
          .map(Number)
      : undefined,
    offering_days:
      rules.careOfferingsEnabled && offeringDays.length > 0
        ? offeringDays
        : undefined,
  };
}

function pruneWeekdayAnswers(
  data: Record<string, unknown>,
  fields: readonly FormField[],
  careDays: readonly string[],
): Record<string, unknown> {
  const next = { ...data };
  for (const field of fields) {
    if (!(field.key in next)) continue;
    if (field.type === "weekday_multi_mode") {
      next[field.key] = asWeekdayMultiModeObject(
        next[field.key],
        careDays,
        departureMultiModeOptionsForField(field),
      );
    } else if (field.type === "weekday_schedule") {
      // Times for days the child is no longer in care stay out.
      const schedule = asScheduleObject(next[field.key]);
      const pruned: Record<string, string> = {};
      for (const day of careDays) {
        const time = schedule[day];
        if (time !== undefined) pruned[day] = time;
      }
      next[field.key] = pruned;
    }
  }
  return next;
}
