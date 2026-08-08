"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import {
  ArrowLeft,
  CalendarClock,
  Check,
  ClipboardList,
  ExternalLink,
  History,
  type LucideIcon,
  Mail,
  Pencil,
  Phone,
  RotateCcw,
  ShieldCheck,
  Trash2,
  UserRound,
  X,
} from "lucide-react";
import {
  type AdminRequestChild,
  type AdminRequestDetail,
  type AdminOfferingAdjustment,
  type AdminRequestChildOffering,
  type AdminRequestGuardian,
  type AdminRequestSchemaField,
  type AdminRequestSummary,
  type ChildStatus,
  type DecisionStatus,
  decideAdminChild,
  getAdminRequest,
  listAdminChildOfferingAdjustments,
  restoreAdminRequest,
  updateAdminChildOfferings,
} from "~/lib/enrollment-admin-api";
import { type CareOffering, listCareOfferings } from "~/lib/care-offering-api";
import { availableCareOfferings } from "~/lib/care-offering-availability";
import { formatOfferingPrice } from "~/lib/care-offering-format";
import { FeaturePill } from "~/components/enrollment/feature-pill";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";
import { formatCalendarDate } from "~/lib/localized-date-format";
import { AdminChildDataCorrection } from "~/components/enrollment/admin-child-data-correction";
import { AdminEnrollmentDeletionModal } from "~/components/enrollment/admin-enrollment-deletion-modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import { createLogger } from "~/lib/logger";
import {
  useCareOfferingsEnabled,
  useWaitlistEnabled,
} from "~/lib/tenant-context";

import {
  CHILD_STATUS_LABELS,
  ChildStatusBadge,
} from "~/components/enrollment/child-status-badge";

