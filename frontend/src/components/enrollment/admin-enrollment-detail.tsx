"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  CalendarClock,
  Check,
  ClipboardList,
  ExternalLink,
  type LucideIcon,
  Mail,
  Phone,
  ShieldCheck,
  UserRound,
  X,
} from "lucide-react";
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

const STATUS_COLORS: Record<
  ChildStatus,
  { bg: string; dot: string; text: string }
> = {
  submitted: { bg: "#EEF3FF", dot: "#5080D8", text: "#355A9A" },
  under_review: { bg: "#EEF3FF", dot: "#5080D8", text: "#355A9A" },
  approved: { bg: "#83CD2D1A", dot: "#83CD2D", text: "#5A8B1F" },
  waitlisted: { bg: "#FFF4E6", dot: "#F78C10", text: "#8A5600" },
  rejected: { bg: "#FF31301A", dot: "#FF3130", text: "#9F1F1E" },
  withdrawn: { bg: "#F3F4F6", dot: "#9CA3AF", text: "#4B5563" },
  pending_renewal: { bg: "#FFF4E6", dot: "#F78C10", text: "#8A5600" },
  auto_renewed: { bg: "#EEF3FF", dot: "#5080D8", text: "#355A9A" },
  pending_admin_review: {
    bg: "#F3F4F6",
    dot: "#9CA3AF",
    text: "#4B5563",
  },
};

const TERMINAL: ReadonlySet<ChildStatus> = new Set([
  "approved",
  "rejected",
  "withdrawn",
]);

interface ActionDef {
  status: DecisionStatus;
  label: string;
  tone: "primary" | "outline" | "danger" | "success";
}

const ACTIONS: ActionDef[] = [
  { status: "approved", label: "Bestätigen", tone: "success" },
  { status: "waitlisted", label: "Warteliste", tone: "outline" },
  { status: "rejected", label: "Ablehnen", tone: "danger" },
  { status: "under_review", label: "Zur Prüfung", tone: "primary" },
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

  const submittedAt = formatDateTime(data.submitted_at, {
    day: "2-digit",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
  const childStats = summarizeChildren(data.children);
  const phaseHref = tenantSlug
    ? `/${tenantSlug}/admin/enrollments/phases/${data.phase_id}`
    : `/admin/enrollments/phases/${data.phase_id}`;
  const statusHref = tenantSlug
    ? `/${tenantSlug}/enroll/status/${data.status_token}`
    : `/enroll/status/${data.status_token}`;

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
          <Link
            href={phaseHref}
            className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Zurück zur Anmeldephase
          </Link>
        </div>
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
          <div className="space-y-6 p-5 sm:p-6">
            <header>
              <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                Anmeldung prüfen
              </p>
              <h1 className="mt-1 text-xl font-semibold text-gray-900">
                {data.guardian_first_name} {data.guardian_last_name}
              </h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                Prüfe die Angaben der Anmeldung, bevor du eine Entscheidung
                speicherst. Die Entscheidung wird pro Kind gesetzt.
              </p>
            </header>

            {error && (
              <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
                {error}
              </div>
            )}
            {info && (
              <div className="rounded-lg border border-[#83CD2D]/20 bg-[#83CD2D]/10 p-3 text-sm text-[#5A8B1F]">
                {info}
              </div>
            )}

            <EnrollmentSummary data={data} submittedAt={submittedAt} />

            <RequestExtraSection request={data} />

            <section className="space-y-3">
              <div>
                <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                  Kinder
                </p>
                <h2 className="mt-1 text-base font-semibold text-gray-900">
                  Angaben der Kinder
                </h2>
                <p className="mt-1 text-sm text-gray-600">
                  Zusatzfragen und Stammdaten-Antworten werden pro Kind
                  angezeigt.
                </p>
              </div>
              {data.children.map((child) => (
                <ChildInformationCard
                  key={child.id}
                  child={child}
                  busy={busyChildId === child.id}
                  reason={reasons[child.id] ?? ""}
                  schemaFields={data.schema_fields}
                  onReasonChange={(value) =>
                    setReasons((prev) => ({ ...prev, [child.id]: value }))
                  }
                  onDecide={(status) => void handleDecide(child.id, status)}
                />
              ))}
            </section>
          </div>

          <aside className="border-t border-gray-100 bg-gray-50/70 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <ReviewSidebar
              childStats={childStats}
              data={data}
              statusHref={statusHref}
              submittedAt={submittedAt}
            />
          </aside>
        </div>
      </section>
    </div>
  );
}

function StatusBadge({ status }: Readonly<{ status: ChildStatus }>) {
  const styles = STATUS_COLORS[status];
  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: styles.dot }}
        aria-hidden="true"
      />
      {STATUS_LABELS[status]}
    </span>
  );
}

