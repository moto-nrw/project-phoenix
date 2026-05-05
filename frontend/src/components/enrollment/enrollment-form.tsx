"use client";

import { useEffect, useState } from "react";
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

interface ChildDraft {
  first_name: string;
  last_name: string;
  date_of_birth: string;
  target_grade_level: string;
  offering_ids: Set<string>;
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

interface Props {
  readonly phaseID: string;
  readonly gradeLevelMax: number;
  readonly onSubmitted: (statusURL: string) => void;
}

/**
 * Public enrollment form. Loads the active schema + open care offerings,
 * collects guardian info + 1..n children + offering selections, and
 * submits to /api/enrollment/{tenantSlug}/submit. The captcha widget is
 * inlined when enrollment.captcha_site_key is present in the payload —
 * captcha verification happens server-side.
 *
 * The form intentionally renders core fields (guardian + child names,
 * email, DOB, grade) hardcoded — the form_schemas.fields JSONB only
 * adds custom fields on top. PR 5 enforced this distinction in the
 * model's CoreFieldKeys map.
 */
export function EnrollmentForm({ phaseID, gradeLevelMax, onSubmitted }: Props) {
  const { tenantSlug } = useTenant();
  const [schema, setSchema] = useState<PublicFormSchema | null>(null);
  const [offerings, setOfferings] = useState<PublicCareOffering[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [guardianFirstName, setGuardianFirstName] = useState("");
  const [guardianLastName, setGuardianLastName] = useState("");
  const [guardianEmail, setGuardianEmail] = useState("");
  const [guardianPhone, setGuardianPhone] = useState("");
  const [agbConsent, setAgbConsent] = useState(false);
  const [dataConsent, setDataConsent] = useState(false);
  const [emailConsent, setEmailConsent] = useState(false);
  const [photoConsent, setPhotoConsent] = useState(false);
  const [children, setChildren] = useState<ChildDraft[]>([blankChild()]);
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
        const [schemaResult, offeringsResult, profileResult] =
          await Promise.all([
            fetchPublicActiveSchema(tenantSlug, phaseID).catch(() => null),
            fetchPublicCareOfferings(tenantSlug, phaseID),
            fetchMyEnrollmentProfile().catch(() => null),
          ]);
        if (cancelled) return;
        setSchema(schemaResult);
        setOfferings(offeringsResult);
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
  }, [tenantSlug, phaseID]);

  const updateChild = (index: number, patch: Partial<ChildDraft>) => {
    setChildren((prev) =>
      prev.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    );
  };

  const toggleOffering = (childIndex: number, offeringID: string) => {
    setChildren((prev) =>
      prev.map((c, i) => {
        if (i !== childIndex) return c;
        const next = new Set(c.offering_ids);
        if (next.has(offeringID)) next.delete(offeringID);
        else next.add(offeringID);
        return { ...c, offering_ids: next };
      }),
    );
  };

