"use client";

import {
  APPROVAL_WITHOUT_OFFERING_COPY,
  approvalNeedsOfferingConfirm,
} from "~/components/enrollment/approval-without-offering";
import { isTerminalChildStatus } from "~/lib/enrollment-status";
import type { ChildStatus } from "~/lib/enrollment-status";
import { InsetPanel } from "~/components/ui/inset-panel";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import {
  CalendarClock,
  Check,
  ClipboardList,
  ExternalLink,
  Mail,
  Phone,
  RotateCcw,
  Trash2,
  X,
} from "lucide-react";
import { StatCard } from "~/components/ui/stat-card";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  type AdminRequestChild,
  type AdminRequestDetail,
  type AdminRequestGuardian,
  type AdminRequestSchemaField,
  type AdminRequestSummary,
  type DecisionStatus,
  type ExistingChildChoice,
  decideAdminChild,
  announceAdminRequestOpened,
  getAdminRequest,
  restoreAdminRequest,
} from "~/lib/enrollment-admin-api";
import { StatusBadge } from "~/components/ui/status-badge";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { Textarea } from "~/components/ui/textarea";
import { AdminEnrollmentSignature } from "~/components/enrollment/admin-enrollment-signature";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { berlinTodayISO } from "~/lib/date-helpers";
import { enrolledClassLabel } from "~/lib/school-class-label";
import { AdminChildDataCorrection } from "~/components/enrollment/admin-child-data-correction";
import { AdminEnrollmentDeletionModal } from "~/components/enrollment/admin-enrollment-deletion-modal";
import { ChildOfferingAdjustment } from "~/components/enrollment/child-offering-adjustment";
import {
  ChildExtraFields,
  ChildOfferings,
  RequestExtraSection,
  formatDateTime,
  formatPlainDate,
} from "~/components/enrollment/enrollment-request-display";
import {
  RequestMailStatus,
  useRequestMails,
} from "~/components/enrollment/request-mail-status";
import {
  existingChildChoiceDescription,
  existingChildChoiceOptions,
  existingChildHintMessage,
  needsExistingChildChoice,
} from "~/components/enrollment/existing-child-choice";
import { ActionHint } from "~/components/ui/action-hint";
import { DECISION_HINTS } from "~/components/enrollment/decision-hints";
import { Button, buttonClassName } from "~/components/ui/button";
import NavigationLink from "~/components/ui/navigation-link";
import { Alert } from "~/components/ui/alert";
import { ChoiceModal } from "~/components/ui/choice-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantRouter } from "~/lib/tenant-router";
import { createLogger } from "~/lib/logger";
import {
  useCareOfferingsEnabled,
  useWaitlistEnabled,
} from "~/lib/tenant-context";

import {
  CHILD_STATUS_LABELS,
  ChildQuotaHeldBadge,
  ChildStatusBadge,
  isHeldForChildQuota,
} from "~/components/enrollment/child-status-badge";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { parentsConcept } from "~/lib/concepts/parents";

const logger = createLogger({ component: "AdminEnrollmentDetail" });

interface ActionDef {
  status: DecisionStatus;
  label: string;
  tone: "primary" | "outline" | "danger" | "success";
}

const ACTIONS: ActionDef[] = [
  {
    status: "approved",
    label: "Bestätigen",
    tone: "success",
  },
  {
    status: "waitlisted",
    label: "Warteliste",
    tone: "outline",
  },
  {
    status: "rejected",
    label: "Ablehnen",
    tone: "danger",
  },
  {
    status: "under_review",
    label: "Zur Prüfung",
    tone: "primary",
  },
];

interface Props {
  readonly requestId: string;
}

