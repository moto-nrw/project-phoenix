"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Mail,
  MessageSquare,
  type LucideIcon,
  UserRound,
  X,
} from "lucide-react";
import {
  approveEnrollmentChangeRequest,
  askEnrollmentChangeRequestQuestion,
  getAdminEnrollmentChangeRequest,
  listAdminEnrollmentChangeRequests,
  rejectEnrollmentChangeRequest,
  type AdminEnrollmentChangeRequest,
  type AdminEnrollmentChangeRequestStatus,
} from "~/lib/enrollment-admin-api";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import { EnrollmentChangeRequestDiff } from "~/components/enrollment/enrollment-change-request-diff";
import { changedEnrollmentChangeRequestLabels } from "~/lib/enrollment-change-request-diff";

const logger = createLogger({ component: "AdminEnrollmentChangeRequests" });

const CHANGE_REQUEST_STATUS_LABELS: Record<
  AdminEnrollmentChangeRequestStatus,
  string
> = {
  pending_review: "Wartet auf Prüfung",
  needs_parent_response: "Rückfrage offen",
  approved: "Freigegeben",
  rejected: "Abgelehnt",
  cancelled: "Abgebrochen",
};

const CHANGE_REQUEST_STATUS_STYLES: Record<
  AdminEnrollmentChangeRequestStatus,
  { bg: string; dot: string; text: string }
> = {
  pending_review: { bg: "#EEF3FF", dot: "#5080D8", text: "#355A9A" },
  needs_parent_response: { bg: "#FFF4E6", dot: "#F78C10", text: "#8A5600" },
  approved: { bg: "#83CD2D1A", dot: "#83CD2D", text: "#5A8B1F" },
  rejected: { bg: "#FF31301A", dot: "#FF3130", text: "#9F1F1E" },
  cancelled: { bg: "#F3F4F6", dot: "#9CA3AF", text: "#4B5563" },
};

const STATUS_OPTIONS: Array<{
  value: "" | AdminEnrollmentChangeRequestStatus;
  label: string;
}> = [
  { value: "", label: "Alle" },
  { value: "pending_review", label: "Wartet auf Prüfung" },
  { value: "needs_parent_response", label: "Rückfrage offen" },
  { value: "approved", label: "Freigegeben" },
  { value: "rejected", label: "Abgelehnt" },
];

