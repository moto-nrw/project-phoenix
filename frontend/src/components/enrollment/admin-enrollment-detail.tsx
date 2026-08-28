"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import {
  CalendarClock,
  Check,
  ClipboardList,
  ExternalLink,
  Mail,
  Pencil,
  Phone,
  RotateCcw,
  Trash2,
  X,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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
import {
  availableCareOfferings,
  careOfferingAvailabilityReason,
} from "~/lib/care-offering-availability";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
  formatOfferingOccupancy,
  offeringIsFull,
} from "~/lib/care-offering-booking-stats";
import { formatOfferingPrice } from "~/lib/care-offering-format";
import { FeaturePill } from "~/components/enrollment/feature-pill";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  BLOCKED_OFFERING_ROW_TONE,
  OfferingRowShell,
} from "~/components/enrollment/offering-row-shell";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { Textarea } from "~/components/ui/textarea";
import { Checkbox } from "~/components/ui/checkbox";
import { formatCustomValue } from "~/lib/enrollment-custom-value-format";
import { formatCalendarDate } from "~/lib/localized-date-format";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { AdminChildDataCorrection } from "~/components/enrollment/admin-child-data-correction";
import { AdminEnrollmentDeletionModal } from "~/components/enrollment/admin-enrollment-deletion-modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import { createLogger } from "~/lib/logger";
import {
  useTenant,
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
  const careOfferingsEnabled = useCareOfferingsEnabled();
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
  const [approvalWithoutOfferingChildId, setApprovalWithoutOfferingChildId] =
    useState<string | null>(null);

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
    return (
      <TenantPage
        title="Anmeldung"
        back
        backHref={tenantPath("/admin/enrollments")}
        backLabel="Zurück zur Anmeldungs-Übersicht"
        statsLoading
        loading
      />
    );
  }
  if (!data) {
    return (
      <TenantPage
        title="Anmeldung"
        back
        backHref={tenantPath("/admin/enrollments")}
        backLabel="Zurück zur Anmeldungs-Übersicht"
        error={error ?? "Anmeldung nicht gefunden."}
      />
    );
  }

  const requestDecision = (
    child: AdminRequestChild,
    status: DecisionStatus,
  ) => {
    if (
      status === "approved" &&
      careOfferingsEnabled &&
      data.care_offering_selection_mode === "optional" &&
      child.offerings_unavailable !== true &&
      (child.offerings?.length ?? 0) === 0
    ) {
      setApprovalWithoutOfferingChildId(child.id);
      return;
    }
    void handleDecide(child.id, status);
  };

  const submittedAt = formatDateTime(data.submitted_at, {
    day: "2-digit",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
  const childStats = summarizeChildren(data.children);
  const statusHref = tenantPath(`/enroll/status/${data.status_token}`);
  // The restore action shows whenever at least one child is withdrawn —
  // exactly the backend's restore precondition. Individual child withdraws
  // never stamp withdrawn_at, and RestoreWithdrawn restores the withdrawn
  // subset even while siblings stay active or terminal.
  const withdrawnChildCount = data.children.filter(
    (child) => child.status === "withdrawn",
  ).length;
  const hasRestorableChildren = withdrawnChildCount > 0;

  // Statuszeile des Seitenkopfs aus der bereits geladenen Anmeldung.
  const statusLine = [
    `${data.children.length} ${data.children.length === 1 ? "Kind" : "Kinder"}`,
    `${childStats.open} offen`,
    `${childStats.approved} bestätigt`,
    data.phase_name,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <TenantPage
      title={`${data.guardian_first_name} ${data.guardian_last_name}`}
      back
      backHref={tenantPath("/admin/enrollments")}
      backLabel="Zurück zur Anmeldungs-Übersicht"
      stats={statusLine}
      leading={<ConceptIconTile concept="enrollments" variant="page" />}
    >
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
          <div className="space-y-6 p-4 sm:p-6">
            {error ? <Alert type="error" message={error} /> : null}
            {info ? <Alert type="success" message={info} /> : null}

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

            <SectionCard
              title="Angaben der Kinder"
              description="Zusatzfragen und Stammdaten-Antworten werden pro Kind angezeigt."
              bodyClassName="mt-4 space-y-3"
            >
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
                  onDecide={(status) => requestDecision(child, status)}
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
            </SectionCard>
          </div>

          <aside className="border-t border-gray-100 bg-gray-50/70 p-4 sm:p-6 lg:border-t-0 lg:border-l">
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
        isOpen={approvalWithoutOfferingChildId !== null}
        onClose={() => setApprovalWithoutOfferingChildId(null)}
        onConfirm={() => {
          const childId = approvalWithoutOfferingChildId;
          setApprovalWithoutOfferingChildId(null);
          if (childId !== null) void handleDecide(childId, "approved");
        }}
        title="Anmeldung bestätigen"
        confirmText="Trotzdem bestätigen"
      >
        <Alert
          type="warning"
          message="Für dieses Kind ist kein Betreuungsangebot gebucht. Das Kind wird trotzdem in die OGS aufgenommen."
        />
      </ConfirmationModal>
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
    </TenantPage>
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
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white shadow-sm">
          <MotoConceptIcon concept="parents" size={16} />
        </span>
        <div>
          <h2 className="text-base font-semibold text-gray-900">
            {data.guardian_first_name} {data.guardian_last_name}
          </h2>
          <p className="mt-1 text-sm text-gray-600">
            {data.phase_name || "Nicht zugeordnet"}
          </p>
        </div>
      </div>

      <div className="mt-4">
        <DataGrid>
          <DataField label="E-Mail">{data.guardian_email}</DataField>
          <DataField label="Telefon">
            {data.guardian_phone ?? "Nicht gesetzt"}
          </DataField>
          <DataField label="Eingegangen">{submittedAt}</DataField>
        </DataGrid>
      </div>

      {data.additional_guardians && data.additional_guardians.length > 0 && (
        <div className="mt-4 space-y-3">
          <h3 className="text-base font-semibold text-gray-900">
            Weitere erziehungsberechtigte Personen
          </h3>
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
        {/* Aktionen des Kindes rechts im Kartenkopf statt als eigene
            Buttonzeile im Karteninhalt. */}
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          {child.status === "approved" && child.created_student_id ? (
            <>
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
            </>
          ) : null}
          <ChildStatusBadge status={child.status} />
        </div>
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
        <ChildOfferings
          offerings={child.offerings}
          upcomingOfferings={child.upcoming_offerings}
          unavailable={child.offerings_unavailable}
          phaseName={phaseName}
        />
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
      {/* Der Erklärtext stand vorher als eigener Block über der Karte und
          gehört als description in ihren Kopf. */}
      <SectionCard
        title="Status der Anmeldung"
        description="Alle Kinder werden einzeln geprüft. Die Statusseite zeigt Eltern den aktuellen Stand der Anmeldung."
      >
        <div className="grid grid-cols-3 gap-2">
          <SidebarMetric label="Kinder" value={data.children.length} />
          <SidebarMetric label="Offen" value={childStats.open} />
          <SidebarMetric label="Bestätigt" value={childStats.approved} />
        </div>
        <div className="mt-4">
          <DataGrid>
            <DataField label="Phase">
              {data.phase_name || "Nicht zugeordnet"}
            </DataField>
            <DataField label="Eingegangen">{submittedAt}</DataField>
          </DataGrid>
        </div>
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
      </SectionCard>

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
          <MotoConceptIcon concept="enrollments" size={18} />
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

      <div className="mt-4">
        <Textarea
          id={`decision-reason-${child.id}`}
          label="Begründung"
          value={reason}
          onChange={(event) => onReasonChange(event.target.value)}
          rows={2}
          placeholder="z. B. Geschwisterkind bevorzugt, voll ausgebucht"
        />
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        {actions.map((action) => {
          const isCurrent = child.status === action.status;
          return (
            <Button
              key={action.status}
              type="button"
              size="md"
              variant={getDecisionButtonVariant(action.tone)}
              disabled={busy || isCurrent}
              onClick={() => onDecide(action.status)}
              className="inline-flex items-center justify-center gap-2"
            >
              {getDecisionIcon(action.status)}
              {busy ? "Speichert…" : action.label}
            </Button>
          );
        })}
      </div>
    </div>
  );
}

function getDecisionButtonVariant(
  tone: ActionDef["tone"],
): "success" | "outline_danger" | "primary" | "outline" {
  if (tone === "success") return "success";
  if (tone === "danger") return "outline_danger";
  if (tone === "primary") return "primary";
  return "outline";
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
    <section className="moto-content-surface space-y-3 rounded-2xl border p-4 shadow-sm sm:p-6">
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
                    backgroundColor:
                      val === true
                        ? MOTO_COLOR_PALETTE.green.base
                        : MOTO_COLOR_PALETTE.neutral.base,
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
        <InfoSection
          title="Zusatzfragen (Eltern)"
          icon={<MotoConceptIcon concept="parents" size={16} />}
        >
          <DataGrid>
            {guardianFields.map((f) => {
              const formatted = formatCustomValue(
                request.custom_data?.[f.key],
                f,
              );
              if (formatted === null) return null;
              return (
                <DataField key={f.key} label={f.label}>
                  {formatted}
                </DataField>
              );
            })}
          </DataGrid>
        </InfoSection>
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
  upcomingOfferings,
  unavailable,
  phaseName,
}: Readonly<{
  offerings?: AdminRequestChildOffering[];
  /** Rendered alongside the current selection, flagged "gilt ab …". */
  upcomingOfferings?: AdminRequestChildOffering[];
  /**
   * The lookup failed. Rendered as a warning rather than nothing: staff
   * decide on this screen, and an absent block reads as "the family booked
   * nothing" — which is how the wrong decision gets made (#2185).
   */
  unavailable?: boolean;
  phaseName?: string;
}>) {
  const rows = [...(offerings ?? []), ...(upcomingOfferings ?? [])];
  if (unavailable) {
    return (
      <div className="mt-3 rounded-lg border border-[#F78C10]/30 bg-[#FFF4E6] p-3">
        <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
          Betreuungsangebote
        </h4>
        <p className="mt-1.5 text-sm text-[#8A5600]">
          Die gebuchten Angebote konnten nicht geladen werden. Bitte die Seite
          neu laden, bevor Sie über dieses Kind entscheiden.
        </p>
      </div>
    );
  }
  if (rows.length === 0) return null;
  return (
    <div className="mt-3 rounded-lg border border-gray-100 bg-gray-50/70 p-3">
      <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
        Betreuungsangebote
      </h4>
      <ul className="mt-1.5 space-y-2 text-sm">
        {rows.map((o) => {
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
                  <span className="text-moto-red-strong text-xs italic">
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
    // Already the inclusive last covered day on the wire.
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

/**
 * One-line occupancy hint under an offering in the admin pickers. Renders
 * nothing when there is no stats entry, so a failed stats load degrades to
 * the previous UI instead of an empty placeholder (#2186).
 */
function OfferingOccupancyLine({
  stats,
}: Readonly<{ stats: CareOfferingBookingStats | undefined }>) {
  const label = formatOfferingOccupancy(stats);
  if (!label) return null;
  return (
    <span
      className={`mt-1 block text-xs ${
        offeringIsFull(stats) ? "text-moto-amber-strong" : "text-gray-500"
      }`}
    >
      {label}
    </span>
  );
}

/**
 * An offering the child's grade level rules out. Parents never see these;
 * admins must, or a configured restriction reads as a missing feature
 * (#2186). A booking the child already holds stays removable — Bestandsschutz
 * means the rule does not revoke it, not that it can never be corrected — but
 * a blocked offering can never be newly added here. The documented workaround
 * is to relax the rule in the Angebots-Katalog first.
 *
 * `automaticDays` are days derived from a trigger offering. The backend
 * re-derives them on every save regardless of the availability rule, so the
 * row must show them as HELD even though there is no manual tick behind them
 * — an empty checkbox next to a booking the backend keeps is a lie (#2186
 * review). They are also not removable here: they disappear only with their
 * trigger.
 */
function BlockedOfferingRow({
  offering,
  gradeLevel,
  gradeLevelMax,
  booked,
  automaticDays,
  stats,
  onRemove,
}: Readonly<{
  offering: CareOffering;
  gradeLevel: number | null | undefined;
  gradeLevelMax: number | null;
  booked: boolean;
  automaticDays: readonly string[];
  stats: CareOfferingBookingStats | undefined;
  onRemove: () => void;
}>) {
  const reason = careOfferingAvailabilityReason(
    offering,
    gradeLevel,
    gradeLevelMax ?? undefined,
  );
  const inputId = `blocked-offering-${offering.id}`;
  const heldAutomatically = automaticDays.length > 0;
  return (
    <OfferingRowShell tone={BLOCKED_OFFERING_ROW_TONE}>
      {/* A real <label> is load-bearing, not decoration: the kit Checkbox
          renders an sr-only input behind a pointer-events-none visual, so
          without the label wrapping it a mouse click on the box does nothing
          (#2186 review). */}
      <label
        htmlFor={inputId}
        className={`flex items-start gap-3 ${booked ? "cursor-pointer" : "cursor-not-allowed"}`}
      >
        <Checkbox
          id={inputId}
          className="mt-0.5"
          checked={booked || heldAutomatically}
          disabled={!booked}
          onChange={onRemove}
          aria-label={`${offering.name} bleibt gebucht`}
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-sm font-medium text-gray-500">
              {offering.name}
            </span>
            {booked ? (
              <StatusBadge
                tone="orange"
                label="bereits gebucht"
                title="Diese Buchung bleibt bestehen. Sie kann hier entfernt, aber nicht erneut hinzugefügt werden."
              />
            ) : null}
            {heldAutomatically ? (
              <StatusBadge
                tone="blue"
                label={`automatisch mitgebucht: ${formatAdminDays(automaticDays)}`}
                title="Diese Tage werden aus dem auslösenden Angebot abgeleitet und bleiben bestehen, solange dieses gebucht ist. Sie können hier nicht einzeln entfernt werden."
              />
            ) : null}
          </div>
          {reason ? (
            <span className="mt-1 block text-xs text-gray-500">{reason}</span>
          ) : null}
          <OfferingOccupancyLine stats={stats} />
        </div>
      </label>
    </OfferingRowShell>
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
  // The tenant's configured ceiling, so a blocked-offering reason names only
  // grades this school actually has (#2186 review).
  const { tenant } = useTenant();
  const gradeLevelMax = isSupportedGradeLevelMax(tenant?.gradeLevelMax)
    ? tenant.gradeLevelMax
    : null;
  const [open, setOpen] = useState(false);
  const [catalog, setCatalog] = useState<CareOffering[]>([]);
  // Offerings the child's grade level rules out. Kept separate from `catalog`
  // so payload building and the auto-add preview keep operating on exactly
  // the selectable set, while the UI can still SHOW the blocked ones with a
  // reason instead of leaving a silent gap in the list (#2186).
  const [blockedCatalog, setBlockedCatalog] = useState<CareOffering[]>([]);
  const [rawCatalog, setRawCatalog] = useState<CareOffering[]>([]);
  const [bookingStats, setBookingStats] = useState<
    Record<string, CareOfferingBookingStats>
  >({});
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
  const [withdrawalConfirmationOpen, setWithdrawalConfirmationOpen] =
    useState(false);
  const [pendingWithdrawalInput, setPendingWithdrawalInput] = useState<{
    reason: string;
    offerings: Array<{ offering_id: string; selected_days?: string[] }>;
  } | null>(null);
  const [withdrawalCreated, setWithdrawalCreated] = useState(false);
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
    const availableIDs = new Set(available.map((offering) => offering.id));
    // Bestandsschutz: a booking the child already holds survives a rule that
    // was tightened after the fact. Dropping it here would silently delete
    // the booking on the next save — the exact failure mode #2186 reports,
    // in the flow meant to fix it. adjustmentPayloadOfferings carries such
    // ids through via the child's existing offerings.
    const nextSelected = new Set(initialManualOfferingIDs(child.offerings));
    for (const offering of available) {
      if (offering.is_active && offering.is_required) {
        nextSelected.add(offering.id);
      }
    }
    setCatalog(available);
    setBlockedCatalog(
      offerings.filter((offering) => !availableIDs.has(offering.id)),
    );
    setSelected(nextSelected);
  };

  const openEditor = async () => {
    if (!careOfferingsEnabled) return;
    setOpen(true);
    setError(null);
    setDays(initialManualOfferingDays(child.offerings));
    // Occupancy is advisory: without it a full offering only announces itself
    // as an error after the whole correction is submitted (#2186). Loaded
    // beside the catalog rather than awaited with it — a slow or failing
    // stats call must never keep the editor in its loading state.
    void fetchCareOfferingBookingStats(phaseId)
      .then(setBookingStats)
      .catch((statsErr: unknown) => {
        setBookingStats({});
        logger.warn("offering_adjustment_booking_stats_failed", {
          error:
            statsErr instanceof Error ? statsErr.message : String(statsErr),
        });
      });
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
      setBlockedCatalog([]);
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

  // Blocked offerings are excluded from `catalog`, so the client-side preview
  // above never covers them. Their derived days therefore have to come from
  // what is on file — otherwise a booking the backend keeps re-deriving reads
  // as unbooked in the editor (#2186 review).
  const automaticDaysOnFile = useMemo(
    () => automaticOfferingDays(child.offerings),
    [child.offerings],
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

  const removeAllCareDays = () => {
    const careOfferingIDs = new Set(
      rawCatalog
        .filter((offering) => offering.counts_as_care)
        .map((offering) => offering.id),
    );
    setSelected(
      new Set([...selected].filter((id) => !careOfferingIDs.has(id))),
    );
  };

  const hasSelectedCareDays = rawCatalog.some(
    (offering) => offering.counts_as_care && selected.has(offering.id),
  );

  const finishSave = async () => {
    setWithdrawalConfirmationOpen(false);
    setPendingWithdrawalInput(null);
    setOpen(false);
    setReason("");
    await loadHistory();
    onSaved();
  };

  const saveAdjustment = async (
    input: {
      reason: string;
      offerings: Array<{ offering_id: string; selected_days?: string[] }>;
    },
    completeWithdrawalConfirmed: boolean,
  ) => {
    await updateAdminChildOfferings(requestId, child.id, {
      ...input,
      completeWithdrawalConfirmed,
    });
    setWithdrawalCreated(completeWithdrawalConfirmed);
    if (completeWithdrawalConfirmed) {
      window.dispatchEvent(new Event("change-requests-refresh"));
    }
    await finishSave();
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
    const input = {
      reason: trimmedReason,
      offerings: adjustmentPayloadOfferings(
        catalog,
        selected,
        days,
        child.offerings,
      ),
    };
    try {
      await saveAdjustment(input, false);
    } catch (err) {
      if (
        (err as { code?: string } | undefined)?.code ===
        "enrollment.complete_withdrawal_confirmation_required"
      ) {
        setPendingWithdrawalInput(input);
        setWithdrawalConfirmationOpen(true);
      } else {
        setError(
          err instanceof Error ? err.message : "Speichern fehlgeschlagen",
        );
      }
    } finally {
      setSaving(false);
    }
  };

  const confirmCompleteWithdrawal = async () => {
    if (!pendingWithdrawalInput) return;
    setSaving(true);
    setError(null);
    try {
      await saveAdjustment(pendingWithdrawalInput, true);
    } catch (err) {
      setWithdrawalConfirmationOpen(false);
      setError(err instanceof Error ? err.message : "Speichern fehlgeschlagen");
    } finally {
      setSaving(false);
    }
  };

  if (!careOfferingsEnabled && history.length === 0) return null;

  return (
    <div className="rounded-lg border border-gray-100 bg-white p-3">
      {withdrawalCreated ? (
        <Alert
          type="warning"
          message="Abmeldung noch abschließen. Die Betreuungstage sind entfernt. Eine berechtigte Person muss die Betreuung jetzt beenden."
        />
      ) : null}
      {careOfferingsEnabled ? (
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h4 className="text-xs font-medium tracking-wide text-gray-500 uppercase">
              Nachbearbeitung
            </h4>
            <p className="mt-1 text-sm text-gray-600">
              {child.offerings_unavailable
                ? "Die gebuchten Angebote konnten nicht geladen werden. Bitte die Seite neu laden, eine Korrektur würde sonst auf einem unbekannten Stand aufsetzen."
                : "Angebote können für dieses bestätigte Kind korrigiert werden."}
            </p>
          </div>
          {/* Correcting on top of an unknown selection would replace the
              family's real bookings with whatever the empty editor holds
              (#2185), so the entry point disappears until the data is back. */}
          {child.offerings_unavailable ? null : (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void openEditor()}
              className="inline-flex items-center justify-center gap-2"
            >
              <Pencil className="h-4 w-4" aria-hidden />
              Bearbeiten
            </Button>
          )}
        </div>
      ) : null}

      {history.length > 0 ? (
        <div
          className={
            careOfferingsEnabled ? "mt-3 border-t border-gray-100 pt-3" : ""
          }
        >
          <div className="flex items-center gap-2 text-xs font-medium tracking-wide text-gray-500 uppercase">
            <MotoConceptIcon concept="changeHistory" size={14} />
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
                  {error ? <Alert type="error" message={error} /> : null}
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
                                className="text-moto-blue focus:ring-moto-blue mt-1 h-4 w-4 rounded border-gray-300"
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
                                    <span className="bg-moto-blue/10 text-moto-blue-strong rounded-full px-2 py-0.5 text-xs">
                                      Pflichtangebot
                                    </span>
                                  ) : null}
                                  {autoDays.length > 0 ? (
                                    <span className="bg-moto-blue/10 text-moto-blue-strong rounded-full px-2 py-0.5 text-xs">
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
                                <OfferingOccupancyLine
                                  stats={bookingStats[offering.id]}
                                />
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
                                        ? "border-moto-blue bg-moto-blue/10 text-moto-blue-strong"
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
                      {blockedCatalog.length > 0 ? (
                        <div className="space-y-2 pt-1">
                          <p className="text-xs font-medium tracking-wide text-gray-500 uppercase">
                            Für dieses Kind nicht wählbar
                          </p>
                          {blockedCatalog.map((offering) => (
                            <BlockedOfferingRow
                              key={offering.id}
                              offering={offering}
                              gradeLevel={child.target_grade_level}
                              gradeLevelMax={gradeLevelMax}
                              booked={selected.has(offering.id)}
                              automaticDays={
                                automaticDaysOnFile[offering.id] ?? []
                              }
                              stats={bookingStats[offering.id]}
                              onRemove={() =>
                                setSelected((prev) => {
                                  const next = new Set(prev);
                                  next.delete(offering.id);
                                  return next;
                                })
                              }
                            />
                          ))}
                        </div>
                      ) : null}
                      {hasSelectedCareDays ? (
                        <Button
                          type="button"
                          variant="secondary"
                          onClick={removeAllCareDays}
                        >
                          Alle Betreuungstage entfernen
                        </Button>
                      ) : null}
                    </div>
                  )}

                  <Textarea
                    name="offering-adjustment-reason"
                    label="Begründung"
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    rows={3}
                    autoComplete="off"
                    placeholder="z. B. Randstunde nach Rücksprache mit der Schule ergänzt"
                  />
                </div>
                <div className="flex justify-end gap-2 border-t border-gray-100 p-4">
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => setOpen(false)}
                  >
                    Abbrechen
                  </Button>
                  <Button
                    type="button"
                    variant="primary"
                    size="md"
                    onClick={() => void handleSave()}
                    disabled={saving || loading || !catalogLoaded}
                  >
                    {saving ? "Speichert…" : "Speichern"}
                  </Button>
                </div>
              </div>
            </div>,
            portalRoot,
          )
        : null}
      <ConfirmationModal
        isOpen={withdrawalConfirmationOpen}
        onClose={() => setWithdrawalConfirmationOpen(false)}
        onConfirm={() => void confirmCompleteWithdrawal()}
        title="Alle Betreuungstage entfernen?"
        confirmText="Änderung speichern"
        cancelText="Zurück"
        isConfirmLoading={saving}
        isDismissDisabled={saving}
      >
        <p>
          Danach ist für {child.first_name} kein Betreuungstag mehr gebucht. Die
          Abmeldung muss anschließend abgeschlossen werden.
        </p>
      </ConfirmationModal>
    </div>
  );
}

function initialManualOfferingIDs(
  offerings?: AdminRequestChildOffering[],
): Set<string> {
  const ids = new Set<string>();
  for (const offering of offerings ?? []) {
    const automaticOnly =
      (offering.automatic_selected_days?.length ?? 0) > 0 &&
      (offering.manual_selected_days?.length ?? 0) === 0;
    if (!automaticOnly) ids.add(offering.offering_id);
  }
  return ids;
}

/**
 * The days each booking on file holds automatically, i.e. derived from a
 * trigger offering rather than ticked by anyone. Keyed by offering id, absent
 * when a booking has none.
 */
function automaticOfferingDays(
  offerings?: AdminRequestChildOffering[],
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const offering of offerings ?? []) {
    const automatic = offering.automatic_selected_days ?? [];
    if (automatic.length > 0) out[offering.offering_id] = [...automatic];
  }
  return out;
}

function initialManualOfferingDays(
  offerings?: AdminRequestChildOffering[],
): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const offering of offerings ?? []) {
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
  for (const offering of existingOfferings ?? []) {
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
    <div className="mt-3">
      <InfoSection
        title="Zusätzliche Angaben zum Kind"
        icon={<MotoConceptIcon concept="children" size={16} />}
      >
        <DataGrid>
          {filled.map(({ field, value }) => (
            <DataField key={field.key} label={field.label}>
              {value}
            </DataField>
          ))}
          {companionNote && (
            <DataField key={DEPARTURE_COMPANION_KEY} label="Mit welchem Kind?">
              {companionNote}
            </DataField>
          )}
        </DataGrid>
      </InfoSection>
    </div>
  );
}