  const addChild = () => setChildren((prev) => [...prev, blankChild()]);
  const removeChild = (index: number) =>
    setChildren((prev) => prev.filter((_, i) => i !== index));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!agbConsent || !dataConsent || !emailConsent) {
      setError("Bitte bestätige alle erforderlichen Zustimmungen.");
      return;
    }

    const payloadChildren: SubmitChildPayload[] = [];
    for (const [i, c] of children.entries()) {
      if (!c.first_name || !c.last_name || !c.date_of_birth) {
        setError(
          `Kind ${i + 1}: Vorname, Nachname und Geburtsdatum sind Pflichtfelder.`,
        );
        return;
      }
      payloadChildren.push({
        first_name: c.first_name.trim(),
        last_name: c.last_name.trim(),
        date_of_birth: c.date_of_birth,
        target_grade_level: c.target_grade_level
          ? Number(c.target_grade_level)
          : undefined,
        custom_data: c.custom,
        offering_ids: Array.from(c.offering_ids).map((id) => Number(id)),
      });
    }

    setSubmitting(true);
    try {
      const result = await submitEnrollment(tenantSlug, {
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
        children: payloadChildren,
        captcha_token: captchaToken || undefined,
      });
      onSubmitted(result.status_url);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("enrollment_submit_failed", { error: message });
      setError(message);
    } finally {
      setSubmitting(false);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-8">
      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800">
          {error}
        </div>
      )}

      <section className="space-y-4 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-gray-900">
          Erziehungsberechtigte/r
        </h2>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <Input
            label="Vorname *"
            value={guardianFirstName}
            onChange={setGuardianFirstName}
            required
          />
          <Input
            label="Nachname *"
            value={guardianLastName}
            onChange={setGuardianLastName}
            required
          />
          <Input
            label="E-Mail *"
            type="email"
            value={guardianEmail}
            onChange={setGuardianEmail}
            required
          />
          <Input
            label="Telefon"
            value={guardianPhone}
            onChange={setGuardianPhone}
          />
        </div>
      </section>

      <section className="space-y-4">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Kind(er)</h2>
          <button
            type="button"
            onClick={addChild}
            className="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
          >
            + Weiteres Kind
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
            className="space-y-3 rounded-lg border border-gray-200 bg-white p-6 shadow-sm"
          >
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold text-gray-700">
                Kind {i + 1}
              </h3>
              {children.length > 1 && (
                <button
                  type="button"
                  onClick={() => removeChild(i)}
                  className="text-xs text-red-600 hover:text-red-800"
                >
                  Entfernen
                </button>
              )}
            </div>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input
                label="Vorname *"
                value={child.first_name}
                onChange={(v) => updateChild(i, { first_name: v })}
                required
              />
              <Input
                label="Nachname *"
                value={child.last_name}
                onChange={(v) => updateChild(i, { last_name: v })}
                required
              />
              <div>
                <span className="block text-xs font-medium text-gray-600">
                  Geburtsdatum *
                </span>
                <DateOfBirthPicker
                  value={child.date_of_birth}
                  onChange={(v) => updateChild(i, { date_of_birth: v })}
                />
              </div>
              <label className="block">
                <span className="block text-xs font-medium text-gray-600">
                  Klassenstufe
                </span>
                <select
                  value={child.target_grade_level}
                  onChange={(e) =>
                    updateChild(i, { target_grade_level: e.target.value })
                  }
                  className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
                >
                  <option value="">– bitte wählen –</option>
                  {Array.from({ length: gradeLevelMax }, (_, n) => n + 1).map(
                    (g) => (
                      <option key={g} value={g}>
                        {g}. Klasse
                      </option>
                    ),
                  )}
                </select>
              </label>
            </div>

            {offerings.length > 0 && (
              <div>
                <p className="mb-2 text-xs font-medium text-gray-600">
                  Betreuungsangebote
                </p>
                <div className="space-y-2">
                  {offerings.map((o) => {
                    const checked = child.offering_ids.has(o.id);
                    return (
                      <label
                        key={o.id}
                        className={`flex cursor-pointer items-start gap-3 rounded-md border p-3 text-sm ${checked ? "border-gray-900 bg-gray-50" : "border-gray-200"}`}
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleOffering(i, o.id)}
                          className="mt-1"
                        />
                        <div>
                          <div className="font-medium text-gray-900">
                            {o.name}
                          </div>
                          {o.description && (
                            <div className="text-xs text-gray-600">
                              {o.description}
                            </div>
                          )}
                          <div className="mt-1 text-xs text-gray-500">
                            Tage:{" "}
                            {o.available_days
                              .map((d) => DAY_LABELS[d] ?? d)
                              .join(", ")}
                            {o.includes_holiday_care &&
                              " · inkl. Ferienbetreuung"}
                            {o.includes_lunch && " · inkl. Mittagessen"}
                          </div>
                        </div>
                      </label>
                    );
                  })}
                </div>
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

      <section className="space-y-3 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-gray-900">Zustimmungen</h2>
        <Consent
          label="Ich akzeptiere die AGB der Schule."
          checked={agbConsent}
          onChange={setAgbConsent}
          required
        />
        <Consent
          label="Ich willige in die Verarbeitung der angegebenen Daten gemäß DSGVO ein."
          checked={dataConsent}
          onChange={setDataConsent}
          required
        />
        <Consent
          label="Die Schule darf mich per E-Mail kontaktieren (Rückfragen, Statusbenachrichtigungen)."
          checked={emailConsent}
          onChange={setEmailConsent}
          required
        />
        <Consent
          label="Mein Kind darf bei Schulveranstaltungen fotografiert werden (optional)."
          checked={photoConsent}
          onChange={setPhotoConsent}
        />
      </section>

      <input
        type="hidden"
        name="captcha_token"
        value={captchaToken}
        onChange={(e) => setCaptchaToken(e.target.value)}
      />
      {/*
        Captcha widget: PR 7 ships the form skeleton + the backend
        Verify path. The Cloudflare Turnstile widget is injected by
        the page wrapper when enrollment.captcha_site_key is set; the
        widget's onSuccess handler calls window.dispatchEvent('moto-captcha')
        with the token, which a small companion component picks up.
        Until that wiring lands, captcha can be disabled per-tenant
        via enrollment.require_captcha=false for development/testing.
      */}

      <button
        type="submit"
        disabled={submitting}
        className="w-full rounded-xl bg-gray-900 py-3 text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
      >
        {submitting ? "Wird übermittelt..." : "Anmeldung absenden"}
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
    custom: {},
  };
}

