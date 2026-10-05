"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { ArrowRight, Check, Mail, UserRound, X } from "lucide-react";
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
import { TenantPage } from "~/components/ui/tenant-page";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EnrollmentChangeRequestDiff } from "~/components/enrollment/enrollment-change-request-diff";
import { ENROLLMENT_CHANGE_REQUEST_STATUS_META } from "~/components/enrollment/enrollment-change-request-status";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Textarea } from "~/components/ui/textarea";
import { formatChatDateTime } from "~/lib/date-helpers";
import { changedEnrollmentChangeRequestLabels } from "~/lib/enrollment-change-request-diff";

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
  const toast = useToast();
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const questionErrors = useApiFormError();
  const reviewErrors = useApiFormError();
  // „Wiederholen“ sendet den aktuellen Stand, nicht den vom Fehler.
  const latestQuestionRef = useRef<() => Promise<void>>(async () => undefined);
  const latestReviewRef = useRef<(approved: boolean) => Promise<void>>(
    async () => undefined,
  );
  const reloadRef = useRef<() => Promise<void>>(async () => undefined);

  const load = useCallback(async () => {
    setLoading(true);
    clearLoadError();
    try {
      const fresh = await getAdminEnrollmentChangeRequest(changeRequestId);
      setData(fresh);
    } catch (err) {
      logger.error("change_request_detail_failed", {
        error: err instanceof Error ? err.message : "unknown",
      });
      await showLoadError(err, {
        object: "die Änderungsanfrage",
        retry: () => void reloadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [changeRequestId, clearLoadError, showLoadError]);

  useLayoutEffect(() => {
    reloadRef.current = load;
  });

  useEffect(() => {
    void load();
  }, [load]);

  const handleQuestion = async () => {
    const body = question.trim();
    if (!body) return;
    setBusy("question");
    questionErrors.clear();
    try {
      const fresh = await askEnrollmentChangeRequestQuestion(
        changeRequestId,
        body,
      );
      setData(fresh);
      setQuestion("");
      toast.success("Die Rückfrage wurde gesendet.");
    } catch (err) {
      logger.error("change_request_question_failed", {
        error: err instanceof Error ? err.message : "unknown",
      });
      await questionErrors.show(err, {
        object: "die Rückfrage",
        retry: () => void latestQuestionRef.current(),
      });
    } finally {
      setBusy(null);
    }
  };
  useLayoutEffect(() => {
    latestQuestionRef.current = handleQuestion;
  });

  const handleReview = async (approved: boolean) => {
    const note = reviewNote.trim();
    if (!note) {
      reviewErrors.invalid("Bitte tragen Sie eine kurze Begründung ein.", {
        note: "Bitte tragen Sie eine Begründung ein.",
      });
      return;
    }
    setBusy(approved ? "approve" : "reject");
    reviewErrors.clear();
    try {
      const fresh = approved
        ? await approveEnrollmentChangeRequest(changeRequestId, note)
        : await rejectEnrollmentChangeRequest(changeRequestId, note);
      setData(fresh);
      setReviewNote("");
      toast.success(
        approved
          ? "Die Änderung wurde freigegeben."
          : "Die Änderung wurde abgelehnt.",
      );
      window.dispatchEvent(new Event("change-requests-refresh"));
    } catch (err) {
      logger.error("change_request_review_failed", {
        error: err instanceof Error ? err.message : "unknown",
        approved,
      });
      await reviewErrors.show(err, {
        object: "die Entscheidung",
        retry: () => void latestReviewRef.current(approved),
      });
    } finally {
      setBusy(null);
    }
  };
  useLayoutEffect(() => {
    latestReviewRef.current = handleReview;
  });

  if (loading) {
    return (
      <TenantPage
        title="Änderungsanfrage"
        back
        backHref="/anfragen"
        backLabel="Zurück zu Anfragen"
        statsLoading
        loading
      />
    );
  }

  if (!data) {
    return (
      <TenantPage
        title="Änderungsanfrage"
        back
        backHref="/anfragen"
        backLabel="Zurück zu Anfragen"
        error={loadError.error ?? "Die Änderungsanfrage wurde nicht gefunden."}
      />
    );
  }

  const request = data.request;
  const canReview = data.status === "pending_review";
  const enrollmentHref = request
    ? tenantPath(`/admin/enrollments/${encodeURIComponent(request.id)}`)
    : tenantPath("/admin/enrollments");

  // Statuszeile des Seitenkopfs: die Zahlen der geladenen Anfrage.
  const changeCount = changedEnrollmentChangeRequestLabels({
    baseSnapshot: data.base_snapshot,
    proposedSnapshot: data.proposed_snapshot,
    diff: data.diff,
  }).length;
  const statusLine = [
    request
      ? `${request.guardian_first_name} ${request.guardian_last_name}`
      : null,
    request
      ? `${request.children.length} ${request.children.length === 1 ? "Kind" : "Kinder"}`
      : null,
    `${changeCount} ${changeCount === 1 ? "Änderung" : "Änderungen"}`,
  ]
    .filter(Boolean)
    .join(" · ");

  return (
    <TenantPage
      title={
        data.origin === "admin" ? "OGS-Korrektur" : "Änderungsanfrage prüfen"
      }
      back
      backHref="/anfragen"
      backLabel="Zurück zu Anfragen"
      stats={statusLine}
      leading={<ConceptIconTile concept="enrollments" variant="page" />}
      actions={
        <div className="flex flex-wrap items-center gap-2">
          <ChangeRequestStatusBadge status={data.status} />
          <span className="text-xs text-gray-500">
            {formatChatDateTime(data.created_at)}
          </span>
        </div>
      }
    >
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_420px]">
          <div className="space-y-5 p-5 sm:p-6">
            <p className="text-sm leading-6 text-gray-600">
              {data.origin === "admin"
                ? "Diese Korrektur wurde direkt an der Anmeldung vorgenommen, in die verknüpften Stammdaten übernommen und protokolliert."
                : "Vergleichen Sie die eingereichten Änderungen mit dem gespeicherten Stand. Rückfragen pausieren die Prüfung, Freigabe übernimmt die Änderung in die Anmeldung."}
            </p>

            <ChangeSummary request={data} />
            {data.origin === "parent" ? <MessageThread request={data} /> : null}
          </div>

          <aside className="border-t border-gray-100 bg-gray-50/70 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="space-y-4 lg:sticky lg:top-6">
              <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                <h2 className="text-base font-semibold text-gray-900">
                  Anmeldung
                </h2>
                {request ? (
                  <>
                    <p className="mt-1 text-sm font-semibold text-gray-900">
                      {request.guardian_first_name} {request.guardian_last_name}
                    </p>
                    <div className="mt-4">
                      <DataGrid columns={1}>
                        <DataField
                          label="E-Mail"
                          icon={<Mail className="h-3.5 w-3.5" />}
                        >
                          {request.guardian_email}
                        </DataField>
                        <DataField
                          label="Kinder"
                          icon={<UserRound className="h-3.5 w-3.5" />}
                        >
                          {String(request.children.length)}
                        </DataField>
                      </DataGrid>
                    </div>
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
                  <h2 className="text-base font-semibold text-gray-900">
                    Rückfrage
                  </h2>
                  <FormErrorAlert
                    message={questionErrors.error}
                    className="mt-3"
                  />
                  <div className="mt-3">
                    <Textarea
                      id="change-request-question"
                      name="body"
                      error={questionErrors.fieldError("body")}
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
                  <h2 className="text-base font-semibold text-gray-900">
                    Entscheidung
                  </h2>
                  <FormErrorAlert
                    message={reviewErrors.error}
                    className="mt-3"
                  />
                  <div className="mt-3">
                    <Textarea
                      id="change-request-review-note"
                      name="note"
                      error={reviewErrors.fieldError("note")}
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
    </TenantPage>
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
        <h2 className="text-base font-semibold text-gray-900">
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
        <h2 className="text-base font-semibold text-gray-900">Nachrichten</h2>
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
