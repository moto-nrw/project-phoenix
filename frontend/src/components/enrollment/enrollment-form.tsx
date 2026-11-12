"use client";

import {
  DocumentAttachmentList,
  type AttachmentLabels,
} from "~/components/enrollment/attachment-list";
import { InsetPanel } from "~/components/ui/inset-panel";
import {
  EnrollmentUploadScope,
  useEnrollmentUploadTarget,
} from "~/components/enrollment/enrollment-file-upload";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
} from "react";
import { useLocale } from "next-intl";
import { Check, Lock, Plus } from "lucide-react";
import { Turnstile, type TurnstileInstance } from "@marsidev/react-turnstile";
import { DateOfBirthPicker } from "~/components/enrollment/date-of-birth-picker";
import { Button } from "~/components/ui/button";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel, RequiredMark } from "~/components/ui/field-label";
import { DataField, DataGrid } from "~/components/ui/data-field";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { Input } from "~/components/ui/input";
import { useTenant } from "~/lib/tenant-context";
import germanEnrollmentFormMessages from "~/i18n/messages/de/enrollmentForm.json";
import { DEFAULT_LOCALE } from "~/i18n/locales";
import {
  submitEnrollment,
  type MeProfileChild,
  type PublicCareOffering,
  type EnrollmentEditDraft,
  type SubmitEnrollmentPayload,
  type SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";
import {
  holidayCareWishAsked,
  type FormField,
  type PublicFormSchema,
  type PublicLegalBlock,
} from "~/lib/enrollment-form-schema-api";
import {
  EmailContactNotice,
  EnrollmentConsent,
  LegalMarkdown,
  resolvePreviewLegalTexts,
} from "~/components/enrollment/enrollment-form-consent";
import {
  CustomFieldInput,
  InfoBlock,
} from "~/components/enrollment/enrollment-form-custom-field";
import {
  asStringMap,
  DEPARTURE_COMPANION_KEY,
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
import {
  useCareFacilityTerm,
  useCareTermTranslations,
} from "~/lib/care-facility-term";
import {
  AT_MOST_ONE_PER_DAY,
  careOfferingDayConflicts,
} from "~/lib/care-offering-day-conflicts";
import {
  copyStableObjectKey,
  getStableObjectKey,
} from "~/lib/stable-object-key";
import {
  availableCareOfferings,
  careOfferingAvailabilityReason,
  careOfferingRuleExcludesNobody,
} from "~/lib/care-offering-availability";
import { type CareOfferingBookingStats } from "~/lib/care-offering-booking-stats";
import {
  OfferingOccupancyLine,
  OfferingRowShell,
  type OfferingRowState,
} from "~/components/enrollment/offering-row-shell";
import {
  classMatchesGrade,
  deriveChild,
  selectableGradeLevels,
  singleModeApplies,
  withRequiredOfferings,
  type ChildDerivation,
  type ChildDerivationContext,
  type ChildDraft,
} from "~/components/enrollment/enrollment-child-derivation";
import {
  blankChild,
  blankGuardian,
  formConfig,
  initialFormDraft,
  prefillGuardian,
  previewFormConfig,
  sanitizeSchoolClasses,
  type EnrollmentFormConfig,
  type EnrollmentFormPrefetchedData,
  type GuardianDraft,
  type PreviewCareOfferings,
} from "~/components/enrollment/enrollment-form-config";
import {
  fieldNameForServerPath,
  formFieldName,
} from "~/components/enrollment/enrollment-form-field-names";
import {
  childrenViolatingCareSelectionRule,
  dayConflictMessage,
  pickupTimeMarks,
  validateEnrollmentForm,
  type EnrollmentFormValues,
} from "~/components/enrollment/enrollment-form-validation";
import { buildEnrollmentPayload } from "~/components/enrollment/enrollment-form-payload";

const logger = createLogger({ component: "EnrollmentForm" });

// Student lifecycle statuses the backend counts as "enrolled"
// (peopledirectory.StudentStatusActive / StudentStatusPending, see
// ExistsEnrolledByNameAndBirthday). The existing_students audience accepts
// nothing else, so the reuse picker must not offer anything else either.
const ENROLLED_CHILD_STATUSES = new Set(["active", "pending"]);

/**
 * A caller that renames the submit button also says what its click does;
 * without both the button sends a new enrollment (#6643).
 */
type SubmitCopy =
  | { readonly submitLabel?: undefined; readonly submitTooltip?: undefined }
  | { readonly submitLabel: string; readonly submitTooltip: string };

type Props = SubmitCopy & PropsBase;

interface PropsBase {
  readonly phaseID?: string;
  readonly gradeLevelMax: number;
  readonly onSubmitted: (statusURL: string) => void;
  /** The phase's form data; without it the form loads a template preview. */
  readonly prefetchedData?: EnrollmentFormPrefetchedData;
  readonly lateInviteToken?: string;
  /** Status-link authorization for file uploads while editing a request. */
  readonly statusToken?: string;
  /** Authenticated parent-portal upload route for restricted phases. */
  readonly uploadEndpoint?: string;
  readonly previewMode?: boolean;
  readonly previewSchema?: PublicFormSchema | null;
  /**
   * STAFF PREVIEW ONLY (#6044). The offerings of one phase, loaded by the
   * staff page, for a preview without a public phase link: an inactive or
   * closed phase answers the public form with an error, so the school could
   * not see its offerings before parents could.
   */
  readonly previewOfferings?: PreviewCareOfferings;
  /**
   * Replaces the public submit, e.g. with the parent-scope endpoint. Returns
   * `null` when it stopped before sending and shows the reason itself.
   */
  readonly submitter?: (
    payload: SubmitEnrollmentPayload,
  ) => Promise<SubmitEnrollmentResult | null>;
  /**
   * Hides the captcha when the caller already authenticated the person
   * (parent session); the authenticated endpoint skips its verification.
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
   * Reduced Halbjahreswechsel flow (#2251): render per-child care offerings
   * and the holiday care question when the target form asks it.
   * Guardian data, child identity, custom fields and consents stay hidden
   * and travel unchanged from initialDraft. The change request captures
   * bookings and the current year's wish. Callers must lock child structure.
   */
  readonly restrictToOfferings?: boolean;
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

/**
 * Public enrollment form: guardian, 1..n children with their offerings,
 * consents, optional signature and captcha. Core fields (names, email,
 * birthday, grade) are fixed; the form template only adds custom fields.
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
  previewOfferings,
  submitter,
  skipCaptcha,
  localizedCopy = false,
  initialDraft,
  lockedGuardianEmail = false,
  lockChildStructure = false,
  restrictToOfferings = false,
  submitLabel,
  submitTooltip,
  showBlockedOfferings = false,
  offeringBookingStats,
  withoutSignature = false,
}: Props) {
  const intl = useCareTermTranslations("enrollmentForm");
  const careTerm = useCareFacilityTerm();
  const activeLocale = useLocale();
  // School-written texts follow the same opt-in as the form chrome: staff
  // previews stay German even when a parent locale cookie exists (#3377).
  const contentLocale = localizedCopy ? activeLocale : DEFAULT_LOCALE;
  const tr = useMemo(
    () => makeEnrollmentFormTranslator(intl, localizedCopy, careTerm),
    [intl, localizedCopy, careTerm],
  );
  // The staff portal loads no parent messages: the attachment buttons take
  // their German labels from the form's own translator.
  const attachmentLabels = useMemo<AttachmentLabels | undefined>(
    () =>
      localizedCopy
        ? undefined
        : {
            open: tr("attachments.open"),
            download: tr("attachments.download"),
            openLabel: (label) => tr("attachments.openLabel", { label }),
            downloadLabel: (label) =>
              tr("attachments.downloadLabel", { label }),
            openHint: tr("attachments.openHint"),
            downloadHint: tr("attachments.downloadHint"),
            popupBlocked: tr("attachments.popupBlocked"),
            errorObject: tr("attachments.errorObject"),
          },
    [localizedCopy, tr],
  );
  const loadErrorObject = tr("errorObject");
  // A template preview without phase offerings shows every weekday; with the
  // staff page's offerings (#6044) the care days follow them as for parents.
  const templatePreview =
    previewMode &&
    !phaseID &&
    prefetchedData === undefined &&
    previewOfferings === undefined;
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

  const [initial] = useState(() => {
    const startConfig = formConfig(prefetchedData);
    return {
      config: startConfig,
      draft: initialFormDraft(startConfig, initialDraft),
    };
  });
  const [config, setConfig] = useState<EnrollmentFormConfig>(initial.config);
  const {
    collectGradeLevel,
    careOfferingsEnabled,
    careOfferingSelectionMode,
    eligibleGradeLevels,
    captchaConfig,
    profile,
  } = config;
  const schoolClassConfig = config.schoolClass;
  // The loaded texts stay German in config; what the form renders is
  // derived per language, so switching the language needs no reload (#3377).
  const schema = useMemo(
    () =>
      config.schema && {
        ...config.schema,
        fields: localizeFormFields(config.schema.fields, contentLocale),
      },
    [config.schema, contentLocale],
  );
  const offerings = useMemo(
    () => localizeOfferings(config.offerings, contentLocale),
    [config.offerings, contentLocale],
  );
  const legalBlocks = useMemo(
    () => localizeLegalBlocks(config.legalTexts?.blocks ?? [], contentLocale),
    [config.legalTexts, contentLocale],
  );
  // The holiday care question is a fixed field of the form (#4703), not a
  // custom field: its answer has its own column and counts for the
  // holiday registration of the Träger once the child is approved.
  const askHolidayCareWish = holidayCareWishAsked(config.schema);
  const holidayCareWishField = useMemo<FormField>(
    () => ({
      key: "holiday_care_wish",
      label: tr("fields.holidayCareWish"),
      help_text: tr("fields.holidayCareWishHint"),
      type: "boolean",
      required: true,
      sort_order: 0,
      applies_to_child: true,
    }),
    [tr],
  );
  // A "new_students" phase rejects any already-enrolled child at submit
  // (#1663), so the prefilled linked children get a notice instead of the
  // reuse panel. Reuse only renews a child on an "existing_students"
  // phase; elsewhere approval would create a duplicate student.
  const isNewStudentsPhase = config.audience === "new_students";
  const isExistingStudentsPhase = config.audience === "existing_students";

  const [availabilityNotices, setAvailabilityNotices] = useState<
    Record<string, boolean>
  >({});
  const [loading, setLoading] = useState(prefetchedData === undefined);
  const [submitting, setSubmitting] = useState(false);
  const [childOfferingErrors, setChildOfferingErrors] = useState<
    Record<number, boolean>
  >({});
  // `${childIndex}_${offeringId}` of a selected parent_choice offering
  // without a day, so that row gets the mark and the focus jump.
  const [offeringDayErrors, setOfferingDayErrors] = useState<
    Record<string, boolean>
  >({});
  // One display path for every failure (#2515): the alert above the form,
  // the fields marked via `fieldError(name)` (formFieldName), and the focus
  // on the first marked field.
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const { fieldError } = formError;
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [loadAttempt, setLoadAttempt] = useState(0);

  const [guardian, setGuardian] = useState<GuardianDraft>(
    initial.draft.guardian,
  );
  const updateGuardianField = (patch: Partial<GuardianDraft>) =>
    setGuardian((prev) => ({ ...prev, ...patch }));
  // The parent's signature (#4786); null until the parent confirms it in
  // the pad. A parent edit starts empty: every version is signed again.
  const [signature, setSignature] = useState<SignatureValue | null>(null);
  const signatureJustConfirmed = useRef(false);
  const [legalConsents, setLegalConsents] = useState<Record<string, boolean>>(
    initial.draft.legalConsents,
  );
  const [children, setChildren] = useState<ChildDraft[]>(
    initial.draft.children,
  );
  // Co-guardians beyond the primary guardian; opt-in.
  const [additionalGuardians, setAdditionalGuardians] = useState<
    GuardianDraft[]
  >(initial.draft.additionalGuardians);
  // Request-level answers (applies_to_child=false), shared by all children.
  const [customData, setCustomData] = useState<Record<string, unknown>>(
    initial.draft.customData,
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
    guardian,
    legalConsents,
    children,
    additionalGuardians,
    customData,
  ]);

  const applyConfig = useCallback((next: EnrollmentFormConfig) => {
    setConfig(next);
    setChildren((prev) => {
      const seeded = prev.map((child) =>
        withRequiredOfferings(child, next.offerings, next.collectGradeLevel),
      );
      return sanitizeSchoolClasses(
        seeded.every((child, i) => child === prev[i]) ? prev : seeded,
        next.schoolClass,
      );
    });
    setGuardian((prev) => prefillGuardian(prev, next));
  }, []);

  // New phase data after mount (the initial data seeded the state above).
  const appliedPrefetch = useRef(prefetchedData);
  useEffect(() => {
    if (prefetchedData === undefined) return;
    if (appliedPrefetch.current === prefetchedData) return;
    appliedPrefetch.current = prefetchedData;
    applyConfig(formConfig(prefetchedData));
    clearLoadError();
    setLoading(false);
  }, [prefetchedData, applyConfig, clearLoadError]);

  // A new edit draft replaces the values; the first one seeded the state.
  const appliedDraft = useRef(initialDraft);
  useEffect(() => {
    if (!initialDraft || appliedDraft.current === initialDraft) return;
    appliedDraft.current = initialDraft;
    const draft = initialFormDraft(config, initialDraft);
    setGuardian(draft.guardian);
    setLegalConsents(draft.legalConsents);
    setChildren(draft.children);
    setAdditionalGuardians(draft.additionalGuardians);
    setCustomData(draft.customData);
  }, [initialDraft, config]);

  // Preview: no phase, so only the legal texts load; the offerings come from
  // the staff page (#6044) or stay empty. A real load failure stops the form
  // instead of collecting consent without the texts.
  useEffect(() => {
    if (prefetchedData !== undefined) return;
    let cancelled = false;
    setLoading(true);
    clearLoadError();
    const schemaToPreview = previewSchema ?? null;
    resolvePreviewLegalTexts(tenantSlug, schemaToPreview)
      .then((legalTexts) => {
        if (!cancelled) {
          applyConfig(
            previewFormConfig(schemaToPreview, legalTexts, previewOfferings),
          );
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("enrollment_form_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          code: (err as Partial<ApiError> | undefined)?.code,
        });
        void showLoadError(err, {
          object: loadErrorObject,
          retry: () => setLoadAttempt((n) => n + 1),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [
    tenantSlug,
    prefetchedData,
    previewSchema,
    previewOfferings,
    loadErrorObject,
    loadAttempt,
    applyConfig,
    clearLoadError,
    showLoadError,
  ]);

  const captchaRef = useRef<TurnstileInstance>(undefined);
  const [captchaToken, setCaptchaToken] = useState("");
  const captchaRequired = !skipCaptcha && captchaConfig?.enabled === true;
  // A failed attempt used up its captcha response, so "Wiederholen" waits
  // for the next one instead of sending without it (#6739).
  const [retryPending, setRetryPending] = useState(false);
  const captchaSectionRef = useRef<HTMLElement>(null);
  const retrySubmit = () => {
    setRetryPending(true);
    // A challenge the parent has to answer sits below the form: show it,
    // or "Wiederholen" seems to do nothing.
    if (captchaRequired) {
      captchaSectionRef.current?.scrollIntoView?.({
        behavior: "smooth",
        block: "center",
      });
    }
  };
  useEffect(() => {
    if (!retryPending || (captchaRequired && !captchaToken)) return;
    setRetryPending(false);
    formRef.current?.requestSubmit();
  }, [retryPending, captchaRequired, captchaToken]);
  // Which legal block text the parent is currently viewing in the modal.
  const [openLegalDoc, setOpenLegalDoc] = useState<PublicLegalBlock | null>(
    null,
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

  // Show-if conditions read the live answers, so blocks appear and vanish as
  // the parent fills the form; hidden answers never reach the payload.
  const fieldsByKey = useMemo(
    () => buildFieldsByKey(schema?.fields ?? []),
    [schema],
  );
  const guardianContext: ConditionContext = {
    guardianAnswers: customData,
    fieldsByKey,
  };
  const derivationContext: ChildDerivationContext = {
    offerings,
    canonicalOfferings: config.offerings,
    collectGradeLevel,
    careOfferingsEnabled,
    schoolClass: schoolClassConfig,
    previewMode: templatePreview,
    guardianAnswers: customData,
    fieldsByKey,
  };
  const derivations = children.map((child) =>
    deriveChild(child, derivationContext),
  );
  const visibleGuardianFields = (schema?.fields ?? []).filter(
    (f) => !f.applies_to_child && isFieldVisible(f, guardianContext),
  );

  const updateChild = (index: number, patch: Partial<ChildDraft>) => {
    setChildren((prev) =>
      prev.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    );
  };

  const toggleOffering = (childIndex: number, offeringID: string) => {
    // "Genau eines" / "höchstens eines" selection_groups behave like radios:
    // selecting one clears the other selected member(s) of the same group.
    // Orthogonal to the phase-level exactly_one mode below; both can apply.
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const derived = deriveChild(c, derivationContext);
        const required = derived.requiredOfferingIds;
        if (required.has(offeringID)) return c;
        const toggled = derived.offerings.find((o) => o.id === offeringID);
        if (!toggled) return c;
        const group = toggled.selection_group?.trim() ?? "";
        const groupRule = toggled.selection_rule ?? "optional";
        const exclusiveGroup =
          group !== "" &&
          (groupRule === "exactly_one" || groupRule === "at_most_one");
        const nextIDs = new Set(c.offering_ids);
        const nextDays = { ...c.offering_days };
        if (nextIDs.has(offeringID)) {
          nextIDs.delete(offeringID);
          delete nextDays[offeringID];
        } else {
          if (careOfferingSelectionMode === "exactly_one") {
            for (const existingID of nextIDs) {
              if (!required.has(existingID)) delete nextDays[existingID];
            }
            nextIDs.clear();
            required.forEach((id) => nextIDs.add(id));
          }
          if (exclusiveGroup) {
            // A required offering stays: the backend checks it is sent.
            for (const sibling of derived.offerings) {
              const id = sibling.id;
              if (
                (sibling.selection_group?.trim() ?? "") === group &&
                id !== offeringID &&
                nextIDs.has(id) &&
                !required.has(id)
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

  // Flips one day of a parent_choice offering; callers check the mode.
  const toggleOfferingDay = (
    childIndex: number,
    offeringID: string,
    day: string,
  ) => {
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const derived = deriveChild(c, derivationContext);
        if (derived.care.automaticDays[offeringID]?.has(day)) return c;
        const next = new Set(c.offering_days[offeringID] ?? []);
        if (next.has(day)) next.delete(day);
        else next.add(day);
        const nextIDs = new Set(c.offering_ids);
        if (next.size > 0) {
          nextIDs.add(offeringID);
        } else if (!derived.requiredOfferingIds.has(offeringID)) {
          nextIDs.delete(offeringID);
        }
        return {
          ...c,
          offering_ids: nextIDs,
          offering_days: { ...c.offering_days, [offeringID]: next },
        };
      }),
    );
    // A picked day clears this offering's "no day picked" mark at once.
    setOfferingDayErrors((prev) => {
      const key = `${childIndex}_${offeringID}`;
      if (!prev[key]) return prev;
      const nextErrors = { ...prev };
      delete nextErrors[key];
      return nextErrors;
    });
  };

  const clearChildOfferingError = (childIndex: number) => {
    if (!childOfferingErrors[childIndex]) return;
    setChildOfferingErrors((prev) => {
      const next = { ...prev };
      delete next[childIndex];
      return next;
    });
  };

  const changeGrade = (
    index: number,
    child: ChildDraft,
    derivation: ChildDerivation,
    grade: string,
  ) => {
    const bookable = new Set(
      deriveChild(
        { ...child, target_grade_level: grade },
        derivationContext,
      ).offerings.map((o) => o.id),
    );
    const removed = [...derivation.care.offeringIds].some(
      (id) => !bookable.has(id),
    );
    setChildren((prev) =>
      prev.map((entry, i) =>
        i === index
          ? withRequiredOfferings(
              {
                ...entry,
                target_grade_level: grade,
                target_school_class:
                  entry.target_school_class &&
                  !classMatchesGrade(entry.target_school_class, grade)
                    ? ""
                    : entry.target_school_class,
                offering_ids: new Set(
                  [...child.offering_ids].filter((id) => bookable.has(id)),
                ),
                offering_days: Object.fromEntries(
                  Object.entries(child.offering_days).filter(([id]) =>
                    bookable.has(id),
                  ),
                ),
              },
              offerings,
              collectGradeLevel,
            )
          : entry,
      ),
    );
    setAvailabilityNotices((prev) =>
      removed
        ? { ...prev, [child.clientId]: true }
        : Object.fromEntries(
            Object.entries(prev).filter(([key]) => key !== child.clientId),
          ),
    );
  };

  const adoptExistingChild = (existing: MeProfileChild) => {
    // A stored grade the phase does not accept (#1663) would prefill a value
    // the select cannot show and fail the whole submit with
    // grade_not_eligible; the required check then asks for one instead.
    // With grades not collected the value stays in the draft but drives no
    // grade rule.
    const reusedGrade =
      existing.grade_level != null ? String(existing.grade_level) : "";
    const grade =
      !collectGradeLevel ||
      selectableGradeLevels(gradeLevelMax, eligibleGradeLevels).includes(
        Number(reusedGrade),
      )
        ? reusedGrade
        : "";
    const slot = withRequiredOfferings(
      {
        ...blankChild(),
        first_name: existing.first_name,
        last_name: existing.last_name,
        target_grade_level: grade,
        // Only a class the phase offers for this grade, so a re-enrolled
        // "2a" child defaults sensibly without a stale value (#1833).
        target_school_class:
          schoolClassConfig.available_classes.includes(existing.school_class) &&
          classMatchesGrade(existing.school_class, grade)
            ? existing.school_class
            : "",
      },
      offerings,
      collectGradeLevel,
    );
    setChildren((prev) => {
      // The first empty slot takes the child, so no empty card stays behind.
      const emptyIdx = prev.findIndex(
        (c) => !c.first_name && !c.last_name && !c.date_of_birth,
      );
      if (emptyIdx >= 0) {
        return prev.map((c, i) => (i === emptyIdx ? slot : c));
      }
      return [...prev, slot];
    });
    setUsedExistingChildIDs((prev) => new Set(prev).add(existing.id));
  };

  const addChild = () =>
    setChildren((prev) => [
      ...prev,
      withRequiredOfferings(blankChild(), offerings, collectGradeLevel),
    ]);
  const removeChild = (index: number) => {
    setChildren((prev) => prev.filter((_, i) => i !== index));
    // Field names and selection marks use child indices. Validate the new
    // structure on the next submit instead of showing another child's error.
    setChildOfferingErrors({});
    setOfferingDayErrors({});
    formError.clear();
  };

  const addGuardian = () =>
    setAdditionalGuardians((prev) => [...prev, blankGuardian()]);
  const updateGuardian = (index: number, patch: Partial<GuardianDraft>) =>
    setAdditionalGuardians((prev) =>
      prev.map((entry, i) =>
        i === index
          ? copyStableObjectKey(entry, { ...entry, ...patch })
          : entry,
      ),
    );
  const removeGuardian = (index: number) =>
    setAdditionalGuardians((prev) => prev.filter((_, i) => i !== index));

  // A form can ask the parent to sign (#4786); the signature field then
  // closes the form after the legal texts.
  const signatureRequired =
    Boolean(schema?.core_requirements?.signature) && !withoutSignature;

  // Why the submit button is blocked; while sending it stays natively
  // disabled without a reason (#5608).
  const submitBlockedReason = submitting
    ? undefined
    : previewMode
      ? tr("buttonHints.previewSubmit")
      : captchaRequired && !captchaConfig.site_key
        ? tr("buttonHints.captchaUnavailable")
        : tr("buttonHints.captchaPending");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setRetryPending(false);
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

    const values: EnrollmentFormValues = {
      guardian,
      additionalGuardians,
      customData,
      guardianContext,
      children,
      derivations,
      legalConsents,
      signature,
    };
    const problems = validateEnrollmentForm(
      values,
      {
        schema,
        offerings,
        careOfferingSelectionMode,
        collectGradeLevel,
        schoolClassRequired: schoolClassConfig.require,
        legalBlocks,
        signatureRequired,
        restrictToOfferings,
      },
      tr,
      contentLocale,
    );
    if (problems) {
      setChildOfferingErrors(problems.childOfferingErrors);
      setOfferingDayErrors(problems.offeringDayErrors);
      formError.invalid(problems.banner, problems.fieldErrors);
      return;
    }
    const payload = buildEnrollmentPayload(values, {
      phaseID,
      schema,
      collectGradeLevel,
      careOfferingsEnabled,
      legalBlocks,
      signatureRequired,
      restrictToOfferings,
      captchaToken: captchaRequired ? captchaToken || undefined : undefined,
      lateInviteToken,
    });

    setSubmitting(true);
    try {
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
      setCaptchaToken("");
      captchaRef.current?.reset();
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
      // The backend names fields by their JSON path, the inputs by name.
      if (err instanceof ApiError && err.errors) {
        err.errors = err.errors.map((entry) => ({
          ...entry,
          field: fieldNameForServerPath(entry.field),
        }));
      }
      // The server re-checked the selection rule but names no child.
      if (code === "enrollment.care_offering_missing") {
        setChildOfferingErrors(
          childrenViolatingCareSelectionRule(
            "at_least_one",
            children,
            derivations,
          ),
        );
      } else if (code === "enrollment.care_offering_exactly_one") {
        setChildOfferingErrors(
          childrenViolatingCareSelectionRule(
            "exactly_one",
            children,
            derivations,
          ),
        );
      }
      const pickupMarks =
        code === "enrollment.pickup_time_not_allowed"
          ? pickupTimeMarks(schema, children, customData)
          : [];
      // "Wiederholen" submits the form again, so it sends what the parent
      // sees now, not the draft of the failed attempt.
      const presentation = await formError.show(err, {
        object: tr("errorObject"),
        retry: retrySubmit,
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
      <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 sm:p-6">
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
          <section className="moto-content-surface space-y-5 rounded-xl border p-4 sm:p-5">
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
                tooltip={tr("buttonHints.addGuardian")}
                className="w-full gap-1.5 whitespace-nowrap sm:w-auto sm:shrink-0"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                {tr("actions.addGuardian")}
              </Button>
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input
                id={`enrollment-${formFieldName.guardian("first_name")}`}
                label={tr("fields.firstName")}
                name={formFieldName.guardian("first_name")}
                autoComplete="given-name"
                controlSize="compact"
                value={guardian.first_name}
                onChange={(e) =>
                  updateGuardianField({ first_name: e.target.value })
                }
                required
                error={fieldError(formFieldName.guardian("first_name"))}
              />
              <Input
                id={`enrollment-${formFieldName.guardian("last_name")}`}
                label={tr("fields.lastName")}
                name={formFieldName.guardian("last_name")}
                autoComplete="family-name"
                controlSize="compact"
                value={guardian.last_name}
                onChange={(e) =>
                  updateGuardianField({ last_name: e.target.value })
                }
                required
                error={fieldError(formFieldName.guardian("last_name"))}
              />
              <Input
                id={`enrollment-${formFieldName.guardian("email")}`}
                label={tr("fields.email")}
                name={formFieldName.guardian("email")}
                type="email"
                spellCheck={false}
                autoComplete="email"
                controlSize="compact"
                value={guardian.email}
                onChange={(e) => updateGuardianField({ email: e.target.value })}
                required
                readOnly={lockedGuardianEmail}
                error={fieldError(formFieldName.guardian("email"))}
              />
              <Input
                id={`enrollment-${formFieldName.guardian("phone")}`}
                label={tr("fields.phone")}
                name={formFieldName.guardian("phone")}
                type="tel"
                autoComplete="tel"
                inputMode="tel"
                controlSize="compact"
                value={guardian.phone}
                onChange={(e) => updateGuardianField({ phone: e.target.value })}
                required={schema?.core_requirements?.guardian_phone === true}
                error={fieldError(formFieldName.guardian("phone"))}
              />
            </div>

            {additionalGuardians.map((g, i) => (
              <div
                key={getStableObjectKey(g, "additional-guardian")}
                className="moto-content-surface space-y-5 rounded-xl border p-3 sm:p-4"
              >
                <div className="flex items-center justify-between gap-3">
                  <SectionLabel>
                    {tr("sections.additionalGuardianLabel", { index: i + 1 })}
                  </SectionLabel>
                  <Button
                    type="button"
                    variant="ghost_danger"
                    size="compact"
                    onClick={() => removeGuardian(i)}
                    tooltip={tr("buttonHints.removeGuardian")}
                  >
                    {tr("actions.remove")}
                  </Button>
                </div>
                <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Input
                    id={`enrollment-${formFieldName.additionalGuardian(i, "first_name")}`}
                    label={tr("fields.firstName")}
                    name={formFieldName.additionalGuardian(i, "first_name")}
                    autoComplete="given-name"
                    controlSize="compact"
                    value={g.first_name}
                    onChange={(e) =>
                      updateGuardian(i, { first_name: e.target.value })
                    }
                    required
                    error={fieldError(
                      formFieldName.additionalGuardian(i, "first_name"),
                    )}
                  />
                  <Input
                    id={`enrollment-${formFieldName.additionalGuardian(i, "last_name")}`}
                    label={tr("fields.lastName")}
                    name={formFieldName.additionalGuardian(i, "last_name")}
                    autoComplete="family-name"
                    controlSize="compact"
                    value={g.last_name}
                    onChange={(e) =>
                      updateGuardian(i, { last_name: e.target.value })
                    }
                    required
                    error={fieldError(
                      formFieldName.additionalGuardian(i, "last_name"),
                    )}
                  />
                  <Input
                    id={`enrollment-${formFieldName.additionalGuardian(i, "email")}`}
                    label={tr("fields.emailPlain")}
                    name={formFieldName.additionalGuardian(i, "email")}
                    type="email"
                    spellCheck={false}
                    autoComplete="email"
                    controlSize="compact"
                    value={g.email}
                    onChange={(e) =>
                      updateGuardian(i, { email: e.target.value })
                    }
                    error={fieldError(
                      formFieldName.additionalGuardian(i, "email"),
                    )}
                  />
                  <Input
                    id={`enrollment-${formFieldName.additionalGuardian(i, "phone")}`}
                    label={tr("fields.phone")}
                    name={formFieldName.additionalGuardian(i, "phone")}
                    type="tel"
                    autoComplete="tel"
                    inputMode="tel"
                    controlSize="compact"
                    value={g.phone}
                    onChange={(e) =>
                      updateGuardian(i, { phone: e.target.value })
                    }
                    error={fieldError(
                      formFieldName.additionalGuardian(i, "phone"),
                    )}
                  />
                </div>
              </div>
            ))}
          </section>
        )}

        {!restrictToOfferings &&
          schema?.fields.some((f) => !f.applies_to_child) && (
            <section className="moto-content-surface space-y-4 rounded-xl border p-4 sm:p-5">
              <SectionHeading
                title={tr("sections.extraTitle")}
                description={tr("sections.extraDescription")}
              />
              {visibleGuardianFields.map((f) =>
                f.type === "information" ? (
                  <InfoBlock
                    key={f.key}
                    field={f}
                    attachmentLabels={attachmentLabels}
                  />
                ) : (
                  <CustomFieldInput
                    key={f.key}
                    field={f}
                    value={customData[f.key]}
                    onChange={(v) =>
                      setCustomData((prev) => ({ ...prev, [f.key]: v }))
                    }
                    inputName={formFieldName.custom(f.key)}
                    error={fieldError(formFieldName.custom(f.key))}
                    tr={tr}
                  />
                ),
              )}
            </section>
          )}

        <section className="moto-content-surface space-y-5 rounded-xl border p-4 sm:p-5">
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
                tooltip={tr("buttonHints.addChild")}
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
              <Alert
                type="warning"
                announce="off"
                message={tr("sections.newStudentsOnlyNotice")}
              />
            )}

          {reusableChildren.length > 0 &&
            !lockChildStructure &&
            isExistingStudentsPhase && (
              <ExistingChildrenPanel
                existing={reusableChildren}
                usedIDs={usedExistingChildIDs}
                tr={tr}
                onAdopt={adoptExistingChild}
              />
            )}

          {children.map((child, i) => {
            const derivation = derivations[i]!;
            const childOfferings = derivation.offerings;
            const materializedCare = derivation.care;
            // Admin flows only (#2186): what the grade rule filtered out,
            // shown greyed with a reason. The parent form leaves this empty.
            const blockedOfferings = showBlockedOfferings
              ? offerings.filter(
                  (offering) =>
                    !childOfferings.some((open) => open.id === offering.id),
                )
              : [];
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
                <SectionCard
                  key={child.clientId}
                  title={name || tr("structured.child")}
                  icon={Lock}
                  bodyClassName="compact:mt-3 mt-4 space-y-3"
                >
                  <StatusBadge label={tr("locked.badge")} tone="green" />
                  <DataGrid columns={1}>
                    <DataField label={tr("locked.birthday")}>
                      {child.date_of_birth
                        ? formatDate(child.date_of_birth, false, activeLocale)
                        : "-"}
                    </DataField>
                    {derivation.grade ? (
                      <DataField label={tr("locked.grade")}>
                        {tr("fields.grade", { grade: derivation.grade })}
                      </DataField>
                    ) : null}
                    <DataField label={tr("locked.care")}>
                      {booked.length > 0
                        ? booked.join(", ")
                        : child.offering_ids.size > 0
                          ? "-"
                          : tr("locked.noCare")}
                    </DataField>
                  </DataGrid>
                  <Alert
                    type="info"
                    announce="off"
                    message={
                      <p className="leading-6">
                        {tr("locked.hint", {
                          name: name || tr("structured.child"),
                        })}
                        <br />
                        {tr("locked.noAccess")}
                      </p>
                    }
                  />
                </SectionCard>
              );
            }
            const offeringCard = (
              offering: PublicCareOffering,
              checked: boolean,
            ) => (
              <OfferingCard
                key={offering.id}
                offering={offering}
                childIndex={i}
                checked={checked}
                selectedDays={materializedCare.offeringDays[offering.id]}
                automaticDays={materializedCare.automaticDays[offering.id]}
                toggleLocked={
                  offering.is_required ||
                  (Boolean(materializedCare.automaticDays[offering.id]) &&
                    !child.offering_ids.has(offering.id))
                }
                dayError={offeringDayErrors[`${i}_${offering.id}`] ?? false}
                stats={offeringBookingStats?.[offering.id]}
                onToggle={() => {
                  toggleOffering(i, offering.id);
                  if (!offering.is_required) clearChildOfferingError(i);
                }}
                onToggleDay={(day) => toggleOfferingDay(i, offering.id, day)}
                tr={tr}
              />
            );
            return (
              <div
                key={child.clientId}
                className="moto-content-surface space-y-5 rounded-xl border p-3 sm:p-4"
              >
                <div className="flex items-center justify-between gap-3">
                  <SectionLabel>
                    {restrictToOfferings && child.first_name
                      ? `${child.first_name} ${child.last_name}`.trim()
                      : `${tr("structured.child")} ${i + 1}`}
                  </SectionLabel>
                  {children.length > 1 && !lockChildStructure && (
                    <Button
                      type="button"
                      variant="ghost_danger"
                      size="compact"
                      onClick={() => removeChild(i)}
                      tooltip={tr("buttonHints.removeChild")}
                    >
                      {tr("actions.remove")}
                    </Button>
                  )}
                </div>
                {!restrictToOfferings && (
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                    <Input
                      id={`enrollment-${formFieldName.child(i, "first_name")}`}
                      label={tr("fields.firstName")}
                      name={formFieldName.child(i, "first_name")}
                      autoComplete="given-name"
                      controlSize="compact"
                      value={child.first_name}
                      onChange={(e) =>
                        updateChild(i, { first_name: e.target.value })
                      }
                      required
                      error={fieldError(formFieldName.child(i, "first_name"))}
                    />
                    <Input
                      id={`enrollment-${formFieldName.child(i, "last_name")}`}
                      label={tr("fields.lastName")}
                      name={formFieldName.child(i, "last_name")}
                      autoComplete="family-name"
                      controlSize="compact"
                      value={child.last_name}
                      onChange={(e) =>
                        updateChild(i, { last_name: e.target.value })
                      }
                      required
                      error={fieldError(formFieldName.child(i, "last_name"))}
                    />
                    <div>
                      <FieldLabel
                        required
                        invalid={Boolean(
                          fieldError(formFieldName.child(i, "date_of_birth")),
                        )}
                      >
                        {tr("fields.dateOfBirth")}
                      </FieldLabel>
                      <DateOfBirthPicker
                        value={child.date_of_birth}
                        onChange={(v) => updateChild(i, { date_of_birth: v })}
                        error={fieldError(
                          formFieldName.child(i, "date_of_birth"),
                        )}
                        tr={tr}
                      />
                    </div>
                    {collectGradeLevel && (
                      <GradeLevelSelect
                        id={`children-${i}-target-grade-level`}
                        value={child.target_grade_level}
                        onChange={(v) => changeGrade(i, child, derivation, v)}
                        max={gradeLevelMax}
                        allowedGrades={eligibleGradeLevels}
                        error={fieldError(
                          formFieldName.child(i, "target_grade_level"),
                        )}
                        tr={tr}
                      />
                    )}
                    {derivation.collectsSchoolClass && (
                      <SchoolClassSelect
                        id={`children-${i}-target-school-class`}
                        value={child.target_school_class}
                        onChange={(v) =>
                          updateChild(i, { target_school_class: v })
                        }
                        classes={derivation.schoolClasses}
                        // Without a class for this grade the pick cannot be
                        // mandatory; "Klasse offen" stays available (#1833).
                        required={
                          schoolClassConfig.require &&
                          derivation.schoolClasses.length > 0
                        }
                        error={fieldError(
                          formFieldName.child(i, "target_school_class"),
                        )}
                        tr={tr}
                      />
                    )}
                  </div>
                )}
                {askHolidayCareWish && !child.locked && (
                  <CustomFieldInput
                    field={holidayCareWishField}
                    inputName={formFieldName.child(i, "holiday_care_wish")}
                    value={child.holiday_care_wish}
                    onChange={(v) =>
                      updateChild(i, {
                        holiday_care_wish:
                          typeof v === "boolean" ? v : undefined,
                      })
                    }
                    error={fieldError(
                      formFieldName.child(i, "holiday_care_wish"),
                    )}
                    tr={tr}
                  />
                )}
                {availabilityNotices[child.clientId] ? (
                  <Alert
                    type="warning"
                    announce="polite"
                    message={tr("care.availabilitySelectionRemoved")}
                  />
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
                            .map((o) => offeringCard(o, true))}
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
                        {/* oxlint-disable-next-line ui-kit/no-hand-rolled-notice -- roter Fehlerrahmen um die Angebotsauswahl, solange sie ungültig ist; kein Hinweis */}
                        <div
                          aria-invalid={
                            childOfferingErrors[i] ? true : undefined
                          }
                          aria-describedby={
                            childOfferingErrors[i]
                              ? `children-${i}-offerings-error`
                              : undefined
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
                              offeringCard(
                                bucket.offering,
                                materializedCare.offeringIds.has(
                                  bucket.offering.id,
                                ),
                              )
                            ) : (
                              <InsetPanel
                                framed
                                padding="compact"
                                tone={
                                  groupDayConflicts(bucket.group).length > 0
                                    ? "danger"
                                    : "muted"
                                }
                                key={`group-${bucket.group}`}
                                aria-invalid={
                                  groupDayConflicts(bucket.group).length > 0
                                    ? true
                                    : undefined
                                }
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
                                    <StatusBadge
                                      tone="blue"
                                      compact
                                      wrap
                                      label={offeringRuleHint(bucket.rule, tr)}
                                    />
                                  )}
                                </div>
                                <div className="space-y-2">
                                  {bucket.offerings.map((o) =>
                                    offeringCard(
                                      o,
                                      materializedCare.offeringIds.has(o.id),
                                    ),
                                  )}
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
                              </InsetPanel>
                            ),
                          )}
                        </div>
                        <FieldErrorHint
                          id={`children-${i}-offerings-error`}
                          message={
                            childOfferingErrors[i]
                              ? careOfferingSelectionMode === "exactly_one"
                                ? tr("care.chooseExactlyOne")
                                : tr("care.chooseAtLeastOne")
                              : null
                          }
                        />
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
                              gradeLevel={derivation.grade ?? ""}
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
                        isFieldVisible(f, derivation.conditionContext),
                    )
                    .map((f) => {
                      if (f.type === "information") {
                        return (
                          <InfoBlock
                            key={f.key}
                            field={f}
                            attachmentLabels={attachmentLabels}
                          />
                        );
                      }
                      const departure =
                        f.target === "student.allowed_departure_modes";
                      const noteName = formFieldName.childCompanionNote(i);
                      return (
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
                            departure
                              ? (child.custom[DEPARTURE_COMPANION_KEY] as
                                  string | undefined)
                              : undefined
                          }
                          onCompanionNoteChange={
                            departure
                              ? (v) =>
                                  updateChild(i, {
                                    custom: {
                                      ...child.custom,
                                      [DEPARTURE_COMPANION_KEY]: v,
                                    },
                                  })
                              : undefined
                          }
                          inputName={formFieldName.childCustom(i, f.key)}
                          error={fieldError(
                            formFieldName.childCustom(i, f.key),
                          )}
                          companionNoteName={noteName}
                          companionNoteError={
                            departure ? fieldError(noteName) : undefined
                          }
                          relevantDays={derivation.careDays}
                          careConstrained={derivation.careDaysConstrained}
                          singleMode={
                            f.type === "weekday_multi_mode" &&
                            singleModeApplies(f, derivation.grade)
                          }
                          tr={tr}
                        />
                      );
                    })}
              </div>
            );
          })}
        </section>

        {legalBlocks.length > 0 && !restrictToOfferings && (
          <section className="moto-content-surface space-y-4 rounded-xl border p-4 sm:p-5">
            <SectionHeading
              title={tr("sections.consentTitle")}
              description={tr("sections.consentDescription")}
            />
            {localizedCopy && activeLocale !== DEFAULT_LOCALE ? (
              <InsetPanel
                as="p"
                framed
                padding="compact"
                className="text-xs leading-5 text-gray-600"
              >
                {tr("legalGermanNotice")}
              </InsetPanel>
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
                    tr={tr}
                  />
                ) : (
                  <EnrollmentConsent
                    key={block.key}
                    name={formFieldName.consent(block.key)}
                    label={block.label}
                    checked={legalConsents[block.key] === true}
                    onChange={(checked) =>
                      setLegalConsents((prev) => ({
                        ...prev,
                        [block.key]: checked,
                      }))
                    }
                    required={block.required}
                    error={fieldError(formFieldName.consent(block.key))}
                    legalText={block.text}
                    legalTitle={block.title}
                    onViewLegal={
                      block.text ? () => setOpenLegalDoc(block) : undefined
                    }
                    tr={tr}
                  />
                )}
                <DocumentAttachmentList
                  attachments={block.attachments}
                  labels={attachmentLabels}
                />
              </div>
            ))}
          </section>
        )}

        {signatureRequired && (
          <section className="moto-content-surface space-y-4 rounded-xl border p-4 sm:p-5">
            <SectionHeading
              title={tr("signature.title")}
              description={tr("signature.description")}
            />
            <SignaturePad
              id={formFieldName.signature}
              value={signature}
              onChange={changeSignature}
              disabled={submitting || previewMode}
              // A confirmed signature answers the missing-signature hint.
              error={
                signature ? undefined : fieldError(formFieldName.signature)
              }
              signerNameHint={
                `${guardian.first_name} ${guardian.last_name}`.trim() ||
                undefined
              }
              labels={signatureLabels}
            />
          </section>
        )}

        <Modal
          isOpen={openLegalDoc !== null}
          onClose={() => setOpenLegalDoc(null)}
          title={openLegalDoc?.title ?? ""}
          size="lg"
        >
          <div className="max-h-[60vh] overflow-y-auto text-sm text-gray-700">
            <LegalMarkdown text={openLegalDoc?.text ?? ""} />
          </div>
        </Modal>

        {/* Turnstile shows only when the caller did not skip the captcha and
          the school enabled it with a site key. Without a site key the alert
          above blocks the form: failing closed beats letting bots through.
          The hidden input sends the token with the form. */}
        {captchaRequired && captchaConfig.site_key && (
          <section
            ref={captchaSectionRef}
            aria-labelledby="captcha-heading"
            className="space-y-2"
          >
            <h2 id="captcha-heading" className="sr-only">
              Sicherheitsprüfung
            </h2>
            <Turnstile
              ref={captchaRef}
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
          disabledReason={submitBlockedReason}
          tooltip={submitTooltip ?? tr("buttonHints.submit")}
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

const GERMAN_ENROLLMENT_FORM_MESSAGES = germanEnrollmentFormMessages as Record<
  string,
  unknown
>;

function makeEnrollmentFormTranslator(
  intl: ReturnType<typeof useCareTermTranslations>,
  localizedCopy: boolean,
  careTerm: string,
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
      // The German staff preview names the school's care like the parents'
      // form does (#6042).
      return interpolateMessage(value, { careTerm, ...values });
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
// in backend/modules/careplan/selection/materialize.go.
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

function offeringRuleHint(rule: string, tr: TranslationFn): string {
  if (rule === AT_MOST_ONE_PER_DAY) return tr("care.ruleAtMostOnePerDay");
  if (rule === "exactly_one") return tr("care.ruleExactlyOne");
  if (rule === "at_least_one") return tr("care.ruleAtLeastOne");
  if (rule === "at_most_one") return tr("care.ruleAtMostOne");
  return "";
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
    <OfferingRowShell state="blocked">
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

// One care-offering row. A required offering shows as a locked "always
// included" card so it never reads as a parent's pick. The day picker stays
// available for parent_choice offerings in both cases.
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
  let rowState: OfferingRowState = "available";
  if (dayError) {
    rowState = "invalid";
  } else if (autoIncluded || checked) {
    rowState = "selected";
  } else if (required || toggleLocked) {
    rowState = "locked";
  }
  const inputId = `children-${childIndex}-offering-${offering.id}`;

  return (
    <OfferingRowShell state={rowState}>
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
          name={formFieldName.childOffering(childIndex, offering.id)}
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
              <StatusBadge
                tone="gray"
                compact
                label={tr("care.requiredBadge")}
              />
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
        ariaDescribedBy={error ? `${id}-error` : undefined}
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
      <FieldErrorHint id={`${id}-error`} message={error} />
    </div>
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
        {tr("fields.schoolClass")}
      </FieldLabel>
      <CustomSelect
        ariaLabelledBy={`${id}-label`}
        ariaDescribedBy={error ? `${id}-error` : undefined}
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
      <FieldErrorHint id={`${id}-error`} message={error} />
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
    <InsetPanel framed padding="md">
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
                disabledReason={tr("buttonHints.adopted")}
                tooltip={tr("buttonHints.adopt")}
                className="shrink-0"
              >
                {used ? tr("structured.adopted") : tr("structured.adopt")}
              </Button>
            </li>
          );
        })}
      </ul>
    </InsetPanel>
  );
}
