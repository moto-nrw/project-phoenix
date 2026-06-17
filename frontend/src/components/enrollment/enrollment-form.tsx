"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Check, FileText, Info, Lock, Plus, Trash2 } from "lucide-react";
import { Turnstile } from "@marsidev/react-turnstile";
import { useTenant } from "~/components/tenant/tenant-provider";
import deMessages from "~/i18n/messages/de.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";
import {
  fetchMyEnrollmentProfile,
  fetchPublicCareOfferings,
  submitEnrollment,
  type MeProfileChild,
  type MeProfileResponse,
  type CareOfferingSelectionMode,
  type PublicCareOffering,
  type SubmitChildPayload,
  type SubmitGuardianPayload,
  type EnrollmentEditDraft,
} from "~/lib/enrollment-submission-api";
import {
  fetchPublicActiveSchema,
  fetchPublicCaptchaConfig,
  fetchPublicLegalTexts,
  type PublicCaptchaConfig,
  type PublicFormSchema,
  type PublicLegalBlock,
  type PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import {
  buildFieldsByKey,
  isFieldVisible,
  visibleAnswerData,
  type ConditionContext,
} from "~/lib/enrollment-field-visibility";
import ReactMarkdown, { type Components } from "react-markdown";
import { Modal } from "~/components/ui/modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { Checkbox } from "~/components/ui/checkbox";
import { createLogger } from "~/lib/logger";
import { useScrollToFirstError } from "~/lib/hooks/use-scroll-to-error";

const logger = createLogger({ component: "EnrollmentForm" });

// Mirror of the backend canonical phone format
// (backend/models/users/phone_validation.go). Optional country code +
// 7..15 chars of digits/spaces/hyphens. Kept in sync so a value the form
// accepts can't later be rejected at student creation on approval.
const GUARDIAN_PHONE_PATTERN = /^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$/;

// Mirror of the backend canonical email format
// (backend/models/users/email_validation.go), the single rule enforced both
// at enrollment submit AND at student creation on approval. Kept in sync so a
// value the form accepts can never be rejected later at approval.
const GUARDIAN_EMAIL_PATTERN = /^[A-Za-z0-9._%-]+@[A-Za-z0-9.-]+[.][A-Za-z]+$/;

interface ChildDraft {
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level: string;
  offering_ids: Set<string>;
  /**
   * Per-offering day picks for offerings whose days_of_week_mode is
   * "parent_choice". Keyed by offering id (same shape as in offering_ids).
   * Absent when the offering uses fixed days. The form submits no days
   * entry for it. For parent_choice offerings the form REQUIRES at
   * least one day before allowing submit.
   */
  offering_days: Record<string, Set<string>>;
  custom: Record<string, unknown>;
}

// GuardianDraft is one ADDITIONAL guardian (co-guardian) the parent can
// add beyond the primary guardian. Only the names are required; email
// and phone are optional. On approval each becomes an additional
// students_guardians link to every enrolled child.
interface GuardianDraft {
  first_name: string;
  last_name: string;
  email: string;
  phone: string;
}

function blankGuardian(): GuardianDraft {
  return { first_name: "", last_name: "", email: "", phone: "" };
}

import type {
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";

interface Props {
  readonly phaseID?: string;
  readonly gradeLevelMax: number;
  readonly onSubmitted: (statusURL: string) => void;
  readonly prefetchedData?: EnrollmentFormPrefetchedData;
  readonly previewMode?: boolean;
  readonly previewSchema?: PublicFormSchema | null;
  /**
   * Optional overrides for the autofill fetcher and submitter, used
   * by the parents portal to swap in parent-scope endpoints
   * (/api/parent/...) instead of the default tenant-scope ones. When
   * unset, the form falls back to the public path.
   */
  readonly profileFetcher?: () => Promise<MeProfileResponse | null>;
  readonly submitter?: (
    payload: SubmitEnrollmentPayload,
  ) => Promise<SubmitEnrollmentResult>;
  /**
   * Hide the captcha widget. Set true when the caller has already
   * authenticated the user (e.g. parent JWT), so the backend's
   * authenticated submit endpoint will skip captcha verification.
   */
  readonly skipCaptcha?: boolean;
  /**
   * Opt into parent/public i18n chrome. Staff/admin previews deliberately keep
   * the German catalog, even if a parent locale cookie exists on the browser.
   */
  readonly localizedCopy?: boolean;
  readonly initialDraft?: EnrollmentEditDraft;
  readonly lockedGuardianEmail?: boolean;
  readonly submitLabel?: string;
}

type TranslationValues = Record<string, string | number | Date>;
type TranslationFn = (key: string, values?: TranslationValues) => string;
type EnrollmentFormTranslator = TranslationFn & {
  raw: (key: string) => unknown;
};

export interface EnrollmentFormPrefetchedData {
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  captchaConfig: PublicCaptchaConfig | null;
  legalTexts: PublicLegalTexts;
  profile?: MeProfileResponse | null;
}

/**
 * Public enrollment form. Loads the active schema + open care offerings,
 * collects guardian info + 1..n children + offering selections, and
 * submits to /api/enrollment/{tenantSlug}/submit. The captcha widget is
 * inlined when enrollment.captcha_site_key is present in the payload,
 * captcha verification happens server-side.
 *
 * The form intentionally renders core fields (guardian + child names,
 * email, DOB, grade) hardcoded. The form_schemas.fields JSONB only
 * adds custom fields on top. PR 5 enforced this distinction in the
 * model's CoreFieldKeys map.
 */
export function EnrollmentForm({
  phaseID,
  gradeLevelMax,
  onSubmitted,
  prefetchedData,
  previewMode = false,
  previewSchema,
  profileFetcher,
  submitter,
  skipCaptcha,
  localizedCopy = false,
  initialDraft,
  lockedGuardianEmail = false,
  submitLabel,
}: Props) {
  const intl = useTranslations("enrollmentForm");
  const activeLocale = useLocale();
  const tr = useMemo(
    () => makeEnrollmentFormTranslator(intl, localizedCopy),
    [intl, localizedCopy],
  );
  const { tenantSlug } = useTenant();
  const initialRequiredOfferingIDs =
    prefetchedData?.offerings.filter((o) => o.is_required).map((o) => o.id) ??
    [];
  const [schema, setSchema] = useState<PublicFormSchema | null>(
    prefetchedData?.schema ?? null,
  );
  const [offerings, setOfferings] = useState<PublicCareOffering[]>(
    prefetchedData?.offerings ?? [],
  );
  const [careOfferingSelectionMode, setCareOfferingSelectionMode] =
    useState<CareOfferingSelectionMode>(
      prefetchedData?.careOfferingSelectionMode ?? "optional",
    );
  // Offerings the school flagged as mandatory. These are pre-selected and
  // locked in the UI so every child carries them.
  const requiredOfferingIDs = useMemo(
    () => offerings.filter((o) => o.is_required).map((o) => o.id),
    [offerings],
  );
  const [loading, setLoading] = useState(prefetchedData === undefined);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [childOfferingErrors, setChildOfferingErrors] = useState<
    Record<number, boolean>
  >({});
  // Per parent_choice offering that is selected but has no day picked.
  // Keyed by `${childIndex}_${offeringId}` so the offending offering row
  // (not the whole group) gets the red border + inline message, and the
  // scroll-to-error effect can land on it via aria-invalid.
  const [offeringDayErrors, setOfferingDayErrors] = useState<
    Record<string, boolean>
  >({});
  // Per-field validation errors, keyed by each input's `name`
  // (guardian_email, children_0_first_name, ...). Drives the red border +
  // inline message on the offending input; rebuilt on every submit.
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  // Scroll to the first invalid field on every submit attempt that leaves an
  // error showing. The form is long (guardian + N children + offerings) with
  // the submit button at the bottom, so an error rendered above is easy to
  // miss. `scrollToError()` is keyed on a per-attempt counter, so a repeated
  // submit with the *same* unchanged message still scrolls back to it; on a
  // successful submit nothing is marked invalid so it's a no-op.
  const { formRef, errorRef, scrollToError } = useScrollToFirstError();

  const [guardianFirstName, setGuardianFirstName] = useState(
    initialDraft?.guardian_first_name ??
      prefetchedData?.profile?.guardian.first_name ??
      "",
  );
  const [guardianLastName, setGuardianLastName] = useState(
    initialDraft?.guardian_last_name ??
      prefetchedData?.profile?.guardian.last_name ??
      "",
  );
  const [guardianEmail, setGuardianEmail] = useState(
    initialDraft?.guardian_email ??
      prefetchedData?.profile?.guardian.email ??
      "",
  );
  const [guardianPhone, setGuardianPhone] = useState(
    initialDraft?.guardian_phone ??
      prefetchedData?.profile?.guardian.phone ??
      "",
  );
  const [legalConsents, setLegalConsents] = useState<Record<string, boolean>>(
    () => draftConsentFlags(initialDraft),
  );
  const [children, setChildren] = useState<ChildDraft[]>([
    ...(initialDraft
      ? draftChildren(initialDraft, initialRequiredOfferingIDs)
      : [blankChild(initialRequiredOfferingIDs)]),
  ]);
  // Additional guardians (co-guardians). Start empty — the primary
  // guardian above is always required; co-guardians are opt-in.
  const [additionalGuardians, setAdditionalGuardians] = useState<
    GuardianDraft[]
  >(() => draftGuardians(initialDraft));
  // Request-level custom fields (applies_to_child=false). Stored
  // separately from per-child custom data because their values are
  // shared across all children in the submission.
  const [customData, setCustomData] = useState<Record<string, unknown>>(
    () => initialDraft?.custom_data ?? {},
  );
  useEffect(() => {
    if (!initialDraft) return;
    setGuardianFirstName(initialDraft.guardian_first_name);
    setGuardianLastName(initialDraft.guardian_last_name);
    setGuardianEmail(initialDraft.guardian_email);
    setGuardianPhone(initialDraft.guardian_phone ?? "");
    setLegalConsents(draftConsentFlags(initialDraft));
    setChildren(draftChildren(initialDraft, requiredOfferingIDs));
    setAdditionalGuardians(draftGuardians(initialDraft));
    setCustomData(initialDraft.custom_data ?? {});
  }, [initialDraft, requiredOfferingIDs]);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaConfig, setCaptchaConfig] =
    useState<PublicCaptchaConfig | null>(prefetchedData?.captchaConfig ?? null);
  // Per-tenant legal block contract. null until fetched.
  const [legalTexts, setLegalTexts] = useState<PublicLegalTexts | null>(
    prefetchedData?.legalTexts ?? null,
  );
  // Which legal block text the parent is currently viewing in the modal.
  const [openLegalDoc, setOpenLegalDoc] = useState<PublicLegalBlock | null>(
    null,
  );
  const [profile, setProfile] = useState<MeProfileResponse | null>(
    prefetchedData?.profile ?? null,
  );
  const [usedExistingChildIDs, setUsedExistingChildIDs] = useState<Set<string>>(
    new Set(),
  );

  useEffect(() => {
    let cancelled = false;
    if (prefetchedData !== undefined) {
      setSchema(prefetchedData.schema);
      setOfferings(prefetchedData.offerings);
      setCareOfferingSelectionMode(prefetchedData.careOfferingSelectionMode);
      setCaptchaConfig(prefetchedData.captchaConfig);
      setLegalTexts(prefetchedData.legalTexts);
      setProfile(prefetchedData.profile ?? null);
      setChildren((prev) =>
        seedRequiredOfferings(
          prev.length > 0 ? prev : [blankChild()],
          prefetchedData.offerings
            .filter((o) => o.is_required)
            .map((o) => o.id),
        ),
      );
      if (prefetchedData.profile) {
        setGuardianFirstName(
          (prev) => prev || prefetchedData.profile?.guardian.first_name || "",
        );
        setGuardianLastName(
          (prev) => prev || prefetchedData.profile?.guardian.last_name || "",
        );
        setGuardianEmail(
          (prev) => prev || prefetchedData.profile?.guardian.email || "",
        );
        setGuardianPhone(
          (prev) => prev || prefetchedData.profile?.guardian.phone || "",
        );
      }
      setError(null);
      setLoading(false);
      return () => {
        cancelled = true;
      };
    }
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const profileLoader = profileFetcher ?? fetchMyEnrollmentProfile;
        const [
          schemaResult,
          offeringsResult,
          profileResult,
          captchaResult,
          legalResult,
        ] = await Promise.all([
          previewSchema !== undefined
            ? Promise.resolve(previewSchema)
            : phaseID
              ? fetchPublicActiveSchema(tenantSlug, phaseID).catch(() => null)
              : Promise.resolve(null),
          phaseID
            ? fetchPublicCareOfferings(tenantSlug, phaseID)
            : Promise.resolve({
                offerings: [],
                careOfferingSelectionMode: "optional" as const,
                careRequired: false,
              }),
          profileLoader().catch(() => null),
          // Skip the captcha config when the caller already authenticated
          // the parent (parent-portal embedded form). Saves a round-trip
          // and avoids rendering the widget on an already-trusted path.
          skipCaptcha
            ? Promise.resolve(null)
            : fetchPublicCaptchaConfig(tenantSlug).catch(() => null),
          // Legal texts are phase/template-aware. NOT best-
          // effort: a real load failure rejects the whole load so the
          // form shows an error instead of collecting legally relevant
          // consent without the configured documents. Unconfigured
          // texts return empty strings (no rejection) and just drop the
          // link.
          previewSchema !== undefined
            ? resolvePreviewLegalTexts(tenantSlug, previewSchema)
            : phaseID
              ? fetchPublicLegalTexts(tenantSlug, phaseID)
              : fetchPublicLegalTexts(tenantSlug),
        ]);
        if (cancelled) return;
        setSchema(schemaResult);
        setOfferings(offeringsResult.offerings);
        setCareOfferingSelectionMode(offeringsResult.careOfferingSelectionMode);
        // Seed mandatory offerings into the children that already exist
        // (the initial blank slot is created before offerings load).
        const requiredIDs = offeringsResult.offerings
          .filter((o) => o.is_required)
          .map((o) => o.id);
        if (requiredIDs.length > 0) {
          setChildren((prev) => seedRequiredOfferings(prev, requiredIDs));
        }
        setCaptchaConfig(captchaResult);
        setLegalTexts(legalResult);
        // Prefill guardian fields from the profile when present. We
        // only fill empty fields so an admin testing the form on a
        // teacher session can still type their own values.
        if (profileResult) {
          setProfile(profileResult);
          setGuardianFirstName(
            (prev) => prev || profileResult.guardian.first_name,
          );
          setGuardianLastName(
            (prev) => prev || profileResult.guardian.last_name,
          );
          setGuardianEmail((prev) => prev || profileResult.guardian.email);
          setGuardianPhone(
            (prev) => prev || (profileResult.guardian.phone ?? ""),
          );
        }
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : tr("submitErrorFallback");
        logger.error("enrollment_form_load_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [
    tenantSlug,
    phaseID,
    prefetchedData,
    previewSchema,
    profileFetcher,
    skipCaptcha,
    tr,
  ]);

  const updateChild = (index: number, patch: Partial<ChildDraft>) => {
    setChildren((prev) =>
      prev.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    );
  };

  const toggleOffering = (childIndex: number, offeringID: string) => {
    // Mandatory offerings are locked - they can never be unselected. The
    // selection mode only governs the choosable (non-required) offerings,
    // so required ones never block toggling a choosable one.
    if (requiredOfferingIDs.includes(offeringID)) return;
    // "Genau eines" / "höchstens eines" selection_groups behave like radios:
    // selecting one clears the other selected member(s) of the same group.
    // Orthogonal to the phase-level exactly_one mode below; both can apply.
    const toggled = offerings.find((o) => o.id === offeringID);
    const group = toggled?.selection_group?.trim() ?? "";
    const groupRule = toggled?.selection_rule ?? "optional";
    const exclusiveGroup =
      group !== "" &&
      (groupRule === "exactly_one" || groupRule === "at_most_one");
    const groupSiblingIDs = exclusiveGroup
      ? new Set(
          offerings
            .filter((o) => (o.selection_group?.trim() ?? "") === group)
            .map((o) => o.id),
        )
      : null;
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const nextIDs = new Set(c.offering_ids);
        const nextDays = { ...c.offering_days };
        if (nextIDs.has(offeringID)) {
          nextIDs.delete(offeringID);
          delete nextDays[offeringID];
        } else {
          if (careOfferingSelectionMode === "exactly_one") {
            for (const existingID of nextIDs) {
              if (!requiredOfferingIDs.includes(existingID)) {
                delete nextDays[existingID];
              }
            }
            nextIDs.clear();
            requiredOfferingIDs.forEach((id) => nextIDs.add(id));
          }
          if (groupSiblingIDs) {
            for (const id of groupSiblingIDs) {
              // Never clear a locked required offering: it's always part of
              // the submission and the UI keeps rendering it checked, so
              // dropping it from the payload would fail the backend's
              // required-offering check.
              if (
                id !== offeringID &&
                nextIDs.has(id) &&
                !requiredOfferingIDs.includes(id)
              ) {
                nextIDs.delete(id);
                delete nextDays[id];
              }
            }
          }
          nextIDs.add(offeringID);
        }
        return { ...c, offering_ids: nextIDs, offering_days: nextDays };
      }),
    );
  };

  // toggleOfferingDay flips one day in an offering's selected-day set.
  // Only meaningful when the offering's days_of_week_mode is "parent_choice"
  // Callers pre-check that.
  const toggleOfferingDay = (
    childIndex: number,
    offeringID: string,
    day: string,
  ) => {
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const current = c.offering_days[offeringID] ?? new Set<string>();
        const next = new Set(current);
        if (next.has(day)) next.delete(day);
        else next.add(day);
        return {
          ...c,
          offering_days: { ...c.offering_days, [offeringID]: next },
        };
      }),
    );
    // Clear the "no day picked" error for this offering as soon as the
    // parent picks one, so the red border + message disappear live.
    setOfferingDayErrors((prev) => {
      const key = `${childIndex}_${offeringID}`;
      if (!prev[key]) return prev;
      const nextErrors = { ...prev };
      delete nextErrors[key];
      return nextErrors;
    });
  };

  const addChild = () =>
    setChildren((prev) => [...prev, blankChild(requiredOfferingIDs)]);
  const removeChild = (index: number) =>
    setChildren((prev) => prev.filter((_, i) => i !== index));

  const addGuardian = () =>
    setAdditionalGuardians((prev) => [...prev, blankGuardian()]);
  const updateGuardian = (index: number, patch: Partial<GuardianDraft>) =>
    setAdditionalGuardians((prev) =>
      prev.map((g, i) => (i === index ? { ...g, ...patch } : g)),
    );
  const removeGuardian = (index: number) =>
    setAdditionalGuardians((prev) => prev.filter((_, i) => i !== index));

  // Conditional field visibility. Contexts are rebuilt from the live answers
  // each render so info blocks and questions appear/disappear as the parent
  // fills the form. Hidden fields are also stripped from the submitted payload
  // (see handleSubmit) so a stale value never leaks.
  const fieldsByKey = useMemo(
    () => buildFieldsByKey(schema?.fields ?? []),
    [schema],
  );
  const guardianCtx: ConditionContext = {
    guardianAnswers: customData,
    fieldsByKey,
  };
  const visibleGuardianFields = (schema?.fields ?? []).filter(
    (f) => !f.applies_to_child && isFieldVisible(f, guardianCtx),
  );
  const legalBlocks = legalTexts?.blocks ?? [];
  const childConditionCtx = (child: ChildDraft): ConditionContext => ({
    guardianAnswers: customData,
    childAnswers: child.custom,
    gradeLevel: child.target_grade_level,
    // Care-offering conditions match by name (templates aren't phase-bound),
    // so resolve the child's selected offering ids to names.
    offeringNames: new Set(
      Array.from(child.offering_ids)
        .map((id) => offerings.find((o) => o.id === id)?.name)
        .filter((name): name is string => Boolean(name))
        .map((name) => name.toLowerCase()),
    ),
    fieldsByKey,
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setChildOfferingErrors({});
    setOfferingDayErrors({});
    setFieldErrors({});
    // Bump per attempt so the scroll-to-error effect re-runs even when this
    // submit produces the same error message as the previous one. Covers
    // every synchronous validation failure below (error is set in the same
    // batch). On success no error is set, so the scroll is a no-op.
    scrollToError();

    if (previewMode) {
      setError(tr("previewError"));
      return;
    }
    if (!phaseID) {
      setError(tr("missingPhase"));
      return;
    }

    // Collect every missing/invalid required field in one pass so they all
    // get marked at once (red border + inline message) instead of bailing
    // on the first. Keys match each input's `name` attribute.
    const newFieldErrors: Record<string, string> = {};
    if (!guardianFirstName.trim()) {
      newFieldErrors.guardian_first_name = tr("errors.firstName");
    }
    if (!guardianLastName.trim()) {
      newFieldErrors.guardian_last_name = tr("errors.lastName");
    }
    const trimmedEmail = guardianEmail.trim();
    if (!trimmedEmail) {
      newFieldErrors.guardian_email = tr("errors.email");
    } else if (!GUARDIAN_EMAIL_PATTERN.test(trimmedEmail)) {
      newFieldErrors.guardian_email = tr("errors.validEmail");
    }
    // Phone is optional, but reject an invalid format before submit so the
    // parent fixes their own typo instead of producing a request that can
    // never be approved (the backend enforces the same rule).
    const trimmedPhone = guardianPhone.trim();
    const guardianPhoneRequired =
      schema?.core_requirements?.guardian_phone === true;
    if (guardianPhoneRequired && !trimmedPhone) {
      newFieldErrors.guardian_phone = tr("errors.phone");
    } else if (trimmedPhone && !GUARDIAN_PHONE_PATTERN.test(trimmedPhone)) {
      newFieldErrors.guardian_phone = tr("errors.validPhone");
    }
    // Additional guardians: a card with ANY field filled must carry both
    // names; email/phone are validated for format only when present.
    // Fully-empty cards are dropped at submit, so they never block it.
    additionalGuardians.forEach((g, i) => {
      const gFirst = g.first_name.trim();
      const gLast = g.last_name.trim();
      const gEmail = g.email.trim();
      const gPhone = g.phone.trim();
      if (!gFirst && !gLast && !gEmail && !gPhone) return;
      if (!gFirst) {
        newFieldErrors[`additional_guardian_${i}_first_name`] =
          tr("errors.firstName");
      }
      if (!gLast) {
        newFieldErrors[`additional_guardian_${i}_last_name`] =
          tr("errors.lastName");
      }
      if (gEmail && !GUARDIAN_EMAIL_PATTERN.test(gEmail)) {
        newFieldErrors[`additional_guardian_${i}_email`] =
          tr("errors.validEmail");
      }
      if (gPhone && !GUARDIAN_PHONE_PATTERN.test(gPhone)) {
        newFieldErrors[`additional_guardian_${i}_phone`] =
          tr("errors.validPhone");
      }
    });
    // Only visible (passing their show-if condition) non-info guardian fields
    // are enforced — a hidden required field must never block submit.
    for (const field of visibleGuardianFields) {
      if (field.type === "information" || !field.required) continue;
      if (customValueMissing(field, customData[field.key])) {
        newFieldErrors[`custom_${field.key}`] = requiredMessageForField(
          field,
          tr,
        );
      }
    }
    for (const [i, c] of children.entries()) {
      const childCtx = childConditionCtx(c);
      if (!c.first_name.trim()) {
        newFieldErrors[`children_${i}_first_name`] = tr("errors.firstName");
      }
      if (!c.last_name.trim()) {
        newFieldErrors[`children_${i}_last_name`] = tr("errors.lastName");
      }
      if (!c.date_of_birth) {
        newFieldErrors[`children_${i}_date_of_birth`] = tr("errors.dob");
      }
      if (!c.target_grade_level) {
        newFieldErrors[`children_${i}_target_grade_level`] = tr("errors.grade");
      }
      for (const field of schema?.fields.filter((f) => f.applies_to_child) ??
        []) {
        if (field.type === "information" || !field.required) continue;
        if (!isFieldVisible(field, childCtx)) continue;
        if (
          customValueMissing(
            field,
            c.custom[field.key],
            relevantCareDaysForChild(c, offerings, previewMode),
          )
        ) {
          newFieldErrors[`children_${i}_custom_${field.key}`] =
            requiredMessageForField(field, tr);
        }
      }
    }
    // Required legal blocks are collected into the same pass so a missing
    // confirmation is marked together with other missing fields. The backend
    // derives the same required keys from the public legal block contract; the
    // messages are localized chrome even though the legal labels/texts stay
    // school-authored.
    for (const block of legalBlocks) {
      if (block.required && !legalConsents[block.key]) {
        newFieldErrors[`consent_${block.key}`] = tr("errors.consentRequired");
      }
    }
    // Offering validation runs in the SAME pass as the field checks above
    // so a missing care-offering day and a missing core field (e.g. phone)
    // are flagged together on a single submit, not one-at-a-time. This pass
    // only collects errors; the payload is assembled afterwards once
    // everything is valid.
    const missingCareIndexes: number[] = [];
    const exactOneCareIndexes: number[] = [];
    // Per-selection_group rule violations (exactly_one / at_least_one /
    // at_most_one within a named group). Orthogonal to the phase-level
    // selection mode above; both are enforced.
    const groupRuleIndexes: number[] = [];
    let firstGroupRuleMessage: string | null = null;
    // Selected parent_choice offerings with no day picked, keyed
    // `${childIndex}_${offeringId}` so each offending row gets a red border +
    // inline message and the scroll-to-error effect can target it.
    const dayErrors: Record<string, boolean> = {};
    let firstDayErrorMessage: string | null = null;
    // The selection mode only constrains the choosable (non-required)
    // offerings. Required offerings are always-on and must not count toward
    // "at least one" / "exactly one"; otherwise a required base offering
    // would make exactly_one unsatisfiable.
    for (const [i, c] of children.entries()) {
      const choosableSelectedCount = [...c.offering_ids].filter(
        (id) => !requiredOfferingIDs.includes(id),
      ).length;
      if (
        careOfferingSelectionMode === "at_least_one" &&
        choosableSelectedCount === 0
      ) {
        missingCareIndexes.push(i);
      }
      if (
        careOfferingSelectionMode === "exactly_one" &&
        choosableSelectedCount !== 1
      ) {
        exactOneCareIndexes.push(i);
      }
      for (const id of c.offering_ids) {
        const offering = offerings.find((o) => o.id === id);
        if (offering?.days_of_week_mode !== "parent_choice") {
          continue;
        }
        const picked = c.offering_days[id];
        if (!picked || picked.size === 0) {
          dayErrors[`${i}_${id}`] = true;
          if (!firstDayErrorMessage) {
            firstDayErrorMessage = tr("errors.dayMissing", {
              child: i + 1,
              offering: offering.name,
            });
          }
        }
      }
      const groupRuleMessage = offeringGroupRuleError(c, offerings, tr);
      if (groupRuleMessage) {
        groupRuleIndexes.push(i);
        firstGroupRuleMessage ??= `${tr("structured.child")} ${i + 1}: ${groupRuleMessage}`;
      }
    }

    const fieldErrorKeys = Object.keys(newFieldErrors);
    const dayErrorCount = Object.keys(dayErrors).length;
    const hasOfferingIssue =
      missingCareIndexes.length > 0 ||
      exactOneCareIndexes.length > 0 ||
      groupRuleIndexes.length > 0 ||
      dayErrorCount > 0;
    if (fieldErrorKeys.length > 0 || hasOfferingIssue) {
      setFieldErrors(newFieldErrors);
      const invalidCareIndexes = [
        ...missingCareIndexes,
        ...exactOneCareIndexes,
        ...groupRuleIndexes,
      ];
      if (invalidCareIndexes.length > 0) {
        setChildOfferingErrors(
          Object.fromEntries(invalidCareIndexes.map((i) => [i, true])),
        );
      }
      setOfferingDayErrors(dayErrors);
      // Banner text:
      //  - exactly one missing field: its own specific message
      //  - only consents missing: the consent summary
      //  - only care-offering issues: the matching offering message
      //  - anything else / mixed: a generic summary
      // Every offending input is red-marked regardless, and the scroll
      // effect lands on the first one.
      const onlyConsents =
        !hasOfferingIssue &&
        fieldErrorKeys.length > 0 &&
        fieldErrorKeys.every((key) => key.startsWith("consent_"));
      let banner = tr("fixMarked");
      if (fieldErrorKeys.length === 1 && !hasOfferingIssue) {
        banner = Object.values(newFieldErrors)[0] ?? banner;
      } else if (onlyConsents) {
        banner = tr("consentSummary");
      } else if (fieldErrorKeys.length === 0 && missingCareIndexes.length > 0) {
        banner = tr("errors.careMissing");
      } else if (
        fieldErrorKeys.length === 0 &&
        exactOneCareIndexes.length > 0
      ) {
        banner = tr("errors.careExactlyOne");
      } else if (
        fieldErrorKeys.length === 0 &&
        dayErrorCount > 0 &&
        firstDayErrorMessage
      ) {
        banner = firstDayErrorMessage;
      } else if (
        fieldErrorKeys.length === 0 &&
        groupRuleIndexes.length > 0 &&
        firstGroupRuleMessage
      ) {
        banner = firstGroupRuleMessage;
      }
      setError(banner);
      return;
    }

    // All required input is valid, assemble the wire payload. Day
    // selections for parent_choice offerings are guaranteed present by the
    // validation above, so this only builds data (no further checks).
    const payloadChildren: SubmitChildPayload[] = children.map((c) => {
      const offeringDaysPayload: Array<{
        offering_id: number;
        selected_days: string[];
      }> = [];
      for (const id of c.offering_ids) {
        const offering = offerings.find((o) => o.id === id);
        if (offering?.days_of_week_mode !== "parent_choice") {
          continue;
        }
        const picked = c.offering_days[id];
        if (!picked || picked.size === 0) continue;
        offeringDaysPayload.push({
          offering_id: Number(id),
          // Preserve the admin-defined order from available_days so the wire
          // payload doesn't fluctuate with the parent's click order.
          selected_days: offering.available_days.filter((d) => picked.has(d)),
        });
      }
      return {
        first_name: c.first_name.trim(),
        last_name: c.last_name.trim(),
        date_of_birth: c.date_of_birth,
        target_grade_level: Number(c.target_grade_level),
        // Strip answers to fields the parent couldn't see (hidden by a
        // show-if condition) so a stale value never reaches the backend.
        custom_data: pruneWeekdayMultiModeAnswers(
          visibleAnswerData(
            schema?.fields ?? [],
            true,
            c.custom,
            childConditionCtx(c),
          ),
          schema?.fields ?? [],
          relevantCareDaysForChild(c, offerings, previewMode),
        ),
        offering_ids: Array.from(c.offering_ids).map((id) => Number(id)),
        offering_days:
          offeringDaysPayload.length > 0 ? offeringDaysPayload : undefined,
      };
    });

    setSubmitting(true);
    try {
      const consentFlags: Record<string, unknown> = {};
      for (const block of legalBlocks) {
        consentFlags[block.key] =
          block.kind === "notice" ? true : legalConsents[block.key] === true;
      }

      // Drop fully-blank co-guardian cards; trim + lowercase the rest.
      // Validation above guarantees any non-blank card has both names.
      const additionalGuardiansPayload: SubmitGuardianPayload[] =
        additionalGuardians
          .map((g) => ({
            first_name: g.first_name.trim(),
            last_name: g.last_name.trim(),
            email: g.email.trim().toLowerCase() || undefined,
            phone: g.phone.trim() || undefined,
          }))
          .filter(
            (g) =>
              g.first_name !== "" || g.last_name !== "" || g.email || g.phone,
          );

      const payload: SubmitEnrollmentPayload = {
        phase_id: Number(phaseID),
        guardian_first_name: guardianFirstName.trim(),
        guardian_last_name: guardianLastName.trim(),
        guardian_email: guardianEmail.trim().toLowerCase(),
        guardian_phone: guardianPhone.trim() || undefined,
        additional_guardians:
          additionalGuardiansPayload.length > 0
            ? additionalGuardiansPayload
            : undefined,
        consent_flags: consentFlags,
        custom_data: visibleAnswerData(
          schema?.fields ?? [],
          false,
          customData,
          guardianCtx,
        ),
        children: payloadChildren,
        captcha_token: skipCaptcha ? undefined : captchaToken || undefined,
      };
      const result = submitter
        ? await submitter(payload)
        : await submitEnrollment(tenantSlug, payload);
      onSubmitted(result.status_url);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : tr("submitErrorFallback");
      const code = (err as { code?: string } | undefined)?.code;
      logger.error("enrollment_submit_failed", { error: message, code });
      if (
        code === "enrollment.care_offering_missing" ||
        code === "enrollment.care_offering_exactly_one"
      ) {
        // Backend re-checked the phase mode (defense-in-depth). Mark
        // every child that violates the current mode. The server
        // doesn't tell us which one, so we derive it locally.
        const empties = children.reduce<Record<number, boolean>>(
          (acc, c, i) => {
            if (
              code === "enrollment.care_offering_missing" &&
              c.offering_ids.size === 0
            ) {
              acc[i] = true;
            }
            if (
              code === "enrollment.care_offering_exactly_one" &&
              c.offering_ids.size !== 1
            ) {
              acc[i] = true;
            }
            return acc;
          },
          {},
        );
        setChildOfferingErrors(empties);
      }
      setError(message);
      // A server-side rejection resolves after the synchronous attempt bump
      // above, so bump again to scroll the late-arriving error into view.
      scrollToError();
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 shadow-sm sm:p-6">
        {tr("loading")}
      </div>
    );
  }

  return (
    <form
      ref={formRef}
      onSubmit={handleSubmit}
      noValidate
      className="space-y-5"
    >
      {error && (
        <div
          ref={errorRef}
          className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm font-medium text-[#CC2626]"
          role="alert"
          aria-live="polite"
        >
          {error}
        </div>
      )}

      <section className="moto-content-surface space-y-5 rounded-xl border p-4 shadow-sm sm:p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <SectionHeading
            kicker={tr("sections.guardianKicker")}
            title={tr("sections.guardianTitle")}
            description={tr("sections.guardianDescription")}
          />
          <button
            type="button"
            onClick={addGuardian}
            className="moto-content-surface inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-lg border px-3 text-sm font-semibold whitespace-nowrap text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto sm:shrink-0"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            {tr("actions.addGuardian")}
          </button>
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Input
            label={tr("fields.firstName")}
            name="guardian_first_name"
            autoComplete="given-name"
            value={guardianFirstName}
            onChange={setGuardianFirstName}
            required
            error={fieldErrors.guardian_first_name}
          />
          <Input
            label={tr("fields.lastName")}
            name="guardian_last_name"
            autoComplete="family-name"
            value={guardianLastName}
            onChange={setGuardianLastName}
            required
            error={fieldErrors.guardian_last_name}
          />
          <Input
            label={tr("fields.email")}
            name="guardian_email"
            type="email"
            autoComplete="email"
            value={guardianEmail}
            onChange={setGuardianEmail}
            required
            readOnly={lockedGuardianEmail}
            error={fieldErrors.guardian_email}
          />
          <Input
            label={tr(
              schema?.core_requirements?.guardian_phone
                ? "fields.phoneRequired"
                : "fields.phone",
            )}
            name="guardian_phone"
            type="tel"
            autoComplete="tel"
            inputMode="tel"
            value={guardianPhone}
            onChange={setGuardianPhone}
            required={schema?.core_requirements?.guardian_phone === true}
            error={fieldErrors.guardian_phone}
          />
        </div>

        {additionalGuardians.map((g, i) => (
          <div
            key={i}
            className="space-y-5 rounded-xl border border-gray-200 bg-white p-3 sm:p-4"
          >
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
                {tr("sections.additionalGuardianLabel", { index: i + 1 })}
              </h3>
              <button
                type="button"
                onClick={() => removeGuardian(i)}
                className="rounded-lg px-2 py-1 text-xs font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
              >
                {tr("actions.remove")}
              </button>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input
                label={tr("fields.firstName")}
                name={`additional_guardian_${i}_first_name`}
                autoComplete="given-name"
                value={g.first_name}
                onChange={(v) => updateGuardian(i, { first_name: v })}
                required
                error={fieldErrors[`additional_guardian_${i}_first_name`]}
              />
              <Input
                label={tr("fields.lastName")}
                name={`additional_guardian_${i}_last_name`}
                autoComplete="family-name"
                value={g.last_name}
                onChange={(v) => updateGuardian(i, { last_name: v })}
                required
                error={fieldErrors[`additional_guardian_${i}_last_name`]}
              />
              <Input
                label={tr("fields.emailPlain")}
                name={`additional_guardian_${i}_email`}
                type="email"
                autoComplete="email"
                value={g.email}
                onChange={(v) => updateGuardian(i, { email: v })}
                error={fieldErrors[`additional_guardian_${i}_email`]}
              />
              <Input
                label={tr("fields.phone")}
                name={`additional_guardian_${i}_phone`}
                type="tel"
                autoComplete="tel"
                inputMode="tel"
                value={g.phone}
                onChange={(v) => updateGuardian(i, { phone: v })}
                error={fieldErrors[`additional_guardian_${i}_phone`]}
              />
            </div>
          </div>
        ))}
      </section>

      {schema?.fields.some((f) => !f.applies_to_child) && (
        <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
          <SectionHeading
            kicker={tr("sections.extraKicker")}
            title={tr("sections.extraTitle")}
            description={tr("sections.extraDescription")}
          />
          {schema.fields
            .filter(
              (f) => !f.applies_to_child && isFieldVisible(f, guardianCtx),
            )
            .map((f) =>
              f.type === "information" ? (
                <InfoBlock key={f.key} field={f} />
              ) : (
                <CustomFieldInput
                  key={f.key}
                  field={f}
                  value={customData[f.key]}
                  onChange={(v) =>
                    setCustomData((prev) => ({ ...prev, [f.key]: v }))
                  }
                  error={fieldErrors[`custom_${f.key}`]}
                  tr={tr}
                />
              ),
            )}
        </section>
      )}

      <section className="moto-content-surface space-y-5 rounded-xl border p-4 shadow-sm sm:p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <SectionHeading
            kicker={tr("sections.childrenKicker")}
            title={tr("sections.childrenTitle")}
            description={tr("sections.childrenDescription")}
          />
          <button
            type="button"
            onClick={addChild}
            className="moto-content-surface inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-lg border px-3 text-sm font-semibold whitespace-nowrap text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto sm:shrink-0"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            {tr("actions.addChild")}
          </button>
        </div>

        {profile && profile.children.length > 0 && (
          <ExistingChildrenPanel
            existing={profile.children}
            usedIDs={usedExistingChildIDs}
            tr={tr}
            onAdopt={(child) => {
              const newSlot: ChildDraft = blankChild(requiredOfferingIDs);
              newSlot.first_name = child.first_name;
              newSlot.last_name = child.last_name;
              newSlot.target_grade_level =
                child.grade_level != null ? String(child.grade_level) : "";
              setChildren((prev) => {
                // Replace the first empty slot if one exists, otherwise
                // append. Avoids leaving a stranded empty card after
                // every adoption.
                const emptyIdx = prev.findIndex(
                  (c) => !c.first_name && !c.last_name && !c.date_of_birth,
                );
                if (emptyIdx >= 0) {
                  return prev.map((c, i) => (i === emptyIdx ? newSlot : c));
                }
                return [...prev, newSlot];
              });
              setUsedExistingChildIDs((prev) => {
                const next = new Set(prev);
                next.add(child.id);
                return next;
              });
            }}
          />
        )}

        {children.map((child, i) => (
          <div
            key={i}
            className="space-y-5 rounded-xl border border-gray-200 bg-white p-3 sm:p-4"
          >
            <div className="flex items-center justify-between gap-3">
              <h3 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
                {tr("structured.child")} {i + 1}
              </h3>
              {children.length > 1 && (
                <button
                  type="button"
                  onClick={() => removeChild(i)}
                  className="rounded-lg px-2 py-1 text-xs font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
                >
                  {tr("actions.remove")}
                </button>
              )}
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input
                label={tr("fields.firstName")}
                name={`children_${i}_first_name`}
                autoComplete="given-name"
                value={child.first_name}
                onChange={(v) => updateChild(i, { first_name: v })}
                required
                error={fieldErrors[`children_${i}_first_name`]}
              />
              <Input
                label={tr("fields.lastName")}
                name={`children_${i}_last_name`}
                autoComplete="family-name"
                value={child.last_name}
                onChange={(v) => updateChild(i, { last_name: v })}
                required
                error={fieldErrors[`children_${i}_last_name`]}
              />
              <div>
                <span className="block text-sm font-semibold text-gray-700">
                  {tr("fields.dateOfBirth")}
                </span>
                <DateOfBirthPicker
                  value={child.date_of_birth}
                  onChange={(v) => updateChild(i, { date_of_birth: v })}
                  error={fieldErrors[`children_${i}_date_of_birth`]}
                  tr={tr}
                />
              </div>
              <GradeLevelSelect
                id={`children-${i}-target-grade-level`}
                value={child.target_grade_level}
                onChange={(v) => updateChild(i, { target_grade_level: v })}
                max={gradeLevelMax}
                error={fieldErrors[`children_${i}_target_grade_level`]}
                tr={tr}
              />
            </div>

            {offerings.length > 0 && (
              <div className="space-y-4">
                {/* Required offerings are part of the enrollment regardless of
                    the parent's choice. They are presented as an "always
                    included" block - locked, no checkbox-style selection - so
                    they never read as "something I picked". The selection mode
                    and its "choose at least one" requirement live entirely in
                    the choosable block below. */}
                {offerings.some((o) => o.is_required) && (
                  <div>
                    <p className="text-sm font-semibold text-gray-700">
                      {tr("care.requiredTitle")}
                    </p>
                    <p className="mt-1 text-xs leading-5 text-gray-500">
                      {tr("care.requiredDescription")}
                    </p>
                    <div className="mt-2 space-y-2">
                      {offerings
                        .filter((o) => o.is_required)
                        .map((o) => (
                          <OfferingCard
                            key={o.id}
                            offering={o}
                            childIndex={i}
                            checked
                            selectedDays={child.offering_days[o.id]}
                            dayError={
                              offeringDayErrors[`${i}_${o.id}`] ?? false
                            }
                            onToggle={() => toggleOffering(i, o.id)}
                            onToggleDay={(day) =>
                              toggleOfferingDay(i, o.id, day)
                            }
                            tr={tr}
                          />
                        ))}
                    </div>
                  </div>
                )}
                {offerings.some((o) => !o.is_required) && (
                  <div>
                    <p className="text-sm font-semibold text-gray-700">
                      {offerings.some((o) => o.is_required)
                        ? tr("care.additional")
                        : tr("care.offers")}
                      {careOfferingSelectionMode !== "optional" && (
                        <span className="ml-1 text-[#FF3130]">*</span>
                      )}
                    </p>
                    <p className="mt-1 text-xs leading-5 text-gray-500">
                      {careOfferingSelectionMode === "exactly_one"
                        ? tr("care.exactlyOneHint")
                        : careOfferingSelectionMode === "at_least_one"
                          ? tr("care.atLeastOneHint")
                          : tr("care.optionalHint")}{" "}
                      {tr("care.reviewHint")}
                    </p>
                    <div
                      aria-invalid={childOfferingErrors[i] ? true : undefined}
                      className={`mt-2 space-y-2 ${
                        childOfferingErrors[i]
                          ? "rounded-md border border-[#FF3130] bg-[#FF3130]/5 p-2"
                          : ""
                      }`}
                    >
                      {groupOfferings(
                        offerings.filter((o) => !o.is_required),
                      ).map((bucket) =>
                        bucket.kind === "single" ? (
                          <OfferingCard
                            key={bucket.offering.id}
                            offering={bucket.offering}
                            childIndex={i}
                            checked={child.offering_ids.has(bucket.offering.id)}
                            selectedDays={
                              child.offering_days[bucket.offering.id]
                            }
                            dayError={
                              offeringDayErrors[`${i}_${bucket.offering.id}`] ??
                              false
                            }
                            onToggle={() => {
                              toggleOffering(i, bucket.offering.id);
                              if (childOfferingErrors[i]) {
                                setChildOfferingErrors((prev) => {
                                  const next = { ...prev };
                                  delete next[i];
                                  return next;
                                });
                              }
                            }}
                            onToggleDay={(day) =>
                              toggleOfferingDay(i, bucket.offering.id, day)
                            }
                            tr={tr}
                          />
                        ) : (
                          <div
                            key={`group-${bucket.group}`}
                            className="rounded-lg border border-gray-200 bg-gray-50/60 p-2"
                          >
                            <div className="mb-2 flex flex-wrap items-center gap-2 px-1">
                              <span className="text-xs font-semibold text-gray-800">
                                {bucket.group}
                              </span>
                              {offeringRuleHint(bucket.rule, tr) && (
                                <span className="rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-[11px] font-medium text-[#3D63B0]">
                                  {offeringRuleHint(bucket.rule, tr)}
                                </span>
                              )}
                            </div>
                            <div className="space-y-2">
                              {bucket.offerings.map((o) => (
                                <OfferingCard
                                  key={o.id}
                                  offering={o}
                                  childIndex={i}
                                  checked={child.offering_ids.has(o.id)}
                                  selectedDays={child.offering_days[o.id]}
                                  dayError={
                                    offeringDayErrors[`${i}_${o.id}`] ?? false
                                  }
                                  onToggle={() => {
                                    toggleOffering(i, o.id);
                                    if (childOfferingErrors[i]) {
                                      setChildOfferingErrors((prev) => {
                                        const next = { ...prev };
                                        delete next[i];
                                        return next;
                                      });
                                    }
                                  }}
                                  onToggleDay={(day) =>
                                    toggleOfferingDay(i, o.id, day)
                                  }
                                  tr={tr}
                                />
                              ))}
                            </div>
                          </div>
                        ),
                      )}
                    </div>
                    {childOfferingErrors[i] && (
                      <p className="mt-1 text-xs text-[#FF3130]">
                        {careOfferingSelectionMode === "exactly_one"
                          ? tr("care.chooseExactlyOne")
                          : tr("care.chooseAtLeastOne")}
                      </p>
                    )}
                  </div>
                )}
              </div>
            )}

            {(schema?.fields ?? [])
              .filter(
                (f) =>
                  f.applies_to_child &&
                  isFieldVisible(f, childConditionCtx(child)),
              )
              .map((f) =>
                f.type === "information" ? (
                  <InfoBlock key={f.key} field={f} />
                ) : (
                  <CustomFieldInput
                    key={f.key}
                    field={f}
                    value={child.custom[f.key]}
                    onChange={(v) =>
                      updateChild(i, {
                        custom: { ...child.custom, [f.key]: v },
                      })
                    }
                    error={fieldErrors[`children_${i}_custom_${f.key}`]}
                    relevantDays={relevantCareDaysForChild(
                      child,
                      offerings,
                      previewMode,
                    )}
                    tr={tr}
                  />
                ),
              )}
          </div>
        ))}
      </section>

      {legalBlocks.length > 0 && (
        <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
          <SectionHeading
            kicker={tr("sections.consentKicker")}
            title={tr("sections.consentTitle")}
            description={tr("sections.consentDescription")}
          />
          {localizedCopy && activeLocale !== DEFAULT_LOCALE ? (
            <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-600">
              {tr("legalGermanNotice")}
            </p>
          ) : null}
          {legalBlocks.map((block) =>
            block.kind === "notice" ? (
              <EmailContactNotice
                key={block.key}
                block={block}
                onViewLegal={
                  block.text ? () => setOpenLegalDoc(block) : undefined
                }
                detailsLabel={tr("actions.details")}
                detailsAriaLabel={tr("actions.detailsAria", {
                  title: block.title,
                })}
              />
            ) : (
              <Consent
                key={block.key}
                name={`consent_${block.key}`}
                label={block.label}
                checked={legalConsents[block.key] === true}
                onChange={(checked) =>
                  setLegalConsents((prev) => ({
                    ...prev,
                    [block.key]: checked,
                  }))
                }
                required={block.required}
                error={fieldErrors[`consent_${block.key}`]}
                legalText={block.text}
                legalTitle={block.title}
                onViewLegal={
                  block.text ? () => setOpenLegalDoc(block) : undefined
                }
                tr={tr}
              />
            ),
          )}
        </section>
      )}

      <Modal
        isOpen={openLegalDoc !== null}
        onClose={() => setOpenLegalDoc(null)}
        title={openLegalDoc?.title ?? ""}
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      >
        <div className="max-h-[60vh] overflow-y-auto text-sm text-gray-700">
          <LegalMarkdown text={openLegalDoc?.text ?? ""} />
        </div>
      </Modal>

      {/*
        Cloudflare Turnstile widget. Only rendered when:
        - the caller didn't skip captcha (parent JWT path skips), and
        - the tenant has configured enrollment.captcha_site_key.
        When require_captcha=true but no site_key is set, we still
        render a warning to the parent and disable submit; failing
        closed is safer than letting bots through silently.
        The hidden input ships the token along with the form post.
      */}
      {!skipCaptcha && captchaConfig?.site_key && (
        <section aria-labelledby="captcha-heading" className="space-y-2">
          <h2 id="captcha-heading" className="sr-only">
            Sicherheitsprüfung
          </h2>
          <Turnstile
            siteKey={captchaConfig.site_key}
            options={{ theme: "light" }}
            onSuccess={(token) => setCaptchaToken(token)}
            onExpire={() => setCaptchaToken("")}
            onError={() => setCaptchaToken("")}
          />
          <input type="hidden" name="captcha_token" value={captchaToken} />
        </section>
      )}
      {!skipCaptcha && captchaConfig?.enabled && !captchaConfig.site_key && (
        <div
          role="alert"
          className="rounded-md border border-[#FF3130]/30 bg-[#FF3130]/5 p-3 text-sm text-[#CC2626]"
        >
          {tr("captchaMisconfigured")}
        </div>
      )}

      <button
        type="submit"
        disabled={
          submitting ||
          previewMode ||
          (!skipCaptcha && !!captchaConfig?.site_key && !captchaToken) ||
          (!skipCaptcha && !!captchaConfig?.enabled && !captchaConfig.site_key)
        }
        className="h-11 w-full rounded-lg bg-gray-900 text-sm font-semibold text-white shadow-sm transition-colors duration-200 hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-400"
      >
        {previewMode
          ? tr("actions.previewSubmit")
          : submitting
            ? tr("actions.submitting")
            : (submitLabel ?? tr("actions.submit"))}
      </button>
    </form>
  );
}

function blankChild(requiredOfferingIDs: readonly string[] = []): ChildDraft {
  return {
    first_name: "",
    last_name: "",
    date_of_birth: "",
    target_grade_level: "",
    // Required offerings are pre-selected and locked; seed them so a new
    // child slot starts compliant.
    offering_ids: new Set(requiredOfferingIDs),
    offering_days: {},
    custom: {},
  };
}

function draftChildren(
  draft: EnrollmentEditDraft,
  requiredOfferingIDs: readonly string[] = [],
): ChildDraft[] {
  if (draft.children.length === 0) return [blankChild(requiredOfferingIDs)];
  return draft.children.map((child) => {
    const offeringIDs = new Set<string>([
      ...requiredOfferingIDs,
      ...(child.offering_ids ?? []),
    ]);
    const offeringDays: Record<string, Set<string>> = {};
    for (const row of child.offering_days ?? []) {
      offeringDays[row.offering_id] = new Set(row.selected_days);
    }
    return {
      first_name: child.first_name,
      last_name: child.last_name,
      date_of_birth: child.date_of_birth,
      target_grade_level: child.target_grade_level?.toString() ?? "",
      offering_ids: offeringIDs,
      offering_days: offeringDays,
      custom: child.custom_data ?? {},
    };
  });
}

function draftGuardians(draft?: EnrollmentEditDraft): GuardianDraft[] {
  return (
    draft?.additional_guardians?.map((guardian) => ({
      first_name: guardian.first_name,
      last_name: guardian.last_name,
      email: guardian.email ?? "",
      phone: guardian.phone ?? "",
    })) ?? []
  );
}

function draftConsentFlags(
  draft?: EnrollmentEditDraft,
): Record<string, boolean> {
  const out: Record<string, boolean> = {};
  for (const [key, value] of Object.entries(draft?.consent_flags ?? {})) {
    out[key] = value === true;
  }
  return out;
}

const GERMAN_ENROLLMENT_FORM_MESSAGES = deMessages.enrollmentForm as Record<
  string,
  unknown
>;

function makeEnrollmentFormTranslator(
  intl: ReturnType<typeof useTranslations<"enrollmentForm">>,
  localizedCopy: boolean,
): EnrollmentFormTranslator {
  if (localizedCopy) {
    return Object.assign(
      (key: string, values?: TranslationValues) => intl(key, values),
      { raw: (key: string) => intl.raw(key) as unknown },
    );
  }
  return Object.assign(
    (key: string, values?: TranslationValues) => {
      const value = readGermanMessage(key);
      if (typeof value !== "string") return key;
      return interpolateMessage(value, values);
    },
    { raw: (key: string) => readGermanMessage(key) },
  );
}

function readGermanMessage(key: string): unknown {
  return key.split(".").reduce<unknown>((current, part) => {
    if (!current || typeof current !== "object") return undefined;
    return (current as Record<string, unknown>)[part];
  }, GERMAN_ENROLLMENT_FORM_MESSAGES);
}

function interpolateMessage(value: string, values?: TranslationValues): string {
  if (!values) return value;
  return Object.entries(values).reduce(
    (current, [key, replacement]) =>
      current.replaceAll(`{${key}}`, String(replacement)),
    value,
  );
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string")
    ? value
    : [];
}

function asStringMap(value: unknown): Record<string, string> {
  if (!value || typeof value !== "object" || Array.isArray(value)) return {};
  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).filter(
      (entry): entry is [string, string] => typeof entry[1] === "string",
    ),
  );
}

