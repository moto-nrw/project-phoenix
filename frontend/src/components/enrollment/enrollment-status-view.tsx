"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import {
  AlertTriangle,
  Check,
  Clock,
  Mail,
  MessageSquare,
  Pencil,
  ShieldCheck,
  UserRound,
} from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import {
  confirmRenewal,
  fetchStatus,
  listEnrollmentChangeRequests,
  patchStatus,
  replyEnrollmentChangeRequest,
  withdrawStatus,
  type EnrollmentChangeRequest,
  type StatusChild,
  type StatusGuardian,
  type StatusResponse,
} from "~/lib/enrollment-submission-api";
import { createLogger } from "~/lib/logger";
import { EnrollmentChangeRequestDiff } from "~/components/enrollment/enrollment-change-request-diff";
import type { EnrollmentChangeRequestDiffCopy } from "~/lib/enrollment-change-request-diff";

const logger = createLogger({ component: "EnrollmentStatusView" });

const STATUS_STYLES: Record<
  StatusChild["status"],
  { dot: string; text: string; bg: string }
> = {
  submitted: { dot: "#5080D8", text: "#374151", bg: "#F3F4F6" },
  under_review: { dot: "#5080D8", text: "#374151", bg: "#F3F4F6" },
  approved: { dot: "#83CD2D", text: "#5A8E1F", bg: "#83CD2D1A" },
  waitlisted: { dot: "#F78C10", text: "#7C4A03", bg: "#F78C101A" },
  rejected: { dot: "#FF3130", text: "#9F1F1E", bg: "#FF31301A" },
  withdrawn: { dot: "#6B7280", text: "#374151", bg: "#F3F4F6" },
  pending_renewal: { dot: "#F78C10", text: "#7C4A03", bg: "#F78C101A" },
  auto_renewed: { dot: "#5080D8", text: "#374151", bg: "#F3F4F6" },
  pending_admin_review: { dot: "#6B7280", text: "#374151", bg: "#F3F4F6" },
};

const TERMINAL_STATUSES = new Set<StatusChild["status"]>([
  "approved",
  "rejected",
  "withdrawn",
]);

const CHANGE_REQUEST_STYLES: Record<
  EnrollmentChangeRequest["status"],
  { bg: string; dot: string; text: string }
> = {
  pending_review: { bg: "#EEF3FF", dot: "#5080D8", text: "#355A9A" },
  needs_parent_response: { bg: "#FFF4E6", dot: "#F78C10", text: "#8A5600" },
  approved: { bg: "#83CD2D1A", dot: "#83CD2D", text: "#5A8B1F" },
  rejected: { bg: "#FF31301A", dot: "#FF3130", text: "#9F1F1E" },
  cancelled: { bg: "#F3F4F6", dot: "#9CA3AF", text: "#4B5563" },
};

const OPEN_CHANGE_REQUEST_STATUSES = new Set<EnrollmentChangeRequest["status"]>(
  ["pending_review", "needs_parent_response"],
);

interface Props {
  readonly token: string;
  readonly justSubmitted?: boolean;
}

