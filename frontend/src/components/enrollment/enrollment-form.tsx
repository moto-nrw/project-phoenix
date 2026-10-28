"use client";

import { AttachmentList } from "~/components/enrollment/attachment-list";
import {
  EnrollmentUploadScope,
  fileValueMissing,
  useEnrollmentUploadTarget,
} from "~/components/enrollment/enrollment-file-upload";
import { useEffect, useMemo, useRef, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Check, Lock, Plus } from "lucide-react";
import { Turnstile } from "@marsidev/react-turnstile";
import { DateOfBirthPicker } from "~/components/enrollment/date-of-birth-picker";
import { Button } from "~/components/ui/button";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { InfoCard, InfoItem } from "~/components/ui/info-card";
import { Input } from "~/components/ui/input";
import { useTenant } from "~/lib/tenant-context";
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
  type PublicPhase,
  type PublicLateInvitePrefill,
  type PublicSchoolClassConfig,
  EMPTY_SCHOOL_CLASS_CONFIG,
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
  EmailContactNotice,
  EnrollmentConsent,
  LegalMarkdown,
  resolvePreviewLegalTexts,
} from "~/components/enrollment/enrollment-form-consent";
import {
  contactListValueMissing,
  phoneListValueMissing,
} from "~/components/enrollment/enrollment-form-contact-inputs";
import {
  CustomFieldInput,
  InfoBlock,
} from "~/components/enrollment/enrollment-form-custom-field";
import {
  asScheduleObject,
  asStringMap,
  asWeekdayBooleanObject,
  asWeekdayMultiModeObject,
  DEPARTURE_COMPANION_KEY,
  departureMultiModeOptionsForField,
  WEEKDAYS,
  type EnrollmentFormTranslator,
  type TranslationFn,
  type TranslationValues,
} from "~/components/enrollment/enrollment-form-shared";
import {
  localizeFormFields,
  localizeLegalBlocks,
  localizeOfferings,
  selectionGroupLabel,
} from "~/lib/enrollment-translations";
import {
  buildFieldsByKey,
  isFieldVisible,
  visibleAnswerData,
  type ConditionContext,
} from "~/lib/enrollment-field-visibility";
import { Modal } from "~/components/ui/modal";
import { CustomSelect } from "~/components/ui/custom-select";
import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { ToggleChip } from "~/components/ui/toggle-chip";
import { createLogger } from "~/lib/logger";
import { formatDate } from "~/lib/date-helpers";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
import {
  SignaturePad,
  type SignaturePadLabels,
  type SignatureValue,
} from "~/components/ui/signature-pad";
import { signatureRequestBody } from "~/lib/signature";
import {
  AT_MOST_ONE_PER_DAY,
  type CareOfferingDayConflict,
  careOfferingDayConflicts,
  conflictOfferingList,
} from "~/lib/care-offering-day-conflicts";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
  availableCareOfferings,
  careOfferingAvailabilityReason,
  careOfferingIsAvailable,
  careOfferingRuleExcludesNobody,
} from "~/lib/care-offering-availability";
import {
  type CareOfferingBookingStats,
  formatOfferingOccupancy,
  offeringIsFull,
} from "~/lib/care-offering-booking-stats";
import {
  BLOCKED_OFFERING_ROW_TONE,
  OfferingRowShell,
} from "~/components/enrollment/offering-row-shell";

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

// Student lifecycle statuses the backend counts as "enrolled"
// (users.StudentStatusActive / StudentStatusPending — see
// ExistsEnrolledByNameAndBirthday). The existing_students audience accepts
// nothing else, so the reuse picker must not offer anything else either.
const ENROLLED_CHILD_STATUSES = new Set(["active", "pending"]);

interface ChildDraft {
  // Stable, client-only identity for React list keys. Never sent to the
  // backend (payload mapping picks fields explicitly). Without it the
  // children list keyed by array index would leak a card's local view state
  // (uniform toggle, expanded days) onto a different child when a slot is
  // removed from the middle of the list.
  clientId: string;
  id?: string;
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level: string;
  /**
   * Concrete future class (e.g. "2a"), collected from grade 2 upwards
   * when the tenant enables it (#1833). Empty string = grade 1, feature
   * off, or "Klasse offen".
   */
  target_school_class: string;
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
  /**
   * Taken over into care (ADR 0003): the card renders read-only and the
   * values travel back unchanged, because the backend refuses a change
   * request that touches this child.
   */
  locked?: boolean;
  /**
   * The bootstrap wire representation of a taken-over child. It bypasses
   * normal form serialization so catalog changes cannot turn an unchanged,
   * read-only child into a rejected change request.
   */
  lockedPayload?: SubmitChildPayload;
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
  readonly lateInviteToken?: string;
  /** Status-link authorization for file uploads while editing a request. */
  readonly statusToken?: string;
  /** Authenticated parent-portal upload route for restricted phases. */
  readonly uploadEndpoint?: string;
  readonly previewMode?: boolean;
  readonly previewSchema?: PublicFormSchema | null;
  /**
   * Optional overrides for the autofill fetcher and submitter, used
   * by the parents portal to swap in parent-scope endpoints
   * (/api/parent/...) instead of the default tenant-scope ones. When
   * unset, the form falls back to the public path. A submitter returns
   * `null` when it stopped before sending and shows the reason itself.
   */
  readonly profileFetcher?: () => Promise<MeProfileResponse | null>;
  readonly submitter?: (
    payload: SubmitEnrollmentPayload,
  ) => Promise<SubmitEnrollmentResult | null>;
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
  readonly lockChildStructure?: boolean;
  /**
   * Reduced Halbjahreswechsel flow (#2251): render ONLY the per-child
   * care-offering selection. Guardian data, child identity, custom
   * fields, and consents stay hidden but travel unchanged in the
   * submit payload (seeded from initialDraft), so the resulting change
   * request diffs exactly the offerings. Callers must also lock the
   * child structure.
   */
  readonly restrictToOfferings?: boolean;
  readonly submitLabel?: string;
  /**
   * ADMIN ONLY (#2186). Renders offerings the child's grade level rules out
   * as disabled entries with the reason, instead of omitting them. Parents
   * must keep seeing only what they can pick — for them a hidden offering is
   * correct; for an admin it is a blackbox that produces support tickets.
   */
  readonly showBlockedOfferings?: boolean;
  /**
   * ADMIN ONLY (#2186). Per-offering occupancy, keyed by offering id. When
   * provided, each offering shows how full it is so a capacity rejection is
   * visible before the form is submitted rather than after.
   */
  readonly offeringBookingStats?: Record<string, CareOfferingBookingStats>;
  /**
   * Leaves the signature field out although the form requires one (#4786):
   * change requests and manual staff entries carry no parent signature.
   */
  readonly withoutSignature?: boolean;
}

export interface EnrollmentFormPrefetchedData {
  schema: PublicFormSchema | null;
  offerings: PublicCareOffering[];
  careOfferingSelectionMode: CareOfferingSelectionMode;
  captchaConfig: PublicCaptchaConfig | null;
  legalTexts: PublicLegalTexts;
  profile?: MeProfileResponse | null;
  lateInvite?: PublicLateInvitePrefill;
  /** Concrete-class config (#1833); absent falls back to disabled. */
  schoolClass?: PublicSchoolClassConfig;
  collectGradeLevel?: boolean;
  careOfferingsEnabled?: boolean;
  /**
   * Phase applicant restriction (#1663). Only meaningful on the parent
   * portal path, which prefills linked children: a "new_students" phase
   * rejects any already-enrolled child, so the form must hide the
   * "reuse an existing child" panel and explain why instead of letting
   * the parent adopt a child the server will reject.
   */
  audience?: PublicPhase["audience"];
  /**
   * Phase grade restriction (#1663). When non-empty the grade select offers
   * exactly these grades, so a parent cannot pick one the server rejects.
   */
  eligibleGradeLevels?: number[];
}