// seedRequiredOfferings adds every mandatory offering id to each existing
// child slot, leaving the rest of the draft untouched. Used after offerings
// load to back-fill the initial blank child created before the fetch resolved.
function seedRequiredOfferings(
  children: ChildDraft[],
  requiredIDs: readonly string[],
): ChildDraft[] {
  return children.map((c) => {
    const nextIDs = new Set(c.offering_ids);
    requiredIDs.forEach((id) => nextIDs.add(id));
    return { ...c, offering_ids: nextIDs };
  });
}

// ---- Care-offering selection groups + rules -----------------------------

type OfferingBucket =
  | { kind: "single"; offering: PublicCareOffering }
  | {
      kind: "group";
      group: string;
      rule: string;
      offerings: PublicCareOffering[];
    };

// Buckets offerings for display: ungrouped offerings render individually (in
// order); offerings sharing a non-empty selection_group collapse into one
// bucket anchored at the group's first member. Mirrors the backend grouping
// in services/enrollment/care_offering_rules.go.
function groupOfferings(offerings: PublicCareOffering[]): OfferingBucket[] {
  const buckets: OfferingBucket[] = [];
  const indexByGroup = new Map<string, number>();
  for (const o of offerings) {
    const group = o.selection_group?.trim() ?? "";
    if (group === "") {
      buckets.push({ kind: "single", offering: o });
      continue;
    }
    const existing = indexByGroup.get(group);
    if (existing === undefined) {
      indexByGroup.set(group, buckets.length);
      buckets.push({
        kind: "group",
        group,
        rule: o.selection_rule ?? "optional",
        offerings: [o],
      });
    } else {
      const bucket = buckets[existing];
      if (bucket && bucket.kind === "group") bucket.offerings.push(o);
    }
  }
  return buckets;
}