export function EnrollmentStatusView({ token, justSubmitted = false }: Props) {
  const t = useTranslations("enrollmentStatus");
  const locale = useLocale();
  const pathname = usePathname();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [changeRequests, setChangeRequests] = useState<
    EnrollmentChangeRequest[]
  >([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [info, setInfo] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [savingEdit, setSavingEdit] = useState(false);
  const [withdrawingChild, setWithdrawingChild] = useState<string | null>(null);
  const [confirmingRenewal, setConfirmingRenewal] = useState(false);
  const [replyingChangeRequest, setReplyingChangeRequest] = useState<
    string | null
  >(null);
  const [replyDrafts, setReplyDrafts] = useState<Record<string, string>>({});

  const [editFirstName, setEditFirstName] = useState("");
  const [editLastName, setEditLastName] = useState("");
  const [editPhone, setEditPhone] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setNotFound(false);
    try {
      const [result, requestChanges] = await Promise.all([
        fetchStatus(token),
        listEnrollmentChangeRequests(token).catch((err) => {
          logger.warn("change_requests_load_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as EnrollmentChangeRequest[];
        }),
      ]);
      if (!result) {
        setNotFound(true);
        setChangeRequests([]);
        return;
      }
      setStatus(result);
      setChangeRequests(requestChanges);
      setEditFirstName(result.guardian_first_name);
      setEditLastName(result.guardian_last_name);
      setEditPhone(result.guardian_phone ?? "");
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.error("status_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [token, t]);

  useEffect(() => {
    void load();
  }, [load]);

  const allEditable = status?.edit_mode === "direct_edit";
  const editHref = pathname?.startsWith("/parents")
    ? `/parents/enroll/status/${encodeURIComponent(token)}/edit`
    : `${pathname?.replace(/\/$/, "") ?? ""}/edit`;

  const handleChangeRequestReply = async (changeRequestId: string) => {
    const body = (replyDrafts[changeRequestId] ?? "").trim();
    if (!body) return;
    setReplyingChangeRequest(changeRequestId);
    setError(null);
    setInfo(null);
    try {
      await replyEnrollmentChangeRequest(token, changeRequestId, body);
      setReplyDrafts((prev) => ({ ...prev, [changeRequestId]: "" }));
      setInfo(t("changeRequestReplySaved"));
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.error("change_request_reply_failed", { error: message });
      setError(message);
    } finally {
      setReplyingChangeRequest(null);
    }
  };

  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!status) return;
    setSavingEdit(true);
    setError(null);
    setInfo(null);
    try {
      await patchStatus(token, {
        guardian_first_name: editFirstName.trim(),
        guardian_last_name: editLastName.trim(),
        guardian_phone: editPhone.trim() || undefined,
      });
      setInfo(t("editSaved"));
      setEditing(false);
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.error("status_edit_failed", { error: message });
      setError(message);
    } finally {
      setSavingEdit(false);
    }
  };

  const handleConfirmRenewal = async () => {
    if (!status) return;
    setConfirmingRenewal(true);
    setError(null);
    setInfo(null);
    try {
      const confirmed = await confirmRenewal(token);
      setInfo(
        confirmed === 1
          ? t("renewalConfirmedOne")
          : t("renewalConfirmedMany", { count: confirmed }),
      );
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.error("status_confirm_renewal_failed", { error: message });
      setError(message);
    } finally {
      setConfirmingRenewal(false);
    }
  };

  const handleWithdraw = async (childID?: string) => {
    if (!status) return;
    const confirmMessage = childID
      ? t("confirmWithdrawChild")
      : t("confirmWithdrawAll");
    if (!window.confirm(confirmMessage)) return;
    setWithdrawingChild(childID ?? "__all__");
    setError(null);
    setInfo(null);
    try {
      await withdrawStatus(token, childID);
      setInfo(childID ? t("withdrawnChild") : t("withdrawnAll"));
      await load();
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.error("status_withdraw_failed", { error: message });
      setError(message);
    } finally {
      setWithdrawingChild(null);
    }
  };

  if (loading) {
    return (
      <div className="moto-content-surface rounded-xl border p-5 text-sm font-medium text-gray-600 shadow-sm sm:p-6">
        {t("loading")}
      </div>
    );
  }

  if (notFound) {
    return (
      <div className="moto-content-surface rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-5 text-center shadow-sm sm:p-6">
        <h1 className="text-xl font-semibold text-[#CC2626]">
          {t("invalidTitle")}
        </h1>
        <p className="mt-2 text-sm text-[#CC2626]">{t("invalidDescription")}</p>
      </div>
    );
  }

  if (!status) {
    return (
      <div className="moto-content-surface rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-5 text-sm text-[#CC2626] shadow-sm sm:p-6">
        {error ?? t("loadFallback")}
      </div>
    );
  }

  const submittedDate = new Date(status.submitted_at).toLocaleDateString(
    locale,
    { day: "2-digit", month: "long", year: "numeric" },
  );

  const allWithdrawn =
    !!status.withdrawn_at ||
    status.children.every((c) => c.status === "withdrawn");
  const hasMultipleChildren = status.children.length > 1;
  const hasOpenChangeRequest = changeRequests.some((request) =>
    OPEN_CHANGE_REQUEST_STATUSES.has(request.status),
  );
  const canRequestChange =
    status.edit_mode === "change_request" && !hasOpenChangeRequest;

  const pendingRenewalCount = status.children.filter(
    (c) => c.status === "pending_renewal",
  ).length;
  const autoRenewedCount = status.children.filter(
    (c) => c.status === "auto_renewed",
  ).length;
  const showOptInBanner = pendingRenewalCount > 0;
  const showOptOutBanner = !showOptInBanner && autoRenewedCount > 0;

  return (
    <div className="mx-auto max-w-5xl space-y-5 sm:space-y-6">
      <section className="moto-content-surface overflow-hidden rounded-3xl border shadow-sm">
        <div className="grid lg:grid-cols-[minmax(0,1fr)_22rem]">
          <div className="p-5 sm:p-8 lg:p-10">
            <div
              className={`flex h-12 w-12 items-center justify-center rounded-xl sm:h-14 sm:w-14 ${
                justSubmitted
                  ? "bg-[#83CD2D]/15 text-[#5A8E1F]"
                  : "moto-content-surface border text-gray-600 shadow-sm"
              }`}
            >
              {justSubmitted ? (
                <Check className="h-7 w-7" aria-hidden="true" />
              ) : (
                <Clock className="h-7 w-7" aria-hidden="true" />
              )}
            </div>
            <p className="mt-6 text-sm font-semibold tracking-wide text-[#5080D8] uppercase">
              {justSubmitted ? t("submittedEyebrow") : t("statusEyebrow")}
            </p>
            <h1 className="mt-2 max-w-2xl text-3xl font-bold tracking-tight text-wrap text-gray-900 sm:text-4xl">
              {justSubmitted ? t("submittedTitle") : t("statusTitle")}
            </h1>
            <p className="mt-4 max-w-2xl text-base leading-7 text-gray-600">
              {justSubmitted
                ? t("submittedDescription")
                : t("statusDescription", { date: submittedDate })}
            </p>
            {(allEditable || canRequestChange) && (
              <div className="mt-6">
                <Link
                  href={editHref}
                  className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                >
                  <Pencil className="h-4 w-4" aria-hidden="true" />
                  {allEditable ? t("editFull") : t("requestChange")}
                </Link>
              </div>
            )}
          </div>
          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-8 lg:border-t-0 lg:border-l">
            <h2 className="text-lg font-semibold text-gray-900">
              {justSubmitted ? t("nextTitle") : t("currentTitle")}
            </h2>
            <ol className="mt-5 space-y-4 text-sm text-gray-600">
              <li className="flex gap-3">
                <StepNumber>1</StepNumber>
                <span>
                  {justSubmitted ? t("stepOneSubmitted") : t("stepOneStatus")}
                </span>
              </li>
              <li className="flex gap-3">
                <StepNumber>2</StepNumber>
                <span>{t("stepTwo")}</span>
              </li>
              <li className="flex gap-3">
                <StepNumber>3</StepNumber>
                <span>{t("stepThree")}</span>
              </li>
            </ol>
          </aside>
        </div>
      </section>

      <section className="grid gap-3 sm:grid-cols-3">
        <StatusSummaryCard
          icon={<Clock className="h-5 w-5" aria-hidden="true" />}
          label={t("receivedLabel")}
          value={submittedDate}
        />
        <StatusSummaryCard
          icon={<ShieldCheck className="h-5 w-5" aria-hidden="true" />}
          label={t("childrenLabel")}
          value={String(status.children.length)}
        />
        <StatusSummaryCard
          icon={<Mail className="h-5 w-5" aria-hidden="true" />}
          label={t("contactLabel")}
          value={status.guardian_email}
        />
      </section>

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {info && (
        <div className="rounded-2xl border border-[#83CD2D]/30 bg-[#83CD2D]/5 p-4 text-sm text-[#5BA01F]">
          {info}
        </div>
      )}

      {(canRequestChange || changeRequests.length > 0) && (
        <ChangeRequestsPanel
          canCreate={canRequestChange}
          editHref={editHref}
          requests={changeRequests}
          replyDrafts={replyDrafts}
          replyingId={replyingChangeRequest}
          onReply={(id) => void handleChangeRequestReply(id)}
          onReplyDraftChange={(id, value) =>
            setReplyDrafts((prev) => ({ ...prev, [id]: value }))
          }
        />
      )}

      {showOptInBanner && (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">
            {t("renewalRequiredTitle")}
          </h2>
          <p className="mt-2 text-sm text-gray-700">
            {t("renewalRequiredText")}
          </p>
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <button
              type="button"
              onClick={() => void handleConfirmRenewal()}
              disabled={confirmingRenewal}
              className="h-10 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {confirmingRenewal ? t("confirming") : t("confirmEnrollment")}
            </button>
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {withdrawingChild === "__all__"
                ? t("declining")
                : t("declineEnrollment")}
            </button>
          </div>
        </section>
      )}
      {showOptOutBanner && (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">
            {t("autoRenewedTitle")}
          </h2>
          <p className="mt-2 text-sm text-gray-700">{t("autoRenewedText")}</p>
          <div className="mt-4">
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="h-10 w-full rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
            >
              {withdrawingChild === "__all__"
                ? t("unsubscribing")
                : t("unsubscribe")}
            </button>
          </div>
        </section>
      )}

      <section className="moto-content-surface space-y-4 rounded-xl border p-5 shadow-sm sm:p-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("enrollmentsEyebrow")}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-gray-900">
              {t("childrenLabel")}
            </h2>
          </div>
        </div>
        <ul className="space-y-3">
          {status.children.map((c) => {
            const canWithdraw =
              hasMultipleChildren && !TERMINAL_STATUSES.has(c.status);
            return (
              <li
                key={c.id}
                className="rounded-xl border border-gray-200 bg-white p-4"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="flex items-start gap-3">
                    <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
                      <UserRound className="h-5 w-5" aria-hidden="true" />
                    </span>
                    <div className="min-w-0">
                      <p className="font-semibold break-words text-gray-900">
                        {c.first_name} {c.last_name}
                      </p>
                      {c.status_reason && (
                        <p className="mt-1 text-sm text-gray-600">
                          {c.status_reason}
                        </p>
                      )}
                    </div>
                  </div>
                  <StatusPill
                    status={c.status}
                    label={t(`status.${c.status}`)}
                  />
                </div>
                <div className="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-3 sm:flex-row sm:items-center sm:justify-between">
                  <p className="text-sm text-gray-500">
                    {c.status === "approved"
                      ? t("childApprovedText")
                      : canWithdraw
                        ? t("childPendingText")
                        : t("childLockedText")}
                  </p>
                  {canWithdraw && (
                    <button
                      type="button"
                      onClick={() => void handleWithdraw(c.id)}
                      disabled={withdrawingChild === c.id}
                      className="h-9 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
                    >
                      {withdrawingChild === c.id
                        ? t("withdrawing")
                        : t("withdrawChild")}
                    </button>
                  )}
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <section className="moto-content-surface space-y-4 rounded-xl border p-5 shadow-sm sm:p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("contactLabel")}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-gray-900">
              {t("guardianTitle")}
            </h2>
          </div>
          {allEditable && !editing && (
            <button
              type="button"
              onClick={() => setEditing(true)}
              className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto"
            >
              <Pencil className="h-4 w-4" aria-hidden="true" />
              {t("edit")}
            </button>
          )}
        </div>
        {!editing ? (
          <dl className="grid gap-3 text-sm text-gray-700 sm:grid-cols-3">
            <div className="rounded-xl border border-gray-200 bg-white p-4">
              <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {t("nameLabel")}
              </dt>
              <dd className="mt-1 font-semibold text-gray-900">
                {status.guardian_first_name} {status.guardian_last_name}
              </dd>
            </div>
            <div className="rounded-xl border border-gray-200 bg-white p-4">
              <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {t("emailLabel")}
              </dt>
              <dd className="mt-1 font-semibold break-all text-gray-900">
                {status.guardian_email}
              </dd>
            </div>
            <div className="rounded-xl border border-gray-200 bg-white p-4">
              <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                {t("phoneLabel")}
              </dt>
              <dd className="mt-1 font-semibold text-gray-900">
                {status.guardian_phone ?? t("notProvided")}
              </dd>
            </div>
            {status.additional_guardians?.map((g: StatusGuardian, i) => (
              <div
                key={i}
                className="rounded-xl border border-gray-200 bg-white p-4 sm:col-span-3"
              >
                <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {t("additionalGuardiansLabel")}
                </dt>
                <dd className="mt-1 font-semibold text-gray-900">
                  {g.first_name} {g.last_name}
                </dd>
                <dd className="mt-1 text-sm text-gray-600">
                  {g.email && g.email.trim() !== ""
                    ? g.email
                    : t("notProvided")}
                  {" · "}
                  {g.phone && g.phone.trim() !== ""
                    ? g.phone
                    : t("notProvided")}
                </dd>
              </div>
            ))}
          </dl>
        ) : (
          <form onSubmit={handleEdit} className="space-y-4 text-sm">
            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block">
                <span className="text-sm font-semibold text-gray-700">
                  {t("firstNameLabel")}
                </span>
                <input
                  type="text"
                  value={editFirstName}
                  onChange={(e) => setEditFirstName(e.target.value)}
                  required
                  className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
                />
              </label>
              <label className="block">
                <span className="text-sm font-semibold text-gray-700">
                  {t("lastNameLabel")}
                </span>
                <input
                  type="text"
                  value={editLastName}
                  onChange={(e) => setEditLastName(e.target.value)}
                  required
                  className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
                />
              </label>
            </div>
            <label className="block">
              <span className="text-sm font-semibold text-gray-700">
                {t("phoneOptionalLabel")}
              </span>
              <input
                type="tel"
                value={editPhone}
                onChange={(e) => setEditPhone(e.target.value)}
                className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
              />
            </label>
            <p className="text-sm text-gray-500">{t("emailImmutable")}</p>
            <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <button
                type="submit"
                disabled={savingEdit}
                className="h-10 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
              >
                {savingEdit ? t("saving") : t("save")}
              </button>
              <button
                type="button"
                onClick={() => {
                  setEditing(false);
                  setEditFirstName(status.guardian_first_name);
                  setEditLastName(status.guardian_last_name);
                  setEditPhone(status.guardian_phone ?? "");
                }}
                disabled={savingEdit}
                className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
              >
                {t("cancel")}
              </button>
            </div>
          </form>
        )}
        {!allEditable && !editing && (
          <p className="text-sm text-gray-500">
            {canRequestChange
              ? t("editLockedWithChangeRequest")
              : t("editLocked")}
          </p>
        )}
      </section>

      {!allWithdrawn && (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex gap-3">
              <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
                <AlertTriangle className="h-5 w-5" aria-hidden="true" />
              </span>
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {t("manageEyebrow")}
                </p>
                <h2 className="mt-1 text-xl font-semibold text-gray-900">
                  {hasMultipleChildren
                    ? t("withdrawAllTitle")
                    : t("withdrawTitle")}
                </h2>
                <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                  {t("withdrawText")}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={() => void handleWithdraw()}
              disabled={withdrawingChild === "__all__"}
              className="h-10 w-full shrink-0 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:border-[#FF3130]/40 hover:bg-[#FF3130]/5 hover:text-[#9F1F1E] focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
            >
              {withdrawingChild === "__all__"
                ? t("withdrawing")
                : hasMultipleChildren
                  ? t("withdrawAllTitle")
                  : t("withdrawTitle")}
            </button>
          </div>
        </section>
      )}
    </div>
  );
}

function ChangeRequestsPanel({
  canCreate,
  editHref,
  onReply,
  onReplyDraftChange,
  replyDrafts,
  replyingId,
  requests,
}: {
  readonly canCreate: boolean;
  readonly editHref: string;
  readonly requests: EnrollmentChangeRequest[];
  readonly replyDrafts: Record<string, string>;
  readonly replyingId: string | null;
  readonly onReplyDraftChange: (id: string, value: string) => void;
  readonly onReply: (id: string) => void;
}) {
  const t = useTranslations("enrollmentStatus");
  const locale = useLocale();
  const diffCopy = useMemo<EnrollmentChangeRequestDiffCopy>(
    () => ({
      snapshotLabels: {
        additional_guardians: t("diff.snapshot.additional_guardians"),
        children: t("diff.snapshot.children"),
        consent_flags: t("diff.snapshot.consent_flags"),
        custom_data: t("diff.snapshot.custom_data"),
        guardian_email: t("diff.snapshot.guardian_email"),
        guardian_first_name: t("diff.snapshot.guardian_first_name"),
        guardian_last_name: t("diff.snapshot.guardian_last_name"),
        guardian_phone: t("diff.snapshot.guardian_phone"),
        phase_id: t("diff.snapshot.phase_id"),
      },
      childSnapshotLabels: {
        custom_data: t("diff.child.custom_data"),
        date_of_birth: t("diff.child.date_of_birth"),
        first_name: t("diff.child.first_name"),
        last_name: t("diff.child.last_name"),
        offering_days: t("diff.child.offering_days"),
        offering_ids: t("diff.child.offering_ids"),
        target_grade_level: t("diff.child.target_grade_level"),
      },
      guardianSnapshotLabels: {
        email: t("diff.guardian.email"),
        first_name: t("diff.guardian.first_name"),
        last_name: t("diff.guardian.last_name"),
        phone: t("diff.guardian.phone"),
      },
      recordValueLabels: {
        automatic_selected_days: t("diff.record.automatic_selected_days"),
        available_days: t("diff.record.available_days"),
        days_of_week_mode: t("diff.record.days_of_week_mode"),
        email: t("diff.record.email"),
        first_name: t("diff.record.first_name"),
        last_name: t("diff.record.last_name"),
        manual_selected_days: t("diff.record.manual_selected_days"),
        offering_id: t("diff.record.offering_id"),
        offering_ids: t("diff.record.offering_ids"),
        phone: t("diff.record.phone"),
        selected_days: t("diff.record.selected_days"),
      },
      weekdayLabels: {
        mon: t("diff.weekday.mon"),
        tue: t("diff.weekday.tue"),
        wed: t("diff.weekday.wed"),
        thu: t("diff.weekday.thu"),
        fri: t("diff.weekday.fri"),
        sat: t("diff.weekday.sat"),
        sun: t("diff.weekday.sun"),
      },
      childEntryLabel: t("diff.childEntry"),
      additionalGuardianEntryLabel: t("diff.additionalGuardianEntry"),
      emptyLabel: t("notProvided"),
      yesLabel: t("diff.yes"),
      noLabel: t("diff.no"),
    }),
    [t],
  );

  return (
    <section className="moto-content-surface space-y-4 rounded-xl border p-5 shadow-sm sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex gap-3">
          <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
            <MessageSquare className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("changeRequestsEyebrow")}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-gray-900">
              {t("changeRequestsTitle")}
            </h2>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
              {t("changeRequestsDescription")}
            </p>
          </div>
        </div>
        {canCreate ? (
          <Link
            href={editHref}
            className="inline-flex h-10 w-full shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            {t("requestChange")}
          </Link>
        ) : null}
      </div>

      {requests.length === 0 ? (
        <p className="rounded-xl border border-gray-200 bg-white p-4 text-sm text-gray-600">
          {t("changeRequestsEmpty")}
        </p>
      ) : (
        <ul className="space-y-3">
          {requests.map((request) => {
            const messages = (request.messages ?? []).filter(
              (message) => !message.internal_only,
            );
            return (
              <li
                key={request.id}
                className="rounded-xl border border-gray-200 bg-white p-4"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-semibold text-gray-900">
                      {t("changeRequestCreated", {
                        date: formatDateTime(request.created_at, locale),
                      })}
                    </p>
                    {request.parent_note ? (
                      <p className="mt-2 text-sm leading-6 text-gray-600">
                        {request.parent_note}
                      </p>
                    ) : null}
                  </div>
                  <ChangeRequestPill status={request.status} />
                </div>

                {messages.length > 0 ? (
                  <ol className="mt-4 space-y-2 border-t border-gray-100 pt-3">
                    {messages.map((message) => (
                      <li
                        key={message.id}
                        className="rounded-lg bg-gray-50 px-3 py-2"
                      >
                        <p className="text-xs font-semibold text-gray-500">
                          {t(`changeRequestAuthor.${message.author_type}`)} ·{" "}
                          {formatDateTime(message.created_at, locale)}
                        </p>
                        <p className="mt-1 text-sm leading-6 whitespace-pre-wrap text-gray-700">
                          {message.body}
                        </p>
                      </li>
                    ))}
                  </ol>
                ) : null}

                <div className="mt-4 border-t border-gray-100 pt-3">
                  <p className="mb-3 text-sm font-semibold text-gray-900">
                    {t("changeRequestDiffTitle")}
                  </p>
                  <EnrollmentChangeRequestDiff
                    baseSnapshot={request.base_snapshot}
                    proposedSnapshot={request.proposed_snapshot}
                    diff={request.diff}
                    beforeLabel={t("changeRequestBefore")}
                    afterLabel={t("changeRequestRequested")}
                    emptyLabel={t("notProvided")}
                    emptyMessage={t("changeRequestDiffEmpty")}
                    copy={diffCopy}
                  />
                </div>

                {request.admin_decision_note ? (
                  <div className="mt-3 rounded-lg bg-gray-50 px-3 py-2 text-sm leading-6 text-gray-700">
                    <span className="font-semibold">
                      {t("changeRequestDecisionNote")}{" "}
                    </span>
                    {request.admin_decision_note}
                  </div>
                ) : null}

                {request.status === "needs_parent_response" ? (
                  <form
                    className="mt-4 space-y-3 border-t border-gray-100 pt-3"
                    onSubmit={(event) => {
                      event.preventDefault();
                      onReply(request.id);
                    }}
                  >
                    <label className="block">
                      <span className="text-sm font-semibold text-gray-700">
                        {t("changeRequestReplyLabel")}
                      </span>
                      <textarea
                        value={replyDrafts[request.id] ?? ""}
                        onChange={(event) =>
                          onReplyDraftChange(request.id, event.target.value)
                        }
                        rows={3}
                        required
                        className="mt-2 w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none"
                      />
                    </label>
                    <button
                      type="submit"
                      disabled={replyingId === request.id}
                      className="inline-flex h-9 items-center justify-center rounded-lg bg-gray-900 px-3 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
                    >
                      {replyingId === request.id
                        ? t("changeRequestReplySending")
                        : t("changeRequestReplySubmit")}
                    </button>
                  </form>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}

function ChangeRequestPill({
  status,
}: {
  readonly status: EnrollmentChangeRequest["status"];
}) {
  const t = useTranslations("enrollmentStatus");
  const styles = CHANGE_REQUEST_STYLES[status];
  return (
    <span
      className="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold"
      style={{
        backgroundColor: styles.bg,
        color: styles.text,
      }}
    >
      <span
        className="h-2 w-2 rounded-full"
        style={{ backgroundColor: styles.dot }}
      />
      {t(`changeRequestStatus.${status}`)}
    </span>
  );
}

function formatDateTime(value: string, locale: string): string {
  return new Date(value).toLocaleString(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function StepNumber({ children }: { readonly children: React.ReactNode }) {
  return (
    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-gray-900 text-xs font-semibold text-white">
      {children}
    </span>
  );
}

function StatusPill({
  status,
  label,
}: {
  readonly status: StatusChild["status"];
  readonly label: string;
}) {
  const styles = STATUS_STYLES[status];
  return (
    <span
      className="inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold"
      style={{
        backgroundColor: styles.bg,
        color: styles.text,
      }}
    >
      <span
        className="h-2 w-2 rounded-full"
        style={{ backgroundColor: styles.dot }}
      />
      {label}
    </span>
  );
}

function StatusSummaryCard({
  icon,
  label,
  value,
}: {
  readonly icon: React.ReactNode;
  readonly label: string;
  readonly value: string;
}) {
  return (
    <div className="moto-content-surface rounded-xl border p-4 shadow-sm">
      <div className="flex items-center gap-3">
        <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm">
          {icon}
        </span>
        <div className="min-w-0">
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {label}
          </p>
          <p className="truncate text-sm font-semibold text-gray-900">
            {value}
          </p>
        </div>
      </div>
    </div>
  );
}