const logger = createLogger({ component: "AdminEnrollmentDetail" });

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
  const waitlistEnabled = useWaitlistEnabled();
  const tenantPath = useTenantAwarePath();
  const router = useTenantRouter();
  const [data, setData] = useState<AdminRequestDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);
  const [busyChildId, setBusyChildId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [deletionTarget, setDeletionTarget] = useState<
    { type: "request" } | { type: "child"; id: string; label: string } | null
  >(null);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoring, setRestoring] = useState(false);

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

  const handleDataCorrected = useCallback(
    (correctedChild: AdminRequestChild) => {
      setData((current) => {
        if (!current) return current;
        return {
          ...current,
          children: current.children.map((existingChild) =>
            existingChild.id === correctedChild.id
              ? { ...existingChild, ...correctedChild }
              : existingChild,
          ),
        };
      });
      void load();
    },
    [load],
  );

  useEffect(() => {
    void load();
  }, [load]);

  const availableActions = waitlistEnabled
    ? ACTIONS
    : ACTIONS.filter((action) => action.status !== "waitlisted");

  const handleDecide = async (childId: string, status: DecisionStatus) => {
    if (!data) return;
    const reason = (reasons[childId] ?? "").trim();
    setBusyChildId(childId);
    setError(null);
    setInfo(null);
    try {
      await decideAdminChild(requestId, childId, status, reason || undefined);
      setInfo(
        `Entscheidung gespeichert: ${CHILD_STATUS_LABELS[status as ChildStatus]}`,
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

  const handleRestore = async () => {
    if (!data) return;
    setRestoring(true);
    setError(null);
    setInfo(null);
    try {
      const result = await restoreAdminRequest(requestId);
      setRestoreOpen(false);
      setInfo(buildRestoreInfoMessage(result));
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("admin_enrollment_restore_failed", {
        error: message,
        request_id: requestId,
      });
      setRestoreOpen(false);
      setError(message);
    } finally {
      setRestoring(false);
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
  const phaseHref = tenantPath(`/admin/enrollments/phases/${data.phase_id}`);
  const statusHref = tenantPath(`/enroll/status/${data.status_token}`);
  // The restore action shows whenever at least one child is withdrawn —
  // exactly the backend's restore precondition. Individual child withdraws
  // never stamp withdrawn_at, and RestoreWithdrawn restores the withdrawn
  // subset even while siblings stay active or terminal.
  const withdrawnChildCount = data.children.filter(
    (child) => child.status === "withdrawn",
  ).length;
  const hasRestorableChildren = withdrawnChildCount > 0;

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

            {data.late_invite_email_mismatch === true &&
              data.late_invite_guardian_email && (
                <Alert
                  type="warning"
                  announce="off"
                  message={`Der Nachzügler-Link wurde für ${data.late_invite_guardian_email} erstellt. Im Antrag wurde ${data.guardian_email} angegeben. Der Elternportal-Zugang bleibt mit der eingeladenen Adresse verknüpft.`}
                />
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
                  requestId={data.id}
                  phaseId={data.phase_id}
                  phaseName={data.phase_name}
                  child={child}
                  busy={busyChildId === child.id}
                  reason={reasons[child.id] ?? ""}
                  schemaFields={data.schema_fields}
                  actions={availableActions}
                  onReasonChange={(value) =>
                    setReasons((prev) => ({ ...prev, [child.id]: value }))
                  }
                  onDecide={(status) => void handleDecide(child.id, status)}
                  onOfferingsChanged={() => void load()}
                  onDataCorrected={handleDataCorrected}
                  onDelete={() =>
                    setDeletionTarget({
                      type: "child",
                      id: child.id,
                      label: `${child.first_name} ${child.last_name}`,
                    })
                  }
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
              onDeleteRequest={() => setDeletionTarget({ type: "request" })}
              onRestoreRequest={
                hasRestorableChildren ? () => setRestoreOpen(true) : undefined
              }
            />
          </aside>
        </div>
      </section>
      <ConfirmationModal
        isOpen={restoreOpen}
        onClose={() => setRestoreOpen(false)}
        onConfirm={() => void handleRestore()}
        title="Anmeldung wiederherstellen"
        confirmText="Wiederherstellen"
        isConfirmLoading={restoring}
      >
        <p className="text-sm text-gray-600">
          {withdrawnChildCount === 1
            ? "Das zurückgezogene Kind wird wieder auf „Eingegangen“ gesetzt und die Anmeldung erneut zur Prüfung geöffnet."
            : `Alle ${withdrawnChildCount} zurückgezogenen Kinder werden wieder auf „Eingegangen“ gesetzt und die Anmeldung erneut zur Prüfung geöffnet.`}{" "}
          Bereits entschiedene Kinder bleiben unverändert. Ist ein gewähltes
          Betreuungsangebot inzwischen voll, kommt das betroffene Kind
          stattdessen auf die Warteliste.
        </p>
      </ConfirmationModal>
      <AdminEnrollmentDeletionModal
        isOpen={deletionTarget !== null}
        requestId={data.id}
        childId={
          deletionTarget?.type === "child" ? deletionTarget.id : undefined
        }
        childLabel={
          deletionTarget?.type === "child" ? deletionTarget.label : undefined
        }
        studentHref={(studentId) => tenantPath(`/students/${studentId}`)}
        onClose={() => setDeletionTarget(null)}
        onDeleted={(impact) => {
          setDeletionTarget(null);
          if (impact.deletes_request) {
            router.push(`/admin/enrollments/phases/${data.phase_id}`);
            return;
          }
          setInfo("Kind wurde vollständig aus der Anmeldung gelöscht.");
          void load();
        }}
      />
    </div>
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

      {data.additional_guardians && data.additional_guardians.length > 0 && (
        <div className="mt-4 space-y-3">
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            Weitere erziehungsberechtigte Personen
          </p>
          {data.additional_guardians.map((g: AdminRequestGuardian) => (
            <div
              key={g.id}
              className="rounded-xl border border-gray-100 bg-gray-50/70 p-3"
            >
              <p className="text-sm font-semibold text-gray-900">
                {g.first_name} {g.last_name}
              </p>
              <div className="mt-1 flex flex-col gap-1 text-sm text-gray-600 sm:flex-row sm:gap-4">
                <span className="flex items-center gap-1.5">
                  <Mail className="h-3.5 w-3.5" aria-hidden="true" />
                  {g.email && g.email.trim() !== "" ? g.email : "Nicht gesetzt"}
                </span>
                <span className="flex items-center gap-1.5">
                  <Phone className="h-3.5 w-3.5" aria-hidden="true" />
                  {g.phone && g.phone.trim() !== "" ? g.phone : "Nicht gesetzt"}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
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
  actions,
  busy,
  child,
  onDecide,
  onDataCorrected,
  onOfferingsChanged,
  onReasonChange,
  onDelete,
  phaseId,
  phaseName,
  reason,
  requestId,
  schemaFields,
}: Readonly<{
  actions: ActionDef[];
  busy: boolean;
  child: AdminRequestChild;
  requestId: string;
  phaseId: string;
  phaseName?: string;
  onDecide: (status: DecisionStatus) => void;
  onDataCorrected: (correctedChild: AdminRequestChild) => void;
  onOfferingsChanged: () => void;
  onReasonChange: (value: string) => void;
  onDelete: () => void;
  reason: string;
  schemaFields?: AdminRequestSchemaField[];
}>) {
  const terminal = TERMINAL.has(child.status);
  const canDeleteFromEnrollment =
    child.created_student_id === undefined &&
    (child.status === "rejected" ||
      child.status === "withdrawn" ||
      child.status === "approved");
  const tenantPath = useTenantAwarePath();
  const studentHref = tenantPath(
    child.created_student_id
      ? `/students/${child.created_student_id}`
      : "/students",
  );
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
            {child.target_school_class ? (
              <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
                Klasse {child.target_school_class}
              </span>
            ) : child.target_grade_level ? (
              <span className="rounded-full bg-gray-100 px-2.5 py-1 font-medium text-gray-700">
                {child.target_grade_level}. Klasse
              </span>
            ) : null}
          </div>
        </div>
        <ChildStatusBadge status={child.status} />
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
        {child.status === "approved" && child.created_student_id ? (
          <div className="flex flex-wrap gap-2">
            <Link
              href={studentHref}
              className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
            >
              Kind &amp; Einladung verwalten
              <ExternalLink className="h-4 w-4" aria-hidden="true" />
            </Link>
            <AdminChildDataCorrection
              requestId={requestId}
              child={child}
              onSaved={onDataCorrected}
            />
          </div>
        ) : null}
        <ChildOfferings offerings={child.offerings} phaseName={phaseName} />
        {child.status === "approved" ? (
          <ChildOfferingAdjustment
            requestId={requestId}
            phaseId={phaseId}
            child={child}
            onSaved={onOfferingsChanged}
          />
        ) : null}
        <ChildExtraFields child={child} schemaFields={schemaFields} />
        {terminal ? (
          <div className="space-y-3">
            <div className="rounded-xl border border-gray-200 bg-gray-50/70 px-3 py-2 text-sm text-gray-600">
              Die Entscheidung ist final. Bei bestätigten Kindern können falsche
              Anmeldedaten weiterhin gezielt korrigiert werden.
            </div>
            {canDeleteFromEnrollment ? (
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                onClick={onDelete}
              >
                <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
                Kind aus Anmeldung löschen
              </Button>
            ) : null}
          </div>
        ) : (
          <DecisionPanel
            actions={actions}
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
  onDeleteRequest,
  onRestoreRequest,
}: Readonly<{
  childStats: ReturnType<typeof summarizeChildren>;
  data: AdminRequestSummary;
  statusHref: string;
  submittedAt: string;
  onDeleteRequest: () => void;
  onRestoreRequest?: () => void;
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
        {onRestoreRequest ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="mt-3 w-full"
            onClick={onRestoreRequest}
          >
            <RotateCcw className="mr-2 h-4 w-4" aria-hidden="true" />
            Anmeldung wiederherstellen
          </Button>
        ) : null}
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          className="mt-3 w-full"
          onClick={onDeleteRequest}
        >
          <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
          Gesamte Anmeldung löschen
        </Button>
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
                  {child.target_school_class
                    ? `Klasse ${child.target_school_class}`
                    : child.target_grade_level
                      ? `${child.target_grade_level}. Klasse`
                      : "Keine Klassenstufe"}
                </p>
              </div>
              <ChildStatusBadge status={child.status} />
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
  actions,
  busy,
  child,
  onDecide,
  onReasonChange,
  reason,
}: Readonly<{
  actions: ActionDef[];
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
        {actions.map((action) => {
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

function buildRestoreInfoMessage(result: {
  restored_children: number;
  waitlisted_children: number;
}): string {
  const restored = result.restored_children;
  const waitlisted = result.waitlisted_children;
  if (waitlisted > 0) {
    const restoredPart =
      restored === 1 ? "1 Kind wurde" : `${restored} Kinder wurden`;
    const waitlistedPart =
      waitlisted === 1 ? "1 Kind steht" : `${waitlisted} Kinder stehen`;
    return `Die Anmeldung wurde wiederhergestellt. ${restoredPart} zurückgesetzt; ${waitlistedPart} auf der Warteliste, weil ein Betreuungsangebot inzwischen voll ist.`;
  }
  return restored === 1
    ? "Die Anmeldung wurde wiederhergestellt. 1 Kind steht wieder auf „Eingegangen“."
    : `Die Anmeldung wurde wiederhergestellt. ${restored} Kinder stehen wieder auf „Eingegangen“.`;
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

export function formatDateTime(
  value: string,
  options?: Intl.DateTimeFormatOptions,
): string {
  return new Date(value).toLocaleString("de-DE", {
    ...options,
    timeZone: "Europe/Berlin",
  });
}

export function formatPlainDate(value: string): string {
  return formatCalendarDate(value, "de-DE");
}

const CONSENT_LABELS: Record<string, string> = {
  agb: "AGB der Schule",
  data_processing: "Datenverarbeitung (DSGVO)",
  email_contact: "E-Mail-Kontakt",
  photo: "Fotos bei Schulveranstaltungen",
};

export function RequestExtraSection({
  request,
}: Readonly<{ request: AdminRequestSummary }>) {
  const guardianFields = (request.schema_fields ?? []).filter(
    (f) => !f.applies_to_child,
  );
  const hasCustom = guardianFields.some(
    (f) => formatCustomValue(request.custom_data?.[f.key], f) !== null,
  );
  const hasConsents = Object.keys(request.consent_flags ?? {}).length > 0;
  // Titles from the pinned schema's legal blocks label custom consents
  // (e.g. "Schwimmbad" instead of "custom_pool"); the static map covers
  // the standard keys for legacy requests without a pinned schema.
  const consentTitles = new Map(
    (request.schema_legal_blocks ?? []).map((block) => [
      block.key,
      block.title,
    ]),
  );

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
                  {consentTitles.get(key) ?? CONSENT_LABELS[key] ?? key}:
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

export function ChildOfferings({
  offerings,
  phaseName,
}: Readonly<{ offerings?: AdminRequestChildOffering[]; phaseName?: string }>) {
  if (!offerings || offerings.length === 0) return null;
  return (
    <div className="mt-3 rounded-lg border border-gray-100 bg-gray-50/70 p-3">
      <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
        Betreuungsangebote
      </h4>
      <ul className="mt-1.5 space-y-2 text-sm">
        {offerings.map((o) => {
          const parentChoice = o.days_of_week_mode === "parent_choice";
          const dayDetails = parentChoice
            ? formatAdminOfferingDaySource(o)
            : formatAdminDays(o.available_days ?? []);
          return (
            <li key={`${o.offering_id}-${o.valid_from ?? "current"}`}>
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span className="font-medium text-gray-900">
                  {o.offering_name || `Angebot #${o.offering_id}`}
                </span>
                {dayDetails ? (
                  <span className="text-xs text-gray-600">
                    {parentChoice ? dayDetails : `Tage: ${dayDetails}`}
                  </span>
                ) : parentChoice ? (
                  <span className="text-xs text-[#CC2626] italic">
                    Keine Tage gewählt
                  </span>
                ) : null}
              </div>
              <OfferingAttributePills offering={o} />
            </li>
          );
        })}
      </ul>
      {/* Where the attributes come from. The support case behind #2185 was a
          coordinator who could not place the "mit Mittagessen" the parents app
          showed — naming the phase turns the badges into something staff can
          act on instead of another mystery. */}
      <p className="mt-2 text-xs text-gray-500">
        Angaben stammen aus der Angebots-Konfiguration
        {phaseName
          ? ` der Anmeldephase „${phaseName}“`
          : " der Anmeldephase"}{" "}
        (Anmeldungen → Betreuungsangebote).
      </p>
    </div>
  );
}

// The offering's attributes and validity, phrased exactly as the parents app
// phrases them for the same booking (#2185) — "mit Mittagessen", not
// "Mittagessen" — so a guardian quoting their screen and the staff member
// reading this row use the same words.
function OfferingAttributePills({
  offering,
}: Readonly<{ offering: AdminRequestChildOffering }>) {
  const price = formatOfferingPrice(offering.price_cents);
  const pills: string[] = [];
  if (offering.includes_lunch) pills.push("mit Mittagessen");
  if (offering.includes_holiday_care) pills.push("mit Ferienbetreuung");
  if (price) pills.push(`${price} pro Monat`);
  if (offering.valid_until) {
    pills.push(`bis ${formatPlainDate(offering.valid_until)}`);
  }
  const startsLater = offering.starts_later === true && !!offering.valid_from;
  if (pills.length === 0 && !startsLater) return null;
  return (
    <div className="mt-1 flex flex-wrap items-center gap-1.5">
      {startsLater ? (
        <StatusBadge
          tone="blue"
          label={`gilt ab ${formatPlainDate(offering.valid_from ?? "")}`}
          title="Diese Buchung ist noch nicht wirksam, sie beginnt erst zum genannten Datum."
        />
      ) : null}
      {pills.map((label) => (
        <FeaturePill key={label} label={label} />
      ))}
    </div>
  );
}

export function ChildOfferingAdjustment({
  child,
  onSaved,
  phaseId,
  requestId,
}: Readonly<{
  child: AdminRequestChild;
  requestId: string;
  phaseId: string;
  onSaved: () => void;
}>) {
  const careOfferingsEnabled = useCareOfferingsEnabled();
  const [open, setOpen] = useState(false);
  const [catalog, setCatalog] = useState<CareOffering[]>([]);
  const [rawCatalog, setRawCatalog] = useState<CareOffering[]>([]);
  const [history, setHistory] = useState<AdminOfferingAdjustment[]>([]);
  const [selected, setSelected] = useState<Set<string>>(() =>
    initialManualOfferingIDs(child.offerings),
  );
  const [days, setDays] = useState<Record<string, string[]>>(() =>
    initialManualOfferingDays(child.offerings),
  );
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(false);
  const [catalogLoaded, setCatalogLoaded] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [portalRoot, setPortalRoot] = useState<HTMLElement | null>(null);

  useEffect(() => {
    setPortalRoot(document.body);
  }, []);

  useEffect(() => {
    if (!careOfferingsEnabled) setOpen(false);
  }, [careOfferingsEnabled]);

  const loadHistory = useCallback(async () => {
    try {
      const rows = await listAdminChildOfferingAdjustments(requestId, child.id);
      setHistory(rows);
    } catch (err) {
      logger.warn("offering_adjustment_history_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        child_id: child.id,
      });
    }
  }, [child.id, requestId]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  const resetEditorSelection = (offerings: CareOffering[]) => {
    const available = availableCareOfferings(
      offerings,
      child.target_grade_level,
    );
    const fetchedIDs = new Set(offerings.map((offering) => offering.id));
    const availableIDs = new Set(available.map((offering) => offering.id));
    const nextSelected = new Set(
      [...initialManualOfferingIDs(child.offerings)].filter(
        (id) => !fetchedIDs.has(id) || availableIDs.has(id),
      ),
    );
    for (const offering of available) {
      if (offering.is_active && offering.is_required) {
        nextSelected.add(offering.id);
      }
    }
    setCatalog(available);
    setSelected(nextSelected);
  };

  const openEditor = async () => {
    if (!careOfferingsEnabled) return;
    setOpen(true);
    setError(null);
    setDays(initialManualOfferingDays(child.offerings));
    if (catalogLoaded) {
      resetEditorSelection(rawCatalog);
      return;
    }
    setLoading(true);
    try {
      const offerings = await listCareOfferings(phaseId);
      setRawCatalog(offerings);
      resetEditorSelection(offerings);
      setCatalogLoaded(true);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Betreuungsangebote konnten nicht geladen werden";
      setError(message);
      setCatalog([]);
      setRawCatalog([]);
      setCatalogLoaded(false);
    } finally {
      setLoading(false);
    }
  };

  const preview = useMemo(
    () => materializeClientOfferingPreview(catalog, selected, days, child),
    [catalog, child, days, selected],
  );

  const handleToggle = (offering: CareOffering) => {
    if (offering.is_required) return;
    const next = new Set(selected);
    if (next.has(offering.id)) {
      next.delete(offering.id);
    } else {
      next.add(offering.id);
      if (
        offering.days_of_week_mode === "parent_choice" &&
        (days[offering.id]?.length ?? 0) === 0
      ) {
        setDays((current) => ({
          ...current,
          [offering.id]: [...offering.available_days],
        }));
      }
    }
    setSelected(next);
  };

  const handleDayToggle = (offering: CareOffering, day: string) => {
    setDays((prev) => {
      const current = new Set(prev[offering.id] ?? []);
      if (current.has(day)) current.delete(day);
      else current.add(day);
      const ordered = offering.available_days.filter((d) => current.has(d));
      return { ...prev, [offering.id]: ordered };
    });
  };

  const handleSave = async () => {
    const trimmedReason = reason.trim();
    if (trimmedReason === "") {
      setError("Bitte eine Begründung eintragen.");
      return;
    }
    if (!catalogLoaded) {
      setError("Betreuungsangebote konnten nicht geladen werden.");
      return;
    }
    setSaving(true);
    setError(null);
    try {
      await updateAdminChildOfferings(requestId, child.id, {
        reason: trimmedReason,
        offerings: adjustmentPayloadOfferings(
          catalog,
          selected,
          days,
          child.offerings,
        ),
      });
      setOpen(false);
      setReason("");
      await loadHistory();
      onSaved();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  if (!careOfferingsEnabled && history.length === 0) return null;

  return (
    <div className="rounded-lg border border-gray-100 bg-white p-3">
      {careOfferingsEnabled ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
              Nachbearbeitung
            </h4>
            <p className="mt-1 text-sm text-gray-600">
              Angebote können für dieses bestätigte Kind korrigiert werden.
            </p>
          </div>
          <button
            type="button"
            onClick={() => void openEditor()}
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <Pencil className="h-4 w-4" aria-hidden />
            Bearbeiten
          </button>
        </div>
      ) : null}

      {history.length > 0 ? (
        <div
          className={
            careOfferingsEnabled ? "mt-3 border-t border-gray-100 pt-3" : ""
          }
        >
          <div className="flex items-center gap-2 text-xs font-medium tracking-wide text-gray-500 uppercase">
            <History className="h-3.5 w-3.5" aria-hidden />
            Änderungshistorie
          </div>
          <ul className="mt-2 space-y-2">
            {history.slice(0, 5).map((entry) => (
              <li key={entry.id} className="text-xs leading-5 text-gray-600">
                <span className="font-medium text-gray-900">
                  {formatDateTime(entry.changed_at)}
                </span>{" "}
                von{" "}
                <span className="font-medium text-gray-900">
                  {entry.actor_name_snapshot ??
                    entry.actor_email_snapshot ??
                    `Account ${entry.actor_account_id}`}
                </span>
                : {entry.reason}
                <span className="block text-gray-500">
                  {formatAdjustmentDiff(entry)}
                </span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {careOfferingsEnabled && open && portalRoot
        ? createPortal(
            <div className="fixed inset-0 z-[9999] overflow-y-auto overscroll-contain bg-black/40 p-4">
              <div className="mx-auto my-8 w-full max-w-2xl rounded-xl bg-white shadow-xl">
                <div className="border-b border-gray-100 p-4">
                  <h3 className="text-base font-semibold text-gray-900">
                    Betreuungsangebote bearbeiten
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    Manuelle Auswahl ändern; automatisch verknüpfte Angebote
                    werden beim Speichern neu berechnet.
                  </p>
                </div>
                <div className="space-y-4 p-4">
                  {error ? (
                    <div
                      role="alert"
                      className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]"
                    >
                      {error}
                    </div>
                  ) : null}
                  {loading ? (
                    <p className="text-sm text-gray-500">
                      Angebote werden geladen…
                    </p>
                  ) : (
                    <div className="space-y-2">
                      {catalog.map((offering) => {
                        const checked = selected.has(offering.id);
                        const autoDays =
                          preview.automaticDays[offering.id] ?? [];
                        return (
                          <div
                            key={offering.id}
                            className="rounded-lg border border-gray-200 p-3"
                          >
                            <label className="flex items-start gap-3">
                              <input
                                type="checkbox"
                                checked={checked}
                                disabled={offering.is_required}
                                onChange={() => handleToggle(offering)}
                                className="mt-1 h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
                              />
                              <span className="min-w-0 flex-1">
                                <span className="flex flex-wrap items-center gap-2">
                                  <span className="text-sm font-medium text-gray-900">
                                    {offering.name}
                                  </span>
                                  {!offering.is_active ? (
                                    <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600">
                                      Inaktiv
                                    </span>
                                  ) : null}
                                  {offering.is_required ? (
                                    <span className="rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs text-[#355A9A]">
                                      Pflichtangebot
                                    </span>
                                  ) : null}
                                  {autoDays.length > 0 ? (
                                    <span className="rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs text-[#355A9A]">
                                      automatisch mitgebucht:{" "}
                                      {formatAdminDays(autoDays)}
                                    </span>
                                  ) : null}
                                </span>
                                {offering.description ? (
                                  <span className="mt-1 block text-xs text-gray-500">
                                    {offering.description}
                                  </span>
                                ) : null}
                              </span>
                            </label>

                            {checked &&
                            offering.days_of_week_mode === "parent_choice" ? (
                              <div className="mt-3 flex flex-wrap gap-2 pl-7">
                                {offering.available_days.map((day) => (
                                  <button
                                    key={day}
                                    type="button"
                                    onClick={() =>
                                      handleDayToggle(offering, day)
                                    }
                                    className={`h-8 rounded-lg border px-3 text-sm font-medium ${
                                      (days[offering.id] ?? []).includes(day)
                                        ? "border-[#5080D8] bg-[#5080D8]/10 text-[#355A9A]"
                                        : "border-gray-200 bg-white text-gray-600 hover:bg-gray-50"
                                    }`}
                                  >
                                    {DAY_LABEL_DE[day] ?? day}
                                  </button>
                                ))}
                              </div>
                            ) : null}
                          </div>
                        );
                      })}
                    </div>
                  )}

                  <label className="block">
                    <span className="text-xs font-medium text-gray-700">
                      Begründung
                    </span>
                    <textarea
                      name="offering-adjustment-reason"
                      value={reason}
                      onChange={(event) => setReason(event.target.value)}
                      rows={3}
                      autoComplete="off"
                      placeholder="z. B. Randstunde nach Rücksprache mit der Schule ergänzt"
                      className="mt-1 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                    />
                  </label>
                </div>
                <div className="flex justify-end gap-2 border-t border-gray-100 p-4">
                  <button
                    type="button"
                    onClick={() => setOpen(false)}
                    className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 hover:bg-gray-50"
                  >
                    Abbrechen
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleSave()}
                    disabled={saving || loading || !catalogLoaded}
                    className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-900 bg-gray-900 px-3 text-sm font-medium text-white hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
                  >
                    {saving ? "Speichert…" : "Speichern"}
                  </button>
                </div>
              </div>
            </div>,
            portalRoot,
          )
        : null}
    </div>
  );
}

// The offering rows that describe what the child is booked into TODAY.
// Since #2185 the payload also carries bookings that only take effect later;
// those are informational for the reader and must never seed the correction
// editor, which replaces the currently effective selection.
function effectiveOfferings(
  offerings?: AdminRequestChildOffering[],
): AdminRequestChildOffering[] {
  return (offerings ?? []).filter((offering) => offering.starts_later !== true);
}

function initialManualOfferingIDs(
  offerings?: AdminRequestChildOffering[],
): Set<string> {
  const ids = new Set<string>();
  for (const offering of effectiveOfferings(offerings)) {
    const automaticOnly =
      (offering.automatic_selected_days?.length ?? 0) > 0 &&
      (offering.manual_selected_days?.length ?? 0) === 0;
    if (!automaticOnly) ids.add(offering.offering_id);
  }
  return ids;
}

function initialManualOfferingDays(
  offerings?: AdminRequestChildOffering[],
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const offering of effectiveOfferings(offerings)) {
    const manual =
      (offering.manual_selected_days?.length ?? 0) > 0
        ? offering.manual_selected_days
        : offering.automatic_selected_days?.length
          ? []
          : offering.selected_days;
    if (manual && manual.length > 0) out[offering.offering_id] = [...manual];
  }
  return out;
}

function adjustmentPayloadOfferings(
  catalog: CareOffering[],
  selected: Set<string>,
  days: Record<string, string[]>,
  existingOfferings?: AdminRequestChildOffering[],
): Array<{ offering_id: string; selected_days?: string[] }> {
  const payload = catalog
    .filter((offering) => selected.has(offering.id))
    .map((offering) => ({
      offering_id: offering.id,
      selected_days:
        offering.days_of_week_mode === "parent_choice"
          ? (days[offering.id] ?? [])
          : undefined,
    }));
  const catalogIDs = new Set(catalog.map((offering) => offering.id));
  for (const offering of effectiveOfferings(existingOfferings)) {
    if (
      catalogIDs.has(offering.offering_id) ||
      !selected.has(offering.offering_id)
    ) {
      continue;
    }
    payload.push({
      offering_id: offering.offering_id,
      selected_days:
        offering.days_of_week_mode === "parent_choice"
          ? (days[offering.offering_id] ?? offering.selected_days ?? [])
          : undefined,
    });
  }
  return payload;
}

function materializeClientOfferingPreview(
  catalog: CareOffering[],
  selected: Set<string>,
  days: Record<string, string[]>,
  child: AdminRequestChild,
): { automaticDays: Record<string, string[]> } {
  const byID = new Map(catalog.map((offering) => [offering.id, offering]));
  const selectedDaysByOffering = new Map<string, string[]>();
  for (const offering of catalog) {
    if (!selected.has(offering.id)) continue;
    selectedDaysByOffering.set(
      offering.id,
      offering.days_of_week_mode === "fixed"
        ? offering.available_days
        : (days[offering.id] ?? []),
    );
  }
  const automaticDays: Record<string, string[]> = {};
  let changed = true;
  while (changed) {
    changed = false;
    for (const target of catalog) {
      if (
        target.days_of_week_mode !== "parent_choice" ||
        !autoAddAppliesToChild(target, child)
      ) {
        continue;
      }
      const daySet = new Set<string>();
      for (const triggerID of target.auto_add_trigger_offering_ids ?? []) {
        const trigger = byID.get(triggerID);
        if (!trigger || !selectedDaysByOffering.has(triggerID)) continue;
        for (const day of selectedDaysByOffering.get(triggerID) ?? []) {
          if (target.available_days.includes(day)) daySet.add(day);
        }
      }
      if (target.is_required && target.includes_lunch) {
        for (const offering of catalog) {
          if (offering.id === target.id || offering.counts_as_care === false) {
            continue;
          }
          for (const day of selectedDaysByOffering.get(offering.id) ?? []) {
            if (target.available_days.includes(day)) daySet.add(day);
          }
        }
      }
      const ordered = target.available_days.filter((day) => daySet.has(day));
      if (ordered.length === 0) continue;
      if (!sameStringArray(automaticDays[target.id] ?? [], ordered)) {
        automaticDays[target.id] = ordered;
        changed = true;
      }
      const merged = unionDaysInOrder(
        target.available_days,
        selectedDaysByOffering.get(target.id) ?? [],
        ordered,
      );
      if (
        !sameStringArray(selectedDaysByOffering.get(target.id) ?? [], merged)
      ) {
        selectedDaysByOffering.set(target.id, merged);
        changed = true;
      }
    }
  }
  return { automaticDays };
}

function unionDaysInOrder(
  order: readonly string[],
  ...groups: readonly string[][]
): string[] {
  const selected = new Set(groups.flat());
  return order.filter((day) => selected.has(day));
}

function sameStringArray(left: readonly string[], right: readonly string[]) {
  return (
    left.length === right.length &&
    left.every((value, idx) => value === right[idx])
  );
}

function autoAddAppliesToChild(
  offering: CareOffering,
  child: AdminRequestChild,
): boolean {
  const levels = offering.auto_add_grade_levels ?? [];
  if (
    levels.length > 0 &&
    (!child.target_grade_level || !levels.includes(child.target_grade_level))
  ) {
    return false;
  }
  return (
    (offering.auto_add_trigger_offering_ids?.length ?? 0) > 0 ||
    (offering.is_required && offering.includes_lunch)
  );
}

function formatAdjustmentDiff(entry: AdminOfferingAdjustment): string {
  const before = formatSnapshotNames(entry.before);
  const after = formatSnapshotNames(entry.after);
  return `Vorher: ${before || "keine"}; nachher: ${after || "keine"}`;
}

function formatSnapshotNames(
  rows: readonly { offering_id: string; offering_name?: string }[],
): string {
  return rows
    .map((row) => row.offering_name || `Angebot #${row.offering_id}`)
    .join(", ");
}

function formatAdminOfferingDaySource(o: AdminRequestChildOffering): string {
  const automatic = formatAdminDays(o.automatic_selected_days ?? []);
  const manualDays =
    (o.manual_selected_days?.length ?? 0) > 0 ||
    (o.automatic_selected_days?.length ?? 0) > 0
      ? (o.manual_selected_days ?? [])
      : (o.selected_days ?? []);
  const manual = formatAdminDays(manualDays);
  if (automatic && manual) {
    return `${automatic} automatisch mitgebucht; ${manual} von Eltern gewählt`;
  }
  if (automatic) return `${automatic} automatisch mitgebucht`;
  if (manual) return `Von Eltern gewählt: ${manual}`;
  return "";
}

function formatAdminDays(days: readonly string[]): string {
  return days.map((d) => DAY_LABEL_DE[d] ?? d).join(", ");
}

// Reserved child custom-data key carrying the coupled "mit wem" note for the
// accompanied departure mode (#1694). It rides alongside
// student.allowed_departure_modes instead of being a standalone schema field,
// so it is NOT in schema_fields and must be rendered explicitly here — the
// backend persists it onto the student on approval, so staff must see it before
// deciding. Matches DEPARTURE_COMPANION_KEY in enrollment-form.tsx and
// TargetStudentDepartureCompanionNote in the backend.
const DEPARTURE_COMPANION_KEY = "student.departure_companion_note";

export function ChildExtraFields({
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
  const companionNote = (
    child.custom_data?.[DEPARTURE_COMPANION_KEY] as string | undefined
  )?.trim();
  if (filled.length === 0 && !companionNote) return null;
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
        {companionNote && (
          <div key={DEPARTURE_COMPANION_KEY}>
            <dt className="text-xs font-medium text-gray-600">
              Mit welchem Kind?
            </dt>
            <dd className="mt-0.5 text-gray-900">{companionNote}</dd>
          </div>
        )}
      </dl>
    </div>
  );
}
