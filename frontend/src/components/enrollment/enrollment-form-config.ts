// What a phase configures for the parent enrollment form, and the starting
// values the form shows. Every caller maps its bootstrap through
// prefetchedDataFromBootstrap; the form turns that into one config and one
// initial draft, whether it mounts, receives new data or loads a preview.

import type { PublicCaptchaConfig } from "~/lib/enrollment-form-schema-api";
import type {
  PublicFormSchema,
  PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import {
  EMPTY_SCHOOL_CLASS_CONFIG,
  type EnrollmentEditDraft,
  type MeProfileResponse,
  type PublicCareOffering,
  type PublicLateInvitePrefill,
  type PublicPhase,
  type PublicSchoolClassConfig,
} from "~/lib/enrollment-submission-api";
import type { CareOfferingSelectionMode } from "~/lib/enrollment-phase-api";
import {
  classMatchesGrade,
  withRequiredOfferings,
  type ChildDraft,
} from "./enrollment-child-derivation";

export interface EnrollmentFormPrefetchedData {
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  captchaConfig: PublicCaptchaConfig | null;
  legalTexts: PublicLegalTexts;
  profile?: MeProfileResponse | null;
  lateInvite?: PublicLateInvitePrefill;
  /** Concrete-class config (#1833); absent means disabled. */
  schoolClass?: PublicSchoolClassConfig;
  collectGradeLevel?: boolean;
  careOfferingsEnabled?: boolean;
  /**
   * Phase applicant restriction (#1663). A "new_students" phase rejects any
   * already-enrolled child, so the form hides the reuse panel for the
   * prefilled linked children and explains why.
   */
  audience?: PublicPhase["audience"];
  /** Phase grade restriction (#1663): the grade select offers only these. */
  eligibleGradeLevels?: number[];
}

/** The fields every enrollment bootstrap response shares. */
interface EnrollmentFormBootstrap {
  phase: Pick<PublicPhase, "audience" | "eligible_grade_levels">;
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  care_offering_selection_mode: CareOfferingSelectionMode;
  collect_grade_level: boolean;
  care_offerings_enabled: boolean;
  legal_texts: PublicLegalTexts;
  school_class?: PublicSchoolClassConfig;
  late_invite?: PublicLateInvitePrefill;
}

interface BootstrapContext {
  /** Autofill for the guardian fields; each caller decides its source. */
  profile?: MeProfileResponse | null;
  /** Only the anonymous public path shows the captcha. */
  captchaConfig?: PublicCaptchaConfig | null;
  /**
   * False for staff entry: trusted paths bypass the phase's audience and
   * grade gates on the server, so the form must not narrow them either.
   */
  phaseRestrictions?: boolean;
}

export function prefetchedDataFromBootstrap(
  bootstrap: EnrollmentFormBootstrap,
  {
    profile = null,
    captchaConfig = null,
    phaseRestrictions = true,
  }: BootstrapContext = {},
): EnrollmentFormPrefetchedData {
  return {
    schema: bootstrap.schema,
    offerings: bootstrap.offerings,
    careOfferingSelectionMode: bootstrap.care_offering_selection_mode,
    collectGradeLevel: bootstrap.collect_grade_level,
    careOfferingsEnabled: bootstrap.care_offerings_enabled,
    captchaConfig,
    legalTexts: bootstrap.legal_texts,
    profile,
    lateInvite: bootstrap.late_invite,
    schoolClass: bootstrap.school_class,
    ...(phaseRestrictions
      ? {
          audience: bootstrap.phase.audience,
          eligibleGradeLevels: bootstrap.phase.eligible_grade_levels ?? [],
        }
      : {}),
  };
}

export interface EnrollmentFormConfig {
  /** German texts as loaded; the form localizes them per render. */
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  collectGradeLevel: boolean;
  careOfferingsEnabled: boolean;
  schoolClass: PublicSchoolClassConfig;
  /** Empty = every grade up to the tenant cap. */
  eligibleGradeLevels: number[];
  captchaConfig: PublicCaptchaConfig | null;
  /** null until loaded. */
  legalTexts: PublicLegalTexts | null;
  profile: MeProfileResponse | null;
  lateInvite?: PublicLateInvitePrefill;
  audience?: PublicPhase["audience"];
}

export function formConfig(
  data: EnrollmentFormPrefetchedData | undefined,
): EnrollmentFormConfig {
  return {
    schema: data?.schema ?? null,
    offerings: data?.offerings ?? [],
    careOfferingSelectionMode: data?.careOfferingSelectionMode ?? "optional",
    collectGradeLevel: data?.collectGradeLevel !== false,
    careOfferingsEnabled: data?.careOfferingsEnabled !== false,
    schoolClass: data?.schoolClass ?? EMPTY_SCHOOL_CLASS_CONFIG,
    eligibleGradeLevels: data?.eligibleGradeLevels ?? [],
    captchaConfig: data?.captchaConfig ?? null,
    legalTexts: data?.legalTexts ?? null,
    profile: data?.profile ?? null,
    lateInvite: data?.lateInvite,
    audience: data?.audience,
  };
}

/** The offerings of one phase for a staff preview without a public link (#6044). */
export interface PreviewCareOfferings {
  schoolClass?: PublicSchoolClassConfig;
  collectGradeLevel?: boolean;
  careOfferingsEnabled?: boolean;
  offerings: PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  eligibleGradeLevels: number[];
}

/** A preview: no phase, so offerings only when the staff page passes them. */
export function previewFormConfig(
  schema: PublicFormSchema | null,
  legalTexts: PublicLegalTexts,
  offerings?: PreviewCareOfferings,
): EnrollmentFormConfig {
  return {
    ...formConfig(
      offerings && {
        schema,
        legalTexts,
        captchaConfig: null,
        offerings: offerings.offerings,
        careOfferingSelectionMode: offerings.careOfferingSelectionMode,
        collectGradeLevel: offerings.collectGradeLevel,
        careOfferingsEnabled: offerings.careOfferingsEnabled,
        schoolClass: offerings.schoolClass,
        eligibleGradeLevels: offerings.eligibleGradeLevels,
      },
    ),
    schema,
    legalTexts,
  };
}

export interface GuardianDraft {
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
}

export function blankGuardian(): GuardianDraft {
  return { first_name: "", last_name: "", email: "", phone: "" };
}

/**
 * The guardian fields prefilled from the late invite or the profile. Only
 * empty fields take a prefill, so values a person typed stay.
 */
export function prefillGuardian(
  current: GuardianDraft,
  config: EnrollmentFormConfig,
): GuardianDraft {
  const invite = config.lateInvite;
  const guardian = config.profile?.guardian;
  const next = {
    first_name:
      current.first_name ||
      invite?.guardian_first_name ||
      guardian?.first_name ||
      "",
    last_name:
      current.last_name ||
      invite?.guardian_last_name ||
      guardian?.last_name ||
      "",
    email: current.email || invite?.guardian_email || guardian?.email || "",
    phone: current.phone || guardian?.phone || "",
  };
  const unchanged = (Object.keys(next) as (keyof GuardianDraft)[]).every(
    (key) => next[key] === current[key],
  );
  return unchanged ? current : next;
}

export interface EnrollmentFormDraft {
  guardian: GuardianDraft;
  additionalGuardians: GuardianDraft[];
  legalConsents: Record<string, boolean>;
  children: ChildDraft[];
  customData: Record<string, unknown>;
}

/** The values the form starts with: an edit draft, or a blank enrollment. */
export function initialFormDraft(
  config: EnrollmentFormConfig,
  draft: EnrollmentEditDraft | undefined,
): EnrollmentFormDraft {
  const seed = (children: ChildDraft[]) =>
    children.map((child) =>
      withRequiredOfferings(child, config.offerings, config.collectGradeLevel),
    );
  if (!draft) {
    return {
      guardian: prefillGuardian(blankGuardian(), config),
      additionalGuardians: [],
      legalConsents: {},
      children: seed([blankChild()]),
      customData: {},
    };
  }
  return {
    guardian: prefillGuardian(
      {
        first_name: draft.guardian_first_name,
        last_name: draft.guardian_last_name,
        email: draft.guardian_email,
        phone: draft.guardian_phone ?? "",
      },
      config,
    ),
    additionalGuardians:
      draft.additional_guardians?.map((guardian) => ({
        first_name: guardian.first_name,
        last_name: guardian.last_name,
        email: guardian.email ?? "",
        phone: guardian.phone ?? "",
      })) ?? [],
    legalConsents: Object.fromEntries(
      Object.entries(draft.consent_flags ?? {}).map(([key, value]) => [
        key,
        value === true,
      ]),
    ),
    children: sanitizeSchoolClasses(
      seed(draftChildren(draft)),
      config.schoolClass,
    ),
    customData: draft.custom_data ?? {},
  };
}

// Brand-new children count up; saved children derive their key from the
// persisted id so rebuilding the same draft does not remount each card.
let childDraftSeq = 0;

export function blankChild(): ChildDraft {
  return {
    clientId: `child-${childDraftSeq++}`,
    first_name: "",
    last_name: "",
    date_of_birth: "",
    target_grade_level: "",
    target_school_class: "",
    offering_ids: new Set(),
    offering_days: {},
    custom: {},
  };
}

function draftChildren(draft: EnrollmentEditDraft): ChildDraft[] {
  if (draft.children.length === 0) return [blankChild()];
  return draft.children.map((child) => {
    const dayRows = new Map(
      (child.offering_days ?? []).map((row) => [row.offering_id, row]),
    );
    const offeringIDs = new Set<string>();
    const offeringDays: Record<string, Set<string>> = {};
    for (const row of child.offering_days ?? []) {
      const hasAutomaticDays = (row.automatic_selected_days ?? []).length > 0;
      const manualDays =
        row.manual_selected_days ?? (hasAutomaticDays ? [] : row.selected_days);
      if (manualDays.length > 0) {
        offeringIDs.add(row.offering_id);
        offeringDays[row.offering_id] = new Set(manualDays);
      }
    }
    for (const id of child.offering_ids ?? []) {
      const row = dayRows.get(id);
      if (
        row &&
        (row.manual_selected_days ?? []).length === 0 &&
        (row.automatic_selected_days ?? []).length > 0
      ) {
        continue;
      }
      offeringIDs.add(id);
    }
    return {
      clientId: `draft-child:${child.id}`,
      id: child.id,
      first_name: child.first_name,
      last_name: child.last_name,
      date_of_birth: child.date_of_birth,
      target_grade_level: child.target_grade_level?.toString() ?? "",
      target_school_class: child.target_school_class ?? "",
      offering_ids: offeringIDs,
      offering_days: offeringDays,
      custom: child.custom_data ?? {},
      locked: child.locked ?? false,
      lockedPayload: child.locked
        ? {
            id: child.id,
            first_name: child.first_name,
            last_name: child.last_name,
            date_of_birth: child.date_of_birth,
            target_grade_level: child.target_grade_level,
            target_school_class: child.target_school_class,
            custom_data: child.custom_data,
            offering_ids: child.offering_ids.map(Number),
            offering_days: child.offering_days?.map((row) => ({
              offering_id: Number(row.offering_id),
              selected_days: row.selected_days,
            })),
          }
        : undefined,
    };
  });
}

/**
 * Collapses a saved class the phase no longer offers (renamed, removed or
 * of another grade) to "" ("Klasse offen"). The stale value has no option
 * in the select and the backend would reject it (#1833). Returns the same
 * array when nothing changed.
 */
export function sanitizeSchoolClasses(
  children: ChildDraft[],
  config: PublicSchoolClassConfig,
): ChildDraft[] {
  let changed = false;
  const next = children.map((c) => {
    const value = c.target_school_class.trim();
    if (value === "") return c;
    const offered =
      config.collect &&
      config.available_classes.includes(value) &&
      classMatchesGrade(value, c.target_grade_level);
    if (offered) return c;
    changed = true;
    return { ...c, target_school_class: "" };
  });
  return changed ? next : children;
}
