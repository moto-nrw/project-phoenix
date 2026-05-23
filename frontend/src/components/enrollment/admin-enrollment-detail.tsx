"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  type AdminRequestChild,
  type AdminRequestChildOffering,
  type AdminRequestSchemaField,
  type AdminRequestSummary,
  type ChildStatus,
  type DecisionStatus,
  decideAdminChild,
  getAdminRequest,
} from "~/lib/enrollment-admin-api";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { Button } from "~/components/ui/button";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentDetail" });

const STATUS_LABELS: Record<ChildStatus, string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
  pending_renewal: "Wartet auf Verlängerung",
  auto_renewed: "Vorgemerkt",
  pending_admin_review: "Manuelle Prüfung",
};

const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  submitted: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  under_review: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  approved: { bg: LOCATION_COLORS.GROUP_ROOM, text: "#FFFFFF" },
  waitlisted: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  rejected: { bg: LOCATION_COLORS.HOME, text: "#FFFFFF" },
  withdrawn: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
  pending_renewal: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  auto_renewed: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  pending_admin_review: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
};

const TERMINAL: ReadonlySet<ChildStatus> = new Set([
  "approved",
  "rejected",
  "withdrawn",
]);

interface ActionDef {
  status: DecisionStatus;
  label: string;
  variant: "primary" | "outline" | "danger" | "success";
}

const ACTIONS: ActionDef[] = [
  { status: "approved", label: "Bestätigen", variant: "success" },
  { status: "waitlisted", label: "Warteliste", variant: "outline" },
  { status: "rejected", label: "Ablehnen", variant: "danger" },
  { status: "under_review", label: "Zur Prüfung", variant: "primary" },
];

interface Props {
  readonly requestId: string;
}

