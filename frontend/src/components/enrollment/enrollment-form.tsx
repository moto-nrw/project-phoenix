"use client";

import { useEffect, useState } from "react";
import { useTenant } from "~/components/tenant/tenant-provider";
import {
  fetchPublicCareOfferings,
  submitEnrollment,
  type PublicCareOffering,
  type SubmitChildPayload,
} from "~/lib/enrollment-submission-api";
import {
  fetchActiveSchema,
  type FormSchema,
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
  readonly calendarPeriodID: string;
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
export function EnrollmentForm({
  calendarPeriodID,
  gradeLevelMax,
  onSubmitted,
}: Props) {
  const { tenantSlug } = useTenant();
  const [schema, setSchema] = useState<FormSchema | null>(null);
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

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const [schemaResult, offeringsResult] = await Promise.all([
          fetchActiveSchema().catch(() => null),
          fetchPublicCareOfferings(tenantSlug),
        ]);
        if (cancelled) return;
        setSchema(schemaResult);
        setOfferings(offeringsResult);
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
  }, [tenantSlug]);

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
        calendar_period_id: Number(calendarPeriodID),
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
              <Input
                label="Geburtsdatum *"
                type="date"
                value={child.date_of_birth}
                onChange={(v) => updateChild(i, { date_of_birth: v })}
                required
              />
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
  readonly field: FormSchema["fields"][number];
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