function EnrollmentSummary({
  data,
  submittedAt,
}: Readonly<{
  data: AdminRequestSummary;
  submittedAt: string;
}>) {
  return (
    <section className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
          <UserRound className="h-4 w-4" aria-hidden="true" />
        </span>
        <div>
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Erziehungsberechtigte Person
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            {data.guardian_first_name} {data.guardian_last_name}
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            {data.phase_name || "Nicht zugeordnet"}
          </p>
        </div>
      </div>

      <dl className="mt-4 grid gap-3 sm:grid-cols-2">
        <InfoItem icon={Mail} label="E-Mail" value={data.guardian_email} />
        <InfoItem
          icon={Phone}
          label="Telefon"
          value={data.guardian_phone ?? "Nicht gesetzt"}
        />
        <InfoItem
          icon={CalendarClock}
          label="Eingegangen"
          value={submittedAt}
        />
      </dl>
    </section>
  );
}

function InfoItem({
  icon: Icon,
  label,
  mono,
  value,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  mono?: boolean;
  value: string;
}>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-gray-50/70 p-3">
      <dt className="flex items-center gap-2 text-xs font-medium text-gray-500 uppercase">
        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
        {label}
      </dt>
      <dd
        className={`mt-1 text-sm text-gray-900 ${mono ? "font-mono break-all" : ""}`}
      >
        {value}
      </dd>
    </div>
  );
}

function ChildInformationCard({
  busy,
  child,
  onDecide,
  onReasonChange,
  reason,
  schemaFields,
}: Readonly<{
  busy: boolean;
  child: AdminRequestChild;
  onDecide: (status: DecisionStatus) => void;
  onReasonChange: (value: string) => void;
  reason: string;
  schemaFields?: AdminRequestSchemaField[];
}>) {
  const terminal = TERMINAL.has(child.status);
  return (
    <article className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-4">
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            {child.first_name} {child.last_name}
          </h3>
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-gray-600">
            <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
              Geburtsdatum: {formatPlainDate(child.date_of_birth)}
            </span>
            {child.target_grade_level ? (
              <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
                {child.target_grade_level}. Klasse
              </span>
            ) : null}
          </div>
        </div>
        <StatusBadge status={child.status} />
      </div>
      <div className="space-y-4 p-4">
        {child.status_reason ? (
          <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2 text-sm text-gray-700">
            <span className="text-xs font-medium text-gray-500">
              Begründung:{" "}
            </span>
            {child.status_reason}
          </div>
        ) : null}
        {child.reviewed_at ? (
          <p className="text-xs text-gray-500">
            Letzte Entscheidung: {formatDateTime(child.reviewed_at)}
          </p>
        ) : null}
        <ChildOfferings offerings={child.offerings} />
        <ChildExtraFields child={child} schemaFields={schemaFields} />
        {terminal ? (
          <div className="rounded-xl border border-gray-200 bg-gray-50/70 px-3 py-2 text-sm text-gray-600">
            Diese Entscheidung ist final.
          </div>
        ) : (
          <DecisionPanel
            child={child}
            busy={busy}
            reason={reason}
            onReasonChange={onReasonChange}
            onDecide={onDecide}
          />
        )}
      </div>
    </article>
  );
}

