"use client";

import { useEffect, useRef, useState } from "react";
import { Check, Plus, Trash2 } from "lucide-react";
import { useTenant } from "~/components/tenant/tenant-provider";
import {
  fetchMyEnrollmentProfile,
  fetchPublicCareOfferings,
  submitEnrollment,
  type MeProfileChild,
  type MeProfileResponse,
  type PublicCareOffering,
  type SubmitChildPayload,
} from "~/lib/enrollment-submission-api";
import {
  fetchPublicActiveSchema,
  type PublicFormSchema,
} from "~/lib/enrollment-form-schema-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentForm" });

// Mirror of the backend canonical phone format
// (backend/models/users/phone_validation.go). Optional country code +
// 7..15 chars of digits/spaces/hyphens. Kept in sync so a value the form
// accepts can't later be rejected at student creation on approval.
const GUARDIAN_PHONE_PATTERN = /^(\+[0-9]{1,3}\s?)?[0-9\s-]{7,15}$/;

// Mirror of the backend canonical email format
// (backend/models/users/email_validation.go) — the single rule enforced both
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
   * Absent when the offering uses fixed days — the form submits no days
   * entry for it. For parent_choice offerings the form REQUIRES at
   * least one day before allowing submit.
   */
  offering_days: Record<string, Set<string>>;
  custom: Record<string, unknown>;
}

const DAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

import type {
  SubmitEnrollmentPayload,
  SubmitEnrollmentResult,
} from "~/lib/enrollment-submission-api";