export function AdminEnrollmentDetail({ requestId }: Props) {
  const tenantSlug = useTenantSlugSafe();
  const [data, setData] = useState<AdminRequestSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [busyChildId, setBusyChildId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const fresh = await getAdminRequest(requestId);
      setData(fresh);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("admin_enrollment_detail_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [requestId]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleDecide = async (childId: string, status: DecisionStatus) => {
    if (!data) return;
    const reason = (reasons[childId] ?? "").trim();
    setBusyChildId(childId);
    setError(null);
    setInfo(null);
    try {
      await decideAdminChild(requestId, childId, status, reason || undefined);
      setInfo(
        `Entscheidung gespeichert: ${STATUS_LABELS[status as ChildStatus]}`,
      );
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("admin_enrollment_decide_failed", {
        error: message,
        request_id: requestId,
        child_id: childId,
        status,
      });
      setError(message);
    } finally {
      setBusyChildId(null);
    }
  };

  if (loading) {
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }
  if (!data) {
    return (
      <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
        {error ?? "Anmeldung nicht gefunden."}
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeaderWithSearch
        title={`Anmeldung von ${data.guardian_first_name} ${data.guardian_last_name}`}
        actionButton={
          <Link
            href={
              tenantSlug
                ? `/${tenantSlug}/admin/enrollments`
                : `/admin/enrollments`
            }
            className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Zur Übersicht
          </Link>
        }
      />
      <p className="-mt-2 text-sm text-gray-600">
        Phase: <strong>{data.phase_name || "Nicht zugeordnet"}</strong>
        {" · "}
        Eingegangen am{" "}
        {new Date(data.submitted_at).toLocaleString("de-DE", {
          day: "2-digit",
          month: "long",
          year: "numeric",
          hour: "2-digit",
          minute: "2-digit",
        })}
      </p>

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-2xl border border-[#83CD2D]/20 bg-[#83CD2D]/10 p-4 text-sm text-[#5A8B1F]">
          {info}
        </div>
      )}

      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <h2 className="text-base font-semibold text-gray-900">
          Erziehungsberechtigte/r
        </h2>
        <dl className="mt-3 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs text-gray-500 uppercase">Name</dt>
            <dd className="text-gray-900">
              {data.guardian_first_name} {data.guardian_last_name}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">E-Mail</dt>
            <dd className="text-gray-900">{data.guardian_email}</dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">Telefon</dt>
            <dd className="text-gray-900">
              {data.guardian_phone ?? "Nicht gesetzt"}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-gray-500 uppercase">Status-Link</dt>
            <dd>
              <code className="rounded bg-gray-100 px-2 py-1 text-xs break-all">
                /enroll/status/{data.status_token}
              </code>
            </dd>
          </div>
        </dl>
      </section>

      <RequestExtraSection request={data} />

      <section className="space-y-3">
        <h2 className="text-base font-semibold text-gray-900">Kinder</h2>
        {data.children.map((c) => {
          const terminal = TERMINAL.has(c.status);
          return (
            <div
              key={c.id}
              className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold text-gray-900">
                    {c.first_name} {c.last_name}
                  </h3>
                  <p className="mt-1 text-xs text-gray-600">
                    Geburtsdatum: {c.date_of_birth}
                    {c.target_grade_level
                      ? ` · ${c.target_grade_level}. Klasse`
                      : ""}
                  </p>
                  {c.status_reason && (
                    <p className="mt-2 text-sm text-gray-700">
                      <span className="text-xs font-medium text-gray-500">
                        Begründung:{" "}
                      </span>
                      {c.status_reason}
                    </p>
                  )}
                  {c.reviewed_at && (
                    <p className="mt-1 text-xs text-gray-500">
                      Letzte Entscheidung:{" "}
                      {new Date(c.reviewed_at).toLocaleString("de-DE", {
                        day: "2-digit",
                        month: "2-digit",
                        year: "numeric",
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </p>
                  )}
                </div>
                <StatusBadge status={c.status} />
              </div>

              <ChildOfferings offerings={c.offerings} />
              <ChildExtraFields child={c} schemaFields={data.schema_fields} />

              {!terminal && (
                <div className="mt-4 space-y-2">
                  <label className="block">
                    <span className="text-xs font-medium text-gray-700">
                      Begründung (optional, sichtbar je nach
                      Anmeldephaseneinstellung)
                    </span>
                    <textarea
                      value={reasons[c.id] ?? ""}
                      onChange={(e) =>
                        setReasons((prev) => ({
                          ...prev,
                          [c.id]: e.target.value,
                        }))
                      }
                      rows={2}
                      placeholder="z. B. Geschwisterkind bevorzugt, voll ausgebucht"
                      className="mt-1 w-full rounded-lg border border-gray-200 px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    />
                  </label>
                  <div className="flex flex-wrap gap-2">
                    {ACTIONS.map((a) => {
                      const isCurrent = c.status === a.status;
                      return (
                        <Button
                          key={a.status}
                          type="button"
                          disabled={busyChildId === c.id || isCurrent}
                          onClick={() => void handleDecide(c.id, a.status)}
                          variant={a.variant}
                          size="sm"
                          className="px-3 py-1.5 text-xs shadow-none disabled:cursor-not-allowed disabled:opacity-40"
                        >
                          {busyChildId === c.id
                            ? "Wird gespeichert..."
                            : a.label}
                        </Button>
                      );
                    })}
                  </div>
                </div>
              )}
              {terminal && (
                <p className="mt-3 text-xs text-gray-500">
                  Diese Entscheidung ist final. Für Promotionen (z. B.
                  Warteliste zu Bestätigt) folgt die volle Workflow-Logik in
                  einer kommenden Version.
                </p>
              )}
            </div>
          );
        })}
      </section>
    </div>
  );
}

function StatusBadge({ status }: Readonly<{ status: ChildStatus }>) {
  const styles = STATUS_COLORS[status];
  return (
    <span
      className="inline-flex rounded-full px-3 py-1 text-xs font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}

// ---- extra-info rendering ---------------------------------------------
//
// RequestExtraSection shows the AGB/photo consent block + every
// request-level custom field (applies_to_child=false) the parent
// filled in. ChildExtraFields does the same per child for per-child
// (applies_to_child=true) fields. Together they surface every answer
// the parent gave beyond the core form so the admin has full context
// when deciding.

const CONSENT_LABELS: Record<string, string> = {
  agb: "AGB der Schule",
  data_processing: "Datenverarbeitung (DSGVO)",
  email_contact: "E-Mail-Kontakt",
  photo: "Fotos bei Schulveranstaltungen",
};

function RequestExtraSection({
  request,
}: Readonly<{ request: AdminRequestSummary }>) {
  const guardianFields = (request.schema_fields ?? []).filter(
    (f) => !f.applies_to_child,
  );
  const hasCustom = guardianFields.some(
    (f) => formatCustomValue(request.custom_data?.[f.key], f) !== null,
  );
  const hasConsents = Object.keys(request.consent_flags ?? {}).length > 0;

  if (!hasCustom && !hasConsents) return null;

  return (
    <section className="moto-content-surface space-y-3 rounded-2xl border p-5 shadow-sm">
      <h2 className="text-base font-semibold text-gray-900">
        Zusätzliche Angaben
      </h2>

      {hasConsents && (
        <div>
          <h3 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
            Zustimmungen
          </h3>
          <ul className="mt-1.5 space-y-1 text-sm">
            {Object.entries(request.consent_flags ?? {}).map(([key, val]) => (
              <li key={key} className="flex items-center gap-2">
                <span
                  className="inline-block h-2 w-2 rounded-full"
                  style={{
                    backgroundColor: val === true ? "#83CD2D" : "#6B7280",
                  }}
                />
                <span className="text-gray-700">
                  {CONSENT_LABELS[key] ?? key}:
                </span>
                <span className="font-medium text-gray-900">
                  {val === true ? "Ja" : val === false ? "Nein" : String(val)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {hasCustom && (
        <div>
          <h3 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
            Zusatzfragen (Eltern)
          </h3>
          <dl className="mt-1.5 space-y-2 text-sm">
            {guardianFields.map((f) => {
              const formatted = formatCustomValue(
                request.custom_data?.[f.key],
                f,
              );
              if (formatted === null) return null;
              return (
                <div key={f.key}>
                  <dt className="text-xs font-medium text-gray-600">
                    {f.label}
                  </dt>
                  <dd className="mt-0.5 text-gray-900">{formatted}</dd>
                </div>
              );
            })}
          </dl>
        </div>
      )}
    </section>
  );
}

// ChildOfferings renders the per-child Betreuungsangebote selection.
// For fixed offerings the day badge shows the offering's full
// schedule (Mo, Di, …). For parent_choice offerings the badge marks
// only the days the parent picked, prefixed with the picked count
// so admins can spot half-week selections at a glance.
const DAY_LABEL_DE: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

function ChildOfferings({
  offerings,
}: Readonly<{ offerings?: AdminRequestChildOffering[] }>) {
  if (!offerings || offerings.length === 0) return null;
  return (
    <div className="mt-3 rounded-lg border border-gray-100 bg-gray-50/70 p-3">
      <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
        Betreuungsangebote
      </h4>
      <ul className="mt-1.5 space-y-2 text-sm">
        {offerings.map((o) => {
          const parentChoice = o.days_of_week_mode === "parent_choice";
          const displayDays = parentChoice
            ? (o.selected_days ?? [])
            : (o.available_days ?? []);
          return (
            <li
              key={o.offering_id}
              className="flex flex-wrap items-baseline gap-x-3 gap-y-1"
            >
              <span className="font-medium text-gray-900">
                {o.offering_name || `Angebot #${o.offering_id}`}
              </span>
              {displayDays.length > 0 ? (
                <span className="text-xs text-gray-600">
                  {parentChoice ? "Elternauswahl: " : "Tage: "}
                  {displayDays.map((d) => DAY_LABEL_DE[d] ?? d).join(", ")}
                </span>
              ) : parentChoice ? (
                <span className="text-xs text-[#CC2626] italic">
                  Keine Tage gewählt
                </span>
              ) : null}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

function ChildExtraFields({
  child,
  schemaFields,
}: Readonly<{
  child: AdminRequestChild;
  schemaFields?: AdminRequestSchemaField[];
}>) {
  const childFields = (schemaFields ?? []).filter((f) => f.applies_to_child);
  const filled = childFields
    .map((f) => ({
      field: f,
      value: formatCustomValue(child.custom_data?.[f.key], f),
    }))
    .filter((row) => row.value !== null);
  if (filled.length === 0) return null;
  return (
    <div className="mt-3 rounded-lg border border-gray-100 bg-gray-50/70 p-3">
      <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
        Zusätzliche Angaben zum Kind
      </h4>
      <dl className="mt-1.5 space-y-1.5 text-sm">
        {filled.map(({ field, value }) => (
          <div key={field.key}>
            <dt className="text-xs font-medium text-gray-600">{field.label}</dt>
            <dd className="mt-0.5 text-gray-900">{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

// formatCustomValue produces a German-friendly display string for any
// JSON-ish value pulled out of custom_data. Returns null when the
// value is effectively empty so callers can hide the row.
//
// When a field definition is supplied and it's a select, the raw
// stored value (e.g. "picked_up") is mapped back to its option label
// (e.g. "Wird abgeholt"). Unknown values fall back to the raw string.
function formatCustomValue(
  v: unknown,
  field?: AdminRequestSchemaField,
): React.ReactNode | null {
  if (v === null || v === undefined) return null;
  if (typeof v === "boolean") return v ? "Ja" : "Nein";
  if (typeof v === "string") {
    const trimmed = v.trim();
    if (trimmed === "") return null;
    if (field?.type === "select" && field.options) {
      const opt = field.options.find((o) => o.value === trimmed);
      if (opt) return opt.label;
    }
    return trimmed;
  }
  if (typeof v === "number") return String(v);

  // Structured types ----------------------------------------------------
  if (Array.isArray(v)) {
    if (v.length === 0) return null;
    return (
      <ul className="space-y-1 text-sm">
        {v.map((row, i) => (
          <li key={i} className="text-gray-800">
            {formatStructuredEntry(row)}
          </li>
        ))}
      </ul>
    );
  }
  if (typeof v === "object") {
    // weekday_schedule { mon: "15:00", tue: "...", ... }
    const o = v as Record<string, unknown>;
    const weekdays = [
      ["mon", "Mo"],
      ["tue", "Di"],
      ["wed", "Mi"],
      ["thu", "Do"],
      ["fri", "Fr"],
    ] as const;
    const cells = weekdays
      .map(([key, label]) => ({ label, value: o[key] }))
      .filter(
        (c) => typeof c.value === "string" && (c.value as string).trim() !== "",
      );
    if (cells.length === 0) return null;
    return (
      <span>
        {cells.map((c, i) => (
          <span key={c.label} className="mr-3 inline-block">
            <span className="text-gray-500">{c.label}:</span>{" "}
            <span className="font-medium">{c.value as string}</span>
            {i < cells.length - 1 ? "" : ""}
          </span>
        ))}
      </span>
    );
  }
  return String(v);
}

function formatStructuredEntry(row: unknown): React.ReactNode {
  if (!row || typeof row !== "object") return String(row);
  const r = row as Record<string, unknown>;

  // contact_list entry → "Vorname Nachname (Beziehung) · E-Mail · Tel … [Notfall] [Abholberechtigt]"
  if (typeof r.first_name === "string" || typeof r.last_name === "string") {
    const parts: string[] = [];
    const fullName = `${typeof r.first_name === "string" ? r.first_name : ""} ${
      typeof r.last_name === "string" ? r.last_name : ""
    }`.trim();
    if (fullName) parts.push(fullName);
    if (typeof r.relationship_type === "string" && r.relationship_type) {
      parts[parts.length - 1] += ` (${r.relationship_type})`;
    }
    if (typeof r.email === "string" && r.email) parts.push(r.email);
    if (Array.isArray(r.phone_numbers)) {
      const phones = (r.phone_numbers as Array<Record<string, unknown>>)
        .map((p) => (typeof p.phone_number === "string" ? p.phone_number : ""))
        .filter(Boolean);
      if (phones.length > 0) parts.push(phones.join(", "));
    }
    if (r.can_pickup === true) parts.push("Abholberechtigt");
    if (r.is_emergency_contact === true) parts.push("Notfallkontakt");
    return parts.join(" · ");
  }

  // phone_list entry
  if (typeof r.phone_number === "string") {
    const labelMap: Record<string, string> = {
      mobile: "Mobil",
      home: "Privat",
      work: "Arbeit",
      other: "Sonstige",
    };
    const t =
      typeof r.phone_type === "string" && r.phone_type in labelMap
        ? labelMap[r.phone_type as string]
        : null;
    return `${r.phone_number}${t ? ` (${t})` : ""}${
      r.is_primary === true ? " ★" : ""
    }`;
  }

  return JSON.stringify(r);
}
