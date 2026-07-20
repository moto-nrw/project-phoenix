"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { ClipboardList, Loader2, Save } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  DepartureSection,
  EnrollmentConsentsSection,
  PersonalInfoSection,
} from "./student-form-fields";
import { StudentCommonFormSections } from "./student-common-form-sections";
import { StudentPhotoSection } from "./student-photo-section";
import { validateStudentForm } from "~/lib/student-form-validation";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { useStudentEnrollmentExtraFields } from "~/lib/hooks/use-student-enrollment-extra-fields";
import {
  deleteStudentPhoto,
  fetchStudentPrivacyConsent,
  type StudentEnrollmentExtraFieldGroup,
  uploadStudentPhoto,
} from "~/lib/student-api";
import {
  ALL_COMPANION_WEEKDAYS,
  fetchStudentCompanions,
  mergeCompanionConfirmations,
  type CompanionExtensionConfirmation,
  type StudentCompanion,
} from "~/lib/student-companion-api";
import { createLogger } from "~/lib/logger";
import { CompanionPlanConflictError, parseConflictExtensions } from "~/lib/api";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  busDaysHaveAny,
  normalizeBusDays,
  type BusDays,
  pickupDaysHaveAny,
  normalizePickupDays,
  type PickupDays,
  type AllowedDepartureModes,
  normalizeDepartureDays,
  departureDaysFromLegacy,
  allowedDepartureModesFromDeparture,
  allowedDepartureToBusDays,
  allowedDepartureToDepartureDays,
  allowedDepartureToPickupDays,
  normalizeAllowedDepartureModes,
  DEPARTURE_WEEKDAYS,
} from "~/lib/student-helpers";
import type { Student } from "~/lib/api";

const logger = createLogger({ component: "StudentStammdatenTab" });
const ENROLLMENT_EXTRA_ACCENT = LOCATION_COLORS.OTHER_ROOM;

interface StudentStammdatenTabProps {
  student: Student;
  groups: Array<{ value: string; label: string }>;
  onSave: (data: Partial<Student>) => Promise<void>;
  /**
   * Refresh hook used after a successful photo upload/delete. The photo
   * mutation runs AFTER onSave (so a failed PUT can never leave a half-saved
   * state on the server), which means the SWR mutate that onSave triggers
   * captures the student BEFORE the new photo_url has landed. Without an
   * explicit refresh here the component falls back to a stale photo_url:
   * a freshly uploaded avatar disappears, a freshly deleted avatar
   * reappears, and only a full page reload reconciles the view.
   */
  onStudentRefresh?: () => void | Promise<void>;
}

// ServerConsent holds the privacy consent fetched separately for the selected
// student. The list endpoint does not carry privacy_consent_accepted /
// data_retention_days — they only come from the per-student detail endpoint —
// so the draft must be seeded from this fetch, otherwise the checkbox would
// always render unchecked and saving would overwrite the real consent.
type ServerConsent = { accepted: boolean; dataRetentionDays: number };

function busDaysEqual(a?: BusDays | null, b?: BusDays | null): boolean {
  const left = normalizeBusDays(a);
  const right = normalizeBusDays(b);
  return (
    Boolean(left.mon) === Boolean(right.mon) &&
    Boolean(left.tue) === Boolean(right.tue) &&
    Boolean(left.wed) === Boolean(right.wed) &&
    Boolean(left.thu) === Boolean(right.thu) &&
    Boolean(left.fri) === Boolean(right.fri)
  );
}

function pickupDaysEqual(
  a?: PickupDays | null,
  b?: PickupDays | null,
): boolean {
  const left = normalizePickupDays(a);
  const right = normalizePickupDays(b);
  return (
    Boolean(left.mon) === Boolean(right.mon) &&
    Boolean(left.tue) === Boolean(right.tue) &&
    Boolean(left.wed) === Boolean(right.wed) &&
    Boolean(left.thu) === Boolean(right.thu) &&
    Boolean(left.fri) === Boolean(right.fri)
  );
}

function allowedDepartureModesEqual(
  a?: AllowedDepartureModes | null,
  b?: AllowedDepartureModes | null,
): boolean {
  const left = normalizeAllowedDepartureModes(a);
  const right = normalizeAllowedDepartureModes(b);
  return DEPARTURE_WEEKDAYS.every((day) => {
    const leftModes = left[day.key] ?? [];
    const rightModes = right[day.key] ?? [];
    return (
      leftModes.length === rightModes.length &&
      leftModes.every((mode, idx) => mode === rightModes[idx])
    );
  });
}