function Input({
  label,
  value,
  onChange,
  type = "text",
  required = false,
}: {
  readonly label: string;
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly type?: string;
  readonly required?: boolean;
}) {
  return (
    <label className="block">
      <span className="block text-xs font-medium text-gray-600">{label}</span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required={required}
        className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm shadow-sm"
      />
    </label>
  );
}

function Consent({
  label,
  checked,
  onChange,
  required = false,
}: {
  readonly label: string;
  readonly checked: boolean;
  readonly onChange: (v: boolean) => void;
  readonly required?: boolean;
}) {
  return (
    <label className="flex items-start gap-3 text-sm">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        required={required}
        className="mt-1"
      />
      <span className="text-gray-700">
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
    <span className="block text-xs font-medium text-gray-600">
      {field.label}
      {field.required && <span className="text-red-500"> *</span>}
    </span>
  );
  const valueStr = typeof value === "string" ? value : "";

  if (field.type === "boolean") {
    return (
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          checked={Boolean(value)}
          onChange={(e) => onChange(e.target.checked)}
        />
        {labelEl}
      </label>
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
          className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
          required={field.required}
        />
      </label>
    );
  }
  if (field.type === "select") {
    return (
      <label className="block">
        {labelEl}
        <select
          value={valueStr}
          onChange={(e) => onChange(e.target.value)}
          required={field.required}
          className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
        >
          <option value="">– bitte wählen –</option>
          {(field.options ?? []).map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
      </label>
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
        type={inputType}
        value={valueStr}
        onChange={(e) => onChange(e.target.value)}
        required={field.required}
        className="mt-1 w-full rounded-md border-gray-300 px-3 py-2 text-sm"
      />
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
 * the parent still types those — the panel just saves the names + the
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
              className="flex items-center justify-between gap-3 rounded-md border border-gray-200 bg-white px-3 py-2"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-gray-900">
                  {c.first_name} {c.last_name}
                </p>
                <p className="truncate text-xs text-gray-500">
                  Klasse: {c.school_class || "—"}
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

// DateOfBirthPicker — three-dropdown picker (Tag / Monat / Jahr).
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
}: {
  readonly value: string;
  readonly onChange: (v: string) => void;
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
      // Don't blank the parent on partial selections — wait until all
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
    <div className="mt-1 grid grid-cols-3 gap-2">
      <select
        value={day}
        onChange={(e) => handleDay(e.target.value)}
        className="rounded-md border border-gray-300 bg-white px-2 py-2 text-sm"
        aria-label="Tag"
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
        className="rounded-md border border-gray-300 bg-white px-2 py-2 text-sm"
        aria-label="Monat"
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
        className="rounded-md border border-gray-300 bg-white px-2 py-2 text-sm"
        aria-label="Jahr"
      >
        <option value="">Jahr</option>
        {years.map((y) => (
          <option key={y} value={String(y)}>
            {y}
          </option>
        ))}
      </select>
    </div>
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
