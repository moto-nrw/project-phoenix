"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ArrowRight,
  Check,
  Mail,
  type LucideIcon,
  UserRound,
  X,
} from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import {
  approveEnrollmentChangeRequest,
  askEnrollmentChangeRequestQuestion,
  getAdminEnrollmentChangeRequest,
  rejectEnrollmentChangeRequest,
  type AdminEnrollmentChangeRequest,
  type AdminEnrollmentChangeRequestStatus,
} from "~/lib/enrollment-admin-api";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { createLogger } from "~/lib/logger";
import { StatusBadge } from "~/components/ui/status-badge";
import { EnrollmentChangeRequestDiff } from "~/components/enrollment/enrollment-change-request-diff";
import { ENROLLMENT_CHANGE_REQUEST_STATUS_META } from "~/components/enrollment/enrollment-change-request-status";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { DetailSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { Textarea } from "~/components/ui/textarea";
import { formatChatDateTime } from "~/lib/date-helpers";

const logger = createLogger({ component: "AdminEnrollmentChangeRequests" });

export function AdminEnrollmentChangeRequestDetail({
  changeRequestId,
}: {
  readonly changeRequestId: string;
}) {
  const tenantPath = useTenantAwarePath();
  const [data, setData] = useState<AdminEnrollmentChangeRequest | null>(null);
  const [question, setQuestion] = useState("");
  const [reviewNote, setReviewNote] = useState("");
  const [busy, setBusy] = useState<"question" | "approve" | "reject" | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const fresh = await getAdminEnrollmentChangeRequest(changeRequestId);
      setData(fresh);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("change_request_detail_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [changeRequestId]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleQuestion = async () => {
    const body = question.trim();
    if (!body) return;
    setBusy("question");
    setError(null);
    setInfo(null);
    try {
      const fresh = await askEnrollmentChangeRequestQuestion(
        changeRequestId,
        body,
      );
      setData(fresh);
      setQuestion("");
      setInfo("Rückfrage gesendet.");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("change_request_question_failed", { error: message });
      setError(message);
    } finally {
      setBusy(null);
    }
  };

  const handleReview = async (approved: boolean) => {
    const note = reviewNote.trim();
    if (!note) {
      setError("Bitte eine kurze Begründung eintragen.");
      return;
    }
    setBusy(approved ? "approve" : "reject");
    setError(null);
    setInfo(null);
    try {
      const fresh = approved
        ? await approveEnrollmentChangeRequest(changeRequestId, note)
        : await rejectEnrollmentChangeRequest(changeRequestId, note);
      setData(fresh);
      setReviewNote("");
      setInfo(approved ? "Änderung freigegeben." : "Änderung abgelehnt.");
      window.dispatchEvent(new Event("change-requests-refresh"));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("change_request_review_failed", {
        error: message,
        approved,
      });
      setError(message);
    } finally {
      setBusy(null);
    }
  };

  if (loading) {
    return (
      <SkeletonRegion label="Änderungsanfrage wird geladen">
        <DetailSkeleton sections={2} fieldsPerSection={4} />
      </SkeletonRegion>
    );
  }

  if (!data) {
    return (
      <Alert
        type="error"
        message={error ?? "Änderungsanfrage nicht gefunden."}
      />
    );
  }

  const request = data.request;
  const canReview = data.status === "pending_review";
  const enrollmentHref = request
    ? tenantPath(`/admin/enrollments/${encodeURIComponent(request.id)}`)
    : tenantPath("/admin/enrollments");

  return (
    <div className="space-y-5">
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_420px]">
          <div className="space-y-5 p-5 sm:p-6">
            <header>
              <div className="flex flex-wrap items-center gap-2">
                <ChangeRequestStatusBadge status={data.status} />
                <span className="text-xs text-gray-500">
                  {formatChatDateTime(data.created_at)}
                </span>
              </div>
              <h1 className="mt-3 text-2xl font-bold text-gray-900 sm:text-3xl">
                {data.origin === "admin"
                  ? "OGS-Korrektur"
                  : "Änderungsanfrage prüfen"}
              </h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                {data.origin === "admin"
                  ? "Diese Korrektur wurde direkt an der Anmeldung vorgenommen, in die verknüpften Stammdaten übernommen und protokolliert."
                  : "Vergleichen Sie die eingereichten Änderungen mit dem gespeicherten Stand. Rückfragen pausieren die Prüfung, Freigabe übernimmt die Änderung in die Anmeldung."}
              </p>
            </header>

            {error ? <Alert type="error" message={error} /> : null}
            {info ? <Alert type="success" message={info} /> : null}

            <ChangeSummary request={data} />
            {data.origin === "parent" ? <MessageThread request={data} /> : null}
          </div>

          <aside className="border-t border-gray-100 bg-gray-50/70 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="space-y-4 lg:sticky lg:top-6">
              <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                  Anmeldung
                </p>
                {request ? (
                  <>
                    <h2 className="mt-1 text-base font-semibold text-gray-900">
                      {request.guardian_first_name} {request.guardian_last_name}
                    </h2>
                    <dl className="mt-4 space-y-3 text-sm">
                      <InfoRow
                        icon={Mail}
                        label="E-Mail"
                        value={request.guardian_email}
                      />
                      <InfoRow
                        icon={UserRound}
                        label="Kinder"
                        value={String(request.children.length)}
                      />
                    </dl>
                  </>
                ) : (
                  <p className="mt-2 text-sm text-gray-600">
                    Anmeldung #{data.request_id}
                  </p>
                )}
                <ButtonLink
                  href={enrollmentHref}
                  variant="outline"
                  size="md"
                  className="mt-4 inline-flex w-full items-center justify-center gap-2"
                >
                  Anmeldung öffnen
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </ButtonLink>
              </section>

              {data.origin === "parent" ? (
                <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                  <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                    Rückfrage
                  </p>
                  <div className="mt-3">
                    <Textarea
                      id="change-request-question"
                      label="Nachricht an Eltern"
                      value={question}
                      onChange={(event) => setQuestion(event.target.value)}
                      rows={4}
                      disabled={!canReview || busy !== null}
                    />
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={() => void handleQuestion()}
                    disabled={!canReview || busy !== null || !question.trim()}
                    className="mt-3 inline-flex w-full items-center justify-center gap-2"
                  >
                    <MotoConceptIcon concept="parentConversations" size={16} />
                    {busy === "question" ? "Sendet…" : "Rückfrage senden"}
                  </Button>
                </section>
              ) : null}

              {data.origin === "parent" ? (
                <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                  <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                    Entscheidung
                  </p>
                  <div className="mt-3">
                    <Textarea
                      id="change-request-review-note"
                      label="Begründung"
                      value={reviewNote}
                      onChange={(event) => setReviewNote(event.target.value)}
                      rows={4}
                      disabled={!canReview || busy !== null}
                      placeholder="Kurz begründen, warum die Änderung übernommen oder abgelehnt wird."
                    />
                  </div>
                  <div className="mt-3 grid gap-2 sm:grid-cols-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="md"
                      onClick={() => void handleReview(true)}
                      disabled={
                        !canReview || busy !== null || !reviewNote.trim()
                      }
                      className="inline-flex items-center justify-center gap-2"
                    >
                      <Check className="h-4 w-4" aria-hidden="true" />
                      {busy === "approve" ? "Speichert…" : "Freigeben"}
                    </Button>
                    <Button
                      type="button"
                      variant="outline_danger"
                      size="md"
                      onClick={() => void handleReview(false)}
                      disabled={
                        !canReview || busy !== null || !reviewNote.trim()
                      }
                      className="inline-flex items-center justify-center gap-2"
                    >
                      <X className="h-4 w-4" aria-hidden="true" />
                      {busy === "reject" ? "Speichert…" : "Ablehnen"}
                    </Button>
                  </div>
                  {!canReview ? (
                    <p className="mt-3 text-xs leading-5 text-gray-500">
                      Entscheidungen sind nur möglich, solange die Anfrage auf
                      Prüfung wartet.
                    </p>
                  ) : null}
                </section>
              ) : null}
            </div>
          </aside>
        </div>
      </section>
    </div>
  );
}