function ReviewSidebar({
  childStats,
  data,
  statusHref,
  submittedAt,
}: Readonly<{
  childStats: ReturnType<typeof summarizeChildren>;
  data: AdminRequestSummary;
  statusHref: string;
  submittedAt: string;
}>) {
  return (
    <div className="space-y-4 lg:sticky lg:top-6">
      <section>
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Prüfung
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Status der Anmeldung
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Alle Kinder werden einzeln geprüft. Die Statusseite zeigt Eltern den
          aktuellen Stand der Anmeldung.
        </p>
      </section>

      <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
        <div className="grid grid-cols-3 gap-2">
          <SidebarMetric label="Kinder" value={data.children.length} />
          <SidebarMetric label="Offen" value={childStats.open} />
          <SidebarMetric label="Bestätigt" value={childStats.approved} />
        </div>
        <dl className="mt-4 space-y-3 text-sm">
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">
              Phase
            </dt>
            <dd className="mt-0.5 font-medium text-gray-900">
              {data.phase_name || "Nicht zugeordnet"}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-gray-500 uppercase">
              Eingegangen
            </dt>
            <dd className="mt-0.5 text-gray-900">{submittedAt}</dd>
          </div>
        </dl>
        <a
          href={statusHref}
          target="_blank"
          rel="noreferrer"
          className="mt-4 inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          Statusseite öffnen
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
        </a>
      </section>

      {data.children.map((child) => {
        return (
          <section
            key={child.id}
            className="moto-content-surface rounded-2xl border p-4 shadow-sm"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="truncate text-sm font-semibold text-gray-900">
                  {child.first_name} {child.last_name}
                </h3>
                <p className="mt-1 text-xs text-gray-500">
                  {child.target_grade_level
                    ? `${child.target_grade_level}. Klasse`
                    : "Keine Klassenstufe"}
                </p>
              </div>
              <StatusBadge status={child.status} />
            </div>
            <div className="mt-3 rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2 text-sm text-gray-600">
              {TERMINAL.has(child.status)
                ? "Entscheidung abgeschlossen"
                : "Entscheidung ausstehend"}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function SidebarMetric({
  label,
  value,
}: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2">
      <span className="block text-lg leading-none font-semibold text-gray-900">
        {value}
      </span>
      <span className="mt-1 block text-xs font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

function DecisionPanel({
  busy,
  child,
  onDecide,
  onReasonChange,
  reason,
}: Readonly<{
  busy: boolean;
  child: AdminRequestChild;
  onDecide: (status: DecisionStatus) => void;
  onReasonChange: (value: string) => void;
  reason: string;
}>) {
  return (
    <div className="rounded-xl border border-gray-200 bg-gray-50/70 p-4">
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
          <ShieldCheck className="h-4 w-4" aria-hidden="true" />
        </span>
        <div>
          <h4 className="text-sm font-semibold text-gray-900">
            Entscheidung speichern
          </h4>
          <p className="mt-1 text-xs leading-5 text-gray-600">
            Optional kann eine Begründung ergänzt werden. Je nach Einstellung
            der Anmeldephase ist sie für Eltern sichtbar.
          </p>
        </div>
      </div>

      <label className="mt-4 block">
        <span className="text-xs font-medium text-gray-700">Begründung</span>
        <textarea
          value={reason}
          onChange={(event) => onReasonChange(event.target.value)}
          rows={2}
          placeholder="z. B. Geschwisterkind bevorzugt, voll ausgebucht"
          className="mt-1 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        />
      </label>

      <div className="mt-3 flex flex-wrap gap-2">
        {ACTIONS.map((action) => {
          const isCurrent = child.status === action.status;
          return (
            <button
              key={action.status}
              type="button"
              disabled={busy || isCurrent}
              onClick={() => onDecide(action.status)}
              className={getDecisionButtonClass(action.tone)}
            >
              {getDecisionIcon(action.status)}
              {busy ? "Speichert..." : action.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function getDecisionButtonClass(tone: ActionDef["tone"]): string {
  const base =
    "inline-flex h-9 items-center justify-center gap-2 rounded-lg px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-45";
  if (tone === "success") {
    return `${base} border border-gray-200 bg-white text-gray-700 hover:border-[#83CD2D]/60 hover:bg-[#83CD2D]/10 hover:text-[#5A8B1F]`;
  }
  if (tone === "danger") {
    return `${base} border border-[#FF3130]/20 bg-white text-[#CC2626] hover:bg-[#FF3130]/10`;
  }
  if (tone === "primary") {
    return `${base} border border-gray-900 bg-gray-900 text-white hover:bg-gray-700`;
  }
  return `${base} border border-gray-200 bg-white text-gray-700 hover:bg-gray-50`;
}

function getDecisionIcon(status: DecisionStatus): React.ReactNode {
  if (status === "approved") {
    return <Check className="h-4 w-4" aria-hidden="true" />;
  }
  if (status === "rejected") {
    return <X className="h-4 w-4" aria-hidden="true" />;
  }
  if (status === "under_review") {
    return <ClipboardList className="h-4 w-4" aria-hidden="true" />;
  }
  return <CalendarClock className="h-4 w-4" aria-hidden="true" />;
}

function summarizeChildren(children: AdminRequestChild[]) {
  let open = 0;
  let approved = 0;
  for (const child of children) {
    if (!TERMINAL.has(child.status)) open += 1;
    if (child.status === "approved") approved += 1;
  }
  return { open, approved };
}

function formatDateTime(
  value: string,
  options?: Intl.DateTimeFormatOptions,
): string {
  return new Date(value).toLocaleString("de-DE", options);
}

function formatPlainDate(value: string): string {
  return new Date(`${value}T00:00:00`).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

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

export function formatCustomValue(
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
    const o = v as Record<string, unknown>;
    const weekdays = [
      ["mon", "Mo"],
      ["tue", "Di"],
      ["wed", "Mi"],
      ["thu", "Do"],
      ["fri", "Fr"],
    ] as const;

    // weekday_boolean (Abholregelung, Buskind): values are per-day booleans
    // ({mon: true, tue: false}). List only the selected days as "Mo, Mi, Fr",
    // mirroring the backend export renderer (formatWeekdayBoolean in
    // export_format.go). Detected by value type so it works without `field`.
    if (weekdays.some(([key]) => typeof o[key] === "boolean")) {
      const days = weekdays
        .filter(([key]) => o[key] === true)
        .map(([, label]) => label);
      if (days.length === 0) return null;
      return days.join(", ");
    }

    const cells = weekdays
      .map(([key, label]) => ({ label, value: o[key] }))
      .filter(
        (c) => typeof c.value === "string" && (c.value as string).trim() !== "",
      );
    if (cells.length === 0) return null;
    return (
      <span>
        {cells.map((c) => (
          <span key={c.label} className="mr-3 inline-block">
            <span className="text-gray-500">{c.label}:</span>{" "}
            <span className="font-medium">{c.value as string}</span>
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
    return parts.join(", ");
  }

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
      r.is_primary === true ? " (Hauptnummer)" : ""
    }`;
  }

  return JSON.stringify(r);
}