// offeringGroupRuleError validates one child's selection against every
// selection_group's rule. Returns a German error message, or null when the
// selection is valid. The backend re-checks the same in
// validateOfferingGroupRules (defense-in-depth).
function offeringGroupRuleError(
  child: ChildDraft,
  offerings: PublicCareOffering[],
  tr: TranslationFn,
): string | null {
  const ruleByGroup = new Map<string, string>();
  for (const o of offerings) {
    const group = o.selection_group?.trim() ?? "";
    const rule = o.selection_rule ?? "optional";
    if (group === "" || rule === "optional") continue;
    ruleByGroup.set(group, rule);
  }
  for (const [group, rule] of ruleByGroup) {
    const count = offerings.filter(
      (o) =>
        (o.selection_group?.trim() ?? "") === group &&
        child.offering_ids.has(o.id),
    ).length;
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

function offeringRuleHint(rule: string, tr: TranslationFn): string {
  if (rule === "exactly_one") return tr("care.ruleExactlyOne");
  if (rule === "at_least_one") return tr("care.ruleAtLeastOne");
  if (rule === "at_most_one") return tr("care.ruleAtMostOne");
  return "";
}

/**
 * Read-only information block (FormFieldInfo). Renders an optional heading +
 * plain-text body to parents; collects no answer. Newlines in the body are
 * preserved via `whitespace-pre-line`. Content is plain text by design (no
 * HTML/markdown), so it is safe to render directly.
 */
function InfoBlock({
  field,
}: {
  readonly field: PublicFormSchema["fields"][number];
}) {
  return (
    <div className="rounded-lg border border-[#5080D8]/20 bg-[#5080D8]/5 p-3">
      {field.label && (
        <p className="text-sm font-semibold text-gray-900">{field.label}</p>
      )}
      {field.content && (
        <p className="mt-1 text-sm leading-6 whitespace-pre-line text-gray-600">
          {field.content}
        </p>
      )}
    </div>
  );
}

// OfferingCard renders one care-offering row. Required offerings show as a
// locked "always included" card (gray, lock indicator) so they never read as
// a parent's pick; choosable offerings keep the interactive green-check
// checkbox. The day picker stays available for parent_choice offerings in
// both cases.
function OfferingCard({
  offering,
  childIndex,
  checked,
  selectedDays,
  dayError,
  onToggle,
  onToggleDay,
  tr,
}: {
  readonly offering: PublicCareOffering;
  readonly childIndex: number;
  readonly checked: boolean;
  readonly selectedDays: Set<string> | undefined;
  readonly dayError: boolean;
  readonly onToggle: () => void;
  readonly onToggleDay: (day: string) => void;
  readonly tr: EnrollmentFormTranslator;
}) {
  const required = offering.is_required;
  const effectiveChecked = required || checked;
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  let stateClass = "border-gray-200 bg-white hover:border-gray-300";
  if (dayError) {
    stateClass = "border-[#FF3130] bg-[#FF3130]/5";
  } else if (required) {
    stateClass = "border-gray-200 bg-gray-50";
  } else if (checked) {
    stateClass = "border-[#83CD2D]/40 bg-[#83CD2D]/10";
  }
  return (
    <label
      aria-invalid={dayError ? true : undefined}
      className={`flex items-start gap-3 rounded-lg border p-3 text-sm transition-colors ${
        required ? "cursor-default" : "cursor-pointer"
      } ${stateClass}`}
    >
      <span className="relative mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-gray-300 bg-white">
        <input
          name={`children_${childIndex}_offering_${offering.id}`}
          type="checkbox"
          checked={effectiveChecked}
          disabled={required}
          onChange={onToggle}
          className={`absolute inset-0 opacity-0 ${
            required ? "cursor-default" : "cursor-pointer"
          }`}
        />
        {required ? (
          <span className="flex h-5 w-5 items-center justify-center rounded-md bg-gray-400 text-white">
            <Lock className="h-3 w-3" />
          </span>
        ) : (
          checked && (
            <span className="flex h-5 w-5 items-center justify-center rounded-md bg-[#83CD2D] text-white">
              <Check className="h-3.5 w-3.5" />
            </span>
          )
        )}
      </span>
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-medium break-words text-gray-900">
            {offering.name}
          </span>
          {required && (
            <span className="rounded-full bg-gray-200 px-2 py-0.5 text-xs font-medium text-gray-600">
              {tr("care.requiredBadge")}
            </span>
          )}
        </div>
        {offering.description && (
          <div className="text-xs break-words text-gray-600">
            {offering.description}
          </div>
        )}
        <div className="mt-1 text-xs text-gray-500">
          {offering.days_of_week_mode === "parent_choice"
            ? tr("care.selectableDays")
            : tr("care.days")}
          {offering.available_days.map((d) => weekdayLabels[d] ?? d).join(", ")}
          {offering.includes_holiday_care && ` · ${tr("care.holiday")}`}
          {offering.includes_lunch && ` · ${tr("care.lunch")}`}
        </div>
        {effectiveChecked && offering.days_of_week_mode === "parent_choice" && (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {offering.available_days.map((day) => {
              const picked = selectedDays?.has(day) ?? false;
              return (
                <button
                  key={day}
                  type="button"
                  onClick={(e) => {
                    // The card's outer <label> would otherwise treat this as a
                    // click on the offering checkbox itself.
                    e.preventDefault();
                    e.stopPropagation();
                    onToggleDay(day);
                  }}
                  className={`rounded-md border px-2 py-0.5 text-xs font-medium transition-colors ${
                    picked
                      ? "border-[#83CD2D] bg-[#83CD2D] text-white"
                      : "border-gray-300 bg-white text-gray-700 hover:border-gray-400"
                  }`}
                  aria-pressed={picked}
                >
                  {weekdayLabels[day] ?? day}
                </button>
              );
            })}
          </div>
        )}
        {dayError && (
          <p className="mt-1 text-xs text-[#FF3130]">
            {tr("errors.dayInline")}
          </p>
        )}
      </div>
    </label>
  );
}

function Input({
  label,
  name,
  value,
  onChange,
  type = "text",
  required = false,
  autoComplete,
  inputMode,
  readOnly = false,
  error,
}: {
  readonly label: string;
  readonly name: string;
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly type?: string;
  readonly required?: boolean;
  readonly autoComplete?: string;
  readonly inputMode?: React.HTMLAttributes<HTMLInputElement>["inputMode"];
  readonly readOnly?: boolean;
  readonly error?: string;
}) {
  const id = `enrollment-${name}`;
  return (
    <label className="block">
      <span className="block text-sm font-semibold text-gray-700">{label}</span>
      <input
        id={id}
        name={name}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        aria-required={required}
        aria-invalid={error ? true : undefined}
        autoComplete={autoComplete}
        inputMode={inputMode}
        readOnly={readOnly}
        spellCheck={type === "email" ? false : undefined}
        className={`mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
          error
            ? "border-[#FF3130] bg-[#FF3130]/5"
            : readOnly
              ? "border-gray-200 bg-gray-50 text-gray-500"
              : "moto-content-surface hover:border-gray-300"
        }`}
      />
      {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
    </label>
  );
}

function GradeLevelSelect({
  id,
  value,
  onChange,
  max,
  error,
  tr,
}: {
  readonly id: string;
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly max: number;
  readonly error?: string;
  readonly tr: EnrollmentFormTranslator;
}) {
  return (
    <label className="block" htmlFor={id}>
      <span className="block text-sm font-semibold text-gray-700">
        {tr("fields.gradeLevel")}
      </span>
      <CustomSelect
        id={id}
        value={value}
        required
        onChange={onChange}
        invalid={Boolean(error)}
        className="mt-1"
        options={[
          { value: "", label: tr("fields.choose"), disabled: true },
          ...Array.from({ length: max }, (_, n) => {
            const grade = String(n + 1);
            return {
              value: grade,
              label: tr("fields.grade", { grade: n + 1 }),
            };
          }),
        ]}
      />
      {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
    </label>
  );
}

function SectionHeading({
  kicker,
  title,
  description,
}: {
  readonly kicker: string;
  readonly title: string;
  readonly description: string;
}) {
  return (
    <div>
      <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
        {kicker}
      </p>
      <h2 className="mt-1 text-lg font-semibold text-gray-900">{title}</h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
    </div>
  );
}

// Explicit element styling for the legal-document Markdown. Defined at
// module scope (not inline in the `components` prop) so the renderers
// keep a stable identity across re-renders. Tailwind v4 + Preflight
// strips default heading/list styling, and the project has no
// @tailwindcss/typography plugin, so each tag is styled by hand.
// Links open in a new tab; react-markdown does not emit raw HTML by
// default, so authored Markdown stays XSS-safe.
//
// Every renderer strips react-markdown's internal `node` prop before
// spreading the rest onto the DOM element — forwarding `node` produces
// an invalid HTML attribute and a React warning.
const LEGAL_MARKDOWN_COMPONENTS: Components = {
  h1: ({ node: _node, children, ...props }) => (
    <h1 className="mt-4 mb-2 text-lg font-bold text-gray-900" {...props}>
      {children}
    </h1>
  ),
  h2: ({ node: _node, children, ...props }) => (
    <h2 className="mt-4 mb-2 text-base font-bold text-gray-900" {...props}>
      {children}
    </h2>
  ),
  h3: ({ node: _node, children, ...props }) => (
    <h3 className="mt-3 mb-1.5 text-sm font-bold text-gray-900" {...props}>
      {children}
    </h3>
  ),
  p: ({ node: _node, children, ...props }) => (
    <p className="mb-3 leading-6" {...props}>
      {children}
    </p>
  ),
  ul: ({ node: _node, children, ...props }) => (
    <ul className="mb-3 list-disc space-y-1 pl-5" {...props}>
      {children}
    </ul>
  ),
  ol: ({ node: _node, children, ...props }) => (
    <ol className="mb-3 list-decimal space-y-1 pl-5" {...props}>
      {children}
    </ol>
  ),
  li: ({ node: _node, children, ...props }) => (
    <li className="leading-6" {...props}>
      {children}
    </li>
  ),
  a: ({ node: _node, children, ...props }) => (
    <a
      className="font-medium text-[#5080D8] underline underline-offset-2 hover:text-[#3F66AE]"
      target="_blank"
      rel="noopener noreferrer"
      {...props}
    >
      {children}
    </a>
  ),
  strong: ({ node: _node, children, ...props }) => (
    <strong className="font-semibold text-gray-900" {...props}>
      {children}
    </strong>
  ),
  em: ({ node: _node, children, ...props }) => (
    <em className="italic" {...props}>
      {children}
    </em>
  ),
};

function LegalMarkdown({ text }: { readonly text: string }) {
  return (
    <ReactMarkdown components={LEGAL_MARKDOWN_COMPONENTS}>{text}</ReactMarkdown>
  );
}

/**
 * Resolves the legal blocks an admin preview shows. Template-owned blocks
 * win; base previews and templates without enabled blocks fall back to the
 * tenant-wide legal settings — the same resolution the live form uses, so
 * the preview can never omit blocks the live form will require.
 */
async function resolvePreviewLegalTexts(
  tenantSlug: string,
  schema: PublicFormSchema | null,
): Promise<PublicLegalTexts> {
  const fromTemplate = previewLegalTexts(schema);
  if (fromTemplate.blocks.length > 0) return fromTemplate;
  return fetchPublicLegalTexts(tenantSlug);
}

function previewLegalTexts(schema: PublicFormSchema | null): PublicLegalTexts {
  const blocks =
    schema?.legal_blocks
      ?.filter((block) => block.enabled)
      .map((block) => ({
        key: block.key,
        kind: block.kind,
        title: block.title,
        label: block.label,
        text: block.text,
        required: block.required,
        sort_order: block.sort_order,
        source: block.source,
      }))
      .sort((a, b) => a.sort_order - b.sort_order) ?? [];
  return {
    agb: blocks.find((block) => block.key === "agb")?.text ?? "",
    agb_document_url: "",
    dsgvo: blocks.find((block) => block.key === "data_processing")?.text ?? "",
    email_contact:
      blocks.find((block) => block.key === "email_contact")?.text ?? "",
    photo: blocks.find((block) => block.key === "photo")?.text ?? "",
    terms_enabled: blocks.some((block) => block.key === "agb"),
    dsgvo_enabled: blocks.some((block) => block.key === "data_processing"),
    email_contact_enabled: blocks.some(
      (block) => block.key === "email_contact",
    ),
    photo_enabled: blocks.some((block) => block.key === "photo"),
    blocks,
  };
}

function Consent({
  name,
  label,
  checked,
  onChange,
  required = false,
  error,
  legalText,
  legalTitle,
  onViewLegal,
  tr,
}: {
  readonly name: string;
  readonly label: string;
  readonly checked: boolean;
  readonly onChange: (v: boolean) => void;
  readonly required?: boolean;
  readonly error?: string;
  /**
   * Optional per-tenant legal document (Markdown). When non-empty, the
   * consent label gains an "anzeigen" link that opens onViewLegal. When
   * empty/undefined the label renders plain — tenants that haven't
   * configured a document still get a working form.
   */
  readonly legalText?: string;
  readonly legalTitle?: string;
  readonly onViewLegal?: () => void;
  readonly tr: TranslationFn;
}) {
  const hasLink = Boolean(legalText && legalText.trim() && onViewLegal);
  return (
    <div>
      <label
        className={`flex cursor-pointer items-center gap-3 rounded-lg border p-3 text-sm transition-colors ${
          checked
            ? "border-[#83CD2D]/40 bg-[#83CD2D]/10"
            : error
              ? "border-[#FF3130] bg-[#FF3130]/5"
              : "border-gray-200 bg-white hover:border-gray-300"
        }`}
      >
        <span className="relative flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-gray-300 bg-white">
          <input
            name={name}
            type="checkbox"
            checked={checked}
            onChange={(e) => onChange(e.target.checked)}
            aria-required={required}
            aria-invalid={error ? true : undefined}
            className="absolute inset-0 cursor-pointer opacity-0"
          />
          {checked && (
            <span className="flex h-5 w-5 items-center justify-center rounded-md bg-[#83CD2D] text-white">
              <Check className="h-3.5 w-3.5" />
            </span>
          )}
        </span>
        <span className="min-w-0 flex-1 leading-6 font-medium text-gray-700">
          {label} {required && <span className="text-[#FF3130]">*</span>}
        </span>
        {hasLink && (
          <LegalDetailsButton
            title={legalTitle ?? label}
            label={tr("actions.details")}
            ariaLabel={tr("actions.detailsAria", {
              title: legalTitle ?? label,
            })}
            onClick={onViewLegal}
            preventLabelToggle
          />
        )}
      </label>
      {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
    </div>
  );
}

/**
 * Informational notice for the e-mail contact. Unlike the consent rows
 * this carries NO checkbox: the school needs the e-mail address to run
 * the enrollment (Rückfragen, Status-Link), which is contract
 * performance, not a consent the parent could decline and still enrol.
 * When the tenant configured an explanatory text it gains a localized
 * "show more" link that opens it in the shared legal modal.
 */
function EmailContactNotice({
  block,
  onViewLegal,
  detailsLabel,
  detailsAriaLabel,
}: {
  readonly block: PublicLegalBlock;
  readonly onViewLegal?: () => void;
  readonly detailsLabel: string;
  readonly detailsAriaLabel: string;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border border-[#5080D8]/25 bg-[#5080D8]/5 p-3 text-sm leading-6 text-gray-700">
      <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-[#5080D8]/30 bg-white text-[#5080D8]">
        <Info className="h-3.5 w-3.5" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="mb-1 flex flex-wrap items-center gap-2">
          <span className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Hinweis
          </span>
        </div>
        <span className="font-medium">{block.label}</span>{" "}
        {onViewLegal && (
          <LegalDetailsButton
            title={block.title}
            label={detailsLabel}
            ariaLabel={detailsAriaLabel}
            onClick={onViewLegal}
          />
        )}
      </div>
    </div>
  );
}

function LegalDetailsButton({
  title,
  label,
  ariaLabel,
  onClick,
  preventLabelToggle = false,
}: {
  readonly title: string;
  readonly label: string;
  readonly ariaLabel: string;
  readonly onClick?: () => void;
  readonly preventLabelToggle?: boolean;
}) {
  return (
    <button
      type="button"
      aria-label={ariaLabel}
      title={title}
      onClick={(event) => {
        if (preventLabelToggle) {
          event.preventDefault();
          event.stopPropagation();
        }
        onClick?.();
      }}
      className="inline-flex h-7 shrink-0 items-center justify-center gap-1.5 rounded-md px-2 text-xs font-semibold text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <FileText className="h-3.5 w-3.5" aria-hidden="true" />
      <span>{label}</span>
    </button>
  );
}

function customValueMissing(
  field: PublicFormSchema["fields"][number],
  value: unknown,
  relevantDays?: readonly string[],
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
  if (field.type === "weekday_schedule") {
    const schedule = asScheduleObject(value);
    return !Object.values(schedule).some((time) => time.trim() !== "");
  }
  if (field.type === "weekday_boolean") {
    // Pickup: an explicit (touched) selection — even an empty one — is the
    // valid "Geht alleine nach Hause" answer. Only a never-touched field
    // (no value object was ever produced) counts as missing, so a required
    // Abholregelung forces the parent to engage with it once. Other
    // weekday_boolean fields (e.g. Buskind) still need at least one day.
    if (field.target === "student.pickup_status") {
      return value === undefined || value === null || typeof value !== "object";
    }
    const days = asWeekdayBooleanObject(value);
    return !Object.values(days).some(Boolean);
  }
  if (field.type === "weekday_mode") {
    // Like pickup, an all-alone (empty) plan is a valid answer; only a
    // never-touched field counts as missing for a required Geh-/Abholregelung.
    return value === undefined || value === null || typeof value !== "object";
  }
  if (field.type === "weekday_multi_mode") {
    const days = relevantDays ?? WEEKDAYS;
    if (days.length === 0) return false;
    const modes = asWeekdayMultiModeObject(value, days);
    return days.some((day) => (modes[day]?.length ?? 0) === 0);
  }
  return typeof value !== "string" || value.trim() === "";
}

function phoneListValueMissing(value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return true;
  return value.some((row) => {
    if (!row || typeof row !== "object") return true;
    const phoneNumber = (row as Partial<PhoneEntry>).phone_number;
    return typeof phoneNumber !== "string" || phoneNumber.trim() === "";
  });
}

function contactListValueMissing(value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return true;
  return value.some((row) => {
    if (!row || typeof row !== "object") return true;
    const contact = row as ContactEntryValue;
    const hasName =
      typeof contact.first_name === "string" &&
      contact.first_name.trim() !== "" &&
      typeof contact.last_name === "string" &&
      contact.last_name.trim() !== "";
    const hasEmail = (contact.email ?? "").trim() !== "";
    const hasPhone =
      Array.isArray(contact.phone_numbers) &&
      contact.phone_numbers.some((phone) => phone.phone_number.trim() !== "");
    return !hasName || (!hasEmail && !hasPhone);
  });
}

function requiredMessageForField(
  field: PublicFormSchema["fields"][number],
  tr: TranslationFn,
): string {
  if (field.type === "boolean") {
    return tr("errors.yesNo");
  }
  if (field.type === "phone_list") {
    return tr("errors.phoneList");
  }
  if (field.type === "contact_list") {
    return tr("errors.contactList");
  }
  if (field.type === "weekday_schedule") {
    return tr("errors.schedule");
  }
  if (field.type === "weekday_boolean") {
    // Pickup accepts an empty selection ("geht alleine nach Hause"), so the
    // required check only asks the parent to confirm the field — not to pick a
    // day. Every other weekday_boolean (Buskind) genuinely needs a day.
    if (field.target === "student.pickup_status") {
      return tr("errors.pickupConfirm");
    }
    return tr("errors.weekdayRequired");
  }
  if (field.type === "weekday_mode") {
    return tr("errors.weekdayModeConfirm");
  }
  if (field.type === "weekday_multi_mode") {
    return tr("errors.weekdayMultiModeRequired");
  }
  if (field.type === "select") {
    return tr("errors.select");
  }
  return tr("errors.required");
}

interface CustomFieldInputProps {
  readonly field: PublicFormSchema["fields"][number];
  readonly value: unknown;
  readonly onChange: (v: unknown) => void;
  readonly error?: string;
  readonly tr: EnrollmentFormTranslator;
  readonly relevantDays?: readonly string[];
}

function CustomFieldInput({
  field,
  value,
  onChange,
  error,
  tr,
  relevantDays,
}: CustomFieldInputProps) {
  const labelEl = (
    <>
      <span className="block text-sm font-semibold text-gray-700">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </span>
      {field.help_text && (
        <span className="mt-1 block text-xs leading-5 text-gray-500">
          {field.help_text}
        </span>
      )}
    </>
  );
  const valueStr = typeof value === "string" ? value : "";

  if (field.type === "boolean") {
    const selectedValue = value === true ? "yes" : value === false ? "no" : "";
    return (
      <fieldset aria-invalid={error ? "true" : undefined}>
        {labelEl}
        <div className="mt-2 grid grid-cols-2 gap-2">
          {[
            { label: tr("structured.yes"), value: "yes", raw: true },
            { label: tr("structured.no"), value: "no", raw: false },
          ].map((option) => (
            <label
              key={option.value}
              className={`flex h-10 cursor-pointer items-center justify-center rounded-lg border px-3 text-sm font-medium transition-colors ${
                selectedValue === option.value
                  ? "border-gray-900 bg-gray-900 text-white"
                  : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
              }`}
            >
              <input
                type="radio"
                name={field.key}
                value={option.value}
                checked={selectedValue === option.value}
                onChange={() => onChange(option.raw)}
                className="sr-only"
              />
              {option.label}
            </label>
          ))}
        </div>
        {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
      </fieldset>
    );
  }
  if (field.type === "textarea") {
    return (
      <label className="block">
        {labelEl}
        <textarea
          value={valueStr}
          onChange={(e) => onChange(e.target.value)}
          rows={3}
          name={field.key}
          className={`moto-content-surface mt-1 w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${error ? "border-[#FF3130]" : ""}`}
          aria-required={field.required}
          aria-invalid={error ? "true" : undefined}
        />
        {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
      </label>
    );
  }
  if (field.type === "select") {
    return (
      <label className="block">
        {labelEl}
        <CustomSelect
          value={valueStr}
          onChange={onChange}
          name={field.key}
          required={field.required}
          invalid={Boolean(error)}
          className="mt-1"
          options={[
            { value: "", label: tr("fields.choose") },
            ...(field.options ?? []).map((option) => ({
              value: option.value,
              label: option.label,
            })),
          ]}
        />
        {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
      </label>
    );
  }
  if (field.type === "phone_list") {
    return (
      <PhoneListInput
        field={field}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_schedule") {
    return (
      <WeekdayScheduleInput
        field={field}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_boolean") {
    return (
      <WeekdayBooleanInput
        field={field}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_mode") {
    return (
      <WeekdayModeInput
        field={field}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }
  if (field.type === "weekday_multi_mode") {
    return (
      <WeekdayMultiModeInput
        field={field}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
        relevantDays={relevantDays}
      />
    );
  }
  if (field.type === "contact_list") {
    return (
      <ContactListInput
        field={field}
        value={value}
        onChange={onChange}
        error={error}
        tr={tr}
      />
    );
  }

  const inputType =
    field.type === "number"
      ? "number"
      : field.type === "date"
        ? "date"
        : "text";
  return (
    <label className="block">
      {labelEl}
      <input
        name={field.key}
        type={inputType}
        value={valueStr}
        onChange={(e) => onChange(e.target.value)}
        aria-required={field.required}
        aria-invalid={error ? "true" : undefined}
        className={`moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${error ? "border-[#FF3130]" : ""}`}
      />
      {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
    </label>
  );
}

// ---- Structured field renderers ----------------------------------------
//
// The shapes here mirror the Go types in
// backend/models/enrollment/form_schema.go (PhoneEntry, WeekdaySchedule,
// ContactEntry). The decision service consumes whatever JSON these
// components emit, so the field names must match exactly.

interface PhoneEntry {
  phone_number: string;
  phone_type: "mobile" | "home" | "work" | "other";
  label?: string;
  is_primary?: boolean;
}

function asPhoneArray(v: unknown): PhoneEntry[] {
  if (!Array.isArray(v)) return [];
  return v.filter((row): row is PhoneEntry => {
    if (!row || typeof row !== "object") return false;
    const r = row as Record<string, unknown>;
    return typeof r.phone_number === "string";
  });
}

function PhoneListInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const phones = asPhoneArray(value);
  const phoneTypeLabels = asStringMap(tr.raw("phoneTypes"));
  const update = (next: PhoneEntry[]) => onChange(next);
  const setRow = (idx: number, patch: Partial<PhoneEntry>) =>
    update(phones.map((p, i) => (i === idx ? { ...p, ...patch } : p)));
  return (
    <fieldset
      className={`rounded-2xl border bg-gray-50/70 p-4 ${error ? "border-[#FF3130]" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      {phones.length === 0 && (
        <p className="mt-2 text-sm text-gray-500">
          {tr("structured.nonePhone")}
        </p>
      )}
      <ul className="space-y-2">
        {phones.map((p, idx) => (
          <li
            key={idx}
            className="moto-content-surface grid items-end gap-3 rounded-xl border p-3 shadow-sm md:grid-cols-[minmax(0,1fr)_180px_auto_auto]"
          >
            <label className="block">
              <span className="text-xs font-medium text-gray-700">
                {tr("structured.number")}
              </span>
              <input
                type="tel"
                value={p.phone_number}
                onChange={(e) => setRow(idx, { phone_number: e.target.value })}
                className="mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              />
            </label>
            <label className="block" htmlFor={`phone-type-${idx}`}>
              <span className="text-xs font-medium text-gray-700">
                {tr("structured.type")}
              </span>
              <CustomSelect
                id={`phone-type-${idx}`}
                value={p.phone_type}
                onChange={(value) =>
                  setRow(idx, {
                    phone_type: value as PhoneEntry["phone_type"],
                  })
                }
                ariaLabel={tr("structured.type")}
                className="mt-1 border-gray-200 bg-white"
                options={(
                  [
                    "mobile",
                    "home",
                    "work",
                    "other",
                  ] as PhoneEntry["phone_type"][]
                ).map((key) => ({
                  value: key,
                  label: phoneTypeLabels[key] ?? key,
                }))}
              />
            </label>
            <StructuredToggle
              checked={Boolean(p.is_primary)}
              onChange={() =>
                update(
                  phones.map((row, i) => ({
                    ...row,
                    is_primary: i === idx,
                  })),
                )
              }
              label={tr("structured.primary")}
              name={`${field.key}-primary`}
              type="radio"
            />
            <button
              type="button"
              onClick={() => update(phones.filter((_, i) => i !== idx))}
              className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[#FF3130]/20 bg-white text-[#CC2626] shadow-sm transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
              aria-label={tr("structured.removeNumber")}
            >
              <Trash2 className="h-4 w-4" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>
      <button
        type="button"
        onClick={() =>
          update([
            ...phones,
            { phone_number: "", phone_type: "mobile" } satisfies PhoneEntry,
          ])
        }
        className="mt-3 inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        {tr("structured.addPhone")}
      </button>
      {error && <p className="mt-2 text-xs text-[#FF3130]">{error}</p>}
    </fieldset>
  );
}

const WEEKDAYS = ["mon", "tue", "wed", "thu", "fri"] as const;

function asScheduleObject(v: unknown): Record<string, string> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, string> = {};
  for (const weekday of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[weekday];
    if (typeof raw === "string") out[weekday] = raw;
  }
  return out;
}

function asWeekdayBooleanObject(v: unknown): Record<string, boolean> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, boolean> = {};
  for (const w of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[w];
    if (typeof raw === "boolean") out[w] = raw;
  }
  return out;
}

type DepartureModeValue = "alone" | "bus" | "pickup";

const DEPARTURE_MULTI_MODE_OPTIONS: ReadonlyArray<DepartureModeValue> = [
  "alone",
  "bus",
  "pickup",
];

function asWeekdayModeObject(v: unknown): Record<string, DepartureModeValue> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, DepartureModeValue> = {};
  for (const w of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[w];
    if (raw === "bus" || raw === "pickup" || raw === "alone") {
      out[w] = raw;
    }
  }
  return out;
}

function asWeekdayMultiModeObject(
  v: unknown,
  allowedDays: readonly string[] = WEEKDAYS,
): Record<string, DepartureModeValue[]> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const allowed = new Set(allowedDays);
  const out: Record<string, DepartureModeValue[]> = {};
  for (const w of WEEKDAYS) {
    if (!allowed.has(w)) continue;
    const raw = (v as Record<string, unknown>)[w];
    if (!Array.isArray(raw)) continue;
    const seen = new Set<DepartureModeValue>();
    for (const item of raw) {
      if (item === "alone" || item === "bus" || item === "pickup") {
        seen.add(item);
      }
    }
    const modes = DEPARTURE_MULTI_MODE_OPTIONS.filter((mode) => seen.has(mode));
    if (modes.length > 0) out[w] = modes;
  }
  return out;
}

const DEPARTURE_MODE_OPTIONS: ReadonlyArray<DepartureModeValue> = [
  "alone",
  "bus",
  "pickup",
];

function selectedCareDaysForChild(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
): string[] {
  const days = new Set<string>();
  for (const id of child.offering_ids) {
    const offering = offerings.find((o) => o.id === id);
    if (!offering) continue;
    const selected =
      offering.days_of_week_mode === "parent_choice"
        ? Array.from(child.offering_days[id] ?? new Set<string>())
        : offering.available_days;
    for (const day of selected) {
      if ((WEEKDAYS as readonly string[]).includes(day)) {
        days.add(day);
      }
    }
  }
  return WEEKDAYS.filter((day) => days.has(day));
}

function relevantCareDaysForChild(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
  previewMode: boolean,
): string[] {
  if (previewMode && offerings.length === 0) {
    return [...WEEKDAYS];
  }
  return selectedCareDaysForChild(child, offerings);
}

function pruneWeekdayMultiModeAnswers(
  data: Record<string, unknown>,
  fields: readonly PublicFormSchema["fields"][number][],
  relevantDays: readonly string[],
): Record<string, unknown> {
  const next = { ...data };
  for (const field of fields) {
    if (field.type !== "weekday_multi_mode" || !(field.key in next)) {
      continue;
    }
    next[field.key] = asWeekdayMultiModeObject(next[field.key], relevantDays);
  }
  return next;
}

function WeekdayScheduleInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const sched = asScheduleObject(value);
  const weekdayLabels = asStringMap(tr.raw("weekdays"));
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-[#FF3130]" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">{tr("structured.emptySchedule")}</p>
      <div className="mt-2 grid gap-2 sm:grid-cols-5">
        {WEEKDAYS.map((weekday) => (
          <label key={weekday} className="block text-xs">
            <span className="block text-gray-600">
              {weekdayLabels[weekday] ?? weekday}
            </span>
            <input
              type="time"
              value={sched[weekday] ?? ""}
              onChange={(e) =>
                onChange({ ...sched, [weekday]: e.target.value })
              }
              className="mt-1 h-9 w-full rounded-md border border-gray-200 bg-white px-2 text-sm"
            />
          </label>
        ))}
      </div>
      {error && <p className="mt-2 text-xs text-[#FF3130]">{error}</p>}
    </fieldset>
  );
}

function WeekdayBooleanInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const days = asWeekdayBooleanObject(value);
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-[#FF3130]" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">
        {field.target === "student.pickup_status"
          ? tr("structured.pickupDaysHelp")
          : tr("structured.busDaysHelp")}
      </p>
      <div className="mt-2 grid gap-2 sm:grid-cols-5">
        {WEEKDAYS.map((w) => {
          const checked = Boolean(days[w]);
          return (
            <label
              key={w}
              className={`flex h-10 cursor-pointer items-center justify-center rounded-lg border px-2 text-xs font-medium transition-colors ${
                checked
                  ? "border-gray-900 bg-gray-900 text-white"
                  : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
              }`}
            >
              <input
                type="checkbox"
                checked={checked}
                onChange={(e) => onChange({ ...days, [w]: e.target.checked })}
                className="sr-only"
              />
              {weekdayLabels[w] ?? w}
            </label>
          );
        })}
      </div>
      {error && <p className="mt-2 text-xs text-[#FF3130]">{error}</p>}
    </fieldset>
  );
}

function WeekdayModeInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const modes = asWeekdayModeObject(value);
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  const departureModeLabels = asStringMap(tr.raw("structured.departureModes"));
  const modeFor = (key: string): DepartureModeValue => {
    const m = modes[key];
    return m === "bus" || m === "pickup" ? m : "alone";
  };
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-[#FF3130]" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">
        {tr("structured.departureModeHelp")}
      </p>
      <div className="mt-2 space-y-2">
        {WEEKDAYS.map((w) => {
          const current = modeFor(w);
          return (
            <div key={w} className="flex items-center gap-2">
              <span className="w-20 shrink-0 text-xs font-medium text-gray-600">
                {weekdayLabels[w] ?? w}
              </span>
              <div className="grid flex-1 grid-cols-3 gap-1">
                {DEPARTURE_MODE_OPTIONS.map((mode) => {
                  const active = current === mode;
                  return (
                    <button
                      key={mode}
                      type="button"
                      aria-pressed={active}
                      onClick={() => {
                        const next = { ...modes };
                        if (mode === "alone") delete next[w];
                        else next[w] = mode;
                        onChange(next);
                      }}
                      className={`flex h-9 items-center justify-center rounded-md border px-1 text-xs font-medium transition-colors ${
                        active
                          ? "border-gray-900 bg-gray-900 text-white"
                          : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                      }`}
                    >
                      {departureModeLabels[mode] ?? mode}
                    </button>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
      {error && <p className="mt-2 text-xs text-[#FF3130]">{error}</p>}
    </fieldset>
  );
}

function WeekdayMultiModeInput({
  field,
  value,
  onChange,
  error,
  tr,
  relevantDays,
}: CustomFieldInputProps) {
  const daysToRender = relevantDays ?? WEEKDAYS;
  const modes = asWeekdayMultiModeObject(value, daysToRender);
  const [expandedDays, setExpandedDays] = useState<Set<string>>(
    () => new Set(daysToRender.filter((day) => (modes[day]?.length ?? 0) > 0)),
  );
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  const departureModeLabels = asStringMap(tr.raw("structured.departureModes"));
  const activeDays = daysToRender.filter(
    (day) => expandedDays.has(day) || (modes[day]?.length ?? 0) > 0,
  );
  const toggleDay = (day: string) => {
    const isActive = activeDays.includes(day);
    const nextExpanded = new Set(expandedDays);
    if (isActive) {
      nextExpanded.delete(day);
      const nextRaw = { ...modes };
      delete nextRaw[day];
      onChange(asWeekdayMultiModeObject(nextRaw, daysToRender));
    } else {
      nextExpanded.add(day);
    }
    setExpandedDays(nextExpanded);
  };
  const updateDay = (day: string, mode: DepartureModeValue) => {
    const current = new Set(modes[day] ?? []);
    if (current.has(mode)) current.delete(mode);
    else current.add(mode);
    const nextRaw = {
      ...modes,
      [day]: DEPARTURE_MULTI_MODE_OPTIONS.filter((m) => current.has(m)),
    };
    onChange(asWeekdayMultiModeObject(nextRaw, daysToRender));
  };
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-[#FF3130]" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">
        {tr("structured.weekdayMultiModeHelp")}
      </p>
      <div className="mt-3">
        {daysToRender.length > 0 ? (
          <div className="grid grid-cols-5 gap-2">
            {daysToRender.map((day) => {
              const active = activeDays.includes(day);
              return (
                <button
                  key={day}
                  type="button"
                  aria-pressed={active}
                  onClick={() => toggleDay(day)}
                  className={`flex h-9 items-center justify-center rounded-lg border px-2 text-xs font-semibold transition-colors ${
                    active
                      ? "border-gray-900 bg-gray-900 text-white"
                      : "border-gray-200 bg-white text-gray-700 hover:border-gray-300 hover:bg-gray-50"
                  }`}
                >
                  {weekdayLabels[day] ?? day}
                </button>
              );
            })}
          </div>
        ) : (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600">
            {tr("structured.weekdayMultiModeNoDays")}
          </p>
        )}
        <div className="mt-3 space-y-2">
          {activeDays.map((day) => (
            <div
              key={day}
              className="rounded-lg border border-gray-200 bg-gray-50/70 p-3"
            >
              <div className="mb-2 text-xs font-semibold text-gray-700">
                {weekdayLabels[day] ?? day}
              </div>
              <div className="grid gap-2 sm:grid-cols-3">
                {DEPARTURE_MULTI_MODE_OPTIONS.map((mode) => {
                  const checked = modes[day]?.includes(mode) ?? false;
                  return (
                    <label
                      key={mode}
                      className={`flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border bg-white px-3 py-2 text-xs font-medium transition-colors ${
                        checked
                          ? "border-gray-900 text-gray-900"
                          : "border-gray-200 text-gray-700 hover:border-gray-300"
                      }`}
                    >
                      <Checkbox
                        checked={checked}
                        onChange={() => updateDay(day, mode)}
                      />
                      <span>{departureModeLabels[mode] ?? mode}</span>
                    </label>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </div>
      {error && <p className="mt-2 text-xs text-[#FF3130]">{error}</p>}
    </fieldset>
  );
}

interface ContactEntryValue {
  first_name: string;
  last_name: string;
  email?: string;
  relationship_type?: string;
  phone_numbers?: PhoneEntry[];
  can_pickup?: boolean;
  is_emergency_contact?: boolean;
  emergency_priority?: number;
}

function asContactArray(v: unknown): ContactEntryValue[] {
  if (!Array.isArray(v)) return [];
  return v.filter((row): row is ContactEntryValue => {
    if (!row || typeof row !== "object") return false;
    const r = row as Record<string, unknown>;
    return typeof r.first_name === "string" && typeof r.last_name === "string";
  });
}

function blankContact(): ContactEntryValue {
  return {
    first_name: "",
    last_name: "",
    relationship_type: "",
    phone_numbers: [],
    can_pickup: false,
    is_emergency_contact: false,
  };
}

function ContactListInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const contacts = asContactArray(value);
  const update = (next: ContactEntryValue[]) => onChange(next);
  const setRow = (idx: number, patch: Partial<ContactEntryValue>) =>
    update(contacts.map((c, i) => (i === idx ? { ...c, ...patch } : c)));
  return (
    <fieldset
      className={`rounded-2xl border bg-white p-4 shadow-sm ${error ? "border-[#FF3130]" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {field.label}
        {field.required && <span className="text-[#FF3130]"> *</span>}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      {contacts.length === 0 && (
        <p className="mt-2 text-sm text-gray-500">
          {tr("structured.noneContact")}
        </p>
      )}
      <ul className="space-y-3">
        {contacts.map((c, idx) => (
          <li
            key={idx}
            className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4"
          >
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  {tr("fields.firstNamePlain")}
                </span>
                <input
                  type="text"
                  value={c.first_name}
                  onChange={(e) => setRow(idx, { first_name: e.target.value })}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  {tr("fields.lastNamePlain")}
                </span>
                <input
                  type="text"
                  value={c.last_name}
                  onChange={(e) => setRow(idx, { last_name: e.target.value })}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  {tr("structured.relationship")}
                </span>
                <input
                  type="text"
                  value={c.relationship_type ?? ""}
                  onChange={(e) =>
                    setRow(idx, { relationship_type: e.target.value })
                  }
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-xs font-medium text-gray-700">
                  {tr("structured.optionalEmail")}
                </span>
                <input
                  type="email"
                  value={c.email ?? ""}
                  onChange={(e) => setRow(idx, { email: e.target.value })}
                  className="mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                />
              </label>
            </div>

            <div className="mt-3">
              <PhoneListInput
                field={{
                  ...field,
                  key: `${field.key}_${idx}_phones`,
                  label: tr("structured.phoneNumbers"),
                  type: "phone_list",
                  required: false,
                }}
                value={c.phone_numbers ?? []}
                onChange={(v) =>
                  setRow(idx, {
                    phone_numbers: Array.isArray(v) ? (v as PhoneEntry[]) : [],
                  })
                }
                tr={tr}
              />
            </div>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div className="grid gap-2 sm:grid-cols-2">
                <StructuredToggle
                  checked={Boolean(c.can_pickup)}
                  onChange={(checked) => setRow(idx, { can_pickup: checked })}
                  label={tr("structured.pickup")}
                />
                <StructuredToggle
                  checked={Boolean(c.is_emergency_contact)}
                  onChange={(checked) =>
                    setRow(idx, { is_emergency_contact: checked })
                  }
                  label={tr("structured.emergency")}
                />
              </div>
              <button
                type="button"
                onClick={() => update(contacts.filter((_, i) => i !== idx))}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-[#FF3130]/20 bg-white px-3 text-sm font-medium text-[#CC2626] shadow-sm transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
                {tr("structured.removeContact")}
              </button>
            </div>
          </li>
        ))}
      </ul>
      <button
        type="button"
        onClick={() => update([...contacts, blankContact()])}
        className="mt-3 inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        {tr("structured.addContact")}
      </button>
      {error && <p className="mt-2 text-xs text-[#FF3130]">{error}</p>}
    </fieldset>
  );
}

function StructuredToggle({
  checked,
  label,
  name,
  onChange,
  type = "checkbox",
}: Readonly<{
  checked: boolean;
  label: string;
  name?: string;
  onChange: (checked: boolean) => void;
  type?: "checkbox" | "radio";
}>) {
  return (
    <label
      className={`flex h-9 cursor-pointer items-center gap-2.5 rounded-lg border px-3 text-sm font-medium transition-colors ${
        checked
          ? "border-[#83CD2D]/40 bg-[#83CD2D]/10 text-gray-900"
          : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
      }`}
    >
      <input
        type={type}
        name={name}
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="sr-only"
      />
      <span
        className={`flex h-5 w-5 shrink-0 items-center justify-center border ${
          type === "radio" ? "rounded-full" : "rounded-md"
        } ${
          checked
            ? "border-[#83CD2D] bg-[#83CD2D] text-white"
            : "border-gray-300 bg-white"
        }`}
        aria-hidden="true"
      >
        {checked ? <Check className="h-3.5 w-3.5" /> : null}
      </span>
      {label}
    </label>
  );
}

interface ExistingChildrenPanelProps {
  readonly existing: MeProfileChild[];
  readonly usedIDs: Set<string>;
  readonly onAdopt: (child: MeProfileChild) => void;
  readonly tr: EnrollmentFormTranslator;
}

/**
 * Surfaced only when the parent is logged in and has linked students.
 * Each row offers a one-click "Übernehmen" that drops the child into
 * the next blank slot. Dates of birth aren't on users.students, so
 * the parent still types those. The panel just saves the names + the
 * grade-level guess from school_class.
 */
function ExistingChildrenPanel({
  existing,
  usedIDs,
  onAdopt,
  tr,
}: ExistingChildrenPanelProps) {
  return (
    <div className="rounded-lg border border-[#83CD2D]/40 bg-[#83CD2D]/5 p-4">
      <h3 className="text-sm font-semibold text-gray-900">
        {tr("structured.existingTitle")}
      </h3>
      <p className="mt-1 text-xs text-gray-600">
        {tr("structured.existingText")}
      </p>
      <ul className="mt-3 space-y-2">
        {existing.map((c) => {
          const used = usedIDs.has(c.id);
          return (
            <li
              key={c.id}
              className="moto-content-surface flex items-center justify-between gap-3 rounded-md border px-3 py-2"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-gray-900">
                  {c.first_name} {c.last_name}
                </p>
                <p className="truncate text-xs text-gray-500">
                  {tr("structured.class", {
                    value: c.school_class || tr("structured.notSet"),
                  })}
                </p>
              </div>
              <button
                type="button"
                onClick={() => onAdopt(c)}
                disabled={used}
                className="shrink-0 rounded-md bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {used ? tr("structured.adopted") : tr("structured.adopt")}
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}

// DateOfBirthPicker uses three dropdowns (Tag / Monat / Jahr).
// Calendar widgets are awkward for DOB because parents need to jump
// 5-10 years back; native <input type="date"> auto-validates partial
// year input and snaps back. Three explicit dropdowns let parents pick fast
// without touching a keyboard. Wire format stays YYYY-MM-DD.
function DateOfBirthPicker({
  value,
  onChange,
  error,
  tr,
}: {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly error?: string;
  readonly tr: EnrollmentFormTranslator;
}) {
  // Internal state so partial selections (only day picked, only month
  // picked, etc.) survive between renders. We sync from `value` when
  // it changes externally (e.g. "Übernehmen" prefill), then push the
  // full YYYY-MM-DD upward only when all three parts are filled.
  const [day, setDay] = useState("");
  const [month, setMonth] = useState("");
  const [year, setYear] = useState("");

  useEffect(() => {
    const parsed = parseDOBParts(value);
    if (parsed) {
      setDay(parsed.day);
      setMonth(parsed.month);
      setYear(parsed.year);
    } else if (value === "") {
      setDay("");
      setMonth("");
      setYear("");
    }
    // value is the only external driver; re-sync only on incoming changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const currentYear = new Date().getFullYear();
  const monthLabels = asStringArray(tr.raw("months"));
  const years: number[] = [];
  for (let y = currentYear; y >= currentYear - 25; y--) years.push(y);

  const daysInMonth =
    month && year ? new Date(Number(year), Number(month), 0).getDate() : 31;

  const emit = (d: string, m: string, y: string) => {
    if (!d || !m || !y) {
      // Don't blank the parent on partial selections. Wait until all
      // three are picked. The parent's `value` may already be empty;
      // that's fine.
      onChange("");
      return;
    }
    const dd = d.padStart(2, "0");
    const mm = m.padStart(2, "0");
    onChange(`${y}-${mm}-${dd}`);
  };

  const handleDay = (v: string) => {
    setDay(v);
    emit(v, month, year);
  };
  const handleMonth = (v: string) => {
    setMonth(v);
    // If the new month has fewer days than the picked day, clamp.
    if (day && year) {
      const max = new Date(Number(year), Number(v), 0).getDate();
      if (Number(day) > max) {
        const clamped = String(max);
        setDay(clamped);
        emit(clamped, v, year);
        return;
      }
    }
    emit(day, v, year);
  };
  const handleYear = (v: string) => {
    setYear(v);
    emit(day, month, v);
  };

  return (
    <>
      <div className="mt-1 grid grid-cols-3 gap-2">
        <CustomSelect
          value={day}
          onChange={handleDay}
          ariaLabel={tr("structured.day")}
          invalid={Boolean(error)}
          options={[
            { value: "", label: tr("structured.day") },
            ...Array.from({ length: daysInMonth }, (_, i) => {
              const dayValue = String(i + 1);
              return { value: dayValue, label: dayValue };
            }),
          ]}
        />
        <CustomSelect
          value={month}
          onChange={handleMonth}
          ariaLabel={tr("structured.month")}
          invalid={Boolean(error)}
          options={[
            { value: "", label: tr("structured.month") },
            ...monthLabels.map((label, idx) => ({
              value: String(idx + 1),
              label,
            })),
          ]}
        />
        <CustomSelect
          value={year}
          onChange={handleYear}
          ariaLabel={tr("structured.year")}
          invalid={Boolean(error)}
          options={[
            { value: "", label: tr("structured.year") },
            ...years.map((yearValue) => ({
              value: String(yearValue),
              label: String(yearValue),
            })),
          ]}
        />
      </div>
      {error && <p className="mt-1 text-xs text-[#FF3130]">{error}</p>}
    </>
  );
}

function parseDOBParts(
  value: string,
): { day: string; month: string; year: string } | null {
  if (!value) return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!m) return null;
  return {
    year: m[1] ?? "",
    month: String(Number(m[2])), // strip leading zero so the dropdown matches
    day: String(Number(m[3])),
  };
}