/**
 * Public enrollment form. Loads the active schema + open care offerings,
 * collects guardian info + 1..n children + offering selections, and
 * submits to /api/enrollment/{tenantSlug}/submit. The captcha widget is
 * inlined only when captcha is enabled and the payload carries a site key;
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
  lateInviteToken,
  statusToken,
  uploadEndpoint,
  previewMode = false,
  previewSchema,
  profileFetcher,
  submitter,
  skipCaptcha,
  localizedCopy = false,
  initialDraft,
  lockedGuardianEmail = false,
  lockChildStructure = false,
  restrictToOfferings = false,
  submitLabel,
  showBlockedOfferings = false,
  offeringBookingStats,
  withoutSignature = false,
}: Props) {
  const intl = useTranslations("enrollmentForm");
  const activeLocale = useLocale();
  // School-written texts follow the same opt-in as the form chrome: staff
  // previews stay German even when a parent locale cookie exists (#3377).
  const contentLocale = localizedCopy ? activeLocale : DEFAULT_LOCALE;
  const tr = useMemo(
    () => makeEnrollmentFormTranslator(intl, localizedCopy),
    [intl, localizedCopy],
  );
  const { tenantSlug } = useTenant();
  const signatureLabels = useMemo(
    (): SignaturePadLabels => ({
      label: tr("signature.label"),
      hint: tr("signature.hint"),
      canvas: tr("signature.canvas"),
      clear: tr("signature.clear"),
      typeInstead: tr("signature.typeInstead"),
      drawInstead: tr("signature.drawInstead"),
      typedName: tr("signature.typedName"),
      confirm: tr("signature.confirm"),
      confirmed: tr("signature.confirmed"),
      confirmedTyped: tr("signature.confirmedTyped"),
      change: tr("signature.change"),
      preview: tr("signature.preview"),
      emptyDrawing: tr("signature.emptyDrawing"),
      nameTooShort: tr("signature.nameTooShort"),
    }),
    [tr],
  );
  // File fields upload at once; the submit waits for running uploads (#4783).
  const { pendingUploads, uploadTarget } = useEnrollmentUploadTarget({
    tenantSlug,
    phaseId: phaseID,
    lateInviteToken,
    statusToken,
    uploadEndpoint,
    locale: contentLocale,
    previewMode,
  });
  // A "new_students" phase rejects any already-enrolled child at submit
  // (#1663). The parent portal prefills the guardian's linked children, so
  // offering to reuse one would always fail server-side — hide the reuse
  // panel and explain the restriction instead.
  const isNewStudentsPhase = prefetchedData?.audience === "new_students";
  // Reusing an existing child (adopting a linked student into a form slot)
  // only produces a correct approval on an "existing_students" phase: there
  // the backend matches the child to the already-enrolled student and RENEWS
  // it. On open/linked_parents phases the same reuse resolves no match and
  // approval fresh-creates a DUPLICATE Person/Student, so the reuse panel is
  // scoped to existing_students only (#1663).
  const isExistingStudentsPhase =
    prefetchedData?.audience === "existing_students";
  const initialOfferings = prefetchedData?.offerings ?? [];
  const initialCollectGradeLevel = prefetchedData?.collectGradeLevel !== false;
  const initialRequiredOfferingIDs = initialOfferings
    .filter((o) => o.is_required && careOfferingIsAvailable(o, undefined))
    .map((o) => o.id);
  // The loaded texts stay German in state; what the form renders is derived
  // per language, so switching the language needs no reload (#3377).
  const [loadedSchema, setSchema] = useState<PublicFormSchema | null>(
    prefetchedData?.schema ?? null,
  );
  const schema = useMemo(
    () =>
      loadedSchema && {
        ...loadedSchema,
        fields: localizeFormFields(loadedSchema.fields, contentLocale),
      },
    [loadedSchema, contentLocale],
  );
  const [loadedOfferings, setOfferings] = useState<PublicCareOffering[]>(
    prefetchedData?.offerings ?? [],
  );
  const offerings = useMemo(
    () => localizeOfferings(loadedOfferings, contentLocale),
    [loadedOfferings, contentLocale],
  );
  const [careOfferingSelectionMode, setCareOfferingSelectionMode] =
    useState<CareOfferingSelectionMode>(
      prefetchedData?.careOfferingSelectionMode ?? "optional",
    );
  const [collectGradeLevel, setCollectGradeLevel] = useState(
    prefetchedData?.collectGradeLevel !== false,
  );
  const [careOfferingsEnabled, setCareOfferingsEnabled] = useState(
    prefetchedData?.careOfferingsEnabled !== false,
  );
  // Concrete-class config (#1833): whether to collect a concrete class,
  // the phase's pick list, and whether it is mandatory from grade 2.
  // Seeded from prefetched data (public page) or fetched in load()
  // (parent portal). Defaults to disabled so the field stays hidden.
  const [schoolClassConfig, setSchoolClassConfig] =
    useState<PublicSchoolClassConfig>(
      prefetchedData?.schoolClass ?? EMPTY_SCHOOL_CLASS_CONFIG,
    );
  // Phase grade restriction (#1663): empty = every grade up to the tenant
  // cap. Seeded from prefetched data (public page + parent portal) or from
  // the care-offerings fallback load below.
  const [eligibleGradeLevels, setEligibleGradeLevels] = useState<number[]>(
    prefetchedData?.eligibleGradeLevels ?? [],
  );
  // Offerings the school flagged as mandatory. These are pre-selected and
  // locked in the UI so every child carries them.
  const requiredOfferingIDs = useMemo(
    () =>
      offerings
        .filter((o) => o.is_required && careOfferingIsAvailable(o, undefined))
        .map((o) => o.id),
    [offerings],
  );
  const [availabilityNotices, setAvailabilityNotices] = useState<
    Record<string, boolean>
  >({});
  const [loading, setLoading] = useState(prefetchedData === undefined);
  const [submitting, setSubmitting] = useState(false);
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
  // The form is long (guardian + N children + offerings) with the submit
  // button at the bottom. FormErrorAlert brings the first field marked
  // aria-invalid into view whenever a failed attempt sets the form error, so
  // every failing path below sets its marks no later than that error.
  const formRef = useRef<HTMLFormElement>(null);
  // One display path for every failure (#2515): a submit error or a local
  // check lands in the alert above the form, the affected fields are marked
  // via `fieldError(name)` (keys are each input's `name`: guardian_email,
  // children_0_first_name, ...), and the first of them gets the focus.
  const formError = useApiFormError(formRef);
  const { fieldError } = formError;
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [loadAttempt, setLoadAttempt] = useState(0);
  const lateInviteFetchOptions = useMemo(
    () => ({ lateInviteToken }),
    [lateInviteToken],
  );

  const [guardianFirstName, setGuardianFirstName] = useState(
    initialDraft?.guardian_first_name ??
      prefetchedData?.lateInvite?.guardian_first_name ??
      prefetchedData?.profile?.guardian.first_name ??
      "",
  );
  const [guardianLastName, setGuardianLastName] = useState(
    initialDraft?.guardian_last_name ??
      prefetchedData?.lateInvite?.guardian_last_name ??
      prefetchedData?.profile?.guardian.last_name ??
      "",
  );
  const [guardianEmail, setGuardianEmail] = useState(
    initialDraft?.guardian_email ??
      prefetchedData?.lateInvite?.guardian_email ??
      prefetchedData?.profile?.guardian.email ??
      "",
  );
  const [guardianPhone, setGuardianPhone] = useState(
    initialDraft?.guardian_phone ??
      prefetchedData?.profile?.guardian.phone ??
      "",
  );
  // The parent's signature (#4786); null until the parent confirms it in
  // the pad. A parent edit starts empty: every version is signed again.
  const [signature, setSignature] = useState<SignatureValue | null>(null);
  const signatureJustConfirmed = useRef(false);
  const [legalConsents, setLegalConsents] = useState<Record<string, boolean>>(
    () => draftConsentFlags(initialDraft),
  );
  const [children, setChildren] = useState<ChildDraft[]>([
    ...(initialDraft
      ? seedRequiredAvailableOfferings(
          draftChildren(initialDraft, initialRequiredOfferingIDs),
          initialOfferings,
          initialCollectGradeLevel,
        )
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
  // A confirmed signature only belongs to the values visible when it was
  // confirmed. Its own state update must not clear it; every later change to
  // signed enrollment data does.
  const changeSignature = (value: SignatureValue | null) => {
    signatureJustConfirmed.current = value !== null;
    setSignature(value);
  };
  useEffect(() => {
    if (signature === null) return;
    if (signatureJustConfirmed.current) {
      signatureJustConfirmed.current = false;
      return;
    }
    setSignature(null);
  }, [
    signature,
    guardianFirstName,
    guardianLastName,
    guardianEmail,
    guardianPhone,
    legalConsents,
    children,
    additionalGuardians,
    customData,
  ]);
  useEffect(() => {
    if (!initialDraft) return;
    setGuardianFirstName(initialDraft.guardian_first_name);
    setGuardianLastName(initialDraft.guardian_last_name);
    setGuardianEmail(initialDraft.guardian_email);
    setGuardianPhone(initialDraft.guardian_phone ?? "");
    setLegalConsents(draftConsentFlags(initialDraft));
    setChildren(
      seedRequiredAvailableOfferings(
        draftChildren(initialDraft, requiredOfferingIDs),
        offerings,
        collectGradeLevel,
      ),
    );
    setAdditionalGuardians(draftGuardians(initialDraft));
    setCustomData(initialDraft.custom_data ?? {});
  }, [initialDraft, offerings, requiredOfferingIDs, collectGradeLevel]);
  // Once the phase's class config is known (prefetched or fetched in load()),
  // drop any hydrated class an edit draft carries that the phase no longer
  // offers so it collapses to "Klasse offen" instead of being silently
  // resubmitted and rejected by the backend validator (#1833).
  useEffect(() => {
    setChildren((prev) => sanitizeDraftSchoolClasses(prev, schoolClassConfig));
  }, [schoolClassConfig]);
  const [captchaToken, setCaptchaToken] = useState("");
  const [captchaConfig, setCaptchaConfig] =
    useState<PublicCaptchaConfig | null>(prefetchedData?.captchaConfig ?? null);
  const captchaRequired = !skipCaptcha && captchaConfig?.enabled === true;
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
  // Children the parent may actually re-enroll. Two independent facts are
  // required, because portal visibility grants neither (#1663):
  //   - the relationship grants parent_portal.enrollment.submit, so a
  //     permission-revoked child never 403s after the form is filled;
  //   - the child is still enrolled (active/pending). The profile lists every
  //     non-alumnus child, but the existing_students gate only accepts
  //     active/pending, so an inactive child would be adopted here and then
  //     rejected as "nicht eingeschrieben" at submit.
  const reusableChildren = useMemo(
    () =>
      (profile?.children ?? []).filter(
        (c) =>
          c.enrollment_submit === true &&
          ENROLLED_CHILD_STATUSES.has(c.status ?? ""),
      ),
    [profile],
  );

  useEffect(() => {
    let cancelled = false;
    if (prefetchedData !== undefined) {
      setSchema(prefetchedData.schema);
      setOfferings(prefetchedData.offerings);
      setCareOfferingSelectionMode(prefetchedData.careOfferingSelectionMode);
      setCollectGradeLevel(prefetchedData.collectGradeLevel !== false);
      setCareOfferingsEnabled(prefetchedData.careOfferingsEnabled !== false);
      setSchoolClassConfig(
        prefetchedData.schoolClass ?? EMPTY_SCHOOL_CLASS_CONFIG,
      );
      setEligibleGradeLevels(prefetchedData.eligibleGradeLevels ?? []);
      setCaptchaConfig(prefetchedData.captchaConfig);
      setLegalTexts(prefetchedData.legalTexts);
      setProfile(prefetchedData.profile ?? null);
      setChildren((prev) =>
        seedRequiredAvailableOfferings(
          prev.length > 0 ? prev : [blankChild()],
          prefetchedData.offerings,
          prefetchedData.collectGradeLevel !== false,
        ),
      );
      if (prefetchedData.lateInvite) {
        setGuardianFirstName(
          (prev) =>
            prev || prefetchedData.lateInvite?.guardian_first_name || "",
        );
        setGuardianLastName(
          (prev) => prev || prefetchedData.lateInvite?.guardian_last_name || "",
        );
        setGuardianEmail(
          (prev) => prev || prefetchedData.lateInvite?.guardian_email || "",
        );
      }
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
      clearLoadError();
      setLoading(false);
      return () => {
        cancelled = true;
      };
    }
    async function load() {
      setLoading(true);
      clearLoadError();
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
              ? // A phase without a form template answers 404, which the
                // client already turns into null. Any other failure must
                // stop the load: the form would silently drop the school's
                // own questions.
                fetchPublicActiveSchema(
                  tenantSlug,
                  phaseID,
                  lateInviteFetchOptions,
                )
              : Promise.resolve(null),
          phaseID
            ? fetchPublicCareOfferings(
                tenantSlug,
                phaseID,
                lateInviteFetchOptions,
              )
            : Promise.resolve({
                offerings: [],
                careOfferingSelectionMode: "optional" as const,
                careRequired: false,
                schoolClass: EMPTY_SCHOOL_CLASS_CONFIG,
                collectGradeLevel: true,
                careOfferingsEnabled: true,
                // No phase, so no phase-level grade restriction (preview).
                eligibleGradeLevels: [] as number[],
              }),
          // Deliberately silent: the profile only prefills the guardian
          // fields, which the parent can type in. The client logs the
          // failure, and the form stays usable without it.
          profileLoader().catch(() => null),
          // Skip the captcha config when the caller already authenticated
          // the parent (parent-portal embedded form). Saves a round-trip
          // and avoids rendering the widget on an already-trusted path.
          skipCaptcha
            ? Promise.resolve(null)
            : // Deliberately silent: the client logs a failed config load.
              // Without it the widget is missing, and the submit shows the
              // server's captcha rejection in the form's alert.
              fetchPublicCaptchaConfig(tenantSlug).catch(() => null),
          // Legal texts are phase/template-aware. NOT best-
          // effort: a real load failure rejects the whole load so the
          // form shows an error instead of collecting legally relevant
          // consent without the configured documents. Unconfigured
          // texts return empty strings (no rejection) and just drop the
          // link.
          previewSchema !== undefined
            ? resolvePreviewLegalTexts(tenantSlug, previewSchema)
            : phaseID
              ? fetchPublicLegalTexts(
                  tenantSlug,
                  phaseID,
                  lateInviteFetchOptions,
                )
              : fetchPublicLegalTexts(tenantSlug),
        ]);
        if (cancelled) return;
        setSchema(schemaResult);
        setOfferings(offeringsResult.offerings);
        setCareOfferingSelectionMode(offeringsResult.careOfferingSelectionMode);
        setCollectGradeLevel(offeringsResult.collectGradeLevel !== false);
        setCareOfferingsEnabled(offeringsResult.careOfferingsEnabled !== false);
        setSchoolClassConfig(
          offeringsResult.schoolClass ?? EMPTY_SCHOOL_CLASS_CONFIG,
        );
        setEligibleGradeLevels(offeringsResult.eligibleGradeLevels ?? []);
        // Seed mandatory offerings into the children that already exist
        // (the initial blank slot is created before offerings load).
        setChildren((prev) =>
          seedRequiredAvailableOfferings(
            prev,
            offeringsResult.offerings,
            offeringsResult.collectGradeLevel !== false,
          ),
        );
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
        logger.error("enrollment_form_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          code: (err as Partial<ApiError> | undefined)?.code,
        });
        void showLoadError(err, {
          object: tr("errorObject"),
          retry: () => setLoadAttempt((n) => n + 1),
        });
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
    lateInviteFetchOptions,
    profileFetcher,
    skipCaptcha,
    tr,
    loadAttempt,
    clearLoadError,
    showLoadError,
  ]);

  const updateChild = (index: number, patch: Partial<ChildDraft>) => {
    setChildren((prev) =>
      prev.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    );
  };

  const careOfferingGradeForChild = (child: ChildDraft) =>
    collectGradeLevel ? child.target_grade_level : undefined;

  const careOfferingsForChild = (child: ChildDraft) =>
    availableCareOfferings(offerings, careOfferingGradeForChild(child));

  const materializeCareOfferingsForChild = (
    child: ChildDraft,
    childOfferings: readonly PublicCareOffering[],
  ) =>
    materializeCareOfferings(
      child,
      childOfferings,
      careOfferingGradeForChild(child),
    );

  const toggleOffering = (childIndex: number, offeringID: string) => {
    // "Genau eines" / "höchstens eines" selection_groups behave like radios:
    // selecting one clears the other selected member(s) of the same group.
    // Orthogonal to the phase-level exactly_one mode below; both can apply.
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const childOfferings = careOfferingsForChild(c);
        const childRequiredIDs = childOfferings
          .filter((o) => o.is_required)
          .map((o) => o.id);
        if (childRequiredIDs.includes(offeringID)) return c;
        const toggled = childOfferings.find((o) => o.id === offeringID);
        if (!toggled) return c;
        const group = toggled.selection_group?.trim() ?? "";
        const groupRule = toggled.selection_rule ?? "optional";
        const exclusiveGroup =
          group !== "" &&
          (groupRule === "exactly_one" || groupRule === "at_most_one");
        const groupSiblingIDs = exclusiveGroup
          ? new Set(
              childOfferings
                .filter((o) => (o.selection_group?.trim() ?? "") === group)
                .map((o) => o.id),
            )
          : null;
        const nextIDs = new Set(c.offering_ids);
        const nextDays = { ...c.offering_days };
        if (nextIDs.has(offeringID)) {
          nextIDs.delete(offeringID);
          delete nextDays[offeringID];
        } else {
          if (careOfferingSelectionMode === "exactly_one") {
            for (const existingID of nextIDs) {
              if (!childRequiredIDs.includes(existingID)) {
                delete nextDays[existingID];
              }
            }
            nextIDs.clear();
            childRequiredIDs.forEach((id) => nextIDs.add(id));
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
                !childRequiredIDs.includes(id)
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
        const childOfferings = careOfferingsForChild(c);
        const childRequiredIDs = childOfferings
          .filter((o) => o.is_required)
          .map((o) => o.id);
        const materialized = materializeCareOfferingsForChild(
          c,
          childOfferings,
        );
        if (materialized.automaticDays[offeringID]?.has(day)) return c;
        const current = c.offering_days[offeringID] ?? new Set<string>();
        const next = new Set(current);
        if (next.has(day)) next.delete(day);
        else next.add(day);
        const nextIDs = new Set(c.offering_ids);
        if (next.size > 0) {
          nextIDs.add(offeringID);
        } else if (!childRequiredIDs.includes(offeringID)) {
          nextIDs.delete(offeringID);
        }
        return {
          ...c,
          offering_ids: nextIDs,
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
      prev.map((guardian, i) =>
        i === index
          ? copyStableObjectKey(guardian, { ...guardian, ...patch })
          : guardian,
      ),
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
  const legalBlocks = useMemo(
    () => localizeLegalBlocks(legalTexts?.blocks ?? [], contentLocale),
    [legalTexts, contentLocale],
  );
  const childConditionCtx = (child: ChildDraft): ConditionContext => ({
    guardianAnswers: customData,
    childAnswers: child.custom,
    gradeLevel: collectGradeLevel ? child.target_grade_level : undefined,
    // Care-offering conditions match the German catalog name (templates aren't
    // phase-bound). Resolve selected ids against the loaded, canonical catalog,
    // not the localized display copy, so this stays aligned with the backend.
    offeringNames: new Set(
      Array.from(
        careOfferingsEnabled
          ? materializeCareOfferingsForChild(
              child,
              careOfferingsForChild(child),
            ).offeringIds
          : [],
      )
        .map((id) => loadedOfferings.find((o) => o.id === id)?.name)
        .filter((name): name is string => Boolean(name))
        .map((name) => name.trim().toLowerCase()),
    ),
    fieldsByKey,
  });

  // A form can ask the parent to sign (#4786); the signature field then
  // closes the form after the legal texts.
  const signatureRequired =
    Boolean(schema?.core_requirements?.signature) && !withoutSignature;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    formError.clear();
    setChildOfferingErrors({});
    setOfferingDayErrors({});

    if (previewMode) {
      formError.invalid(tr("previewError"));
      return;
    }
    if (!phaseID) {
      formError.invalid(tr("missingPhase"));
      return;
    }
    if (pendingUploads > 0) {
      formError.invalid(tr("upload.pendingSubmit"));
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
      if (c.locked) continue;
      const childCtx = childConditionCtx(c);
      const childOfferings = careOfferingsForChild(c);
      if (!c.first_name.trim()) {
        newFieldErrors[`children_${i}_first_name`] = tr("errors.firstName");
      }
      if (!c.last_name.trim()) {
        newFieldErrors[`children_${i}_last_name`] = tr("errors.lastName");
      }
      if (!c.date_of_birth) {
        newFieldErrors[`children_${i}_date_of_birth`] = tr("errors.dob");
      }
      if (collectGradeLevel && !c.target_grade_level) {
        newFieldErrors[`children_${i}_target_grade_level`] = tr("errors.grade");
      }
      // Concrete class is only required when the tenant collects it, the
      // phase makes it mandatory, the child is grade 2 or higher (grade 1
      // never shows the field), and this grade actually has a matching
      // class to pick. A phase may require classes yet only offer them for
      // some grades, so a grade with no matching class stays "Klasse offen"
      // rather than an unsubmittable required-but-empty dropdown. Mirror the
      // backend gate (#1833).
      if (
        collectGradeLevel &&
        schoolClassConfig.require &&
        collectsConcreteClassForGrade(
          schoolClassConfig,
          c.target_grade_level,
        ) &&
        !c.target_school_class.trim()
      ) {
        newFieldErrors[`children_${i}_target_school_class`] =
          tr("errors.schoolClass");
      }
      for (const field of schema?.fields.filter((f) => f.applies_to_child) ??
        []) {
        if (field.type === "information" || !field.required) continue;
        if (!isFieldVisible(field, childCtx)) continue;
        if (
          customValueMissing(
            field,
            c.custom[field.key],
            relevantCareDaysForChild(
              c,
              childOfferings,
              previewMode,
              careOfferingGradeForChild(c),
              offerings.length > 0,
            ),
            careDaysAreConstrained(offerings, previewMode),
          )
        ) {
          newFieldErrors[`children_${i}_custom_${field.key}`] =
            requiredMessageForField(
              field,
              tr,
              careDaysAreConstrained(offerings, previewMode),
            );
        }
      }
      // The coupled "mit wem" note (#1694) is required whenever this child has
      // the accompanied mode selected on any relevant care day. A schema may
      // carry more than one field targeting student.allowed_departure_modes
      // (only keys are unique), and each WeekdayMultiModeInput shows the note
      // input off its own value, so check every visible departure field, not
      // just the first, to keep validation aligned with the UI.
      const relevantDays = relevantCareDaysForChild(
        c,
        childOfferings,
        previewMode,
        careOfferingGradeForChild(c),
        offerings.length > 0,
      );
      const accompaniedSelected = (schema?.fields ?? [])
        .filter(
          (f) =>
            f.target === "student.allowed_departure_modes" &&
            f.applies_to_child &&
            isFieldVisible(f, childCtx),
        )
        .some((f) => {
          const modesObj = asWeekdayMultiModeObject(
            c.custom[f.key],
            relevantDays,
          );
          return relevantDays.some((day) =>
            (modesObj[day] ?? []).includes("accompanied"),
          );
        });
      if (accompaniedSelected) {
        const note = (
          c.custom[DEPARTURE_COMPANION_KEY] as string | undefined
        )?.trim();
        if (!note) {
          newFieldErrors[`children_${i}_departure_companion_note`] = tr(
            "structured.departureCompanionRequired",
          );
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
      if (c.locked) continue;
      const childOfferings = careOfferingsForChild(c);
      const childRequiredIDs = childOfferings
        .filter((o) => o.is_required)
        .map((o) => o.id);
      const selectionModeApplies =
        offerings.length === 0 || childOfferings.some((o) => !o.is_required);
      const materializedCare = materializeCareOfferingsForChild(
        c,
        childOfferings,
      );
      const choosableSelectedCount = [...c.offering_ids].filter(
        (id) =>
          childOfferings.some((o) => o.id === id) &&
          !childRequiredIDs.includes(id),
      ).length;
      if (
        selectionModeApplies &&
        careOfferingSelectionMode === "at_least_one" &&
        choosableSelectedCount === 0
      ) {
        missingCareIndexes.push(i);
      }
      if (
        selectionModeApplies &&
        careOfferingSelectionMode === "exactly_one" &&
        choosableSelectedCount !== 1
      ) {
        exactOneCareIndexes.push(i);
      }
      for (const id of materializedCare.offeringIds) {
        const offering = childOfferings.find((o) => o.id === id);
        if (offering?.days_of_week_mode !== "parent_choice") {
          continue;
        }
        const picked = materializedCare.offeringDays[id];
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
      const groupRuleMessage = offeringGroupRuleError(
        materializedCare,
        childOfferings,
        tr,
        contentLocale,
      );
      if (groupRuleMessage) {
        groupRuleIndexes.push(i);
        firstGroupRuleMessage ??= `${tr("structured.child")} ${i + 1}: ${groupRuleMessage}`;
      }
    }

    // Reduced flow (#2251): hidden sections must never block submit — their
    // values travel unchanged from the draft, and an error on an invisible
    // field would be a dead end for the parent. Only the offering checks
    // below apply; the backend re-validates the full payload anyway.
    if (restrictToOfferings) {
      for (const key of Object.keys(newFieldErrors)) {
        delete newFieldErrors[key];
      }
    }
    if (signatureRequired && signature === null) {
      newFieldErrors[SIGNATURE_FIELD] = tr("signature.required");
    }
    const fieldErrorKeys = Object.keys(newFieldErrors);
    const dayErrorCount = Object.keys(dayErrors).length;
    const hasOfferingIssue =
      missingCareIndexes.length > 0 ||
      exactOneCareIndexes.length > 0 ||
      groupRuleIndexes.length > 0 ||
      dayErrorCount > 0;
    if (fieldErrorKeys.length > 0 || hasOfferingIssue) {
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
      formError.invalid(banner, newFieldErrors);
      return;
    }

    // All required input is valid, assemble the wire payload. Day
    // selections for parent_choice offerings are guaranteed present by the
    // validation above, so this only builds data (no further checks).
    const payloadChildren: SubmitChildPayload[] = children.map((c) => {
      if (c.lockedPayload) return c.lockedPayload;
      const childOfferings = careOfferingsForChild(c);
      const childOfferingIDs = new Set(childOfferings.map((o) => o.id));
      const offeringDaysPayload: Array<{
        offering_id: number;
        selected_days: string[];
      }> = [];
      for (const id of c.offering_ids) {
        if (!childOfferingIDs.has(id)) continue;
        const offering = childOfferings.find((o) => o.id === id);
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
      // Strip answers to fields the parent couldn't see (hidden by a
      // show-if condition) so a stale value never reaches the backend.
      const customData = restrictToOfferings
        ? { ...c.custom }
        : pruneWeekdayAnswers(
            visibleAnswerData(
              schema?.fields ?? [],
              true,
              c.custom,
              childConditionCtx(c),
            ),
            schema?.fields ?? [],
            relevantCareDaysForChild(
              c,
              childOfferings,
              previewMode,
              careOfferingGradeForChild(c),
              offerings.length > 0,
            ),
          );
      // Couple the "mit wem" note (#1694) back in: it rides on a reserved key
      // (not a schema field), so visibleAnswerData drops it. A schema may carry
      // more than one field targeting student.allowed_departure_modes (only keys
      // are unique, so conditional variants are allowed), so check every visible
      // departure field, not just the first. Re-attach the note only when some
      // surviving modes actually allow the accompanied mode and a note was
      // entered, so a stale note never reaches the backend.
      const hasAccompaniedDeparture = (schema?.fields ?? [])
        .filter(
          (f) =>
            f.target === "student.allowed_departure_modes" &&
            f.applies_to_child,
        )
        .some((f) => {
          const modesVal = customData[f.key];
          return (
            !!modesVal &&
            typeof modesVal === "object" &&
            Object.values(modesVal as Record<string, unknown>).some(
              (arr) => Array.isArray(arr) && arr.includes("accompanied"),
            )
          );
        });
      if (hasAccompaniedDeparture) {
        const note = (
          c.custom[DEPARTURE_COMPANION_KEY] as string | undefined
        )?.trim();
        if (note) {
          customData[DEPARTURE_COMPANION_KEY] = note;
        }
      }
      return {
        ...(c.id ? { id: c.id } : {}),
        first_name: c.first_name.trim(),
        last_name: c.last_name.trim(),
        date_of_birth: c.date_of_birth,
        target_grade_level: collectGradeLevel
          ? Number(c.target_grade_level)
          : undefined,
        // Only send a concrete class when the phase collects it for this
        // child's grade and a class was actually chosen; the backend
        // re-validates and normalises (#1833/#1663). Grade 1 is collected
        // only when the phase offers a grade-1 class, otherwise omitted
        // ("Klasse offen").
        target_school_class:
          collectGradeLevel &&
          collectsConcreteClassForGrade(
            schoolClassConfig,
            c.target_grade_level,
          ) &&
          c.target_school_class.trim()
            ? c.target_school_class.trim()
            : undefined,
        custom_data: customData,
        offering_ids: careOfferingsEnabled
          ? Array.from(c.offering_ids)
              .filter((id) => childOfferingIDs.has(id))
              .map(Number)
          : undefined,
        offering_days:
          careOfferingsEnabled && offeringDaysPayload.length > 0
            ? offeringDaysPayload
            : undefined,
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
        custom_data: restrictToOfferings
          ? { ...customData }
          : visibleAnswerData(
              schema?.fields ?? [],
              false,
              customData,
              guardianCtx,
            ),
        children: payloadChildren,
        captcha_token: captchaRequired ? captchaToken || undefined : undefined,
        late_invite_token: lateInviteToken?.trim()
          ? lateInviteToken.trim()
          : undefined,
        signature:
          signatureRequired && signature
            ? signatureRequestBody(signature)
            : undefined,
      };
      const result = submitter
        ? await submitter(payload)
        : await submitEnrollment(tenantSlug, payload);
      // The caller stopped before sending and shows its own reason.
      if (!result) return;
      const statusURL = new URL(result.status_url, globalThis.location.origin);
      if (
        result.warnings?.some(
          (warning) => warning.code === "enrollment.duplicate_detected",
        )
      ) {
        statusURL.searchParams.set("duplicate_warning", "1");
      }
      onSubmitted(
        /^https?:\/\//i.test(result.status_url)
          ? statusURL.toString()
          : `${statusURL.pathname}${statusURL.search}${statusURL.hash}`,
      );
    } catch (err) {
      const code = err instanceof ApiError ? err.code : undefined;
      // A rejected input is expected; only server failures and crashes are
      // errors in the log.
      const expected = err instanceof ApiError && (err.status ?? 500) < 500;
      const logContext = {
        error: err instanceof Error ? err.message : String(err),
        code,
      };
      if (expected) logger.warn("enrollment_submit_failed", logContext);
      else logger.error("enrollment_submit_failed", logContext);
      // The backend names fields by their JSON path; the form's inputs by
      // their `name`. Translate, so the shared path marks the right input.
      if (err instanceof ApiError && err.errors) {
        err.errors = err.errors.map((entry) => ({
          ...entry,
          field: submitFieldName(entry.field),
        }));
      }
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
      let pickupMarks: string[] = [];
      if (code === "enrollment.pickup_time_not_allowed") {
        // The backend rejected a pickup time outside the field's configured
        // fixed list. The server doesn't say which field/child, so derive it
        // locally (same approach as the care-offering case above). Two passes:
        //   - offList: a time the *client's* known list already rejects — a
        //     stale saved value the form still shows flagged "nicht mehr
        //     verfügbar".
        //   - answered: any constrained pickup field that has a time at all.
        //     Used as a fallback when offList is empty: that happens when the
        //     admin removed a time *after* the form loaded, so the client list
        //     is stale and the submitted value still looks valid here. Marking
        //     every answered constrained field keeps the rejection from being
        //     silently swallowed (the offending field turns red and the scroll
        //     lands on it). The backend rejected it regardless — this is UX.
        const offList: string[] = [];
        const answered: string[] = [];
        for (const f of schema?.fields ?? []) {
          if (
            f.target !== "schedule.pickup" ||
            !f.allowed_times?.length ||
            f.type !== "weekday_schedule"
          ) {
            continue;
          }
          const mark = (answer: unknown, key: string): void => {
            const times = Object.values(asScheduleObject(answer)).filter(
              Boolean,
            );
            if (times.length === 0) return;
            answered.push(key);
            if (times.some((time) => !f.allowed_times!.includes(time))) {
              offList.push(key);
            }
          };
          if (f.applies_to_child) {
            children.forEach((c, i) =>
              mark(c.custom[f.key], `children_${i}_custom_${f.key}`),
            );
          } else {
            mark(customData[f.key], `custom_${f.key}`);
          }
        }
        pickupMarks = offList.length > 0 ? offList : answered;
      }
      // "Wiederholen" submits the form again, so it sends what the parent
      // sees now, not the draft of the failed attempt.
      const presentation = await formError.show(err, {
        object: tr("errorObject"),
        retry: () => formRef.current?.requestSubmit(),
      });
      if (
        presentation &&
        presentation.fields.length === 0 &&
        pickupMarks.length > 0
      ) {
        formError.invalid(
          presentation.message,
          Object.fromEntries(
            pickupMarks.map((key) => [key, presentation.message]),
          ),
        );
      }
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

  // Without schema, offerings and legal texts the form cannot be filled in
  // correctly, so a failed load replaces it instead of showing it empty.
  if (loadError) {
    return <LoadErrorAlert error={loadError} />;
  }

  return (
    <EnrollmentUploadScope target={uploadTarget}>
      <form
        ref={formRef}
        onSubmit={handleSubmit}
        noValidate
        className="space-y-5"
      >
        <FormErrorAlert message={formError.error} />
        {/* No submit can pass the bot protection: the form fails as a whole,
          so families read it before they fill anything in. */}
        <LoadErrorAlert
          error={
            captchaRequired && !captchaConfig.site_key
              ? tr("captchaMisconfigured")
              : null
          }
        />

        {!restrictToOfferings && (
          <section className="moto-content-surface space-y-5 rounded-xl border p-4 shadow-sm sm:p-5">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <SectionHeading
                title={tr("sections.guardianTitle")}
                description={tr("sections.guardianDescription")}
              />
              <Button
                type="button"
                variant="surface"
                size="md"
                onClick={addGuardian}
                className="w-full gap-1.5 whitespace-nowrap sm:w-auto sm:shrink-0"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                {tr("actions.addGuardian")}
              </Button>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input
                id="enrollment-guardian_first_name"
                label={tr("fields.firstName")}
                name="guardian_first_name"
                autoComplete="given-name"
                controlSize="compact"
                value={guardianFirstName}
                onChange={(e) => setGuardianFirstName(e.target.value)}
                required
                error={fieldError("guardian_first_name")}
              />
              <Input
                id="enrollment-guardian_last_name"
                label={tr("fields.lastName")}
                name="guardian_last_name"
                autoComplete="family-name"
                controlSize="compact"
                value={guardianLastName}
                onChange={(e) => setGuardianLastName(e.target.value)}
                required
                error={fieldError("guardian_last_name")}
              />
              <Input
                id="enrollment-guardian_email"
                label={tr("fields.email")}
                name="guardian_email"
                type="email"
                spellCheck={false}
                autoComplete="email"
                controlSize="compact"
                value={guardianEmail}
                onChange={(e) => setGuardianEmail(e.target.value)}
                required
                readOnly={lockedGuardianEmail}
                error={fieldError("guardian_email")}
              />
              <Input
                id="enrollment-guardian_phone"
                label={tr(
                  schema?.core_requirements?.guardian_phone
                    ? "fields.phoneRequired"
                    : "fields.phone",
                )}
                name="guardian_phone"
                type="tel"
                autoComplete="tel"
                inputMode="tel"
                controlSize="compact"
                value={guardianPhone}
                onChange={(e) => setGuardianPhone(e.target.value)}
                required={schema?.core_requirements?.guardian_phone === true}
                error={fieldError("guardian_phone")}
              />
            </div>

            {additionalGuardians.map((g, i) => (
              <div
                key={getStableObjectKey(g, "additional-guardian")}
                className="moto-content-surface space-y-5 rounded-xl border p-3 shadow-sm sm:p-4"
              >
                <div className="flex items-center justify-between gap-3">
                  <h3 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
                    {tr("sections.additionalGuardianLabel", { index: i + 1 })}
                  </h3>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={() => removeGuardian(i)}
                    className="text-moto-red-strong hover:bg-moto-red/10 hover:text-moto-red-strong"
                  >
                    {tr("actions.remove")}
                  </Button>
                </div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Input
                    id={`enrollment-additional_guardian_${i}_first_name`}
                    label={tr("fields.firstName")}
                    name={`additional_guardian_${i}_first_name`}
                    autoComplete="given-name"
                    controlSize="compact"
                    value={g.first_name}
                    onChange={(e) =>
                      updateGuardian(i, { first_name: e.target.value })
                    }
                    required
                    error={fieldError(`additional_guardian_${i}_first_name`)}
                  />
                  <Input
                    id={`enrollment-additional_guardian_${i}_last_name`}
                    label={tr("fields.lastName")}
                    name={`additional_guardian_${i}_last_name`}
                    autoComplete="family-name"
                    controlSize="compact"
                    value={g.last_name}
                    onChange={(e) =>
                      updateGuardian(i, { last_name: e.target.value })
                    }
                    required
                    error={fieldError(`additional_guardian_${i}_last_name`)}
                  />
                  <Input
                    id={`enrollment-additional_guardian_${i}_email`}
                    label={tr("fields.emailPlain")}
                    name={`additional_guardian_${i}_email`}
                    type="email"
                    spellCheck={false}
                    autoComplete="email"
                    controlSize="compact"
                    value={g.email}
                    onChange={(e) =>
                      updateGuardian(i, { email: e.target.value })
                    }
                    error={fieldError(`additional_guardian_${i}_email`)}
                  />
                  <Input
                    id={`enrollment-additional_guardian_${i}_phone`}
                    label={tr("fields.phone")}
                    name={`additional_guardian_${i}_phone`}
                    type="tel"
                    autoComplete="tel"
                    inputMode="tel"
                    controlSize="compact"
                    value={g.phone}
                    onChange={(e) =>
                      updateGuardian(i, { phone: e.target.value })
                    }
                    error={fieldError(`additional_guardian_${i}_phone`)}
                  />
                </div>
              </div>
            ))}
          </section>
        )}

        {!restrictToOfferings &&
          schema?.fields.some((f) => !f.applies_to_child) && (
            <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
              <SectionHeading
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
                      inputName={`custom_${f.key}`}
                      error={fieldError(`custom_${f.key}`)}
                      tr={tr}
                    />
                  ),
                )}
            </section>
          )}

        <section className="moto-content-surface space-y-5 rounded-xl border p-4 shadow-sm sm:p-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <SectionHeading
              title={tr(
                restrictToOfferings
                  ? "sections.offeringsOnlyTitle"
                  : "sections.childrenTitle",
              )}
              description={tr(
                restrictToOfferings
                  ? "sections.offeringsOnlyDescription"
                  : "sections.childrenDescription",
              )}
            />
            {!lockChildStructure ? (
              <Button
                type="button"
                variant="surface"
                size="md"
                onClick={addChild}
                className="w-full gap-1.5 whitespace-nowrap sm:w-auto sm:shrink-0"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                {tr("actions.addChild")}
              </Button>
            ) : null}
          </div>

          {profile &&
            profile.children.length > 0 &&
            !lockChildStructure &&
            isNewStudentsPhase && (
              <p className="border-moto-orange/30 bg-moto-orange/5 rounded-lg border px-4 py-3 text-sm text-gray-700">
                {tr("sections.newStudentsOnlyNotice")}
              </p>
            )}

          {reusableChildren.length > 0 &&
            !lockChildStructure &&
            isExistingStudentsPhase && (
              <ExistingChildrenPanel
                existing={reusableChildren}
                usedIDs={usedExistingChildIDs}
                tr={tr}
                onAdopt={(child) => {
                  const newSlot: ChildDraft = blankChild(requiredOfferingIDs);
                  newSlot.first_name = child.first_name;
                  newSlot.last_name = child.last_name;
                  // The phase may restrict eligible grades (#1663), and the
                  // grade select offers exactly those. Adopting a child whose
                  // stored grade falls outside the restriction would prefill a
                  // value the parent can neither see nor change, pass the
                  // client-side "grade is set" check, and only fail at submit
                  // with grade_not_eligible for the whole form. Drop the
                  // ineligible prefill so the required-field check forces a
                  // grade the phase accepts. Grades are not collected at all
                  // when collect_grade_level is off — the phase-side
                  // collectability guard keeps that pair from carrying a
                  // restriction — the value stays in the draft but must not
                  // drive any hidden grade-dependent controls.
                  const reusedGrade =
                    child.grade_level != null ? String(child.grade_level) : "";
                  newSlot.target_grade_level =
                    !collectGradeLevel ||
                    selectableGradeLevels(
                      gradeLevelMax,
                      eligibleGradeLevels,
                    ).includes(Number(reusedGrade))
                      ? reusedGrade
                      : "";
                  for (const offering of availableCareOfferings(
                    offerings,
                    collectGradeLevel ? newSlot.target_grade_level : undefined,
                  )) {
                    if (offering.is_required) {
                      newSlot.offering_ids.add(offering.id);
                    }
                  }
                  // Prefill the concrete class only when it matches one of the
                  // phase's offered classes, so re-enrolling an existing "2a"
                  // child defaults sensibly without injecting a stale/unknown
                  // value into the dropdown (#1833).
                  newSlot.target_school_class =
                    schoolClassConfig.available_classes.includes(
                      child.school_class,
                    ) &&
                    classMatchesGrade(
                      child.school_class,
                      newSlot.target_grade_level,
                    )
                      ? child.school_class
                      : "";
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

          {children.map((child, i) => {
            const childOfferings = careOfferingsForChild(child);
            // Admin flows only (#2186): everything the grade rule filtered out,
            // so the picker can show it greyed with a reason. The parent form
            // leaves this empty and is byte-for-byte unchanged.
            const blockedOfferings = showBlockedOfferings
              ? offerings.filter(
                  (offering) =>
                    !childOfferings.some((open) => open.id === offering.id),
                )
              : [];
            const materializedCare = materializeCareOfferingsForChild(
              child,
              childOfferings,
            );
            const dayConflicts = careOfferingDayConflicts(
              childOfferings,
              materializedCare.offeringIds,
              materializedCare.offeringDays,
            );
            const groupDayConflicts = (group: string) =>
              dayConflicts.filter((conflict) => conflict.group === group);
            // #4515: offerings limited by grade stay hidden until a grade is
            // picked. Say so up front, or a parent reads the missing block as
            // "no care offered here" or "comes in a later step". Only when the
            // grade select is on screen and some selectable grade would reveal
            // more offerings, so a switched-off grade question or a catalog
            // without grade rules never asks for a grade.
            const gradeChosen = Number(child.target_grade_level) >= 1;
            const hasGradeRestrictedOffering = offerings.some(
              (offering) =>
                !careOfferingRuleExcludesNobody(
                  offering.availability_rule,
                  gradeLevelMax,
                ),
            );
            const offeringsAwaitGrade =
              careOfferingsEnabled &&
              collectGradeLevel &&
              !restrictToOfferings &&
              !gradeChosen &&
              hasGradeRestrictedOffering &&
              selectableGradeLevels(gradeLevelMax, eligibleGradeLevels).some(
                (grade) =>
                  availableCareOfferings(offerings, grade).length >
                  childOfferings.length,
              );
            const noOfferingsForGrade =
              careOfferingsEnabled &&
              collectGradeLevel &&
              gradeChosen &&
              offerings.length > 0 &&
              childOfferings.length === 0 &&
              blockedOfferings.length === 0;
            const showCareSection =
              careOfferingsEnabled &&
              (childOfferings.length > 0 ||
                blockedOfferings.length > 0 ||
                offeringsAwaitGrade ||
                noOfferingsForGrade);
            if (child.locked) {
              const name = `${child.first_name} ${child.last_name}`.trim();
              const booked = offerings
                .filter((offering) => child.offering_ids.has(offering.id))
                .map((offering) => offering.name);
              return (
                <InfoCard
                  key={child.clientId}
                  title={name || tr("structured.child")}
                  icon={
                    <Lock
                      className="h-5 w-5 text-gray-600"
                      aria-hidden="true"
                    />
                  }
                >
                  <StatusBadge label={tr("locked.badge")} tone="green" />
                  <InfoItem
                    label={tr("locked.birthday")}
                    value={
                      child.date_of_birth
                        ? formatDate(child.date_of_birth, false, activeLocale)
                        : "-"
                    }
                  />
                  {collectGradeLevel && child.target_grade_level ? (
                    <InfoItem
                      label={tr("locked.grade")}
                      value={tr("fields.grade", {
                        grade: child.target_grade_level,
                      })}
                    />
                  ) : null}
                  <InfoItem
                    label={tr("locked.care")}
                    value={
                      booked.length > 0
                        ? booked.join(", ")
                        : child.offering_ids.size > 0
                          ? "-"
                          : tr("locked.noCare")
                    }
                  />
                  <p className="border-moto-blue/30 bg-moto-blue/5 rounded-lg border px-3 py-2 text-sm leading-6 text-gray-700">
                    {tr("locked.hint", {
                      name: name || tr("structured.child"),
                    })}
                    <br />
                    {tr("locked.noAccess")}
                  </p>
                </InfoCard>
              );
            }
            return (
              <div
                key={child.clientId}
                className="moto-content-surface space-y-5 rounded-xl border p-3 shadow-sm sm:p-4"
              >
                <div className="flex items-center justify-between gap-3">
                  <h3 className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
                    {restrictToOfferings && child.first_name
                      ? `${child.first_name} ${child.last_name}`.trim()
                      : `${tr("structured.child")} ${i + 1}`}
                  </h3>
                  {children.length > 1 && !lockChildStructure && (
                    <Button
                      type="button"
                      variant="ghost"
                      size="compact"
                      onClick={() => removeChild(i)}
                      className="text-moto-red-strong hover:bg-moto-red/10 hover:text-moto-red-strong"
                    >
                      {tr("actions.remove")}
                    </Button>
                  )}
                </div>
                {!restrictToOfferings && (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <Input
                      id={`enrollment-children_${i}_first_name`}
                      label={tr("fields.firstName")}
                      name={`children_${i}_first_name`}
                      autoComplete="given-name"
                      controlSize="compact"
                      value={child.first_name}
                      onChange={(e) =>
                        updateChild(i, { first_name: e.target.value })
                      }
                      required
                      error={fieldError(`children_${i}_first_name`)}
                    />
                    <Input
                      id={`enrollment-children_${i}_last_name`}
                      label={tr("fields.lastName")}
                      name={`children_${i}_last_name`}
                      autoComplete="family-name"
                      controlSize="compact"
                      value={child.last_name}
                      onChange={(e) =>
                        updateChild(i, { last_name: e.target.value })
                      }
                      required
                      error={fieldError(`children_${i}_last_name`)}
                    />
                    <div>
                      <FieldLabel
                        required
                        invalid={Boolean(
                          fieldError(`children_${i}_date_of_birth`),
                        )}
                      >
                        {tr("fields.dateOfBirth")}
                      </FieldLabel>
                      <DateOfBirthPicker
                        value={child.date_of_birth}
                        onChange={(v) => updateChild(i, { date_of_birth: v })}
                        error={fieldError(`children_${i}_date_of_birth`)}
                        tr={tr}
                      />
                    </div>
                    {collectGradeLevel && (
                      <GradeLevelSelect
                        id={`children-${i}-target-grade-level`}
                        value={child.target_grade_level}
                        onChange={(v) => {
                          const nextOfferings = availableCareOfferings(
                            offerings,
                            v,
                          );
                          const nextAvailableIDs = new Set(
                            nextOfferings.map((o) => o.id),
                          );
                          const before = materializeCareOfferingsForChild(
                            child,
                            childOfferings,
                          ).offeringIds;
                          const nextIDs = new Set(
                            [...child.offering_ids].filter((id) =>
                              nextAvailableIDs.has(id),
                            ),
                          );
                          nextOfferings
                            .filter((o) => o.is_required)
                            .forEach((o) => nextIDs.add(o.id));
                          const nextDays = Object.fromEntries(
                            Object.entries(child.offering_days).filter(([id]) =>
                              nextAvailableIDs.has(id),
                            ),
                          );
                          const removed = [...before].some(
                            (id) => !nextAvailableIDs.has(id),
                          );
                          setChildren((prev) =>
                            prev.map((entry, index) =>
                              index === i
                                ? {
                                    ...entry,
                                    target_grade_level: v,
                                    target_school_class:
                                      entry.target_school_class &&
                                      !classMatchesGrade(
                                        entry.target_school_class,
                                        v,
                                      )
                                        ? ""
                                        : entry.target_school_class,
                                    offering_ids: nextIDs,
                                    offering_days: nextDays,
                                  }
                                : entry,
                            ),
                          );
                          setAvailabilityNotices((prev) =>
                            removed
                              ? { ...prev, [child.clientId]: true }
                              : Object.fromEntries(
                                  Object.entries(prev).filter(
                                    ([key]) => key !== child.clientId,
                                  ),
                                ),
                          );
                        }}
                        max={gradeLevelMax}
                        allowedGrades={eligibleGradeLevels}
                        error={fieldError(`children_${i}_target_grade_level`)}
                        tr={tr}
                      />
                    )}
                    {collectGradeLevel &&
                      collectsConcreteClassForGrade(
                        schoolClassConfig,
                        child.target_grade_level,
                      ) &&
                      (() => {
                        // Options are filtered to this child's grade; a phase may
                        // require classes yet offer none for this grade, in which
                        // case the pick cannot be mandatory (no valid option to
                        // choose), so "Klasse offen" stays available (#1833).
                        const gradeClasses =
                          schoolClassConfig.available_classes.filter((c) =>
                            classMatchesGrade(c, child.target_grade_level),
                          );
                        return (
                          <SchoolClassSelect
                            id={`children-${i}-target-school-class`}
                            value={child.target_school_class}
                            onChange={(v) =>
                              updateChild(i, { target_school_class: v })
                            }
                            classes={gradeClasses}
                            required={
                              schoolClassConfig.require &&
                              gradeClasses.length > 0
                            }
                            error={fieldError(
                              `children_${i}_target_school_class`,
                            )}
                            tr={tr}
                          />
                        );
                      })()}
                  </div>
                )}
                {availabilityNotices[child.clientId] ? (
                  <p
                    role="status"
                    className="border-moto-amber/50 bg-moto-amber/10 text-moto-amber-strong rounded-lg border px-3 py-2 text-sm"
                  >
                    {tr("care.availabilitySelectionRemoved")}
                  </p>
                ) : null}

                {showCareSection && (
                  <div className="space-y-4">
                    {childOfferings.length === 0 &&
                      blockedOfferings.length === 0 && (
                        <p className="text-sm font-semibold text-gray-700">
                          {tr("care.offers")}
                        </p>
                      )}
                    {offeringsAwaitGrade && (
                      <Alert
                        type="info"
                        message={tr("care.chooseGradeFirst")}
                      />
                    )}
                    {noOfferingsForGrade && (
                      <Alert
                        type="info"
                        message={tr("care.noOfferingsForGrade", {
                          grade: child.target_grade_level,
                        })}
                      />
                    )}
                    {/* Required offerings are part of the enrollment regardless of
                    the parent's choice. They are presented as an "always
                    included" block - locked, no checkbox-style selection - so
                    they never read as "something I picked". The selection mode
                    and its "choose at least one" requirement live entirely in
                    the choosable block below. */}
                    {childOfferings.some((o) => o.is_required) && (
                      <div>
                        <p className="text-sm font-semibold text-gray-700">
                          {tr("care.requiredTitle")}
                        </p>
                        <p className="mt-1 text-xs leading-5 text-gray-500">
                          {tr("care.requiredDescription")}
                        </p>
                        <div className="mt-2 space-y-2">
                          {childOfferings
                            .filter((o) => o.is_required)
                            .map((o) => (
                              <OfferingCard
                                key={o.id}
                                offering={o}
                                childIndex={i}
                                checked
                                selectedDays={
                                  materializedCare.offeringDays[o.id]
                                }
                                automaticDays={
                                  materializedCare.automaticDays[o.id]
                                }
                                toggleLocked
                                dayError={
                                  offeringDayErrors[`${i}_${o.id}`] ?? false
                                }
                                stats={offeringBookingStats?.[o.id]}
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
                    {childOfferings.some((o) => !o.is_required) && (
                      <div>
                        <p className="text-sm font-semibold text-gray-700">
                          {childOfferings.some((o) => o.is_required)
                            ? tr("care.additional")
                            : tr("care.offers")}
                          {careOfferingSelectionMode !== "optional" && (
                            <RequiredMark />
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
                          aria-invalid={
                            childOfferingErrors[i] ? true : undefined
                          }
                          className={`mt-2 space-y-2 ${
                            childOfferingErrors[i]
                              ? "border-moto-red bg-moto-red/5 rounded-md border p-2"
                              : ""
                          }`}
                        >
                          {groupOfferings(
                            childOfferings.filter((o) => !o.is_required),
                          ).map((bucket) =>
                            bucket.kind === "single" ? (
                              <OfferingCard
                                key={bucket.offering.id}
                                offering={bucket.offering}
                                childIndex={i}
                                checked={materializedCare.offeringIds.has(
                                  bucket.offering.id,
                                )}
                                selectedDays={
                                  materializedCare.offeringDays[
                                    bucket.offering.id
                                  ]
                                }
                                automaticDays={
                                  materializedCare.automaticDays[
                                    bucket.offering.id
                                  ]
                                }
                                toggleLocked={
                                  Boolean(
                                    materializedCare.automaticDays[
                                      bucket.offering.id
                                    ],
                                  ) &&
                                  !child.offering_ids.has(bucket.offering.id)
                                }
                                dayError={
                                  offeringDayErrors[
                                    `${i}_${bucket.offering.id}`
                                  ] ?? false
                                }
                                stats={
                                  offeringBookingStats?.[bucket.offering.id]
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
                                aria-invalid={
                                  groupDayConflicts(bucket.group).length > 0
                                    ? true
                                    : undefined
                                }
                                className={`rounded-lg border p-2 ${
                                  groupDayConflicts(bucket.group).length > 0
                                    ? "border-moto-red bg-moto-red/5"
                                    : "border-gray-200 bg-gray-50/60"
                                }`}
                              >
                                <div className="mb-2 flex flex-wrap items-center gap-2 px-1">
                                  <span className="text-xs font-semibold text-gray-800">
                                    {selectionGroupLabel(
                                      offerings,
                                      bucket.group,
                                      contentLocale,
                                    )}
                                  </span>
                                  {offeringRuleHint(bucket.rule, tr) && (
                                    <span className="bg-moto-blue/10 text-moto-blue-hover rounded-full px-2 py-0.5 text-xs font-medium">
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
                                      checked={materializedCare.offeringIds.has(
                                        o.id,
                                      )}
                                      selectedDays={
                                        materializedCare.offeringDays[o.id]
                                      }
                                      automaticDays={
                                        materializedCare.automaticDays[o.id]
                                      }
                                      toggleLocked={
                                        Boolean(
                                          materializedCare.automaticDays[o.id],
                                        ) && !child.offering_ids.has(o.id)
                                      }
                                      dayError={
                                        offeringDayErrors[`${i}_${o.id}`] ??
                                        false
                                      }
                                      stats={offeringBookingStats?.[o.id]}
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
                                {groupDayConflicts(bucket.group).map(
                                  (conflict) => (
                                    <p
                                      key={conflict.day}
                                      className="text-moto-red mt-2 px-1 text-xs"
                                    >
                                      {dayConflictMessage(
                                        conflict,
                                        childOfferings,
                                        tr,
                                        contentLocale,
                                      )}
                                    </p>
                                  ),
                                )}
                              </div>
                            ),
                          )}
                        </div>
                        {childOfferingErrors[i] && (
                          <p className="text-moto-red mt-1 text-xs">
                            {careOfferingSelectionMode === "exactly_one"
                              ? tr("care.chooseExactlyOne")
                              : tr("care.chooseAtLeastOne")}
                          </p>
                        )}
                      </div>
                    )}
                    {blockedOfferings.length > 0 && (
                      <div>
                        <p className="text-sm font-semibold text-gray-700">
                          Für dieses Kind nicht wählbar
                        </p>
                        <p className="mt-1 text-xs leading-5 text-gray-500">
                          Diese Angebote sind durch eine Verfügbarkeitsregel im
                          Angebots-Katalog eingeschränkt. Um sie freizugeben,
                          passe die Regel dort an.
                        </p>
                        <div className="mt-2 space-y-2">
                          {blockedOfferings.map((offering) => (
                            <BlockedOfferingCard
                              key={offering.id}
                              offering={offering}
                              gradeLevel={child.target_grade_level}
                              gradeLevelMax={gradeLevelMax}
                              stats={offeringBookingStats?.[offering.id]}
                            />
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {!restrictToOfferings &&
                  (schema?.fields ?? [])
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
                          companionNote={
                            f.target === "student.allowed_departure_modes"
                              ? (child.custom[DEPARTURE_COMPANION_KEY] as
                                  string | undefined)
                              : undefined
                          }
                          onCompanionNoteChange={
                            f.target === "student.allowed_departure_modes"
                              ? (v) =>
                                  updateChild(i, {
                                    custom: {
                                      ...child.custom,
                                      [DEPARTURE_COMPANION_KEY]: v,
                                    },
                                  })
                              : undefined
                          }
                          inputName={`children_${i}_custom_${f.key}`}
                          error={fieldError(`children_${i}_custom_${f.key}`)}
                          companionNoteName={`children_${i}_departure_companion_note`}
                          companionNoteError={
                            f.target === "student.allowed_departure_modes"
                              ? fieldError(
                                  `children_${i}_departure_companion_note`,
                                )
                              : undefined
                          }
                          relevantDays={relevantCareDaysForChild(
                            child,
                            childOfferings,
                            previewMode,
                            careOfferingGradeForChild(child),
                            offerings.length > 0,
                          )}
                          careConstrained={careDaysAreConstrained(
                            offerings,
                            previewMode,
                          )}
                          singleMode={
                            f.type === "weekday_multi_mode" &&
                            singleModeApplies(f, child.target_grade_level)
                          }
                          tr={tr}
                        />
                      ),
                    )}
              </div>
            );
          })}
        </section>

        {legalBlocks.length > 0 && !restrictToOfferings && (
          <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
            <SectionHeading
              title={tr("sections.consentTitle")}
              description={tr("sections.consentDescription")}
            />
            {localizedCopy && activeLocale !== DEFAULT_LOCALE ? (
              <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs leading-5 text-gray-600">
                {tr("legalGermanNotice")}
              </p>
            ) : null}
            {legalBlocks.map((block) => (
              <div key={block.key}>
                {block.kind === "notice" ? (
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
                  <EnrollmentConsent
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
                    error={fieldError(`consent_${block.key}`)}
                    legalText={block.text}
                    legalTitle={block.title}
                    onViewLegal={
                      block.text ? () => setOpenLegalDoc(block) : undefined
                    }
                    tr={tr}
                  />
                )}
                <AttachmentList attachments={block.attachments} />
              </div>
            ))}
          </section>
        )}

        {signatureRequired && (
          <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
            <SectionHeading
              title={tr("signature.title")}
              description={tr("signature.description")}
            />
            <SignaturePad
              id={SIGNATURE_FIELD}
              value={signature}
              onChange={changeSignature}
              disabled={submitting || previewMode}
              // A confirmed signature answers the missing-signature hint.
              error={signature ? undefined : fieldError(SIGNATURE_FIELD)}
              signerNameHint={
                `${guardianFirstName} ${guardianLastName}`.trim() || undefined
              }
              labels={signatureLabels}
            />
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
        - the school enabled captcha and the config carries a site key (the server requires the
          platform's TURNSTILE_SITE_KEY at startup).
        When require_captcha=true but no site_key arrives, we still
        render a warning to the parent and disable submit; failing
        closed is safer than letting bots through silently.
        The hidden input ships the token along with the form post.
      */}
        {captchaRequired && captchaConfig.site_key && (
          <section aria-labelledby="captcha-heading" className="space-y-2">
            <h2 id="captcha-heading" className="sr-only">
              Sicherheitsprüfung
            </h2>
            <Turnstile
              siteKey={captchaConfig.site_key}
              options={{ theme: "light" }}
              onSuccess={(token: string) => setCaptchaToken(token)}
              onExpire={() => setCaptchaToken("")}
              onError={() => setCaptchaToken("")}
            />
            <input type="hidden" name="captcha_token" value={captchaToken} />
          </section>
        )}
        <Button
          type="submit"
          variant="primary"
          size="lg"
          disabled={
            submitting ||
            previewMode ||
            (captchaRequired && (!captchaConfig.site_key || !captchaToken))
          }
          className="w-full disabled:cursor-not-allowed"
        >
          {previewMode
            ? tr("actions.previewSubmit")
            : submitting
              ? tr("actions.submitting")
              : (submitLabel ?? tr("actions.submit"))}
        </Button>
      </form>
    </EnrollmentUploadScope>
  );
}

// The signature field's name; the backend names the same field `signature`.
const SIGNATURE_FIELD = "signature";

// Monotonic source for brand-new ChildDraft.clientId values. Saved edit-draft
// children derive their keys from the persisted child id so rebuilding the
// same draft after offerings load does not remount each card.
let childDraftSeq = 0;
function nextChildClientId(): string {
  return `child-${childDraftSeq++}`;
}

function savedChildClientId(childID: string): string {
  return `draft-child:${childID}`;
}

function blankChild(requiredOfferingIDs: readonly string[] = []): ChildDraft {
  return {
    clientId: nextChildClientId(),
    first_name: "",
    last_name: "",
    date_of_birth: "",
    target_grade_level: "",
    target_school_class: "",
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
    const dayRows = new Map(
      (child.offering_days ?? []).map((row) => [row.offering_id, row]),
    );
    const offeringIDs = new Set<string>(requiredOfferingIDs);
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
      clientId: savedChildClientId(child.id),
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

// sanitizeDraftSchoolClasses collapses any hydrated target_school_class the
// current phase no longer offers (admin renamed or removed it, or it belongs
// to another grade) back to "" ("Klasse offen"). An edit draft carries the
// class value the request was last saved with; if the phase's list changed
// since, that stale value has no matching dropdown option, passes the client's
// required-only check, and is then rejected by the backend validator as "not
// offered by this phase" — blocking otherwise valid edits. Mirrors the
// profile-adoption guard (#1833). Returns the same array reference when
// nothing changed so the reconcile effect bails out.
function sanitizeDraftSchoolClasses(
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

// Add mandatory offerings that apply to each child's visible grade after the
// catalog loads. Conditional required offerings stay absent while their grade
// input is empty and are added by the grade-change handler once applicable.
function seedRequiredAvailableOfferings(
  children: ChildDraft[],
  offerings: readonly PublicCareOffering[],
  collectGradeLevel: boolean,
): ChildDraft[] {
  return children.map((c) => {
    if (c.locked) return c;
    const nextIDs = new Set(c.offering_ids);
    availableCareOfferings(
      offerings,
      collectGradeLevel ? c.target_grade_level : undefined,
    )
      .filter((offering) => offering.is_required)
      .forEach((offering) => nextIDs.add(offering.id));
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
// in backend/modules/enrollment/selection/materialize.go.
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

/**
 * The input `name` for a field the backend names by its JSON path (#2515):
 * `children.0.first_name` → `children_0_first_name`,
 * `children.0.custom_data.allergies` → `children_0_custom_allergies`,
 * `custom_data.x` → `custom_x`, `consent_flags.x` → `consent_x`,
 * `additional_guardians.1.email` → `additional_guardian_1_email`.
 */
export function submitFieldName(path: string): string {
  return path
    .replace(
      /^children\.(\d+)\.custom_data\.student\.departure_companion_note$/,
      "children.$1.departure_companion_note",
    )
    .replace(/^additional_guardians\./, "additional_guardian.")
    .replace(/(^|\.)custom_data\./, "$1custom.")
    .replace(/^consent_flags\./, "consent.")
    .replaceAll(".", "_");
}

// offeringGroupRuleError validates one child's selection against every
// selection_group's rule. Returns a German error message, or null when the
// selection is valid. The backend re-checks the same in
// validateOfferingGroupRules (defense-in-depth).
function offeringGroupRuleError(
  materialized: Pick<MaterializedCareSelection, "offeringIds" | "offeringDays">,
  offerings: PublicCareOffering[],
  tr: EnrollmentFormTranslator,
  locale: string,
): string | null {
  const selectedOfferingIds = materialized.offeringIds;
  const [dayConflict] = careOfferingDayConflicts(
    offerings,
    selectedOfferingIds,
    materialized.offeringDays,
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
        selectedOfferingIds.has(o.id),
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

// dayConflictMessage names the day and the offerings of one conflict, so the
// family sees what to change without the form changing it for them (#4514).
function dayConflictMessage(
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

function offeringRuleHint(rule: string, tr: TranslationFn): string {
  if (rule === AT_MOST_ONE_PER_DAY) return tr("care.ruleAtMostOnePerDay");
  if (rule === "exactly_one") return tr("care.ruleExactlyOne");
  if (rule === "at_least_one") return tr("care.ruleAtLeastOne");
  if (rule === "at_most_one") return tr("care.ruleAtMostOne");
  return "";
}

// OfferingCard renders one care-offering row. Required offerings show as a
// locked "always included" card (gray, lock indicator) so they never read as
// a parent's pick; choosable offerings keep the interactive green-check
// checkbox. The day picker stays available for parent_choice offerings in
// both cases.
/**
 * Occupancy line under an offering in ADMIN flows (#2186). Renders nothing
 * without stats, so the parent form and a failed stats load look exactly as
 * they did before.
 */
function OfferingOccupancyLine({
  stats,
}: Readonly<{ stats: CareOfferingBookingStats | undefined }>) {
  const label = formatOfferingOccupancy(stats);
  if (!label) return null;
  return (
    <div
      className={`mt-1 text-xs ${
        offeringIsFull(stats) ? "text-moto-amber-strong" : "text-gray-500"
      }`}
    >
      {label}
    </div>
  );
}

/**
 * An offering an availability rule rules out for this child, shown greyed
 * with the reason. Admin flows only — for parents, silently omitting an
 * unavailable offering is the correct behaviour (#2186).
 */
function BlockedOfferingCard({
  offering,
  gradeLevel,
  gradeLevelMax,
  stats,
}: Readonly<{
  offering: PublicCareOffering;
  gradeLevel: string;
  gradeLevelMax: number;
  stats: CareOfferingBookingStats | undefined;
}>) {
  const reason = careOfferingAvailabilityReason(
    offering,
    gradeLevel,
    gradeLevelMax,
  );
  return (
    <OfferingRowShell tone={BLOCKED_OFFERING_ROW_TONE}>
      <div className="flex items-start gap-3">
        <span
          aria-hidden
          className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-gray-300 bg-gray-100 text-gray-400"
        >
          <Lock className="h-3 w-3" />
        </span>
        <div className="min-w-0 flex-1">
          <span className="font-medium break-words text-gray-500">
            {offering.name}
          </span>
          {reason ? (
            <div className="mt-1 text-xs text-gray-500">{reason}</div>
          ) : null}
          <OfferingOccupancyLine stats={stats} />
        </div>
      </div>
    </OfferingRowShell>
  );
}

function OfferingCard({
  offering,
  childIndex,
  checked,
  selectedDays,
  automaticDays,
  toggleLocked = false,
  dayError,
  stats,
  onToggle,
  onToggleDay,
  tr,
}: {
  readonly offering: PublicCareOffering;
  readonly childIndex: number;
  readonly checked: boolean;
  readonly selectedDays: Set<string> | undefined;
  readonly automaticDays?: Set<string>;
  readonly toggleLocked?: boolean;
  readonly dayError: boolean;
  /** Admin-only occupancy (#2186); undefined on the parent form. */
  readonly stats?: CareOfferingBookingStats;
  readonly onToggle: () => void;
  readonly onToggleDay: (day: string) => void;
  readonly tr: EnrollmentFormTranslator;
}) {
  const required = offering.is_required;
  const effectiveChecked = required || checked;
  const inputLocked = required || toggleLocked;
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  const autoIncluded =
    !required &&
    toggleLocked &&
    offering.available_days.some((day) => automaticDays?.has(day) ?? false);
  let stateClass = "border-gray-200 bg-white hover:border-gray-300";
  if (dayError) {
    stateClass = "border-moto-red bg-moto-red/5";
  } else if (autoIncluded || checked) {
    stateClass = "border-moto-green/40 bg-moto-green/10";
  } else if (required || toggleLocked) {
    stateClass = "border-gray-200 bg-gray-50";
  }
  const inputId = `children-${childIndex}-offering-${offering.id}`;

  return (
    <OfferingRowShell tone={stateClass} invalid={dayError}>
      <label
        htmlFor={inputId}
        className={`flex items-start gap-3 ${
          inputLocked ? "cursor-default" : "cursor-pointer"
        }`}
      >
        {/* A locked row (required or added automatically) keeps the native
            box for assistive tech, but shows why it cannot be changed: a
            lock, or the green check of an automatic addition. */}
        <Checkbox
          id={inputId}
          name={`children_${childIndex}_offering_${offering.id}`}
          checked={effectiveChecked}
          disabled={inputLocked}
          onChange={onToggle}
          className={inputLocked ? "hidden" : "mt-0.5"}
        />
        {inputLocked ? (
          <span
            aria-hidden="true"
            className={`mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md ${
              autoIncluded
                ? "bg-moto-green-fill text-white"
                : "bg-gray-400 text-gray-900"
            }`}
          >
            {autoIncluded ? (
              <Check className="h-3.5 w-3.5" />
            ) : (
              <Lock className="h-3 w-3" />
            )}
          </span>
        ) : null}
        <div className="min-w-0 flex-1">
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
            {offering.available_days
              .map((d) => weekdayLabels[d] ?? d)
              .join(", ")}
            {offering.includes_holiday_care && ` · ${tr("care.holiday")}`}
            {offering.includes_lunch && ` · ${tr("care.lunch")}`}
          </div>
          <OfferingOccupancyLine stats={stats} />
        </div>
      </label>
      {effectiveChecked && offering.days_of_week_mode === "parent_choice" && (
        <div className="mt-2 ml-8 flex flex-wrap gap-1.5">
          {offering.available_days.map((day) => {
            const picked = selectedDays?.has(day) ?? false;
            const automatic = automaticDays?.has(day) ?? false;
            const selected = picked || automatic;
            return (
              <ToggleChip
                key={day}
                pressed={selected}
                disabled={automatic}
                onPressedChange={() => {
                  if (automatic) return;
                  onToggleDay(day);
                }}
              >
                {weekdayLabels[day] ?? day}
              </ToggleChip>
            );
          })}
        </div>
      )}
      {dayError && (
        <p className="text-moto-red mt-1 ml-8 text-xs">
          {tr("errors.dayInline")}
        </p>
      )}
    </OfferingRowShell>
  );
}

function GradeLevelSelect({
  id,
  value,
  onChange,
  max,
  allowedGrades,
  error,
  tr,
}: {
  readonly id: string;
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly max: number;
  /**
   * Phase grade restriction (#1663); empty means unrestricted. Offering a
   * grade the submit gate rejects would let a parent complete the entire
   * form and then fail with grade_not_eligible, so the select presents the
   * eligible grades only — the same reason the class pick list is narrowed
   * server-side.
   */
  readonly allowedGrades: number[];
  readonly error?: string;
  readonly tr: EnrollmentFormTranslator;
}) {
  const grades = selectableGradeLevels(max, allowedGrades);
  return (
    <div>
      <FieldLabel
        htmlFor={id}
        id={`${id}-label`}
        required
        invalid={Boolean(error)}
      >
        {tr("fields.gradeLevel")}
      </FieldLabel>
      <CustomSelect
        ariaLabelledBy={`${id}-label`}
        id={id}
        value={value}
        required
        onChange={onChange}
        invalid={Boolean(error)}
        options={[
          { value: "", label: tr("fields.choose"), disabled: true },
          ...grades.map((grade) => ({
            value: String(grade),
            label: tr("fields.grade", { grade }),
          })),
        ]}
      />
      {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
    </div>
  );
}

// selectableGradeLevels is the list of grades the grade select actually
// offers: every grade up to the tenant cap, narrowed by the phase's
// restriction (#1663). Callers that PREFILL a grade must check against this
// list too — a value the select does not offer is invisible in the UI yet
// still submitted, and the backend then rejects the whole form with
// grade_not_eligible.
//
// A restriction naming only grades above the tenant cap is broken config (the
// cap was lowered after the phase was set up). Fall back to the full range
// rather than rendering an empty select; the server still rejects the
// submission with the authoritative error.
function selectableGradeLevels(max: number, allowedGrades: number[]): number[] {
  const everyGrade = Array.from({ length: max }, (_, n) => n + 1);
  const eligible = everyGrade.filter((grade) => allowedGrades.includes(grade));
  return allowedGrades.length > 0 && eligible.length > 0
    ? eligible
    : everyGrade;
}

// schoolClassGradePrefix returns the first run of digits in a school class
// name ("2a" -> "2", "Klasse 12b" -> "12"), matching getSchoolYear and the
// backend schoolclass helper.
function schoolClassGradePrefix(schoolClass: string): string {
  const match = /(\d+)/.exec(schoolClass);
  return match?.[1] ?? "";
}

// classMatchesGrade decides whether a concrete class may be offered to a
// child in the given grade. The phase's pick list mixes classes from
// every grade ("2a", "2b", "3a") and the whole list is sent for every
// grade, so a grade-2 child must not see "3a" — the backend validator
// rejects that mismatch on submit. Classes without a numeric prefix carry
// no derivable grade and stay available for every grade (#1833).
function classMatchesGrade(schoolClass: string, gradeLevel: string): boolean {
  const prefix = schoolClassGradePrefix(schoolClass);
  return prefix === "" || prefix === gradeLevel.trim();
}

// collectsConcreteClassForGrade decides whether the concrete-class field is
// shown/collected for a child of the given grade. Grade >= 2 is unchanged
// (#1833): collected whenever the phase offers a class matching the grade,
// prefixless classes included. Grade 1 is opt-in (#1663): the backend decides
// it (CollectsGrade1Class -> `collect_grade_1`), because a phase restricted to
// a prefixless class ("Bienen") collects a grade-1 class too and the narrowed
// pick list alone cannot say so. The prefix-only rule stays as the fallback
// for a payload without the field, keeping the pre-#1663 behaviour: a phase
// that never added a grade-1 class leaves grade 1 grade-level-only.
function collectsConcreteClassForGrade(
  config: PublicSchoolClassConfig,
  gradeLevel: string,
): boolean {
  if (!config.collect) return false;
  const grade = Number(gradeLevel);
  if (!Number.isFinite(grade) || grade < 1) return false;
  if (grade >= 2) {
    return config.available_classes.some((cls) =>
      classMatchesGrade(cls, gradeLevel),
    );
  }
  if (config.collect_grade_1 === true) return true;
  return config.available_classes.some(
    (cls) => schoolClassGradePrefix(cls) === "1",
  );
}

// SchoolClassSelect renders the concrete-class dropdown shown for grade
// >= 2 when the tenant collects it (#1833). Options come from the phase's
// admin-managed list. When the phase does not require a class, a "Klasse
// offen" option (value "") lets the parent leave it unassigned; when it
// is required, the empty option is a disabled placeholder instead.
function SchoolClassSelect({
  id,
  value,
  onChange,
  classes,
  required,
  error,
  tr,
}: {
  readonly id: string;
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly classes: readonly string[];
  readonly required: boolean;
  readonly error?: string;
  readonly tr: EnrollmentFormTranslator;
}) {
  return (
    <div>
      <FieldLabel
        htmlFor={id}
        id={`${id}-label`}
        required={required}
        invalid={Boolean(error)}
      >
        {tr(required ? "fields.schoolClassRequired" : "fields.schoolClass")}
      </FieldLabel>
      <CustomSelect
        ariaLabelledBy={`${id}-label`}
        id={id}
        value={value}
        required={required}
        onChange={onChange}
        invalid={Boolean(error)}
        options={[
          required
            ? { value: "", label: tr("fields.choose"), disabled: true }
            : { value: "", label: tr("fields.schoolClassOpen") },
          ...classes.map((c) => ({ value: c, label: c })),
        ]}
      />
      {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
    </div>
  );
}

function SectionHeading({
  title,
  description,
}: {
  readonly title: string;
  readonly description: string;
}) {
  return (
    <div>
      <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
    </div>
  );
}

function customValueMissing(
  field: PublicFormSchema["fields"][number],
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
    // Care-offering-constrained: the shown days are exactly the child's care
    // days, so each one needs a time -- an empty day is a missing answer, not
    // a "keine Angabe". Unconstrained (all weekdays shown): keep the historical
    // "at least one day" rule so children attending only some days aren't blocked.
    if (careConstrained) {
      return days.some((day) => (schedule[day] ?? "").trim() === "");
    }
    return !days.some((day) => (schedule[day] ?? "").trim() !== "");
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
  field: PublicFormSchema["fields"][number],
  tr: TranslationFn,
  careConstrained = false,
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
    // Care-constrained forms require a time on every shown care day, so the
    // generic "at least one time" prompt would mislead a parent who filled one.
    return tr(careConstrained ? "errors.scheduleEveryDay" : "errors.schedule");
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
  if (field.type === "file") {
    return tr("errors.file");
  }
  return tr("errors.required");
}

// singleModeApplies reports whether a field's Heimweg-Beschränkung (#2381)
// restricts the child's declared target grade. An unparseable or missing
// grade is never restricted — mirrors the backend's SingleModeAppliesTo.
function singleModeApplies(
  field: PublicFormSchema["fields"][number],
  targetGradeLevel: string,
): boolean {
  const grade = Number(targetGradeLevel);
  if (!Number.isInteger(grade)) return false;
  return (field.single_mode_grades ?? []).includes(grade);
}

function selectedCareDaysForChild(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
  gradeLevel: string | undefined,
): string[] {
  const materialized = materializeCareOfferings(child, offerings, gradeLevel);
  const days = new Set<string>();
  for (const id of materialized.offeringIds) {
    const offering = offerings.find((o) => o.id === id);
    if (!offering) continue;
    const selected =
      offering.days_of_week_mode === "parent_choice"
        ? Array.from(materialized.offeringDays[id] ?? new Set<string>())
        : offering.available_days;
    for (const day of selected) {
      if ((WEEKDAYS as readonly string[]).includes(day)) {
        days.add(day);
      }
    }
  }
  return WEEKDAYS.filter((day) => days.has(day));
}

interface MaterializedCareSelection {
  offeringIds: Set<string>;
  offeringDays: Record<string, Set<string>>;
  automaticDays: Record<string, Set<string>>;
}

function materializeCareOfferings(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
  gradeLevel: string | undefined,
): MaterializedCareSelection {
  const offeringIds = new Set(child.offering_ids);
  const offeringDays: Record<string, Set<string>> = {};
  const automaticDays: Record<string, Set<string>> = {};
  for (const [id, days] of Object.entries(child.offering_days)) {
    offeringDays[id] = new Set(days);
  }

  let changed = true;
  while (changed) {
    changed = false;
    for (const target of offerings) {
      if (!autoAddAppliesToGrade(gradeLevel, target)) continue;
      const triggerIDs = target.auto_add_trigger_offering_ids ?? [];
      if (triggerIDs.length === 0 && !isRequiredLunchOffering(target)) continue;

      const auto = new Set<string>();
      const targetDays = new Set(target.available_days);
      for (const triggerID of triggerIDs) {
        if (!offeringIds.has(triggerID)) continue;
        const trigger = offerings.find((offering) => offering.id === triggerID);
        if (!trigger) continue;
        const triggerDays =
          trigger.days_of_week_mode === "parent_choice"
            ? Array.from(offeringDays[triggerID] ?? new Set<string>())
            : trigger.available_days;
        for (const day of triggerDays) {
          if (targetDays.has(day)) auto.add(day);
        }
      }
      if (isRequiredLunchOffering(target)) {
        for (const source of offerings) {
          if (source.id === target.id || !(source.counts_as_care ?? true)) {
            continue;
          }
          if (!offeringIds.has(source.id)) {
            continue;
          }
          const sourceDays =
            source.days_of_week_mode === "parent_choice"
              ? Array.from(offeringDays[source.id] ?? new Set<string>())
              : source.available_days;
          for (const day of sourceDays) {
            if (targetDays.has(day)) auto.add(day);
          }
        }
      }
      if (auto.size === 0) continue;

      if (!offeringIds.has(target.id)) {
        offeringIds.add(target.id);
        changed = true;
      }
      if (!sameSet(automaticDays[target.id], auto)) {
        automaticDays[target.id] = auto;
        changed = true;
      }
      const merged = new Set(offeringDays[target.id] ?? []);
      const beforeSize = merged.size;
      for (const day of auto) merged.add(day);
      if (
        merged.size !== beforeSize ||
        !sameSet(offeringDays[target.id], merged)
      ) {
        offeringDays[target.id] = merged;
        changed = true;
      }
    }
  }

  return { offeringIds, offeringDays, automaticDays };
}

function sameSet(left: Set<string> | undefined, right: Set<string>): boolean {
  if (!left || left.size !== right.size) return false;
  for (const value of left) {
    if (!right.has(value)) return false;
  }
  return true;
}

function isRequiredLunchOffering(offering: PublicCareOffering): boolean {
  return (
    offering.is_required &&
    offering.includes_lunch &&
    offering.days_of_week_mode === "parent_choice"
  );
}

function autoAddAppliesToGrade(
  gradeValue: string | undefined,
  offering: PublicCareOffering,
): boolean {
  const gradeLevels = offering.auto_add_grade_levels ?? [];
  if (gradeLevels.length === 0) return true;
  const grade = Number(gradeValue);
  if (!Number.isFinite(grade) || grade <= 0) return false;
  return gradeLevels.includes(grade);
}

function relevantCareDaysForChild(
  child: ChildDraft,
  offerings: readonly PublicCareOffering[],
  previewMode: boolean,
  gradeLevel: string | undefined,
  hasOfferingCatalog = offerings.length > 0,
): string[] {
  // No offerings to derive care days from (care-offering feature unused, or
  // template preview): every weekday is relevant so day-scoped fields stay
  // usable. When offerings DO exist but the child has selected none, the
  // result is empty on purpose -- the parent must pick care days first.
  if (previewMode || !hasOfferingCatalog) {
    return [...WEEKDAYS];
  }
  return selectedCareDaysForChild(child, offerings, gradeLevel);
}

// Whether the visible weekdays are genuinely derived from selected care
// offerings (and not the unconstrained "show every weekday" fallback). Only
// then is a per-day time mandatory: the shown days ARE the child's care days,
// so an empty one is a real omission. Without offerings all five weekdays show
// and forcing a time for each would block children who attend only some days.
function careDaysAreConstrained(
  offerings: readonly PublicCareOffering[],
  previewMode: boolean,
): boolean {
  return !previewMode && offerings.length > 0;
}

function pruneWeekdayAnswers(
  data: Record<string, unknown>,
  fields: readonly PublicFormSchema["fields"][number][],
  relevantDays: readonly string[],
): Record<string, unknown> {
  const next = { ...data };
  for (const field of fields) {
    if (!(field.key in next)) continue;
    if (field.type === "weekday_multi_mode") {
      next[field.key] = asWeekdayMultiModeObject(
        next[field.key],
        relevantDays,
        departureMultiModeOptionsForField(field),
      );
    } else if (field.type === "weekday_schedule") {
      // Drop pickup times for days the child is no longer in care (e.g. an
      // offering was deselected after a time was entered) so stale entries
      // never reach the backend.
      const schedule = asScheduleObject(next[field.key]);
      const pruned: Record<string, string> = {};
      for (const day of relevantDays) {
        const time = schedule[day];
        if (time !== undefined) pruned[day] = time;
      }
      next[field.key] = pruned;
    }
  }
  return next;
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
    <div className="border-moto-green/40 bg-moto-green/5 rounded-lg border p-4">
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
              <Button
                type="button"
                variant="primary"
                size="compact"
                onClick={() => onAdopt(c)}
                disabled={used}
                className="shrink-0"
              >
                {used ? tr("structured.adopted") : tr("structured.adopt")}
              </Button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