export function AdminEnrollmentDetail({ requestId }: Props) {
  const {
    mails: requestMails,
    loadError: requestMailsError,
    refresh: refreshRequestMails,
  } = useRequestMails(requestId);
  const careOfferingsEnabled = useCareOfferingsEnabled();
  const waitlistEnabled = useWaitlistEnabled();
  const tenantPath = useTenantAwarePath();
  const router = useTenantRouter();
  const [data, setData] = useState<AdminRequestDetail | null>(null);
  const today = useBerlinToday();
  const [weeklyPlanCheckedOn, setWeeklyPlanCheckedOn] = useState<string | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const toast = useToast();
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  // Eine Entscheidung scheitert an genau einem Kind: der Kasten steht dort.
  const decisionErrors = useApiFormError();
  const [decisionErrorChildId, setDecisionErrorChildId] = useState<
    string | null
  >(null);
  const restoreErrors = useApiFormError();
  // „Wiederholen“ ruft die aktuelle Fassung auf, nicht die vom Fehler.
  const reloadRef = useRef<() => Promise<void>>(async () => undefined);
  const latestDecideRef = useRef<
    (
      childId: string,
      status: DecisionStatus,
      existingChildChoice?: ExistingChildChoice,
    ) => Promise<void>
  >(async () => undefined);
  const latestRestoreRef = useRef<() => Promise<void>>(async () => undefined);
  const [busyChildId, setBusyChildId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [deletionTarget, setDeletionTarget] = useState<
    { type: "request" } | { type: "child"; id: string; label: string } | null
  >(null);
  const [restoreOpen, setRestoreOpen] = useState(false);
  const [restoring, setRestoring] = useState(false);
  const [approvalWithoutOfferingChildId, setApprovalWithoutOfferingChildId] =
    useState<{ id: string; existingChildChoice?: ExistingChildChoice } | null>(
      null,
    );
  // Ein Kind, das vielleicht schon angemeldet ist: das Büro wählt vor dem
  // Bestätigen das bestehende Kind oder ein neues (#6053).
  const [existingChildChoiceChild, setExistingChildChoiceChild] =
    useState<AdminRequestChild | null>(null);

  // `quiet` refreshes without the page's loading state, so a card that just
  // saved keeps showing its result (#5102).
  const load = useCallback(
    async (options?: { quiet?: boolean }) => {
      if (!options?.quiet) setLoading(true);
      clearLoadError();
      try {
        const checkedOn = berlinTodayISO();
        const fresh = await getAdminRequest(requestId);
        setData(fresh);
        setWeeklyPlanCheckedOn(checkedOn);
        // Das Öffnen hat die Anmeldung für diese Person gelesen (#3778).
        announceAdminRequestOpened();
      } catch (err) {
        logger.error("admin_enrollment_detail_load_failed", {
          error: err instanceof Error ? err.message : "unknown",
        });
        await showLoadError(err, {
          object: "die Anmeldung",
          retry: () => void reloadRef.current(),
        });
      } finally {
        setLoading(false);
      }
    },
    [requestId, clearLoadError, showLoadError],
  );
  useLayoutEffect(() => {
    reloadRef.current = load;
  });

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
      void load({ quiet: true });
    },
    [load],
  );

  useEffect(() => {
    void load();
  }, [load]);

  const hasPendingWeeklyPlan = data?.children.some(
    (child) => child.status === "approved" && child.weekly_plan_on,
  );
  useEffect(() => {
    if (hasPendingWeeklyPlan && weeklyPlanCheckedOn !== today) {
      void load({ quiet: true });
    }
  }, [today, weeklyPlanCheckedOn, hasPendingWeeklyPlan, load]);

  useEffect(() => {
    if (!hasPendingWeeklyPlan) return;
    const refreshOnVisible = () => {
      if (document.visibilityState === "visible") void load({ quiet: true });
    };
    document.addEventListener("visibilitychange", refreshOnVisible);
    return () =>
      document.removeEventListener("visibilitychange", refreshOnVisible);
  }, [hasPendingWeeklyPlan, load]);

  const availableActions = waitlistEnabled
    ? ACTIONS
    : ACTIONS.filter((action) => action.status !== "waitlisted");

  const handleDecide = async (
    childId: string,
    status: DecisionStatus,
    existingChildChoice?: ExistingChildChoice,
  ) => {
    if (!data) return;
    const reason = (reasons[childId] ?? "").trim();
    setBusyChildId(childId);
    decisionErrors.clear();
    setDecisionErrorChildId(childId);
    let saved = false;
    try {
      // The office's choice travels only when there was one to make.
      await (existingChildChoice
        ? decideAdminChild(
            requestId,
            childId,
            status,
            reason || undefined,
            existingChildChoice,
          )
        : decideAdminChild(requestId, childId, status, reason || undefined));
      saved = true;
    } catch (err) {
      logger.error("admin_enrollment_decide_failed", {
        error: err instanceof Error ? err.message : "unknown",
        request_id: requestId,
        child_id: childId,
        status,
      });
      await decisionErrors.show(err, {
        object: "die Entscheidung",
        retry: () =>
          void latestDecideRef.current(childId, status, existingChildChoice),
      });
    } finally {
      setBusyChildId(null);
    }
    if (!saved) return;
    toast.success(
      `Die Entscheidung wurde gespeichert: ${CHILD_STATUS_LABELS[status as ChildStatus]}.`,
    );
    await Promise.all([load(), refreshRequestMails()]);
  };
  useLayoutEffect(() => {
    latestDecideRef.current = handleDecide;
  });

  const handleRestore = async () => {
    if (!data) return;
    setRestoring(true);
    restoreErrors.clear();
    let result: Awaited<ReturnType<typeof restoreAdminRequest>> | null = null;
    try {
      result = await restoreAdminRequest(requestId);
    } catch (err) {
      logger.error("admin_enrollment_restore_failed", {
        error: err instanceof Error ? err.message : "unknown",
        request_id: requestId,
      });
      // Der Dialog bleibt offen: dort steht, warum es nicht ging.
      await restoreErrors.show(err, {
        object: "die Anmeldung",
        retry: () => void latestRestoreRef.current(),
      });
    } finally {
      setRestoring(false);
    }
    if (!result) return;
    setRestoreOpen(false);
    toast.success(buildRestoreInfoMessage(result));
    await load();
  };
  useLayoutEffect(() => {
    latestRestoreRef.current = handleRestore;
  });

  if (loading) {
    return (
      <TenantPage
        title="Anmeldung"
        back
        backHref="/admin/enrollments"
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
        backHref="/admin/enrollments"
        backLabel="Zurück zur Anmeldungs-Übersicht"
        error={loadError.error ?? "Die Anmeldung wurde nicht gefunden."}
      />
    );
  }

  const requestDecision = (
    child: AdminRequestChild,
    status: DecisionStatus,
    existingChildChoice?: ExistingChildChoice,
  ) => {
    if (
      status === "approved" &&
      existingChildChoice === undefined &&
      needsExistingChildChoice(child)
    ) {
      setExistingChildChoiceChild(child);
      return;
    }
    if (
      approvalNeedsOfferingConfirm({
        status,
        careOfferingsEnabled,
        selectionMode: data.care_offering_selection_mode,
        bookedOfferings: child.offerings_unavailable
          ? undefined
          : (child.offerings?.length ?? 0),
      })
    ) {
      setApprovalWithoutOfferingChildId({ id: child.id, existingChildChoice });
      return;
    }
    void handleDecide(child.id, status, existingChildChoice);
  };

  const submittedAt = formatDateTime(data.submitted_at, {
    day: "2-digit",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
  const childStats = summarizeChildren(data.children);
  const statusHref = tenantPath(
    `/anmeldung/status/${encodeURIComponent(data.status_token)}`,
  );
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
      backHref={`/admin/enrollments/phases/${encodeURIComponent(data.phase_id)}`}
      backLabel="Zurück zur Anmeldephase"
      stats={statusLine}
      leading={<ConceptIconTile concept={enrollmentsConcept} size="page" />}
      overlays={
        <>
          <ConfirmationModal
            isOpen={approvalWithoutOfferingChildId !== null}
            onClose={() => setApprovalWithoutOfferingChildId(null)}
            onConfirm={() => {
              const pending = approvalWithoutOfferingChildId;
              setApprovalWithoutOfferingChildId(null);
              if (pending !== null) {
                void handleDecide(
                  pending.id,
                  "approved",
                  pending.existingChildChoice,
                );
              }
            }}
            title={APPROVAL_WITHOUT_OFFERING_COPY.title}
            confirmText={APPROVAL_WITHOUT_OFFERING_COPY.confirmText}
          >
            <Alert
              type="warning"
              message={APPROVAL_WITHOUT_OFFERING_COPY.message}
            />
          </ConfirmationModal>
          {existingChildChoiceChild ? (
            <ChoiceModal
              isOpen
              title="Ist das Kind schon angemeldet?"
              description={existingChildChoiceDescription(
                existingChildChoiceChild,
              )}
              options={existingChildChoiceOptions(existingChildChoiceChild)}
              onClose={() => setExistingChildChoiceChild(null)}
              onSelect={(value) => {
                const child = existingChildChoiceChild;
                setExistingChildChoiceChild(null);
                requestDecision(
                  child,
                  "approved",
                  value === "new"
                    ? { createNewStudent: true }
                    : { existingStudentId: value },
                );
              }}
            />
          ) : null}
          <ConfirmationModal
            isOpen={restoreOpen}
            onClose={() => {
              setRestoreOpen(false);
              restoreErrors.clear();
            }}
            onConfirm={() => void handleRestore()}
            title="Anmeldung wiederherstellen"
            confirmText="Wiederherstellen"
            isConfirmLoading={restoring}
            loadingText="Wird wiederhergestellt …"
          >
            <FormErrorAlert message={restoreErrors.error} className="mb-3" />
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
            child={
              deletionTarget?.type === "child" ? deletionTarget : undefined
            }
            studentHref={(studentId) =>
              tenantPath(`/students/${encodeURIComponent(studentId)}`)
            }
            onClose={() => setDeletionTarget(null)}
            onDeleted={(impact) => {
              setDeletionTarget(null);
              if (impact.deletes_request) {
                router.push(
                  `/admin/enrollments/phases/${encodeURIComponent(data.phase_id)}`,
                );
                return;
              }
              toast.success("Das Kind wurde aus der Anmeldung gelöscht.");
              void load();
            }}
          />
        </>
      }
    >
      <SectionCard padding="none">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_380px] xl:grid-cols-[minmax(0,1fr)_430px]">
          <div className="space-y-6 p-5 max-sm:p-4">
            {/* Ein gescheitertes Neuladen nach einer Änderung: die Seite zeigt
                weiter den letzten Stand und sagt, dass er veraltet sein kann. */}
            <LoadErrorAlert error={loadError.error} />
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

            {data.signature ? (
              <AdminEnrollmentSignature
                requestId={requestId}
                signature={data.signature}
              />
            ) : null}

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
                  weeklyPlanToday={weeklyPlanCheckedOn === today ? today : null}
                  busy={busyChildId === child.id}
                  reason={reasons[child.id] ?? ""}
                  schemaFields={data.schema_fields}
                  actions={availableActions}
                  decisionError={
                    decisionErrorChildId === child.id
                      ? decisionErrors.error
                      : null
                  }
                  decisionFieldError={
                    decisionErrorChildId === child.id
                      ? decisionErrors.fieldError("reason")
                      : undefined
                  }
                  onReasonChange={(value) =>
                    setReasons((prev) => ({ ...prev, [child.id]: value }))
                  }
                  onDecide={(status) => requestDecision(child, status)}
                  onOfferingsChanged={() => void load({ quiet: true })}
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

          <aside className="border-t border-gray-100 bg-gray-50/70 p-5 max-sm:p-4 lg:border-t-0 lg:border-l">
            <RequestMailStatus
              mails={requestMails}
              loadError={requestMailsError}
            />
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
      </SectionCard>
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
    <InsetPanel as="section" framed padding="md">
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white shadow-sm">
          <MotoConceptIcon concept={parentsConcept} size={16} />
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
            <InsetPanel framed key={g.id}>
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
            </InsetPanel>
          ))}
        </div>
      )}
    </InsetPanel>
  );
}