// The save path of this tab goes through the generic CRUD service, which throws
// a plain Error carrying the HTTP status, while other callers reach the typed
// CompanionPlanConflictError from studentService.updateStudent — detect both. A
// 409 from this form can only be the companion-plan conflict: the form never
// submits sick/excused, the only other conflicting field pair on the student
// PUT.
function isCompanionPlanConflict(err: unknown): boolean {
  if (err instanceof CompanionPlanConflictError) return true;
  return (
    err instanceof Error && (err as Error & { status?: number }).status === 409
  );
}

const COMPANION_CONFLICT_FALLBACK =
  "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.";

// Digs the backend's German conflict message out of whichever wrapping the
// error arrived in; falls back to the generic sentence.
function companionConflictMessage(err: unknown): string {
  if (err instanceof CompanionPlanConflictError) return err.message;
  if (err instanceof Error) {
    const match = /\{.*\}/s.exec(err.message);
    if (match) {
      try {
        const parsed = JSON.parse(match[0]) as { message?: string };
        if (typeof parsed.message === "string" && parsed.message.trim()) {
          return parsed.message;
        }
      } catch {
        // Not JSON — use the fallback text.
      }
    }
  }
  return COMPANION_CONFLICT_FALLBACK;
}

// The confirmable conflicts the backend reported, dug out of whichever
// wrapping the error arrived in. An empty list means we could not read them —
// the retry then confirms nothing and the backend asks again, which is the
// safe direction.
function companionConflictExtensions(
  err: unknown,
): CompanionExtensionConfirmation[] {
  if (err instanceof CompanionPlanConflictError) return err.conflicts;
  if (err instanceof Error) {
    const match = /\{.*\}/s.exec(err.message);
    if (match) return parseConflictExtensions(match[0]);
  }
  return [];
}

// Order-independent fingerprint of a companion list, so reordering (or a
// re-fetch handing the same links back in another order) is not mistaken for an
// edit. Used both for the dirty check and to keep the two in sync.
function companionsFingerprint(companions: StudentCompanion[]): string {
  return companions
    .map((companion) => {
      const days = ALL_COMPANION_WEEKDAYS.filter((day) =>
        companion.weekdays.includes(day),
      );
      return `${companion.companion_student_id}:${days.join(",")}`;
    })
    .sort()
    .join("|");
}

function buildDraft(
  student: Student,
  photosEnabled: boolean,
  consent: ServerConsent | null,
): Partial<Student> {
  const draft: Partial<Student> = {
    first_name: student.first_name ?? "",
    second_name: student.second_name ?? "",
    school_class: student.school_class ?? "",
    group_id: student.group_id ?? "",
    birthday: student.birthday ?? "",
    address_street: student.address_street ?? "",
    address_postal_code: student.address_postal_code ?? "",
    address_city: student.address_city ?? "",
    health_info: student.health_info ?? "",
    supervisor_notes: student.supervisor_notes ?? "",
    extra_info: student.extra_info ?? "",
    departure_companion_note: student.departure_companion_note ?? "",
    privacy_consent_accepted:
      consent?.accepted ?? student.privacy_consent_accepted ?? false,
    data_retention_days:
      consent?.dataRetentionDays ?? student.data_retention_days ?? 30,
    bus: student.bus ?? false,
    bus_days: normalizeBusDays(student.bus_days),
    pickup_days: normalizePickupDays(student.pickup_days),
    allowed_departure_modes: student.allowed_departure_modes
      ? normalizeAllowedDepartureModes(student.allowed_departure_modes)
      : allowedDepartureModesFromDeparture(
          student.departure_days ??
            departureDaysFromLegacy(student.bus_days, student.pickup_days),
        ),
    departure_days: student.departure_days
      ? normalizeDepartureDays(student.departure_days)
      : departureDaysFromLegacy(student.bus_days, student.pickup_days),
    pickup_status: student.pickup_status ?? "",
  };
  // Only mirror consent state into the form when the photo feature is
  // visible. When the feature is off the backend's populatePhotoFields
  // strips photo_consent_given_at from the response while the DB row
  // still carries it. A `Boolean(undefined)` default would round-trip
  // `photo_consent_given: false` through the regular student PUT, and
  // backend applyPhotoConsent reads that as a true→false withdrawal —
  // silently nilling out the original consent timestamp + acting account
  // on every unrelated edit. Leaving the field undefined makes
  // mapStudentRequest omit it from the JSON, so applyPhotoConsent treats
  // it as a no-op (req.PhotoConsentGiven == nil).
  if (photosEnabled) {
    draft.photo_consent_given = Boolean(student.photo_consent_given_at);
  }
  return draft;
}