function ChangeSummary({
  request,
}: {
  readonly request: AdminEnrollmentChangeRequest;
}) {
  return (
    <section className="moto-content-surface space-y-4 rounded-2xl border p-4 shadow-sm sm:p-6">
      <div>
        <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
          Änderungen
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          {request.origin === "admin"
            ? "Protokollierte OGS-Korrektur"
            : "Eingereichte Korrektur"}
        </h2>
      </div>

      {request.parent_note ? (
        <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2 text-sm leading-6 text-gray-700">
          <span className="font-semibold">Hinweis der Eltern: </span>
          {request.parent_note}
        </div>
      ) : null}

      {request.origin === "admin" && request.admin_decision_note ? (
        <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2 text-sm leading-6 text-gray-700">
          <span className="font-semibold">Grund: </span>
          {request.admin_decision_note}
        </div>
      ) : null}

      <EnrollmentChangeRequestDiff
        baseSnapshot={request.base_snapshot}
        proposedSnapshot={request.proposed_snapshot}
        diff={request.diff}
      />
    </section>
  );
}

function MessageThread({
  request,
}: {
  readonly request: AdminEnrollmentChangeRequest;
}) {
  const messages = request.messages ?? [];

  return (
    <section className="moto-content-surface space-y-4 rounded-2xl border p-4 shadow-sm sm:p-6">
      <div>
        <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
          Verlauf
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Nachrichten
        </h2>
      </div>
      {messages.length === 0 ? (
        <EmptyState
          variant="compact"
          title="Noch keine Nachrichten"
          description="Sobald eine Rückfrage gestellt oder beantwortet wird, erscheint sie hier."
        />
      ) : (
        <ol className="space-y-3">
          {messages.map((message) => (
            <li
              key={message.id}
              className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2"
            >
              <p className="text-xs font-semibold text-gray-500">
                {messageAuthorLabel(message.author_type)} ·{" "}
                {formatChatDateTime(message.created_at)}
              </p>
              <p className="mt-1 text-sm leading-6 whitespace-pre-wrap text-gray-700">
                {message.body}
              </p>
            </li>
          ))}
        </ol>
      )}
      {request.admin_decision_note ? (
        <div className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2 text-sm leading-6 text-gray-700">
          <span className="font-semibold">Entscheidung: </span>
          {request.admin_decision_note}
        </div>
      ) : null}
    </section>
  );
}

function InfoRow({
  icon: Icon,
  label,
  value,
}: {
  readonly icon: LucideIcon;
  readonly label: string;
  readonly value: string;
}) {
  return (
    <div>
      <dt className="flex items-center gap-2 text-xs font-medium text-gray-500 uppercase">
        <Icon className="h-3.5 w-3.5" aria-hidden="true" />
        {label}
      </dt>
      <dd className="mt-0.5 break-words text-gray-900">{value}</dd>
    </div>
  );
}

function ChangeRequestStatusBadge({
  status,
}: {
  readonly status: AdminEnrollmentChangeRequestStatus;
}) {
  return (
    <StatusBadge
      label={ENROLLMENT_CHANGE_REQUEST_STATUS_META[status].label}
      tone={ENROLLMENT_CHANGE_REQUEST_STATUS_META[status].tone}
    />
  );
}

function messageAuthorLabel(authorType: string): string {
  if (authorType === "parent") return "Eltern";
  if (authorType === "system") return "System";
  return "OGS";
}