interface Props {
  readonly phaseID?: string;
  readonly gradeLevelMax: number;
  readonly onSubmitted: (statusURL: string) => void;
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
  previewMode = false,
  previewSchema,
  profileFetcher,
  submitter,
  skipCaptcha,
}: Props) {
  const { tenantSlug } = useTenant();
  const [schema, setSchema] = useState<PublicFormSchema | null>(null);
  const [offerings, setOfferings] = useState<PublicCareOffering[]>([]);
  const [careRequired, setCareRequired] = useState(false);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [childOfferingErrors, setChildOfferingErrors] = useState<
    Record<number, boolean>
  >({});
  // Per-field validation errors, keyed by each input's `name`
  // (guardian_email, children_0_first_name, ...). Drives the red border +
  // inline message on the offending input; rebuilt on every submit.
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  // Scroll the top error banner into view on every submit attempt that
  // leaves an error showing. The form is long (guardian + N children +
  // offerings) with the submit button at the bottom, so an error rendered
  // only at the top is easy to miss. Keyed on a per-attempt counter rather
  // than the error string (the app's shared useScrollToError hook) so a
  // repeated submit with the *same* unchanged message still scrolls back
  // up. On a successful submit the banner isn't rendered, so the ref is
  // null and the scroll is a no-op.
  const errorRef = useRef<HTMLDivElement>(null);
  const [submitAttempt, setSubmitAttempt] = useState(0);

  useEffect(() => {
    if (submitAttempt > 0) {
      errorRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }, [submitAttempt]);

  const [guardianFirstName, setGuardianFirstName] = useState("");
  const [guardianLastName, setGuardianLastName] = useState("");
  const [guardianEmail, setGuardianEmail] = useState("");
  const [guardianPhone, setGuardianPhone] = useState("");
  const [agbConsent, setAgbConsent] = useState(false);
  const [dataConsent, setDataConsent] = useState(false);
  const [emailConsent, setEmailConsent] = useState(false);
  const [photoConsent, setPhotoConsent] = useState(false);
  const [children, setChildren] = useState<ChildDraft[]>([blankChild()]);
  // Request-level custom fields (applies_to_child=false). Stored
  // separately from per-child custom data because their values are
  // shared across all children in the submission.
  const [customData, setCustomData] = useState<Record<string, unknown>>({});
  const [captchaToken, setCaptchaToken] = useState("");
  const [profile, setProfile] = useState<MeProfileResponse | null>(null);
  const [usedExistingChildIDs, setUsedExistingChildIDs] = useState<Set<string>>(
    new Set(),
  );

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const profileLoader = profileFetcher ?? fetchMyEnrollmentProfile;
        const [schemaResult, offeringsResult, profileResult] =
          await Promise.all([
            previewSchema !== undefined
              ? Promise.resolve(previewSchema)
              : phaseID
                ? fetchPublicActiveSchema(tenantSlug, phaseID).catch(() => null)
                : Promise.resolve(null),
            phaseID
              ? fetchPublicCareOfferings(tenantSlug, phaseID)
              : Promise.resolve({ offerings: [], careRequired: false }),
            profileLoader().catch(() => null),
          ]);
        if (cancelled) return;
        setSchema(schemaResult);
        setOfferings(offeringsResult.offerings);
        setCareRequired(offeringsResult.careRequired);
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
          err instanceof Error ? err.message : "Unbekannter Fehler";
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
  }, [tenantSlug, phaseID, previewSchema, profileFetcher]);

  const updateChild = (index: number, patch: Partial<ChildDraft>) => {
    setChildren((prev) =>
      prev.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    );
  };

  const toggleOffering = (childIndex: number, offeringID: string) => {
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const nextIDs = new Set(c.offering_ids);
        const nextDays = { ...c.offering_days };
        if (nextIDs.has(offeringID)) {
          nextIDs.delete(offeringID);
          delete nextDays[offeringID];
        } else {
          nextIDs.add(offeringID);
        }
        return { ...c, offering_ids: nextIDs, offering_days: nextDays };
      }),
    );
  };

  // toggleOfferingDay flips one day in an offering's selected-day set.
  // Only meaningful when the offering's days_of_week_mode is "parent_choice"
  // — callers pre-check that.
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
  };

  const addChild = () => setChildren((prev) => [...prev, blankChild()]);
  const removeChild = (index: number) =>
    setChildren((prev) => prev.filter((_, i) => i !== index));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setChildOfferingErrors({});
    setFieldErrors({});
    // Bump per attempt so the scroll-to-error effect re-runs even when this
    // submit produces the same error message as the previous one. Covers
    // every synchronous validation failure below (error is set in the same
    // batch). On success no error is set, so the scroll is a no-op.
    setSubmitAttempt((n) => n + 1);

    if (previewMode) {
      setError(
        "Dies ist eine Vorschau. Aus dieser Ansicht wird keine Anmeldung abgeschickt.",
      );
      return;
    }
    if (!phaseID) {
      setError("Für diese Anmeldung fehlt der Anmeldezeitraum.");
      return;
    }

    // Collect every missing/invalid required field in one pass so they all
    // get marked at once (red border + inline message) instead of bailing
    // on the first. Keys match each input's `name` attribute.
    const newFieldErrors: Record<string, string> = {};
    if (!guardianFirstName.trim()) {
      newFieldErrors.guardian_first_name = "Bitte Vornamen angeben.";
    }
    if (!guardianLastName.trim()) {
      newFieldErrors.guardian_last_name = "Bitte Nachnamen angeben.";
    }
    const trimmedEmail = guardianEmail.trim();
    if (!trimmedEmail) {
      newFieldErrors.guardian_email = "Bitte E-Mail-Adresse angeben.";
    } else if (!GUARDIAN_EMAIL_PATTERN.test(trimmedEmail)) {
      newFieldErrors.guardian_email = "Bitte gültige E-Mail-Adresse angeben.";
    }
    // Phone is optional, but reject an invalid format before submit so the
    // parent fixes their own typo instead of producing a request that can
    // never be approved (the backend enforces the same rule).
    const trimmedPhone = guardianPhone.trim();
    if (trimmedPhone && !GUARDIAN_PHONE_PATTERN.test(trimmedPhone)) {
      newFieldErrors.guardian_phone = "Bitte gültige Telefonnummer angeben.";
    }
    for (const [i, c] of children.entries()) {
      if (!c.first_name.trim()) {
        newFieldErrors[`children_${i}_first_name`] = "Bitte Vornamen angeben.";
      }
      if (!c.last_name.trim()) {
        newFieldErrors[`children_${i}_last_name`] = "Bitte Nachnamen angeben.";
      }
      if (!c.date_of_birth) {
        newFieldErrors[`children_${i}_date_of_birth`] =
          "Bitte Geburtsdatum angeben.";
      }
      if (!c.target_grade_level) {
        newFieldErrors[`children_${i}_target_grade_level`] =
          "Bitte Klassenstufe angeben.";
      }
    }
    // Required consents are collected into the same pass so a missing one is
    // marked (red checkbox) together with any other missing field, instead of
    // only showing in the banner.
    if (!agbConsent) {
      newFieldErrors.consent_agb = "Bitte den AGB zustimmen.";
    }
    if (!dataConsent) {
      newFieldErrors.consent_data_processing =
        "Bitte der Datenverarbeitung zustimmen.";
    }
    if (!emailConsent) {
      newFieldErrors.consent_email_contact =
        "Bitte dem E-Mail-Kontakt zustimmen.";
    }
    const errorKeys = Object.keys(newFieldErrors);
    if (errorKeys.length > 0) {
      setFieldErrors(newFieldErrors);
      // Banner text:
      //  - only consents missing (1+)  → the dedicated consent message
      //  - exactly one other field     → that field's specific message
      //  - anything else / mixed       → a generic summary
      // (the per-field red marking always shows which inputs are affected).
      const onlyConsents = errorKeys.every((key) => key.startsWith("consent_"));
      const [firstMessage] = Object.values(newFieldErrors);
      let banner = "Bitte korrigiere die rot markierten Felder.";
      if (onlyConsents) {
        banner = "Bitte bestätige alle erforderlichen Zustimmungen.";
      } else if (errorKeys.length === 1 && firstMessage) {
        banner = firstMessage;
      }
      setError(banner);
      return;
    }

    const payloadChildren: SubmitChildPayload[] = [];
    const missingCareIndexes: number[] = [];
    for (const [i, c] of children.entries()) {
      // Core fields are already guaranteed present by the field-collection
      // pass above; this loop only builds the payload + validates offerings.
      if (careRequired && c.offering_ids.size === 0) {
        missingCareIndexes.push(i);
      }

      // Build the optional offering_days payload: one entry per
      // parent_choice offering the parent picked. Fixed offerings get
      // no entry (backend treats absence as "use the admin-fixed
      // days"). Reject submit when a parent_choice offering is picked
      // but has no day selected — the message names the offending row.
      const offeringDaysPayload: Array<{
        offering_id: number;
        selected_days: string[];
      }> = [];
      for (const id of c.offering_ids) {
        const offering = offerings.find((o) => o.id === id);
        if (!offering) continue;
        if (offering.days_of_week_mode !== "parent_choice") continue;
        const picked = c.offering_days[id];
        if (!picked || picked.size === 0) {
          setError(
            `Kind ${i + 1}: Beim Angebot „${offering.name}" muss mindestens ein Tag ausgewählt werden.`,
          );
          return;
        }
        offeringDaysPayload.push({
          offering_id: Number(id),
          // Preserve the admin-defined order from available_days so
          // the wire payload doesn't fluctuate with the parent's click
          // order; deterministic ordering helps idempotency / dedup.
          selected_days: offering.available_days.filter((d) => picked.has(d)),
        });
      }

      payloadChildren.push({
        first_name: c.first_name.trim(),
        last_name: c.last_name.trim(),
        date_of_birth: c.date_of_birth,
        target_grade_level: Number(c.target_grade_level),
        custom_data: c.custom,
        offering_ids: Array.from(c.offering_ids).map((id) => Number(id)),
        offering_days:
          offeringDaysPayload.length > 0 ? offeringDaysPayload : undefined,
      });
    }
    if (missingCareIndexes.length > 0) {
      setChildOfferingErrors(
        Object.fromEntries(missingCareIndexes.map((i) => [i, true])),
      );
      setError(
        "Bitte wähle für jedes Kind mindestens ein Betreuungsangebot aus.",
      );
      return;
    }

    setSubmitting(true);
    try {
      const payload: SubmitEnrollmentPayload = {
        phase_id: Number(phaseID),
        guardian_first_name: guardianFirstName.trim(),
        guardian_last_name: guardianLastName.trim(),
        guardian_email: guardianEmail.trim().toLowerCase(),
        guardian_phone: guardianPhone.trim() || undefined,
        consent_flags: {
          agb: agbConsent,
          data_processing: dataConsent,
          email_contact: emailConsent,
          photo: photoConsent,
        },
        custom_data: customData,
        children: payloadChildren,
        captcha_token: skipCaptcha ? undefined : captchaToken || undefined,
      };
      const result = submitter
        ? await submitter(payload)
        : await submitEnrollment(tenantSlug, payload);
      onSubmitted(result.status_url);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      const code = (err as { code?: string } | undefined)?.code;
      logger.error("enrollment_submit_failed", { error: message, code });
      if (code === "enrollment.care_offering_missing") {
        // Backend re-checked the setting (defense-in-depth). Mark every
        // child without an offering. The server doesn't tell us which
        // one, so we highlight all that are empty.
        const empties = children.reduce<Record<number, boolean>>(
          (acc, c, i) => {
            if (c.offering_ids.size === 0) acc[i] = true;
            return acc;
          },
          {},
        );
        setChildOfferingErrors(empties);
      }
      setError(message);
      // A server-side rejection resolves after the synchronous attempt bump
      // above, so bump again to scroll the late-arriving error into view.
      setSubmitAttempt((n) => n + 1);
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 shadow-sm sm:p-6">
        Formular wird geladen…
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="space-y-5">
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
        <SectionHeading
          kicker="Schritt 1"
          title="Erziehungsberechtigte Person"
          description="Geben Sie die Person an, die Rückfragen beantworten kann und den Status-Link erhalten soll."
        />
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Input
            label="Vorname *"
            name="guardian_first_name"
            autoComplete="given-name"
            value={guardianFirstName}
            onChange={setGuardianFirstName}
            required
            error={fieldErrors.guardian_first_name}
          />
          <Input
            label="Nachname *"
            name="guardian_last_name"
            autoComplete="family-name"
            value={guardianLastName}
            onChange={setGuardianLastName}
            required
            error={fieldErrors.guardian_last_name}
          />
          <Input
            label="E-Mail *"
            name="guardian_email"
            type="email"
            autoComplete="email"
            value={guardianEmail}
            onChange={setGuardianEmail}
            required
            error={fieldErrors.guardian_email}
          />
          <Input
            label="Telefon"
            name="guardian_phone"
            type="tel"
            autoComplete="tel"
            inputMode="tel"
            value={guardianPhone}
            onChange={setGuardianPhone}
            error={fieldErrors.guardian_phone}
          />
        </div>
      </section>

      {schema?.fields.some((f) => !f.applies_to_child) && (
        <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
          <SectionHeading
            kicker="Zusatzfragen"
            title="Weitere Angaben"
            description="Die OGS benötigt diese Angaben zusätzlich zu den Basisdaten. Pflichtfragen sind mit einem Stern markiert."
          />
          {schema.fields
            .filter((f) => !f.applies_to_child)
            .map((f) => (
              <CustomFieldInput
                key={f.key}
                field={f}
                value={customData[f.key]}
                onChange={(v) =>
                  setCustomData((prev) => ({ ...prev, [f.key]: v }))
                }
              />
            ))}
        </section>
      )}

      <section className="moto-content-surface space-y-5 rounded-xl border p-4 shadow-sm sm:p-5">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <SectionHeading
            kicker="Schritt 2"
            title="Kind oder Kinder"
            description="Erfassen Sie jedes Kind einzeln. Für Geschwister können Sie über die Schaltfläche weitere Kind-Daten hinzufügen."
          />
          <button
            type="button"
            onClick={addChild}
            className="moto-content-surface inline-flex h-9 w-full items-center justify-center gap-1.5 rounded-lg border px-3 text-sm font-semibold whitespace-nowrap text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto sm:shrink-0"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Weiteres Kind
          </button>
        </div>

        {profile && profile.children.length > 0 && (
          <ExistingChildrenPanel
            existing={profile.children}
            usedIDs={usedExistingChildIDs}
            onAdopt={(child) => {
              const newSlot: ChildDraft = blankChild();
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
                Kind {i + 1}
              </h3>
              {children.length > 1 && (
                <button
                  type="button"
                  onClick={() => removeChild(i)}
                  className="rounded-lg px-2 py-1 text-xs font-medium text-[#CC2626] transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
                >
                  Entfernen
                </button>
              )}
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input
                label="Vorname *"
                name={`children_${i}_first_name`}
                autoComplete="given-name"
                value={child.first_name}
                onChange={(v) => updateChild(i, { first_name: v })}
                required
                error={fieldErrors[`children_${i}_first_name`]}
              />
              <Input
                label="Nachname *"
                name={`children_${i}_last_name`}
                autoComplete="family-name"
                value={child.last_name}
                onChange={(v) => updateChild(i, { last_name: v })}
                required
                error={fieldErrors[`children_${i}_last_name`]}
              />
              <div>
                <span className="block text-sm font-semibold text-gray-700">
                  Geburtsdatum *
                </span>
                <DateOfBirthPicker
                  value={child.date_of_birth}
                  onChange={(v) => updateChild(i, { date_of_birth: v })}
                  error={fieldErrors[`children_${i}_date_of_birth`]}
                />
              </div>
              <GradeLevelSelect
                name={`children_${i}_target_grade_level`}
                value={child.target_grade_level}
                onChange={(v) => updateChild(i, { target_grade_level: v })}
                max={gradeLevelMax}
                error={fieldErrors[`children_${i}_target_grade_level`]}
              />
            </div>

            {offerings.length > 0 && (
              <div>
                <div className="mb-2">
                  <p className="text-sm font-semibold text-gray-700">
                    Betreuungsangebote
                    {careRequired && (
                      <span className="ml-1 text-[#EF4444]">*</span>
                    )}
                  </p>
                  <p className="mt-1 text-xs leading-5 text-gray-500">
                    Wählen Sie die Angebote aus, die für dieses Kind gewünscht
                    sind. Die OGS prüft freie Plätze nach dem Absenden.
                  </p>
                </div>
                <div
                  className={`space-y-2 ${
                    childOfferingErrors[i]
                      ? "rounded-md border border-[#EF4444] bg-red-50 p-2"
                      : ""
                  }`}
                >
                  {offerings.map((o) => {
                    const checked = child.offering_ids.has(o.id);
                    return (
                      <label
                        key={o.id}
                        className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 text-sm transition-colors ${
                          checked
                            ? "border-[#83CD2D]/40 bg-[#83CD2D]/10"
                            : "border-gray-200 bg-white hover:border-gray-300"
                        }`}
                      >
                        <span className="relative mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-gray-300 bg-white">
                          <input
                            name={`children_${i}_offering_${o.id}`}
                            type="checkbox"
                            checked={checked}
                            onChange={() => {
                              toggleOffering(i, o.id);
                              if (childOfferingErrors[i]) {
                                setChildOfferingErrors((prev) => {
                                  const next = { ...prev };
                                  delete next[i];
                                  return next;
                                });
                              }
                            }}
                            className="absolute inset-0 cursor-pointer opacity-0"
                          />
                          {checked && (
                            <span className="flex h-5 w-5 items-center justify-center rounded-md bg-[#83CD2D] text-white">
                              <Check className="h-3.5 w-3.5" />
                            </span>
                          )}
                        </span>
                        <div className="min-w-0">
                          <div className="font-medium break-words text-gray-900">
                            {o.name}
                          </div>
                          {o.description && (
                            <div className="text-xs break-words text-gray-600">
                              {o.description}
                            </div>
                          )}
                          <div className="mt-1 text-xs text-gray-500">
                            {o.days_of_week_mode === "parent_choice"
                              ? "Wählbare Tage: "
                              : "Tage: "}
                            {o.available_days
                              .map((d) => DAY_LABELS[d] ?? d)
                              .join(", ")}
                            {o.includes_holiday_care &&
                              " · inkl. Ferienbetreuung"}
                            {o.includes_lunch && " · inkl. Mittagessen"}
                          </div>
                          {checked &&
                            o.days_of_week_mode === "parent_choice" && (
                              <div className="mt-2 flex flex-wrap gap-1.5">
                                {o.available_days.map((day) => {
                                  const picked =
                                    child.offering_days[o.id]?.has(day) ??
                                    false;
                                  return (
                                    <button
                                      key={day}
                                      type="button"
                                      onClick={(e) => {
                                        // The card's outer <label> would
                                        // otherwise treat this as a click on
                                        // the offering checkbox itself.
                                        e.preventDefault();
                                        e.stopPropagation();
                                        toggleOfferingDay(i, o.id, day);
                                      }}
                                      className={`rounded-md border px-2 py-0.5 text-xs font-medium transition-colors ${
                                        picked
                                          ? "border-[#83CD2D] bg-[#83CD2D] text-white"
                                          : "border-gray-300 bg-white text-gray-700 hover:border-gray-400"
                                      }`}
                                      aria-pressed={picked}
                                    >
                                      {DAY_LABELS[day] ?? day}
                                    </button>
                                  );
                                })}
                              </div>
                            )}
                        </div>
                      </label>
                    );
                  })}
                </div>
                {childOfferingErrors[i] && (
                  <p className="mt-1 text-xs text-[#EF4444]">
                    Bitte mindestens ein Betreuungsangebot auswählen.
                  </p>
                )}
              </div>
            )}

            {schema?.fields
              .filter((f) => f.applies_to_child)
              .map((f) => (
                <CustomFieldInput
                  key={f.key}
                  field={f}
                  value={child.custom[f.key]}
                  onChange={(v) =>
                    updateChild(i, { custom: { ...child.custom, [f.key]: v } })
                  }
                />
              ))}
          </div>
        ))}
      </section>

      <section className="moto-content-surface space-y-4 rounded-xl border p-4 shadow-sm sm:p-5">
        <SectionHeading
          kicker="Schritt 3"
          title="Zustimmungen"
          description="Bitte lesen Sie die Zustimmungen sorgfältig. Erforderliche Zustimmungen müssen aktiv ausgewählt werden."
        />
        <Consent
          name="consent_agb"
          label="Ich akzeptiere die AGB der Schule."
          checked={agbConsent}
          onChange={setAgbConsent}
          required
          error={!!fieldErrors.consent_agb}
        />
        <Consent
          name="consent_data_processing"
          label="Ich willige in die Verarbeitung der angegebenen Daten gemäß DSGVO ein."
          checked={dataConsent}
          onChange={setDataConsent}
          required
          error={!!fieldErrors.consent_data_processing}
        />
        <Consent
          name="consent_email_contact"
          label="Die Schule darf mich per E-Mail kontaktieren (Rückfragen, Statusbenachrichtigungen)."
          checked={emailConsent}
          onChange={setEmailConsent}
          required
          error={!!fieldErrors.consent_email_contact}
        />
        <Consent
          name="consent_photo"
          label="Mein Kind darf bei Schulveranstaltungen fotografiert werden (optional)."
          checked={photoConsent}
          onChange={setPhotoConsent}
        />
      </section>

      {!skipCaptcha && (
        <input
          type="hidden"
          name="captcha_token"
          value={captchaToken}
          onChange={(e) => setCaptchaToken(e.target.value)}
        />
      )}
      {/*
        Captcha widget: PR 7 ships the form skeleton + the backend
        Verify path. The Cloudflare Turnstile widget is injected by
        the page wrapper when enrollment.captcha_site_key is set; the
        widget's onSuccess handler calls window.dispatchEvent('moto-captcha')
        with the token, which a small companion component picks up.
        Until that wiring lands, captcha can be disabled per-tenant
        via enrollment.require_captcha=false for development/testing.
        skipCaptcha=true skips the widget entirely, set by callers
        that have already authenticated the user (parent JWT path).
      */}

      <button
        type="submit"
        disabled={submitting || previewMode}
        className="h-11 w-full rounded-lg bg-gray-900 text-sm font-semibold text-white shadow-sm transition-colors duration-200 hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-400"
      >
        {previewMode
          ? "Vorschau, nicht absenden"
          : submitting
            ? "Wird übermittelt…"
            : "Anmeldung absenden"}
      </button>
    </form>
  );
}

function blankChild(): ChildDraft {
  return {
    first_name: "",
    last_name: "",
    date_of_birth: "",
    target_grade_level: "",
    offering_ids: new Set(),
    offering_days: {},
    custom: {},
  };
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
        spellCheck={type === "email" ? false : undefined}
        className={`mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
          error
            ? "border-[#EF4444] bg-red-50"
            : "moto-content-surface hover:border-gray-300"
        }`}
      />
      {error && <p className="mt-1 text-xs text-[#EF4444]">{error}</p>}
    </label>
  );
}

function GradeLevelSelect({
  name,
  value,
  onChange,
  max,
  error,
}: {
  readonly name: string;
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly max: number;
  readonly error?: string;
}) {
  return (
    <label className="block">
      <span className="block text-sm font-semibold text-gray-700">
        Klassenstufe *
      </span>
      <select
        name={name}
        value={value}
        required
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={error ? true : undefined}
        className={`moto-select mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
          error
            ? "border-[#EF4444] bg-red-50"
            : "moto-content-surface hover:border-gray-300"
        }`}
      >
        <option value="" disabled>
          Bitte wählen
        </option>
        {Array.from({ length: max }, (_, n) => n + 1).map((g) => (
          <option key={g} value={g}>
            {g}. Klasse
          </option>
        ))}
      </select>
      {error && <p className="mt-1 text-xs text-[#EF4444]">{error}</p>}
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

function Consent({
  name,
  label,
  checked,
  onChange,
  required = false,
  error = false,
}: {
  readonly name: string;
  readonly label: string;
  readonly checked: boolean;
  readonly onChange: (v: boolean) => void;
  readonly required?: boolean;
  readonly error?: boolean;
}) {
  return (
    <label
      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 text-sm transition-colors ${
        checked
          ? "border-[#83CD2D]/40 bg-[#83CD2D]/10"
          : error
            ? "border-[#EF4444] bg-red-50"
            : "border-gray-200 bg-white hover:border-gray-300"
      }`}
    >
      <span className="relative mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-md border border-gray-300 bg-white">
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
      <span className="leading-6 font-medium text-gray-700">
        {label} {required && <span className="text-red-500">*</span>}
      </span>
    </label>
  );
}

interface CustomFieldInputProps {
  readonly field: PublicFormSchema["fields"][number];
  readonly value: unknown;
  readonly onChange: (v: unknown) => void;
}

function CustomFieldInput({ field, value, onChange }: CustomFieldInputProps) {
  const labelEl = (
    <span className="block text-sm font-semibold text-gray-700">
      {field.label}
      {field.required && <span className="text-red-500"> *</span>}
    </span>
  );
  const valueStr = typeof value === "string" ? value : "";

  if (field.type === "boolean") {
    const selectedValue = value === true ? "yes" : value === false ? "no" : "";
    return (
      <fieldset>
        {labelEl}
        <div className="mt-2 grid grid-cols-2 gap-2">
          {[
            { label: "Ja", value: "yes", raw: true },
            { label: "Nein", value: "no", raw: false },
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
          className="moto-content-surface mt-1 w-full rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          aria-required={field.required}
        />
      </label>
    );
  }
  if (field.type === "select") {
    return (
      <label className="block">
        {labelEl}
        <select
          name={field.key}
          value={valueStr}
          onChange={(e) => onChange(e.target.value)}
          aria-required={field.required}
          className="moto-select moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <option value="">Bitte wählen</option>
          {(field.options ?? []).map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
    );
  }
  if (field.type === "phone_list") {
    return <PhoneListInput field={field} value={value} onChange={onChange} />;
  }
  if (field.type === "weekday_schedule") {
    return (
      <WeekdayScheduleInput field={field} value={value} onChange={onChange} />
    );
  }
  if (field.type === "contact_list") {
    return <ContactListInput field={field} value={value} onChange={onChange} />;
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
        className="moto-content-surface mt-1 h-10 w-full rounded-lg border px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      />
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

const PHONE_TYPE_LABELS: Record<PhoneEntry["phone_type"], string> = {
  mobile: "Mobil",
  home: "Privat",
  work: "Arbeit",
  other: "Sonstige",
};

function asPhoneArray(v: unknown): PhoneEntry[] {
  if (!Array.isArray(v)) return [];
  return v.filter((row): row is PhoneEntry => {
    if (!row || typeof row !== "object") return false;
    const r = row as Record<string, unknown>;
    return typeof r.phone_number === "string";
  });
}

function PhoneListInput({ field, value, onChange }: CustomFieldInputProps) {
  const phones = asPhoneArray(value);
  const update = (next: PhoneEntry[]) => onChange(next);
  const setRow = (idx: number, patch: Partial<PhoneEntry>) =>
    update(phones.map((p, i) => (i === idx ? { ...p, ...patch } : p)));
  return (
    <fieldset className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {field.label}
        {field.required && <span className="text-red-500"> *</span>}
      </legend>
      {phones.length === 0 && (
        <p className="mt-2 text-sm text-gray-500">
          Noch keine Nummer eingetragen.
        </p>
      )}
      <ul className="space-y-2">
        {phones.map((p, idx) => (
          <li
            key={idx}
            className="moto-content-surface grid items-end gap-3 rounded-xl border p-3 shadow-sm md:grid-cols-[minmax(0,1fr)_180px_auto_auto]"
          >
            <label className="block">
              <span className="text-xs font-medium text-gray-700">Nummer</span>
              <input
                type="tel"
                value={p.phone_number}
                onChange={(e) => setRow(idx, { phone_number: e.target.value })}
                className="mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              />
            </label>
            <label className="block">
              <span className="text-xs font-medium text-gray-700">Typ</span>
              <select
                value={p.phone_type}
                onChange={(e) =>
                  setRow(idx, {
                    phone_type: e.target.value as PhoneEntry["phone_type"],
                  })
                }
                className="moto-select mt-1 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                {(
                  Object.keys(PHONE_TYPE_LABELS) as PhoneEntry["phone_type"][]
                ).map((k) => (
                  <option key={k} value={k}>
                    {PHONE_TYPE_LABELS[k]}
                  </option>
                ))}
              </select>
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
              label="Hauptnummer"
              name={`${field.key}-primary`}
              type="radio"
            />
            <button
              type="button"
              onClick={() => update(phones.filter((_, i) => i !== idx))}
              className="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-[#FF3130]/20 bg-white text-[#CC2626] shadow-sm transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
              aria-label="Nummer entfernen"
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
        Telefonnummer hinzufügen
      </button>
    </fieldset>
  );
}

const WEEKDAYS = [
  { key: "mon", label: "Montag" },
  { key: "tue", label: "Dienstag" },
  { key: "wed", label: "Mittwoch" },
  { key: "thu", label: "Donnerstag" },
  { key: "fri", label: "Freitag" },
] as const;

function asScheduleObject(v: unknown): Record<string, string> {
  if (!v || typeof v !== "object" || Array.isArray(v)) return {};
  const out: Record<string, string> = {};
  for (const w of WEEKDAYS) {
    const raw = (v as Record<string, unknown>)[w.key];
    if (typeof raw === "string") out[w.key] = raw;
  }
  return out;
}

function WeekdayScheduleInput({
  field,
  value,
  onChange,
}: CustomFieldInputProps) {
  const sched = asScheduleObject(value);
  return (
    <fieldset className="rounded-lg border border-gray-200 p-3">
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <span className="text-red-500"> *</span>}
      </legend>
      <p className="text-xs text-gray-500">
        Leere Felder bedeuten: an diesem Tag keine Angabe.
      </p>
      <div className="mt-2 grid gap-2 sm:grid-cols-5">
        {WEEKDAYS.map((w) => (
          <label key={w.key} className="block text-xs">
            <span className="block text-gray-600">{w.label}</span>
            <input
              type="time"
              value={sched[w.key] ?? ""}
              onChange={(e) => onChange({ ...sched, [w.key]: e.target.value })}
              className="mt-1 h-9 w-full rounded-md border border-gray-200 bg-white px-2 text-sm"
            />
          </label>
        ))}
      </div>
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

function ContactListInput({ field, value, onChange }: CustomFieldInputProps) {
  const contacts = asContactArray(value);
  const update = (next: ContactEntryValue[]) => onChange(next);
  const setRow = (idx: number, patch: Partial<ContactEntryValue>) =>
    update(contacts.map((c, i) => (i === idx ? { ...c, ...patch } : c)));
  return (
    <fieldset className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm">
      <legend className="px-1 text-sm font-semibold text-gray-900">
        {field.label}
        {field.required && <span className="text-red-500"> *</span>}
      </legend>
      {contacts.length === 0 && (
        <p className="mt-2 text-sm text-gray-500">
          Noch kein zusätzlicher Kontakt eingetragen.
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
                  Vorname
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
                  Nachname
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
                  Beziehung (z. B. Oma, Onkel, Nachbarin)
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
                  E-Mail (optional)
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
                  label: "Telefonnummern",
                  type: "phone_list",
                  required: false,
                }}
                value={c.phone_numbers ?? []}
                onChange={(v) =>
                  setRow(idx, {
                    phone_numbers: Array.isArray(v) ? (v as PhoneEntry[]) : [],
                  })
                }
              />
            </div>

            <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
              <div className="grid gap-2 sm:grid-cols-2">
                <StructuredToggle
                  checked={Boolean(c.can_pickup)}
                  onChange={(checked) => setRow(idx, { can_pickup: checked })}
                  label="Abholberechtigt"
                />
                <StructuredToggle
                  checked={Boolean(c.is_emergency_contact)}
                  onChange={(checked) =>
                    setRow(idx, { is_emergency_contact: checked })
                  }
                  label="Notfallkontakt"
                />
              </div>
              <button
                type="button"
                onClick={() => update(contacts.filter((_, i) => i !== idx))}
                className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-[#FF3130]/20 bg-white px-3 text-sm font-medium text-[#CC2626] shadow-sm transition-colors hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-[#FF3130]/30 focus-visible:outline-none"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
                Kontakt entfernen
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
        Kontakt hinzufügen
      </button>
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
}: ExistingChildrenPanelProps) {
  return (
    <div className="rounded-lg border border-[#83CD2D]/40 bg-[#83CD2D]/5 p-4">
      <h3 className="text-sm font-semibold text-gray-900">Bestehende Kinder</h3>
      <p className="mt-1 text-xs text-gray-600">
        Diese Kinder sind schon bei der Schule erfasst. Klicke auf „Übernehmen"
        um sie schnell in die Anmeldung einzutragen. Geburtsdatum musst du noch
        eintragen.
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
                  Klasse: {c.school_class || "Nicht gesetzt"}
                </p>
              </div>
              <button
                type="button"
                onClick={() => onAdopt(c)}
                disabled={used}
                className="shrink-0 rounded-md bg-gray-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {used ? "✓ übernommen" : "Übernehmen"}
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
// year input and snaps back. Three <select>s let parents pick fast
// without touching a keyboard. Wire format stays YYYY-MM-DD.
const MONTH_LABELS = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
];

function DateOfBirthPicker({
  value,
  onChange,
  error,
}: {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly error?: string;
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

  const selectClass = `moto-select h-10 rounded-lg border px-3 text-sm shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
    error
      ? "border-[#EF4444] bg-red-50"
      : "moto-content-surface hover:border-gray-300"
  }`;

  return (
    <>
      <div className="mt-1 grid grid-cols-3 gap-2">
        <select
          value={day}
          onChange={(e) => handleDay(e.target.value)}
          className={selectClass}
          aria-label="Tag"
          aria-invalid={error ? true : undefined}
        >
          <option value="">Tag</option>
          {Array.from({ length: daysInMonth }, (_, i) => i + 1).map((d) => (
            <option key={d} value={String(d)}>
              {d}
            </option>
          ))}
        </select>
        <select
          value={month}
          onChange={(e) => handleMonth(e.target.value)}
          className={selectClass}
          aria-label="Monat"
          aria-invalid={error ? true : undefined}
        >
          <option value="">Monat</option>
          {MONTH_LABELS.map((label, idx) => (
            <option key={label} value={String(idx + 1)}>
              {label}
            </option>
          ))}
        </select>
        <select
          value={year}
          onChange={(e) => handleYear(e.target.value)}
          className={selectClass}
          aria-label="Jahr"
          aria-invalid={error ? true : undefined}
        >
          <option value="">Jahr</option>
          {years.map((y) => (
            <option key={y} value={String(y)}>
              {y}
            </option>
          ))}
        </select>
      </div>
      {error && <p className="mt-1 text-xs text-[#EF4444]">{error}</p>}
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
    month: String(Number(m[2])), // strip leading zero so the <select> matches
    day: String(Number(m[3])),
  };
}