export function AdminEnrollmentChangeRequestsList() {
  const tenantPath = useTenantAwarePath();
  const [rows, setRows] = useState<AdminEnrollmentChangeRequest[]>([]);
  const [status, setStatus] = useState<"" | AdminEnrollmentChangeRequestStatus>(
    "",
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const fresh = await listAdminEnrollmentChangeRequests(
        status ? { status } : {},
      );
      setRows(fresh);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("change_requests_list_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [status]);

  useEffect(() => {
    void load();
  }, [load]);

  const stats = useMemo(() => summarizeChangeRequests(rows), [rows]);

  return (
    <div className="space-y-5">
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Anmeldungen
            </p>
            <h1 className="mt-1 text-xl font-semibold text-gray-900">
              Änderungsanfragen
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
              Prüfe Korrekturen von Familien und sieh protokollierte
              OGS-Korrekturen ein. Freigegebene Änderungen werden in die
              Anmeldung und bei bestätigten Kindern in die Stammdaten
              übernommen.
            </p>
          </div>
          <label
            className="block w-full sm:w-64"
            htmlFor="change-request-status-filter"
          >
            <span className="text-xs font-semibold text-gray-500 uppercase">
              Status
            </span>
            <CustomSelect
              id="change-request-status-filter"
              value={status}
              onChange={(value) =>
                setStatus(value as "" | AdminEnrollmentChangeRequestStatus)
              }
              ariaLabel="Status"
              className="mt-2"
              options={STATUS_OPTIONS}
            />
          </label>
        </div>

        <div className="mt-5 grid gap-3 sm:grid-cols-3">
          <Metric label="Offen" value={stats.open} />
          <Metric label="Rückfragen" value={stats.waitingForParent} />
          <Metric label="Abgeschlossen" value={stats.closed} />
        </div>
      </section>

      {error ? (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      ) : null}

      {loading ? (
        <p className="text-sm text-gray-500">
          Änderungsanfragen werden geladen...
        </p>
      ) : rows.length === 0 ? (
        <section className="moto-content-surface rounded-2xl border p-5 text-sm text-gray-600 shadow-sm">
          Keine Änderungsanfragen für den gewählten Status.
        </section>
      ) : (
        <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
          <ul className="divide-y divide-gray-100">
            {rows.map((row) => {
              const request = row.request;
              return (
                <li key={row.id}>
                  <Link
                    href={tenantPath(
                      `/admin/enrollments/change-requests/${encodeURIComponent(row.id)}`,
                    )}
                    className="group grid gap-4 p-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
                  >
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <ChangeRequestStatusBadge status={row.status} />
                        {row.origin === "admin" ? (
                          <span className="rounded-full bg-[#5080D8]/10 px-2.5 py-1 text-xs font-medium text-[#355A9A]">
                            OGS-Korrektur
                          </span>
                        ) : null}
                        <span className="text-xs text-gray-500">
                          {formatDateTime(row.created_at)}
                        </span>
                      </div>
                      <h2 className="mt-2 text-sm font-semibold text-gray-900">
                        {request
                          ? `${request.guardian_first_name} ${request.guardian_last_name}`
                          : `Anmeldung #${row.request_id}`}
                      </h2>
                      <p className="mt-1 truncate text-sm text-gray-600">
                        {row.parent_note?.trim() ||
                          row.admin_decision_note?.trim() ||
                          changedFieldLabels(row).join(", ") ||
                          "Keine Notiz hinterlegt"}
                      </p>
                      {request ? (
                        <p className="mt-1 text-xs text-gray-500">
                          {request.phase_name || "Keine Phase"} ·{" "}
                          {request.children.length} Kind(er)
                        </p>
                      ) : null}
                    </div>
                    <span className="inline-flex h-9 items-center justify-center gap-2 rounded-lg px-3 text-sm font-medium text-gray-700 group-hover:bg-white group-hover:shadow-sm">
                      Öffnen
                      <ArrowRight className="h-4 w-4" aria-hidden="true" />
                    </span>
                  </Link>
                </li>
              );
            })}
          </ul>
        </section>
      )}
    </div>
  );
}

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
    return <p className="text-sm text-gray-500">Wird geladen...</p>;
  }

  if (!data) {
    return (
      <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
        {error ?? "Änderungsanfrage nicht gefunden."}
      </div>
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
        <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
          <Link
            href={tenantPath("/admin/enrollments/change-requests")}
            className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <ArrowLeft className="h-4 w-4" aria-hidden="true" />
            Zurück zu Änderungsanfragen
          </Link>
        </div>

        <div className="grid gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_420px]">
          <div className="space-y-5 p-5 sm:p-6">
            <header>
              <div className="flex flex-wrap items-center gap-2">
                <ChangeRequestStatusBadge status={data.status} />
                <span className="text-xs text-gray-500">
                  {formatDateTime(data.created_at)}
                </span>
              </div>
              <h1 className="mt-3 text-xl font-semibold text-gray-900">
                {data.origin === "admin"
                  ? "OGS-Korrektur"
                  : "Änderungsanfrage prüfen"}
              </h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                {data.origin === "admin"
                  ? "Diese Korrektur wurde direkt an der Anmeldung vorgenommen, in die verknüpften Stammdaten übernommen und protokolliert."
                  : "Vergleiche die eingereichten Änderungen mit dem gespeicherten Stand. Rückfragen pausieren die Prüfung, Freigabe übernimmt die Änderung in die Anmeldung."}
              </p>
            </header>

            {error ? (
              <div className="rounded-lg border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
                {error}
              </div>
            ) : null}
            {info ? (
              <div className="rounded-lg border border-[#83CD2D]/20 bg-[#83CD2D]/10 p-3 text-sm text-[#5A8B1F]">
                {info}
              </div>
            ) : null}

            <ChangeSummary request={data} />
            {data.origin === "parent" ? <MessageThread request={data} /> : null}
          </div>

          <aside className="border-t border-gray-100 bg-gray-50/70 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="space-y-4 lg:sticky lg:top-6">
              <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
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
                <Link
                  href={enrollmentHref}
                  className="mt-4 inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-gray-300 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  Anmeldung öffnen
                  <ArrowRight className="h-4 w-4" aria-hidden="true" />
                </Link>
              </section>

              {data.origin === "parent" ? (
                <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                  <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    Rückfrage
                  </p>
                  <label className="mt-3 block">
                    <span className="text-sm font-semibold text-gray-700">
                      Nachricht an Eltern
                    </span>
                    <textarea
                      value={question}
                      onChange={(event) => setQuestion(event.target.value)}
                      rows={4}
                      disabled={!canReview || busy !== null}
                      className="mt-2 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none disabled:bg-gray-50"
                    />
                  </label>
                  <button
                    type="button"
                    onClick={() => void handleQuestion()}
                    disabled={!canReview || busy !== null || !question.trim()}
                    className="mt-3 inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                  >
                    <MessageSquare className="h-4 w-4" aria-hidden="true" />
                    {busy === "question" ? "Sendet..." : "Rückfrage senden"}
                  </button>
                </section>
              ) : null}

              {data.origin === "parent" ? (
                <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
                  <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    Entscheidung
                  </p>
                  <label className="mt-3 block">
                    <span className="text-sm font-semibold text-gray-700">
                      Begründung
                    </span>
                    <textarea
                      value={reviewNote}
                      onChange={(event) => setReviewNote(event.target.value)}
                      rows={4}
                      disabled={!canReview || busy !== null}
                      placeholder="Kurz begründen, warum die Änderung übernommen oder abgelehnt wird."
                      className="mt-2 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none disabled:bg-gray-50"
                    />
                  </label>
                  <div className="mt-3 grid gap-2 sm:grid-cols-2">
                    <button
                      type="button"
                      onClick={() => void handleReview(true)}
                      disabled={
                        !canReview || busy !== null || !reviewNote.trim()
                      }
                      className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:border-[#83CD2D]/60 hover:bg-[#83CD2D]/10 hover:text-[#5A8B1F] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                    >
                      <Check className="h-4 w-4" aria-hidden="true" />
                      {busy === "approve" ? "Speichert..." : "Freigeben"}
                    </button>
                    <button
                      type="button"
                      onClick={() => void handleReview(false)}
                      disabled={
                        !canReview || busy !== null || !reviewNote.trim()
                      }
                      className="inline-flex h-9 items-center justify-center gap-2 rounded-lg border border-[#FF3130]/20 bg-white px-3 text-sm font-semibold text-[#CC2626] shadow-sm hover:bg-[#FF3130]/10 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                    >
                      <X className="h-4 w-4" aria-hidden="true" />
                      {busy === "reject" ? "Speichert..." : "Ablehnen"}
                    </button>
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
    <section className="moto-content-surface space-y-4 rounded-2xl border p-5 shadow-sm">
      <div>
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
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
    <section className="moto-content-surface space-y-4 rounded-2xl border p-5 shadow-sm">
      <div>
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Verlauf
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Nachrichten
        </h2>
      </div>
      {messages.length === 0 ? (
        <p className="text-sm text-gray-600">Noch keine Nachrichten.</p>
      ) : (
        <ol className="space-y-3">
          {messages.map((message) => (
            <li
              key={message.id}
              className="rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2"
            >
              <p className="text-xs font-semibold text-gray-500">
                {messageAuthorLabel(message.author_type)} ·{" "}
                {formatDateTime(message.created_at)}
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

function Metric({
  label,
  value,
}: {
  readonly label: string;
  readonly value: number;
}) {
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

function ChangeRequestStatusBadge({
  status,
}: {
  readonly status: AdminEnrollmentChangeRequestStatus;
}) {
  const styles = CHANGE_REQUEST_STATUS_STYLES[status];
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
      {CHANGE_REQUEST_STATUS_LABELS[status]}
    </span>
  );
}

function changedFieldLabels(request: AdminEnrollmentChangeRequest): string[] {
  return changedEnrollmentChangeRequestLabels({
    baseSnapshot: request.base_snapshot,
    proposedSnapshot: request.proposed_snapshot,
    diff: request.diff,
  });
}

function summarizeChangeRequests(rows: AdminEnrollmentChangeRequest[]) {
  let open = 0;
  let waitingForParent = 0;
  let closed = 0;
  for (const row of rows) {
    if (row.status === "pending_review") open += 1;
    if (row.status === "needs_parent_response") waitingForParent += 1;
    if (
      row.status === "approved" ||
      row.status === "rejected" ||
      row.status === "cancelled"
    ) {
      closed += 1;
    }
  }
  return { open, waitingForParent, closed };
}

function formatDateTime(value: string): string {
  return new Date(value).toLocaleString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function messageAuthorLabel(authorType: string): string {
  if (authorType === "parent") return "Eltern";
  if (authorType === "system") return "System";
  return "OGS";
}