function ChildInformationCard({
  actions,
  busy,
  child,
  decisionError,
  decisionFieldError,
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
  weeklyPlanToday,
}: Readonly<{
  actions: ActionDef[];
  busy: boolean;
  child: AdminRequestChild;
  decisionError: FormErrorInput;
  decisionFieldError?: string;
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
  weeklyPlanToday: string | null;
}>) {
  const terminal = isTerminalChildStatus(child.status);
  const existingChildHint = terminal ? null : existingChildHintMessage(child);
  const canDeleteFromEnrollment =
    terminal && child.created_student_id === undefined;
  const tenantPath = useTenantAwarePath();
  const studentHref = tenantPath(
    child.created_student_id
      ? `/students/${encodeURIComponent(child.created_student_id)}`
      : "/students",
  );
  return (
    <SectionCard padding="none">
      <div className="flex flex-wrap items-start justify-between gap-3 border-b border-gray-100 p-5 max-sm:p-4">
        <div>
          <h3 className="text-base font-semibold text-gray-900">
            {child.first_name} {child.last_name}
          </h3>
          <div className="mt-2 flex flex-wrap gap-2 text-xs text-gray-600">
            <StatusBadge
              tone="gray"
              label={`Geburtsdatum: ${formatPlainDate(child.date_of_birth)}`}
            />
            {enrolledClassLabel(child) ? (
              <StatusBadge tone="gray" label={enrolledClassLabel(child)} />
            ) : null}
            {child.holiday_care_wish === undefined ? null : (
              <StatusBadge
                tone="gray"
                label={
                  child.holiday_care_wish
                    ? "Ferienbetreuung: gewünscht"
                    : "Ferienbetreuung: nicht gewünscht"
                }
              />
            )}
          </div>
        </div>
        {/* Aktionen des Kindes rechts im Kartenkopf statt als eigene
            Buttonzeile im Karteninhalt. */}
        <div className="flex max-w-full min-w-0 flex-wrap items-center gap-2">
          {child.status === "approved" && child.created_student_id ? (
            <>
              <ActionHint text="Öffnet die Akte des Kindes mit der Einladung für die Eltern.">
                <NavigationLink
                  href={studentHref}
                  className={buttonClassName({
                    variant: "outline",
                    size: "md",
                    className: "gap-2",
                  })}
                >
                  Kind &amp; Einladung verwalten
                  <ExternalLink className="h-4 w-4" aria-hidden="true" />
                </NavigationLink>
              </ActionHint>
              <AdminChildDataCorrection
                requestId={requestId}
                child={child}
                onSaved={onDataCorrected}
              />
            </>
          ) : null}
          <ChildStatusBadge status={child.status} />
          {isHeldForChildQuota(child) ? <ChildQuotaHeldBadge /> : null}
        </div>
      </div>
      <div className="space-y-4 p-5 max-sm:p-4">
        {isHeldForChildQuota(child) ? (
          <Alert
            type="warning"
            announce="off"
            message="Beim Schuljahreswechsel war das Kinderkontingent voll. Darum wurde dieses Kind nicht automatisch verlängert. Bitte entscheiden Sie selbst."
          />
        ) : null}
        {existingChildHint ? (
          <Alert type="warning" announce="off" message={existingChildHint} />
        ) : null}
        {child.status_reason ? (
          <InsetPanel
            framed
            padding="compact"
            className="text-sm text-gray-700"
          >
            <span className="text-xs font-medium text-gray-500">
              Begründung:{" "}
            </span>
            {child.status_reason}
          </InsetPanel>
        ) : null}
        {child.reviewed_at ? (
          <p className="text-xs text-gray-500">
            Letzte Entscheidung: {formatDateTime(child.reviewed_at)}
          </p>
        ) : null}
        {child.status === "approved" &&
        child.weekly_plan_on &&
        weeklyPlanToday ? (
          <Alert
            type={child.weekly_plan_on < weeklyPlanToday ? "warning" : "info"}
            announce="off"
            message={weeklyPlanMessage(child.weekly_plan_on, weeklyPlanToday)}
          />
        ) : null}
        <ChildOfferings
          offerings={child.offerings}
          upcomingOfferings={child.upcoming_offerings}
          unavailable={child.offerings_unavailable}
          phaseName={phaseName}
        />
        {child.status === "approved" ? (
          <ChildOfferingAdjustment
            key={`${child.id}:${child.target_grade_level ?? ""}`}
            requestId={requestId}
            phaseId={phaseId}
            child={child}
            onSaved={onOfferingsChanged}
          />
        ) : null}
        <ChildExtraFields child={child} schemaFields={schemaFields} />
        {terminal ? (
          <div className="space-y-3">
            <InsetPanel
              framed
              padding="compact"
              className="text-sm text-gray-600"
            >
              Die Entscheidung ist final. Bei bestätigten Kindern können falsche
              Anmeldedaten weiterhin gezielt korrigiert werden.
            </InsetPanel>
            {canDeleteFromEnrollment ? (
              <Button
                type="button"
                variant="outline_danger"
                size="md"
                tooltip="moto fragt vorher nach und löscht dann das Kind aus der Anmeldung."
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
            error={decisionError}
            reasonError={decisionFieldError}
            reason={reason}
            onReasonChange={onReasonChange}
            onDecide={onDecide}
          />
        )}
      </div>
    </SectionCard>
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
          <StatCard
            variant="inline"
            label="Kinder"
            value={data.children.length}
          />
          <StatCard variant="inline" label="Offen" value={childStats.open} />
          <StatCard
            variant="inline"
            label="Bestätigt"
            value={childStats.approved}
          />
        </div>
        <div className="mt-4">
          <DataGrid>
            <DataField label="Phase">
              {data.phase_name || "Nicht zugeordnet"}
            </DataField>
            <DataField label="Eingegangen">{submittedAt}</DataField>
          </DataGrid>
        </div>
        <ActionHint text="Öffnet in einem neuen Tab die Statusseite, die die Eltern sehen.">
          <a
            href={statusHref}
            target="_blank"
            rel="noreferrer"
            className={buttonClassName({
              variant: "outline",
              size: "md",
              className: "mt-4 w-full gap-2",
            })}
          >
            Statusseite öffnen
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
          </a>
        </ActionHint>
        {onRestoreRequest ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            className="mt-3 w-full"
            tooltip="moto fragt vorher nach und öffnet dann zurückgezogene Kinder wieder."
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
          tooltip="moto fragt vorher nach und löscht dann die ganze Anmeldung."
          onClick={onDeleteRequest}
        >
          <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
          Gesamte Anmeldung löschen
        </Button>
      </SectionCard>

      {data.children.map((child) => {
        return (
          <SectionCard key={child.id}>
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="truncate text-sm font-semibold text-gray-900">
                  {child.first_name} {child.last_name}
                </h3>
                <p className="mt-1 text-xs text-gray-500">
                  {enrolledClassLabel(child) || "Keine Klassenstufe"}
                </p>
              </div>
              <ChildStatusBadge status={child.status} />
            </div>
            {isHeldForChildQuota(child) ? (
              <div className="mt-2">
                <ChildQuotaHeldBadge />
              </div>
            ) : null}
            <InsetPanel
              framed
              padding="compact"
              className="mt-3 text-sm text-gray-600"
            >
              {isTerminalChildStatus(child.status)
                ? "Entscheidung abgeschlossen"
                : "Entscheidung ausstehend"}
            </InsetPanel>
          </SectionCard>
        );
      })}
    </div>
  );
}

function DecisionPanel({
  actions,
  busy,
  child,
  error,
  onDecide,
  onReasonChange,
  reason,
  reasonError,
}: Readonly<{
  actions: ActionDef[];
  busy: boolean;
  child: AdminRequestChild;
  error: FormErrorInput;
  reasonError?: string;
  onDecide: (status: DecisionStatus) => void;
  onReasonChange: (value: string) => void;
  reason: string;
}>) {
  return (
    <InsetPanel framed padding="md">
      <div className="flex items-start gap-3">
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
          <MotoConceptIcon concept={enrollmentsConcept} size={18} />
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

      <FormErrorAlert message={error} className="mt-4" />

      <div className="mt-4">
        <Textarea
          id={`decision-reason-${child.id}`}
          name="reason"
          error={reasonError}
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
              disabledReason={
                isCurrent ? "Das Kind hat diesen Stand schon." : undefined
              }
              tooltip={DECISION_HINTS[action.status]}
              onClick={() => onDecide(action.status)}
              className="inline-flex items-center justify-center gap-2"
            >
              {getDecisionIcon(action.status)}
              {busy ? "Speichert…" : action.label}
            </Button>
          );
        })}
      </div>
    </InsetPanel>
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
    if (!isTerminalChildStatus(child.status)) open += 1;
    if (child.status === "approved") approved += 1;
  }
  return { open, approved };
}

// Only classify a pending plan against the day its enrollment was fetched.
// A mounted page must recheck the plan before reporting an overdue failure.
function weeklyPlanMessage(on: string, today: string): string {
  if (on < today) {
    return `Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung sollten ab ${formatPlainDate(on)} gelten. moto konnte sie noch nicht übernehmen. Darum gelten noch die bisherigen. moto versucht es jede Stunde neu. Bleibt dieser Hinweis, melden Sie sich bitte beim moto-Team.`;
  }
  return on === today
    ? "Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung gelten ab heute. moto übernimmt sie im Lauf des Tages."
    : `Abholzeiten, Betreuungstage und Heimweg aus dieser Anmeldung gelten ab ${formatPlainDate(on)}. Bis dahin bleiben die bisherigen.`;
}