export function StudentStammdatenTab({
  student,
  groups,
  onSave,
  onStudentRefresh,
}: StudentStammdatenTabProps) {
  const { enabled: photosEnabled } = useStudentPhotosEnabled();
  // Privacy consent is fetched separately (the list doesn't carry it); null
  // until the per-student fetch below resolves.
  const [serverConsent, setServerConsent] = useState<ServerConsent | null>(
    null,
  );
  const [privacyConsentLoadError, setPrivacyConsentLoadError] = useState<
    string | null
  >(null);
  const [formData, setFormData] = useState<Partial<Student>>(() =>
    buildDraft(student, photosEnabled, null),
  );
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);
  // Laufgemeinschaft: fetched separately (own table) and submitted together
  // with the departure plan, because a link is only legal on a day that plan
  // allows "Anderes Kind".
  //
  // The submitted list REPLACES the stored one, so it may only travel once it
  // IS the stored one: an empty list from a pending or failed load would delete
  // the child's Laufgemeinschaft on any unrelated save, and this component is
  // reused across children, so a load that fails after switching would submit
  // the PREVIOUS child's links. Hence the explicit status, reset on every id
  // change, plus the untouched-since-load snapshot the dirty check compares
  // against.
  const [companions, setCompanions] = useState<StudentCompanion[]>([]);
  const [loadedCompanions, setLoadedCompanions] = useState<StudentCompanion[]>(
    [],
  );
  const [companionsStatus, setCompanionsStatus] = useState<
    "loading" | "ready" | "error"
  >("loading");
  const [reloadCompanions, setReloadCompanions] = useState(0);
  const [extendCompanionPlans, setExtendCompanionPlans] = useState(false);
  // WHAT the user confirmed, not just that they did: the backend widens a
  // companion's plan only for the children and weekdays listed here, so a
  // conflict that appeared after the question was asked is refused again
  // instead of riding along on the earlier yes.
  const [confirmedExtensions, setConfirmedExtensions] = useState<
    CompanionExtensionConfirmation[]
  >([]);
  // Set when the backend refused because a linked child's own departure plan
  // does not allow the requested days (409). Answering yes re-sends the same
  // payload with extend_companion_plans — mirroring PersonalInfoFormModal.
  const [planConflict, setPlanConflict] = useState<string | null>(null);

  useEffect(() => {
    if (!student.id) return;
    let cancelled = false;
    setCompanionsStatus("loading");
    setCompanions([]);
    setLoadedCompanions([]);
    setExtendCompanionPlans(false);
    setConfirmedExtensions([]);
    setPlanConflict(null);
    fetchStudentCompanions(String(student.id))
      .then((loaded) => {
        if (cancelled) return;
        setCompanions(loaded);
        setLoadedCompanions(loaded);
        setCompanionsStatus("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setCompanionsStatus("error");
        logger.error("failed to load companions", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [student.id, reloadCompanions]);
  const {
    groups: enrollmentExtraGroups,
    loading: enrollmentExtraLoading,
    hasError: enrollmentExtraLoadError,
  } = useStudentEnrollmentExtraFields(
    student.id,
    student.has_full_access !== false,
  );

  // Pending photo state — held locally until the user clicks Speichern. The
  // user's mental model: picking a file or clicking "Foto entfernen" is
  // just like editing any other form field. Nothing leaves the browser
  // until the explicit Speichern click. Both states are mutually
  // exclusive — picking overrides removal, removing overrides a pick.
  const [pendingPhotoBlob, setPendingPhotoBlob] = useState<Blob | null>(null);
  const [pendingPhotoRemoved, setPendingPhotoRemoved] = useState(false);

  // Reset the editable draft only when the user picks a *different* student
  // (id changes) — NOT on every `student` prop mutation. Background refetches
  // (e.g. after a successful photo upload triggers a parent SWR mutate) ship
  // a new student object reference with the same id; if we reacted to those
  // we'd silently wipe any other unsaved edits (a half-typed name, an
  // unsaved health-info change …) and disable the Speichern button.
  // `originalDraft` still recomputes on every student change so isDirty
  // correctly compares against the latest *server* state.
  // We deliberately depend only on student.id; `student` is read on each
  // run via the closure and the deps rule is suppressed on the array line.
  useEffect(() => {
    setFormData(buildDraft(student, photosEnabled, null));
    setErrors({});
    // Drop any uncommitted photo state when navigating to a different
    // student so we never carry one student's pending pick into another's
    // form view.
    setPendingPhotoBlob(null);
    setPendingPhotoRemoved(false);
    // photosEnabled intentionally omitted: a mid-form feature flip should
    // not wipe the user's other unsaved edits. The dedicated
    // photo-consent sync effect below handles the flip without resetting
    // unrelated fields.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [student.id]);

  // Sync photo_consent_given against the server when the feature flips
  // mid-session (or the server's consent timestamp changes underneath
  // us). The main reset effect above is gated on student.id — if it
  // also depended on photosEnabled it would wipe unrelated unsaved
  // edits whenever an admin toggles the feature. This focused effect
  // touches ONLY the consent key:
  //   - off → on: pull the server's current consent state into the
  //     draft so the checkbox renders the correct initial value.
  //     Otherwise originalDraft would carry photo_consent_given:true
  //     while formData kept the old undefined, leaving the form in a
  //     falsely-dirty state with the checkbox showing unchecked even
  //     though the server has consent recorded.
  //   - on → off: drop the key so the JSON layer omits it and the
  //     backend's applyPhotoConsent treats the PUT as a no-op for the
  //     consent column instead of seeing a synthetic false→withdrawal.
  // The early-return when next.photo_consent_given already equals the
  // server value avoids unnecessary re-renders. The effect deliberately
  // depends ONLY on photosEnabled and student.photo_consent_given_at,
  // so it stays quiet during in-flight checkbox edits (no server-side
  // input changed → no run). The narrow case where the server's
  // timestamp updates while the user has a different value selected
  // (another tab toggled consent mid-edit) does overwrite the local
  // value — acceptable since the SSE-driven student refresh tells the
  // user the row changed underneath them.
  useEffect(() => {
    setFormData((prev) => {
      const next = { ...prev } as Partial<Student>;
      if (photosEnabled) {
        const serverConsent = Boolean(student.photo_consent_given_at);
        if (next.photo_consent_given === serverConsent) return prev;
        next.photo_consent_given = serverConsent;
        return next;
      }
      if (next.photo_consent_given === undefined) return prev;
      delete next.photo_consent_given;
      return next;
    });
  }, [photosEnabled, student.photo_consent_given_at]);

  // Load the real privacy consent for the selected student. The list-level
  // `student` object does not carry privacy_consent_accepted /
  // data_retention_days — they come only from the per-student detail endpoint.
  // Without this fetch the checkbox would always render unchecked and a save
  // would write the stored consent back to false.
  useEffect(() => {
    let cancelled = false;
    setServerConsent(null);
    setPrivacyConsentLoadError(null);
    fetchStudentPrivacyConsent(student.id)
      .then((consent) => {
        if (cancelled) return;
        // A 404 (null) means no consent recorded yet → treat as not accepted.
        setServerConsent(
          consent
            ? {
                accepted: consent.accepted,
                dataRetentionDays: consent.dataRetentionDays,
              }
            : { accepted: false, dataRetentionDays: 30 },
        );
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setPrivacyConsentLoadError(
          "Datenschutzeinwilligung konnte nicht geladen werden. Bitte laden Sie die Seite neu.",
        );
        logger.warn("privacy_consent_load_failed", {
          student_id: student.id,
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [student.id]);

  // Mirror the freshly loaded consent into the editable draft. Like the
  // photo-consent sync effect, this touches ONLY the privacy keys so unrelated
  // in-flight edits are preserved. originalDraft (below) also depends on
  // serverConsent, so the form is not falsely marked dirty after the load.
  useEffect(() => {
    if (serverConsent === null) return;
    setFormData((prev) => {
      if (
        prev.privacy_consent_accepted === serverConsent.accepted &&
        prev.data_retention_days === serverConsent.dataRetentionDays
      ) {
        return prev;
      }
      return {
        ...prev,
        privacy_consent_accepted: serverConsent.accepted,
        data_retention_days: serverConsent.dataRetentionDays,
      };
    });
  }, [serverConsent]);

  const originalDraft = useMemo(
    () => buildDraft(student, photosEnabled, serverConsent),
    [student, photosEnabled, serverConsent],
  );
  const companionsDirty = useMemo(
    () =>
      companionsStatus === "ready" &&
      companionsFingerprint(loadedCompanions) !==
        companionsFingerprint(companions),
    [companions, companionsStatus, loadedCompanions],
  );
  const isDirty = useMemo(() => {
    if (pendingPhotoBlob !== null) return true;
    if (pendingPhotoRemoved) return true;
    // The picker writes to its own state, not into formData — without this the
    // Save button stays disabled for a companion-only edit.
    if (companionsDirty) return true;
    const keys = Object.keys(originalDraft) as Array<keyof Student>;
    return keys.some((key) => {
      if (key === "bus_days") {
        return !busDaysEqual(originalDraft.bus_days, formData.bus_days);
      }
      if (key === "pickup_days") {
        return !pickupDaysEqual(
          originalDraft.pickup_days,
          formData.pickup_days,
        );
      }
      if (key === "departure_days") {
        return !allowedDepartureModesEqual(
          originalDraft.allowed_departure_modes,
          formData.allowed_departure_modes,
        );
      }
      if (key === "allowed_departure_modes") {
        return !allowedDepartureModesEqual(
          originalDraft.allowed_departure_modes,
          formData.allowed_departure_modes,
        );
      }
      return originalDraft[key] !== formData[key];
    });
  }, [
    companionsDirty,
    originalDraft,
    formData,
    pendingPhotoBlob,
    pendingPhotoRemoved,
  ]);

  const handleChange = useCallback(
    (
      field: keyof Student,
      value: string | boolean | number | BusDays | AllowedDepartureModes | null,
    ) => {
      setFormData((prev) => ({ ...prev, [field]: value }));
      if (errors[field]) {
        setErrors((prev) => {
          const next = { ...prev };
          delete next[field];
          return next;
        });
      }
    },
    [errors],
  );

  const validateForm = useCallback(() => {
    const next = validateStudentForm(
      formData,
      {
        firstName: true,
        lastName: true,
        schoolClass: false,
      },
      {
        // A link answers "mit wem" too — without this, an accompanied plan
        // backed only by a Laufgemeinschaft could never be saved from this tab.
        // While the stored links are unknown, claiming "none" would block an
        // unrelated edit for the wrong reason; the backend re-checks the rule
        // against the stored links either way.
        hasCompanionLink: companionsStatus !== "ready" || companions.length > 0,
      },
    );
    setErrors(next);
    return Object.keys(next).length === 0;
  }, [companions, companionsStatus, formData]);

  // Consent toggle. Picking + then withdrawing consent in the same session
  // is contradictory (you can't upload a photo without consent), so we
  // drop any pending pick when consent goes off.
  const handleConsentChange = useCallback(
    (value: boolean) => {
      handleChange("photo_consent_given", value);
      if (!value) setPendingPhotoBlob(null);
    },
    [handleChange],
  );

  const handlePickPhoto = useCallback((blob: Blob | null) => {
    setPendingPhotoBlob(blob);
    setPendingPhotoRemoved(false);
  }, []);

  const handleMarkRemoved = useCallback(() => {
    setPendingPhotoBlob(null);
    setPendingPhotoRemoved(true);
  }, []);

  // Reverses a not-yet-saved removal so the existing server photo stays
  // in place when the user clicks Speichern. Triggered by the secondary
  // button in StudentPhotoSection ("Entfernung rückgängig"). pendingBlob
  // is already null in this branch (handleMarkRemoved cleared it) — but
  // we don't touch it here either way to avoid clobbering a freshly
  // picked replacement that arrived through some other path.
  const handleCancelRemove = useCallback(() => {
    setPendingPhotoRemoved(false);
  }, []);

  // Custom submit. The form's photo state lives next to its other fields,
  // so submit orchestrates: (1) regular student PUT for the rest of the
  // form FIRST; (2) photo upload OR delete only after the PUT succeeds.
  //
  // Order rationale: if the photo step ran first and the PUT failed, the
  // photo mutation would already be committed on the server while the UI
  // reports "save failed" — a non-atomic edit, and a retry would replay
  // the photo on top of partially saved state. Doing the PUT first means
  // a PUT failure leaves nothing changed; a photo failure after a
  // successful PUT is surfaced as a partial-success error so the user
  // knows the rest was saved and only the photo needs another try.
  //
  // Withdrawal path: when the user un-ticks consent, the PUT itself
  // clears photo_path + unlinks the file via the backend's
  // applyPhotoConsent. We skip the explicit DELETE in that branch — a
  // second call would 404 / no-op but adds noise.
  const performSave = useCallback(
    async (
      extendPlans: boolean,
      confirmed: CompanionExtensionConfirmation[],
    ) => {
      setSaving(true);
      const submitData: Partial<Student> = { ...formData };
      submitData.allowed_departure_modes = normalizeAllowedDepartureModes(
        formData.allowed_departure_modes ??
          allowedDepartureModesFromDeparture(
            formData.departure_days ??
              departureDaysFromLegacy(formData.bus_days, formData.pickup_days),
          ),
      );
      submitData.departure_days = allowedDepartureToDepartureDays(
        submitData.allowed_departure_modes,
      );
      if (
        submitData.photo_consent_given === originalDraft.photo_consent_given
      ) {
        delete submitData.photo_consent_given;
      }
      // Only when the stored list is known. A pending or failed load leaves the
      // key off entirely, which the backend reads as "don't touch the links"
      // instead of "delete them all".
      if (companionsStatus === "ready") {
        submitData.companions = companions.map((companion) => ({
          companion_student_id: companion.companion_student_id,
          weekdays: companion.weekdays,
        }));
        submitData.extend_companion_plans = extendPlans;
        submitData.confirmed_companion_extensions = confirmed;
      }
      try {
        await onSave(submitData);
        setPlanConflict(null);
      } catch (err) {
        // A companion-plan 409 is a question, not a failure: nothing was
        // written, and re-sending with extend_companion_plans after the user
        // confirms completes the save. Only possible when a list traveled.
        if (companionsStatus === "ready" && isCompanionPlanConflict(err)) {
          setPlanConflict(companionConflictMessage(err));
          setConfirmedExtensions((current) =>
            mergeCompanionConfirmations(
              current,
              companionConflictExtensions(err),
            ),
          );
          setSaving(false);
          return;
        }
        const message = err instanceof Error ? err.message : String(err);
        logger.error("error saving student", { error: message });
        setErrors({
          submit: "Fehler beim Speichern. Bitte versuchen Sie es erneut.",
        });
        setSaving(false);
        return;
      }
      // The links are now the stored ones. student.id doesn't change on a
      // refetch, so the load effect won't re-run — move the snapshot forward
      // here or the form stays permanently dirty after a companion edit.
      if (companionsStatus === "ready") {
        setLoadedCompanions(companions);
      }

      const consentNowOn = Boolean(formData.photo_consent_given);
      let mutatedPhoto = false;
      try {
        if (pendingPhotoBlob && consentNowOn) {
          await uploadStudentPhoto(student.id, pendingPhotoBlob, {
            consentAcknowledged: true,
          });
          mutatedPhoto = true;
        } else if (pendingPhotoRemoved && consentNowOn) {
          await deleteStudentPhoto(student.id);
          mutatedPhoto = true;
        }
        setPendingPhotoBlob(null);
        setPendingPhotoRemoved(false);
      } catch (err) {
        // Form data is already persisted at this point — surface a
        // partial-success message so the user can retry just the photo
        // step. We deliberately keep pendingPhotoBlob / pendingPhotoRemoved
        // populated so re-clicking Speichern replays the photo mutation.
        const message = err instanceof Error ? err.message : String(err);
        logger.error("error saving student photo", { error: message });
        setErrors({
          submit:
            "Daten gespeichert, aber das Foto konnte nicht aktualisiert werden. Bitte versuchen Sie es erneut.",
        });
      } finally {
        setSaving(false);
      }

      // Refresh AFTER the photo mutation so the student object the parent
      // hands back carries the new photo_url. onSave's SWR mutate fires
      // before this point, so without a second refresh we'd render the
      // student in its pre-photo state.
      if (mutatedPhoto && onStudentRefresh) {
        try {
          await onStudentRefresh();
        } catch (err) {
          // Don't surface refresh errors as save errors — the mutation
          // itself succeeded; the next list-level refetch will reconcile.
          const message = err instanceof Error ? err.message : String(err);
          logger.warn("student_refresh_after_photo_failed", { error: message });
        }
      }
    },
    [
      companions,
      companionsStatus,
      formData,
      onSave,
      onStudentRefresh,
      originalDraft.photo_consent_given,
      pendingPhotoBlob,
      pendingPhotoRemoved,
      student.id,
    ],
  );

  const handleSubmit = useCallback(
    async (event: React.FormEvent) => {
      event.preventDefault();
      if (!validateForm()) return;
      if (privacyConsentLoadError) {
        setErrors({ submit: privacyConsentLoadError });
        return;
      }
      await performSave(extendCompanionPlans, confirmedExtensions);
    },
    [
      confirmedExtensions,
      extendCompanionPlans,
      performSave,
      privacyConsentLoadError,
      validateForm,
    ],
  );

  // The user confirmed widening the linked child's departure plan: repeat the
  // identical save with the confirmation flag set.
  const handleConfirmExtendPlans = useCallback(() => {
    setExtendCompanionPlans(true);
    setPlanConflict(null);
    void performSave(true, confirmedExtensions);
  }, [confirmedExtensions, performSave]);

  return (
    <form onSubmit={handleSubmit} noValidate className="space-y-5">
      {planConflict ? (
        <div className="rounded-lg border border-[#F78C10] bg-[#F78C10]/5 p-3">
          <p className="text-sm text-gray-900">{planConflict}</p>
          <p className="mt-1 text-xs text-gray-600">
            Soll „Anderes Kind“ im Heimweg des verknüpften Kindes ergänzt
            werden? Bestehende Heimwege bleiben erhalten.
          </p>
          <div className="mt-2 flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setPlanConflict(null)}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              disabled={saving}
              onClick={handleConfirmExtendPlans}
            >
              Ergänzen und speichern
            </Button>
          </div>
        </div>
      ) : null}
      {errors.submit ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3">
          <p className="text-sm text-red-800">{errors.submit}</p>
        </div>
      ) : null}
      {privacyConsentLoadError ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3">
          <p className="text-sm text-red-800">{privacyConsentLoadError}</p>
        </div>
      ) : null}
      {companionsStatus === "error" ? (
        <div className="rounded-lg border border-[#FF3130] bg-[#FF3130]/5 p-3">
          <p className="text-sm text-gray-900">
            Die Laufgemeinschaft konnte nicht geladen werden und wird unten
            nicht angezeigt. Andere Angaben lassen sich speichern, die
            bestehenden Verknüpfungen bleiben dabei unverändert.
          </p>
          <div className="mt-2 flex justify-end">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setReloadCompanions((count) => count + 1)}
            >
              Erneut laden
            </Button>
          </div>
        </div>
      ) : null}

      <PersonalInfoSection
        formData={formData}
        onChange={handleChange}
        errors={errors}
        groups={groups}
        requiredFields={{
          firstName: true,
          lastName: true,
          schoolClass: false,
        }}
      />

      {/* Photo section sits directly under Personalien — operators expect
          the avatar control next to name/class, not buried below
          health/notes. Gated on the per-tenant feature flag.
          Photo selection is local-only — picking a file just stores a
          Blob in this component's state; the actual upload runs in
          handleSubmit when the user clicks Speichern. */}
      {photosEnabled ? (
        <StudentPhotoSection
          student={student}
          consentGiven={Boolean(formData.photo_consent_given)}
          onConsentChange={handleConsentChange}
          pendingPhotoBlob={pendingPhotoBlob}
          pendingPhotoRemoved={pendingPhotoRemoved}
          onPickPhoto={handlePickPhoto}
          onMarkRemoved={handleMarkRemoved}
          onCancelRemove={handleCancelRemove}
        />
      ) : null}

      <StudentCommonFormSections
        formData={formData}
        errors={errors}
        onChange={handleChange}
      />

      <EnrollmentExtraFieldsSection
        groups={enrollmentExtraGroups}
        loading={enrollmentExtraLoading}
        hasError={enrollmentExtraLoadError}
      />

      <DepartureSection
        days={formData.allowed_departure_modes}
        onChange={(value) => {
          const allowed = normalizeAllowedDepartureModes(value);
          const departure = allowedDepartureToDepartureDays(allowed);
          const busDays = allowedDepartureToBusDays(allowed);
          const pickupDays = allowedDepartureToPickupDays(allowed);
          setFormData((prev) => ({
            ...prev,
            allowed_departure_modes: allowed,
            departure_days: departure,
            // Keep the derived legacy fields consistent for any reader.
            bus_days: busDays,
            bus: busDaysHaveAny(busDays),
            pickup_days: pickupDays,
            pickup_status: pickupDaysHaveAny(pickupDays)
              ? "Wird abgeholt"
              : "Geht alleine nach Hause",
          }));
        }}
        companionNote={formData.departure_companion_note}
        onCompanionNoteChange={(value) =>
          setFormData((prev) => ({
            ...prev,
            departure_companion_note: value,
          }))
        }
        companionNoteError={errors.departure_companion_note}
        companions={companions}
        // Hidden until the stored links are known (the section drops the picker
        // without a change handler): an edit made against a pending or failed
        // load would be silently discarded on save, since the list only travels
        // when it is the stored one.
        onCompanionsChange={
          companionsStatus === "ready" ? setCompanions : undefined
        }
        companionStudentId={String(student.id)}
        onCompanionExtensionConfirmed={(confirmation) => {
          setExtendCompanionPlans(true);
          setConfirmedExtensions((current) =>
            mergeCompanionConfirmations(current, [confirmation]),
          );
        }}
      />

      <EnrollmentConsentsSection
        agbAcceptedAt={student.agb_accepted_at}
        dataProcessingAcceptedAt={student.data_processing_accepted_at}
        emailContactAcceptedAt={student.email_contact_accepted_at}
        photoConsentGivenAt={student.photo_consent_given_at}
      />

      <div className="sticky bottom-0 -mx-6 -mb-6 flex items-center justify-end gap-2 border-t border-gray-100 bg-white/95 px-6 py-3 backdrop-blur-sm">
        <Button
          type="submit"
          variant="primary"
          disabled={saving || !isDirty || privacyConsentLoadError !== null}
        >
          {saving ? (
            <>
              <Loader2 className="mr-1.5 h-4 w-4 animate-spin" aria-hidden />
              Speichern...
            </>
          ) : (
            <>
              <Save className="mr-1.5 h-4 w-4" aria-hidden />
              Speichern
            </>
          )}
        </Button>
      </div>
    </form>
  );
}

function EnrollmentExtraFieldsSection({
  groups,
  loading,
  hasError,
}: Readonly<{
  groups: StudentEnrollmentExtraFieldGroup[];
  loading: boolean;
  hasError: boolean;
}>) {
  if (loading && groups.length === 0) {
    return (
      <section className="moto-content-surface rounded-xl border p-3 shadow-sm md:p-4">
        <EnrollmentExtraFieldsTitle />
        <p className="text-sm text-gray-600">Zusatzangaben werden geladen...</p>
      </section>
    );
  }

  if (hasError) {
    return (
      <section className="moto-content-surface rounded-xl border p-3 shadow-sm md:p-4">
        <EnrollmentExtraFieldsTitle />
        <Alert
          type="error"
          message="Zusatzangaben konnten nicht geladen werden."
        />
      </section>
    );
  }

  if (groups.length === 0) return null;
  const prefixWithPhase = groups.length > 1;

  return (
    <section className="moto-content-surface rounded-xl border p-3 shadow-sm md:p-4">
      <EnrollmentExtraFieldsTitle />
      <dl className="grid grid-cols-1 gap-3 md:grid-cols-2 md:gap-4">
        {groups.flatMap((group) =>
          group.fields.map((field) => {
            const value = formatCustomValue(field.value, field);
            if (value === null) return null;
            const label =
              prefixWithPhase && group.phase_name
                ? `${group.phase_name} · ${field.label}`
                : field.label;
            return (
              <ReadOnlyInfoField
                key={`${group.request_id}-${field.key}`}
                label={label}
                value={value}
              />
            );
          }),
        )}
      </dl>
    </section>
  );
}

function EnrollmentExtraFieldsTitle() {
  return (
    <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-4 md:text-sm">
      <ClipboardList
        className="h-3.5 w-3.5 md:h-4 md:w-4"
        style={{ color: ENROLLMENT_EXTRA_ACCENT }}
        aria-hidden
      />
      Zusatzangaben
    </h3>
  );
}

function ReadOnlyInfoField({
  label,
  value,
}: Readonly<{
  label: string;
  value: ReactNode;
}>) {
  return (
    <div>
      <p className="mb-1 block text-xs font-medium text-gray-700">{label}</p>
      <div className="min-h-9 rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900">
        {value}
      </div>
    </div>
  );
}
