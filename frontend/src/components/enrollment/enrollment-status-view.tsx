"use client";

import Link from "next/link";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { usePathname } from "next/navigation";
import { AlertTriangle, Check, Clock, Pencil } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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
import {
  useApiErrorDisplay,
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormError } from "~/components/ui/form-error";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { EnrollmentChangeRequestDiff } from "~/components/enrollment/enrollment-change-request-diff";
import type { EnrollmentChangeRequestDiffCopy } from "~/lib/enrollment-change-request-diff";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { PublicEnrollmentContentSkeleton } from "~/components/enrollment/public-enrollment-shell";

const logger = createLogger({ component: "EnrollmentStatusView" });

const STATUS_STYLES: Record<
  StatusChild["status"],
  { dot: string; text: string; bg: string }
> = {
  submitted: {
    dot: MOTO_COLOR_PALETTE.blue.base,
    text: MOTO_COLOR_PALETTE.neutral.strong,
    bg: MOTO_COLOR_PALETTE.neutral.soft,
  },
  under_review: {
    dot: MOTO_COLOR_PALETTE.blue.base,
    text: MOTO_COLOR_PALETTE.neutral.strong,
    bg: MOTO_COLOR_PALETTE.neutral.soft,
  },
  approved: {
    dot: MOTO_COLOR_PALETTE.green.base,
    text: MOTO_COLOR_PALETTE.green.strong,
    bg: MOTO_COLOR_PALETTE.green.soft,
  },
  waitlisted: {
    dot: MOTO_COLOR_PALETTE.orange.base,
    text: MOTO_COLOR_PALETTE.orange.strong,
    bg: MOTO_COLOR_PALETTE.orange.soft,
  },
  rejected: {
    dot: MOTO_COLOR_PALETTE.red.base,
    text: MOTO_COLOR_PALETTE.red.strong,
    bg: MOTO_COLOR_PALETTE.red.soft,
  },
  withdrawn: {
    dot: MOTO_COLOR_PALETTE.neutral.base,
    text: MOTO_COLOR_PALETTE.neutral.strong,
    bg: MOTO_COLOR_PALETTE.neutral.soft,
  },
  pending_renewal: {
    dot: MOTO_COLOR_PALETTE.orange.base,
    text: MOTO_COLOR_PALETTE.orange.strong,
    bg: MOTO_COLOR_PALETTE.orange.soft,
  },
  auto_renewed: {
    dot: MOTO_COLOR_PALETTE.blue.base,
    text: MOTO_COLOR_PALETTE.neutral.strong,
    bg: MOTO_COLOR_PALETTE.neutral.soft,
  },
  pending_admin_review: {
    dot: MOTO_COLOR_PALETTE.neutral.base,
    text: MOTO_COLOR_PALETTE.neutral.strong,
    bg: MOTO_COLOR_PALETTE.neutral.soft,
  },
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
  pending_review: {
    bg: MOTO_COLOR_PALETTE.blue.soft,
    dot: MOTO_COLOR_PALETTE.blue.base,
    text: MOTO_COLOR_PALETTE.blue.strong,
  },
  needs_parent_response: {
    bg: MOTO_COLOR_PALETTE.orange.soft,
    dot: MOTO_COLOR_PALETTE.orange.base,
    text: MOTO_COLOR_PALETTE.orange.strong,
  },
  approved: {
    bg: MOTO_COLOR_PALETTE.green.soft,
    dot: MOTO_COLOR_PALETTE.green.base,
    text: MOTO_COLOR_PALETTE.green.strong,
  },
  rejected: {
    bg: MOTO_COLOR_PALETTE.red.soft,
    dot: MOTO_COLOR_PALETTE.red.base,
    text: MOTO_COLOR_PALETTE.red.strong,
  },
  cancelled: {
    bg: MOTO_COLOR_PALETTE.neutral.soft,
    dot: MOTO_COLOR_PALETTE.neutral.light,
    text: MOTO_COLOR_PALETTE.neutral.strong,
  },
};

const OPEN_CHANGE_REQUEST_STATUSES = new Set<EnrollmentChangeRequest["status"]>(
  ["pending_review", "needs_parent_response"],
);

interface Props {
  readonly token: string;
  readonly justSubmitted?: boolean;
  readonly duplicateWarning?: boolean;
}

export function EnrollmentStatusView({
  token,
  justSubmitted = false,
  duplicateWarning = false,
}: Props) {
  const t = useTranslations("enrollmentStatus");
  const toast = useToast();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  const [changeRequests, setChangeRequests] = useState<
    EnrollmentChangeRequest[]
  >([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  // One display path for every failure (#2515): loads stay where the data
  // is missing, the two forms report in their own alert, the buttons
  // without a form (confirm, withdraw) in a toast.
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const {
    error: requestsLoadError,
    show: showRequestsLoadError,
    clear: clearRequestsLoadError,
  } = useApiLoadError();
  const editError = useApiFormError();
  const replyError = useApiFormError();
  const withdrawError = useApiFormError();
  const [replyErrorId, setReplyErrorId] = useState<string | null>(null);
  const { show: showActionError } = useApiErrorDisplay();

  const [editing, setEditing] = useState(false);
  const [savingEdit, setSavingEdit] = useState(false);
  const [withdrawingChild, setWithdrawingChild] = useState<string | null>(null);
  const [withdrawTarget, setWithdrawTarget] = useState<{
    childID?: string;
  } | null>(null);
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
    clearLoadError();
    clearRequestsLoadError();
    setNotFound(false);
    try {
      const [result, requestChanges] = await Promise.all([
        fetchStatus(token),
        // A failed list must not read as "no change requests yet": the
        // panel shows the load error instead of its empty text.
        listEnrollmentChangeRequests(token).catch((err: unknown) => {
          logger.warn("change_requests_load_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          void showRequestsLoadError(err, {
            object: t("errorObjectChangeRequests"),
            retry: () => void loadRef.current(),
          });
          return null;
        }),
      ]);
      if (!result) {
        setNotFound(true);
        setChangeRequests([]);
        return;
      }
      setStatus(result);
      setChangeRequests(requestChanges ?? []);
      setEditFirstName(result.guardian_first_name);
      setEditLastName(result.guardian_last_name);
      setEditPhone(result.guardian_phone ?? "");
    } catch (err) {
      logger.error("status_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: t("errorObject"),
        retry: () => void loadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [
    token,
    t,
    clearLoadError,
    clearRequestsLoadError,
    showLoadError,
    showRequestsLoadError,
  ]);
  // Retries call the latest load, not the one of the failed attempt.
  const loadRef = useRef(load);
  useLayoutEffect(() => {
    loadRef.current = load;
  });

  useEffect(() => {
    void load();
  }, [load]);

  const replyToChangeRequest = async (changeRequestId: string) => {
    const body = (replyDrafts[changeRequestId] ?? "").trim();
    if (!body) return;
    setReplyingChangeRequest(changeRequestId);
    replyError.clear();
    setReplyErrorId(changeRequestId);
    try {
      await replyEnrollmentChangeRequest(token, changeRequestId, body);
      setReplyDrafts((prev) => ({ ...prev, [changeRequestId]: "" }));
      toast.success(t("changeRequestReplySaved"));
      await load();
    } catch (err) {
      logger.error("change_request_reply_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void replyError.show(err, {
        object: t("errorObjectReply"),
        // Sends the answer as it reads now, not the failed draft.
        retry: () => void replyRef.current(changeRequestId),
      });
    } finally {
      setReplyingChangeRequest(null);
    }
  };
  const replyRef = useRef(replyToChangeRequest);
  useLayoutEffect(() => {
    replyRef.current = replyToChangeRequest;
  });

  const saveEdit = async () => {
    if (!status) return;
    setSavingEdit(true);
    editError.clear();
    try {
      await patchStatus(token, {
        guardian_first_name: editFirstName.trim(),
        guardian_last_name: editLastName.trim(),
        guardian_phone: editPhone.trim() || undefined,
      });
      toast.success(t("editSaved"));
      setEditing(false);
      await load();
    } catch (err) {
      logger.error("status_edit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void editError.show(err, {
        object: t("errorObjectEdit"),
        retry: () => void saveEditRef.current(),
      });
    } finally {
      setSavingEdit(false);
    }
  };
  const saveEditRef = useRef(saveEdit);
  useLayoutEffect(() => {
    saveEditRef.current = saveEdit;
  });
  const handleEdit = async (e: React.FormEvent) => {
    e.preventDefault();
    await saveEdit();
  };
  const cancelEdit = () => {
    editError.clear();
    setEditing(false);
  };

  const handleConfirmRenewal = async () => {
    if (!status) return;
    setConfirmingRenewal(true);
    try {
      const confirmed = await confirmRenewal(token);
      toast.success(
        confirmed === 1
          ? t("renewalConfirmedOne")
          : t("renewalConfirmedMany", { count: confirmed }),
      );
      await load();
    } catch (err) {
      logger.error("status_confirm_renewal_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showActionError(err, {
        object: t("errorObject"),
        retry: () => void confirmRenewalRef.current(),
      });
    } finally {
      setConfirmingRenewal(false);
    }
  };
  const confirmRenewalRef = useRef(handleConfirmRenewal);
  useLayoutEffect(() => {
    confirmRenewalRef.current = handleConfirmRenewal;
  });

  const requestWithdraw = (childID?: string) => {
    if (!status) return;
    withdrawError.clear();
    setWithdrawTarget({ childID });
  };

  const closeWithdraw = () => {
    withdrawError.clear();
    setWithdrawTarget(null);
  };

  const withdraw = async (childID: string | undefined) => {
    setWithdrawingChild(childID ?? "__all__");
    withdrawError.clear();
    try {
      await withdrawStatus(token, childID);
      setWithdrawTarget(null);
      toast.success(childID ? t("withdrawnChild") : t("withdrawnAll"));
      await load();
    } catch (err) {
      logger.error("status_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The error stays in the open dialog; its confirm button tries again,
      // so a retry still passes the confirmation.
      void withdrawError.show(err, { object: t("errorObject") });
    } finally {
      setWithdrawingChild(null);
    }
  };

  const confirmWithdraw = async () => {
    if (!status || !withdrawTarget) return;
    await withdraw(withdrawTarget.childID);
  };

  if (loading) {
    return (
      <>
        <span role="status" className="sr-only">
          {t("loading")}
        </span>
        <PublicEnrollmentContentSkeleton sections={2} />
      </>
    );
  }

  if (notFound) {
    return (
      <div className="moto-content-surface border-moto-red/30 bg-moto-red/5 rounded-xl border p-5 text-center shadow-sm sm:p-6">
        <h1 className="text-moto-red-strong text-xl font-semibold">
          {t("invalidTitle")}
        </h1>
        <p className="text-moto-red-strong mt-2 text-sm">
          {t("invalidDescription")}
        </p>
      </div>
    );
  }

  if (!status) {
    return <LoadErrorAlert error={loadError ?? t("loadFallback")} />;
  }

  return (
    <>
      <EnrollmentStatusContent
        changeRequests={changeRequests}
        confirmingRenewal={confirmingRenewal}
        duplicateWarning={duplicateWarning}
        editFirstName={editFirstName}
        editLastName={editLastName}
        editPhone={editPhone}
        editing={editing}
        errors={{
          load: loadError,
          requests: requestsLoadError,
          edit: editError.error,
          editField: editError.fieldError,
          reply: replyError.error,
          replyId: replyErrorId,
        }}
        justSubmitted={justSubmitted}
        replyDrafts={replyDrafts}
        replyingChangeRequest={replyingChangeRequest}
        savingEdit={savingEdit}
        status={status}
        token={token}
        withdrawingChild={withdrawingChild}
        onChangeRequestReply={replyToChangeRequest}
        onConfirmRenewal={handleConfirmRenewal}
        onEdit={handleEdit}
        onCancelEdit={cancelEdit}
        onWithdraw={requestWithdraw}
        setEditFirstName={setEditFirstName}
        setEditLastName={setEditLastName}
        setEditPhone={setEditPhone}
        setEditing={setEditing}
        setReplyDrafts={setReplyDrafts}
      />
      <ConfirmationModal
        isOpen={withdrawTarget !== null}
        onClose={closeWithdraw}
        onConfirm={confirmWithdraw}
        title={
          withdrawTarget?.childID
            ? t("withdrawModalTitleChild")
            : t("withdrawModalTitleAll")
        }
        confirmText={t("withdrawModalConfirm")}
        cancelText={t("cancel")}
        loadingText={t("withdrawing")}
        closeLabel={t("modalClose")}
        backdropLabel={t("modalBackdropClose")}
        isConfirmLoading={withdrawingChild !== null}
        isDismissDisabled={withdrawingChild !== null}
        confirmVariant="danger"
      >
        <p className="text-sm leading-6 text-gray-600">
          {withdrawTarget?.childID
            ? t("withdrawModalBodyChild")
            : t("withdrawModalBodyAll")}
        </p>
        <FormErrorAlert message={withdrawError.error} className="mt-3" />
      </ConfirmationModal>
    </>
  );
}

/** What failed on the status page, each shown where it belongs (#2515). */
interface StatusErrors {
  readonly load: FormError | null;
  readonly requests: FormError | null;
  readonly edit: FormError | null;
  readonly editField: (name: string) => string | undefined;
  readonly reply: FormError | null;
  /** The change request whose answer form shows `reply`. */
  readonly replyId: string | null;
}

interface EnrollmentStatusContentProps {
  readonly changeRequests: EnrollmentChangeRequest[];
  readonly confirmingRenewal: boolean;
  readonly duplicateWarning: boolean;
  readonly editFirstName: string;
  readonly editLastName: string;
  readonly editPhone: string;
  readonly editing: boolean;
  readonly errors: StatusErrors;
  readonly justSubmitted: boolean;
  readonly replyDrafts: Record<string, string>;
  readonly replyingChangeRequest: string | null;
  readonly savingEdit: boolean;
  readonly status: StatusResponse;
  readonly token: string;
  readonly withdrawingChild: string | null;
  readonly onChangeRequestReply: (changeRequestId: string) => Promise<void>;
  readonly onConfirmRenewal: () => Promise<void>;
  readonly onEdit: (event: React.FormEvent) => Promise<void>;
  readonly onCancelEdit: () => void;
  readonly onWithdraw: (childId?: string) => void;
  readonly setEditFirstName: (value: string) => void;
  readonly setEditLastName: (value: string) => void;
  readonly setEditPhone: (value: string) => void;
  readonly setEditing: (value: boolean) => void;
  readonly setReplyDrafts: React.Dispatch<
    React.SetStateAction<Record<string, string>>
  >;
}

function EnrollmentStatusContent({
  changeRequests,
  confirmingRenewal,
  duplicateWarning,
  editFirstName,
  editLastName,
  editPhone,
  editing,
  errors,
  justSubmitted,
  replyDrafts,
  replyingChangeRequest,
  savingEdit,
  status,
  token,
  withdrawingChild,
  onChangeRequestReply,
  onConfirmRenewal,
  onEdit,
  onCancelEdit,
  onWithdraw,
  setEditFirstName,
  setEditLastName,
  setEditPhone,
  setEditing,
  setReplyDrafts,
}: EnrollmentStatusContentProps) {
  const t = useTranslations("enrollmentStatus");
  const locale = useLocale();
  const pathname = usePathname();
  const submittedDate = new Date(status.submitted_at).toLocaleDateString(
    locale,
    {
      timeZone: "Europe/Berlin",
      day: "2-digit",
      month: "long",
      year: "numeric",
    },
  );
  const allEditable = status.edit_mode === "direct_edit";
  const allWithdrawn =
    !!status.withdrawn_at ||
    status.children.every((child) => child.status === "withdrawn");
  const hasMultipleChildren = status.children.length > 1;
  const hasOpenChangeRequest = changeRequests.some((request) =>
    OPEN_CHANGE_REQUEST_STATUSES.has(request.status),
  );
  const canRequestChange =
    status.edit_mode === "change_request" && !hasOpenChangeRequest;
  // Every child taken over into care: the status link stays readable and
  // points at the parents app instead of a change form (ADR 0003).
  const allLocked =
    status.children.length > 0 &&
    status.children.every((child) => child.locked);
  const anyLocked = status.children.some((child) => child.locked);
  const pendingRenewalCount = status.children.filter(
    (child) => child.status === "pending_renewal",
  ).length;
  const autoRenewedCount = status.children.filter(
    (child) => child.status === "auto_renewed",
  ).length;
  const showOptInBanner = pendingRenewalCount > 0;
  const showOptOutBanner = !showOptInBanner && autoRenewedCount > 0;
  const editHref = pathname?.startsWith("/parents")
    ? `/parents/anmeldung/status/${encodeURIComponent(token)}/edit`
    : `${pathname?.replace(/\/$/, "") ?? ""}/edit`;
  const adjustHref = pathname?.startsWith("/parents")
    ? `/parents/anmeldung/status/${encodeURIComponent(token)}/adjust`
    : `${pathname?.replace(/\/$/, "") ?? ""}/adjust`;
  const onParentsHost = pathname?.startsWith("/parents") ?? false;
  const parentsHref = onParentsHost ? "/" : "/parents";
  // The status page stays public on both hosts. Its status token, rather than
  // the host path, determines whether this family can use the parent app.
  const parentAccess = parentAppAccess(
    status.parent_portal_access,
    status.has_parent_account,
  );

  return (
    <div className="mx-auto max-w-5xl space-y-5 sm:space-y-6">
      {duplicateWarning ? (
        <div className="border-moto-orange/30 bg-moto-orange/10 text-moto-orange-strong rounded-xl border px-4 py-3 text-sm leading-6">
          {t("duplicateWarning")}
        </div>
      ) : null}
      <EnrollmentStatusHero
        allEditable={allEditable}
        canRequestChange={canRequestChange}
        editHref={editHref}
        justSubmitted={justSubmitted}
        submittedDate={submittedDate}
      />
      <EnrollmentStatusSummary status={status} submittedDate={submittedDate} />

      <LoadErrorAlert error={errors.load} />

      {allLocked ? (
        <section className="moto-content-surface space-y-2 rounded-2xl border p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">
            {t("lockedAllTitle")}
          </h2>
          <p className="text-sm leading-6 text-gray-600">
            {status.children.length > 1
              ? t("lockedAllBodyMany")
              : t("lockedAllBodyOne")}
          </p>
          {parentAccess === "invitation" ? (
            <NoParentAccountSteps withTitle />
          ) : parentAccess === "contact_ogs" ? (
            <ParentAppUnavailableHelp withTitle />
          ) : (
            <>
              {parentAccess === "unknown" ? (
                <p className="text-sm leading-6 text-gray-600">
                  {t("noAccessFallback")}
                </p>
              ) : null}
              <ButtonLink href={parentsHref} className="w-full sm:w-auto">
                {t("lockedAllAction")}
              </ButtonLink>
            </>
          )}
        </section>
      ) : null}
      {!allLocked &&
      anyLocked &&
      (parentAccess === "invitation" || parentAccess === "contact_ogs") ? (
        <SectionCard
          title={
            parentAccess === "invitation"
              ? t("noAccountTitle")
              : t("unavailableAccountTitle")
          }
          bodyClassName="space-y-2"
        >
          {parentAccess === "invitation" ? (
            <NoParentAccountSteps />
          ) : (
            <ParentAppUnavailableHelp />
          )}
        </SectionCard>
      ) : null}

      {canRequestChange || changeRequests.length > 0 || errors.requests ? (
        <ChangeRequestsPanel
          canCreate={canRequestChange}
          editHref={editHref}
          requests={changeRequests}
          loadError={errors.requests}
          replyError={errors.reply}
          replyErrorId={errors.replyId}
          replyDrafts={replyDrafts}
          replyingId={replyingChangeRequest}
          onReply={onChangeRequestReply}
          onReplyDraftChange={(id, value) =>
            setReplyDrafts((previous) => ({ ...previous, [id]: value }))
          }
        />
      ) : null}

      <RenewalBanners
        adjustHref={canRequestChange ? adjustHref : null}
        confirmingRenewal={confirmingRenewal}
        showOptInBanner={showOptInBanner}
        showOptOutBanner={showOptOutBanner}
        withdrawingAll={withdrawingChild === "__all__"}
        onConfirmRenewal={onConfirmRenewal}
        onWithdraw={onWithdraw}
      />
      <EnrollmentChildrenSection
        enrollments={status.children}
        hasMultipleChildren={hasMultipleChildren}
        justSubmitted={justSubmitted}
        parentsHref={
          parentAccess === "invitation" || parentAccess === "contact_ogs"
            ? null
            : parentsHref
        }
        withdrawingChild={withdrawingChild}
        onWithdraw={onWithdraw}
      />
      <GuardianSection
        allEditable={allEditable}
        canRequestChange={canRequestChange}
        editFirstName={editFirstName}
        editLastName={editLastName}
        editPhone={editPhone}
        editing={editing}
        savingEdit={savingEdit}
        status={status}
        editError={errors.edit}
        editFieldError={errors.editField}
        onEdit={onEdit}
        onCancelEdit={onCancelEdit}
        setEditFirstName={setEditFirstName}
        setEditLastName={setEditLastName}
        setEditPhone={setEditPhone}
        setEditing={setEditing}
      />
      {!allLocked ? (
        <WithdrawAllSection
          allWithdrawn={allWithdrawn}
          hasMultipleChildren={hasMultipleChildren}
          justSubmitted={justSubmitted}
          withdrawingAll={withdrawingChild === "__all__"}
          onWithdraw={onWithdraw}
        />
      ) : null}
    </div>
  );
}

interface EnrollmentStatusHeroProps {
  readonly allEditable: boolean;
  readonly canRequestChange: boolean;
  readonly editHref: string;
  readonly justSubmitted: boolean;
  readonly submittedDate: string;
}

function EnrollmentStatusHero({
  allEditable,
  canRequestChange,
  editHref,
  justSubmitted,
  submittedDate,
}: EnrollmentStatusHeroProps) {
  const t = useTranslations("enrollmentStatus");
  const statusIconClass = justSubmitted
    ? "bg-moto-green/15 text-moto-green-strong"
    : "moto-content-surface border text-gray-600 shadow-sm";

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="grid lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="p-5 sm:p-8 lg:p-10">
          <div
            className={`flex h-12 w-12 items-center justify-center rounded-xl sm:h-14 sm:w-14 ${statusIconClass}`}
          >
            {justSubmitted ? (
              <Check className="h-7 w-7" aria-hidden="true" />
            ) : (
              <Clock className="h-7 w-7" aria-hidden="true" />
            )}
          </div>
          <p className="text-moto-blue mt-6 text-sm font-semibold tracking-wide uppercase">
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
          {allEditable || canRequestChange ? (
            <div className="mt-6">
              <Link
                href={editHref}
                className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <Pencil className="h-4 w-4" aria-hidden="true" />
                {allEditable ? t("editFull") : t("requestChange")}
              </Link>
            </div>
          ) : null}
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
  );
}

function EnrollmentStatusSummary({
  status,
  submittedDate,
}: Readonly<{ status: StatusResponse; submittedDate: string }>) {
  const t = useTranslations("enrollmentStatus");
  return (
    <section className="grid gap-3 sm:grid-cols-3">
      <StatusSummaryCard
        icon={<Clock className="h-5 w-5" aria-hidden="true" />}
        label={t("receivedLabel")}
        value={submittedDate}
      />
      <StatusSummaryCard
        icon={<MotoConceptIcon concept="children" size={22} />}
        label={t("childrenLabel")}
        value={String(status.children.length)}
      />
      <StatusSummaryCard
        icon={<MotoConceptIcon concept="parentConversations" size={20} />}
        label={t("contactLabel")}
        value={status.guardian_email}
      />
    </section>
  );
}

interface RenewalBannersProps {
  /** Link to the reduced offerings/weekdays flow; null while a change request is already open (#2251). */
  readonly adjustHref: string | null;
  readonly confirmingRenewal: boolean;
  readonly showOptInBanner: boolean;
  readonly showOptOutBanner: boolean;
  readonly withdrawingAll: boolean;
  readonly onConfirmRenewal: () => Promise<void>;
  readonly onWithdraw: (childId?: string) => void;
}

function RenewalBanners({
  adjustHref,
  confirmingRenewal,
  showOptInBanner,
  showOptOutBanner,
  withdrawingAll,
  onConfirmRenewal,
  onWithdraw,
}: RenewalBannersProps) {
  const t = useTranslations("enrollmentStatus");
  const handleConfirmRenewal = async () => {
    await onConfirmRenewal();
  };
  const handleWithdraw = () => {
    onWithdraw();
  };

  return (
    <>
      {showOptInBanner ? (
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
              onClick={handleConfirmRenewal}
              disabled={confirmingRenewal}
              className="h-10 rounded-lg bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {confirmingRenewal ? t("confirming") : t("confirmEnrollment")}
            </button>
            {adjustHref && (
              <ButtonLink href={adjustHref} variant="surface" size="md">
                {t("renewalAdjust")}
              </ButtonLink>
            )}
            <button
              type="button"
              onClick={handleWithdraw}
              disabled={withdrawingAll}
              className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {withdrawingAll ? t("declining") : t("declineEnrollment")}
            </button>
          </div>
          <p className="mt-3 text-xs text-gray-500">{t("renewalAdjustHint")}</p>
        </section>
      ) : null}
      {showOptOutBanner ? (
        <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
          <h2 className="text-lg font-semibold text-gray-900">
            {t("autoRenewedTitle")}
          </h2>
          <p className="mt-2 text-sm text-gray-700">{t("autoRenewedText")}</p>
          <div className="mt-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            {adjustHref && (
              <ButtonLink href={adjustHref} variant="surface" size="md">
                {t("renewalAdjust")}
              </ButtonLink>
            )}
            <button
              type="button"
              onClick={handleWithdraw}
              disabled={withdrawingAll}
              className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {withdrawingAll ? t("unsubscribing") : t("unsubscribe")}
            </button>
          </div>
          <p className="mt-3 text-xs text-gray-500">{t("renewalAdjustHint")}</p>
        </section>
      ) : null}
    </>
  );
}

/**
 * Whether the family can log in to the parents app: "unknown" when the
 * backend could not check it, so the page keeps its link to the login.
 */
type ParentAppAccess = "account" | "invitation" | "contact_ogs" | "unknown";

function parentAppAccess(
  parentPortalAccess: StatusResponse["parent_portal_access"],
  hasParentAccount: boolean | undefined,
): ParentAppAccess {
  if (parentPortalAccess !== undefined) {
    if (parentPortalAccess === "account") return "account";
    if (parentPortalAccess === "invitation") return "invitation";
    if (parentPortalAccess === "contact_ogs") return "contact_ogs";
    return "unknown";
  }
  if (hasParentAccount === undefined) return "unknown";
  return hasParentAccount ? "account" : "invitation";
}

/**
 * The way into the parents app for a family without an account: the login
 * cannot work yet, and a password reset sends nothing without an account, so
 * the page points at the invitation instead (#3742).
 */
function NoParentAccountSteps({ withTitle = false }: { withTitle?: boolean }) {
  const t = useTranslations("enrollmentStatus");
  return (
    <div className="space-y-2 text-sm leading-6 text-gray-600">
      {withTitle ? (
        <h3 className="pt-2 font-semibold text-gray-900">
          {t("noAccountTitle")}
        </h3>
      ) : null}
      <p>{t("noAccountIntro")}</p>
      <ol className="list-decimal space-y-1 pl-5">
        <li>{t("noAccountStepOpen")}</li>
        <li>{t("noAccountStepAccept")}</li>
        <li>{t("noAccountStepPassword")}</li>
      </ol>
      <p>{t("noAccountHelp")}</p>
    </div>
  );
}

function ParentAppUnavailableHelp({
  withTitle = false,
}: {
  withTitle?: boolean;
}) {
  const t = useTranslations("enrollmentStatus");
  return (
    <div className="space-y-2 text-sm leading-6 text-gray-600">
      {withTitle ? (
        <h3 className="pt-2 font-semibold text-gray-900">
          {t("unavailableAccountTitle")}
        </h3>
      ) : null}
      <p>{t("unavailableAccountHelp")}</p>
    </div>
  );
}

function childStatusTextKey(
  child: StatusChild,
  canWithdraw: boolean,
): "childApprovedText" | "childPendingText" | "childLockedText" {
  if (child.status === "approved") return "childApprovedText";
  if (canWithdraw) return "childPendingText";
  return "childLockedText";
}

interface EnrollmentChildRowProps {
  readonly canWithdraw: boolean;
  readonly child: StatusChild;
  readonly isWithdrawing: boolean;
  readonly onWithdraw: (childId?: string) => void;
  /** Null when the status page cannot safely direct the family to login. */
  readonly parentsHref: string | null;
}

function EnrollmentChildRow({
  canWithdraw,
  child,
  isWithdrawing,
  onWithdraw,
  parentsHref,
}: EnrollmentChildRowProps) {
  const t = useTranslations("enrollmentStatus");
  const handleWithdraw = () => {
    onWithdraw(child.id);
  };

  return (
    <li className="moto-content-surface rounded-xl border p-4 shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border shadow-sm">
            <MotoConceptIcon concept="children" size={20} />
          </span>
          <div className="min-w-0">
            <p className="font-semibold break-words text-gray-900">
              {child.first_name} {child.last_name}
            </p>
            {child.status_reason ? (
              <p className="mt-1 text-sm text-gray-600">
                {child.status_reason}
              </p>
            ) : null}
            {child.locked ? (
              <div className="mt-1 flex flex-wrap items-center gap-2 text-sm text-gray-600">
                <span>{t("lockedChildHint")}</span>
                {parentsHref ? (
                  <ButtonLink href={parentsHref} size="sm">
                    {t("lockedAllAction")}
                  </ButtonLink>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
        <StatusPill status={child.status} label={t(`status.${child.status}`)} />
      </div>
      <div className="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-gray-500">
          {t(childStatusTextKey(child, canWithdraw))}
        </p>
        {canWithdraw ? (
          <button
            type="button"
            onClick={handleWithdraw}
            disabled={isWithdrawing}
            className="h-9 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50 sm:w-auto"
          >
            {isWithdrawing ? t("withdrawing") : t("withdrawChild")}
          </button>
        ) : null}
      </div>
    </li>
  );
}

function EnrollmentChildrenSection({
  enrollments,
  hasMultipleChildren,
  justSubmitted,
  withdrawingChild,
  onWithdraw,
  parentsHref,
}: Readonly<{
  enrollments: StatusChild[];
  hasMultipleChildren: boolean;
  justSubmitted: boolean;
  withdrawingChild: string | null;
  onWithdraw: (childId?: string) => void;
  parentsHref: string | null;
}>) {
  const t = useTranslations("enrollmentStatus");
  return (
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
        {enrollments.map((child) => {
          const canWithdraw =
            hasMultipleChildren &&
            !justSubmitted &&
            !TERMINAL_STATUSES.has(child.status);
          return (
            <EnrollmentChildRow
              key={child.id}
              canWithdraw={canWithdraw}
              child={child}
              isWithdrawing={withdrawingChild === child.id}
              onWithdraw={onWithdraw}
              parentsHref={parentsHref}
            />
          );
        })}
      </ul>
    </section>
  );
}

interface GuardianSectionProps {
  readonly allEditable: boolean;
  readonly canRequestChange: boolean;
  readonly editFirstName: string;
  readonly editLastName: string;
  readonly editPhone: string;
  readonly editing: boolean;
  readonly savingEdit: boolean;
  readonly status: StatusResponse;
  readonly editError: FormError | null;
  readonly editFieldError: (name: string) => string | undefined;
  readonly onEdit: (event: React.FormEvent) => Promise<void>;
  readonly onCancelEdit: () => void;
  readonly setEditFirstName: (value: string) => void;
  readonly setEditLastName: (value: string) => void;
  readonly setEditPhone: (value: string) => void;
  readonly setEditing: (value: boolean) => void;
}

function GuardianSection({
  allEditable,
  canRequestChange,
  editFirstName,
  editLastName,
  editPhone,
  editing,
  savingEdit,
  status,
  editError,
  editFieldError,
  onEdit,
  onCancelEdit,
  setEditFirstName,
  setEditLastName,
  setEditPhone,
  setEditing,
}: GuardianSectionProps) {
  const t = useTranslations("enrollmentStatus");
  const handleCancel = () => {
    onCancelEdit();
    setEditFirstName(status.guardian_first_name);
    setEditLastName(status.guardian_last_name);
    setEditPhone(status.guardian_phone ?? "");
  };

  return (
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
        {allEditable && !editing ? (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className="inline-flex h-9 w-full items-center justify-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto"
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            {t("edit")}
          </button>
        ) : null}
      </div>
      {!editing ? (
        <GuardianDetails status={status} />
      ) : (
        <form onSubmit={onEdit} className="space-y-4 text-sm">
          <FormErrorAlert message={editError} />
          <div className="grid gap-4 sm:grid-cols-2">
            <div>
              <label className="block">
                <span className="text-sm font-semibold text-gray-700">
                  {t("firstNameLabel")}
                </span>
                <input
                  type="text"
                  name="guardian_first_name"
                  value={editFirstName}
                  onChange={(event) => setEditFirstName(event.target.value)}
                  aria-describedby={
                    editFieldError("guardian_first_name")
                      ? "status-edit-guardian_first_name-error"
                      : undefined
                  }
                  aria-invalid={
                    editFieldError("guardian_first_name") ? true : undefined
                  }
                  required
                  className={`mt-2 h-10 w-full rounded-lg border ${editFieldError("guardian_first_name") ? "border-moto-red" : "border-gray-200"} bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none`}
                />
              </label>
              {editFieldError("guardian_first_name") ? (
                <p
                  id="status-edit-guardian_first_name-error"
                  className="text-moto-red mt-1 text-xs"
                >
                  {editFieldError("guardian_first_name")}
                </p>
              ) : null}
            </div>
            <div>
              <label className="block">
                <span className="text-sm font-semibold text-gray-700">
                  {t("lastNameLabel")}
                </span>
                <input
                  type="text"
                  name="guardian_last_name"
                  value={editLastName}
                  onChange={(event) => setEditLastName(event.target.value)}
                  aria-describedby={
                    editFieldError("guardian_last_name")
                      ? "status-edit-guardian_last_name-error"
                      : undefined
                  }
                  aria-invalid={
                    editFieldError("guardian_last_name") ? true : undefined
                  }
                  required
                  className={`mt-2 h-10 w-full rounded-lg border ${editFieldError("guardian_last_name") ? "border-moto-red" : "border-gray-200"} bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none`}
                />
              </label>
              {editFieldError("guardian_last_name") ? (
                <p
                  id="status-edit-guardian_last_name-error"
                  className="text-moto-red mt-1 text-xs"
                >
                  {editFieldError("guardian_last_name")}
                </p>
              ) : null}
            </div>
          </div>
          <div>
            <label className="block">
              <span className="text-sm font-semibold text-gray-700">
                {t("phoneOptionalLabel")}
              </span>
              <input
                type="tel"
                name="guardian_phone"
                value={editPhone}
                onChange={(event) => setEditPhone(event.target.value)}
                aria-describedby={
                  editFieldError("guardian_phone")
                    ? "status-edit-guardian_phone-error"
                    : undefined
                }
                aria-invalid={
                  editFieldError("guardian_phone") ? true : undefined
                }
                className={`mt-2 h-10 w-full rounded-lg border ${editFieldError("guardian_phone") ? "border-moto-red" : "border-gray-200"} bg-white px-3 text-sm shadow-sm focus:border-gray-400 focus-visible:ring-2 focus-visible:ring-gray-300 focus-visible:outline-none`}
              />
            </label>
            {editFieldError("guardian_phone") ? (
              <p
                id="status-edit-guardian_phone-error"
                className="text-moto-red mt-1 text-xs"
              >
                {editFieldError("guardian_phone")}
              </p>
            ) : null}
          </div>
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
              onClick={handleCancel}
              disabled={savingEdit}
              className="h-10 rounded-lg border border-gray-200 bg-white px-4 text-sm font-semibold text-gray-700 shadow-sm hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:opacity-50"
            >
              {t("cancel")}
            </button>
          </div>
        </form>
      )}
      {!allEditable && !editing ? (
        <p className="text-sm text-gray-500">
          {canRequestChange
            ? t("editLockedWithChangeRequest")
            : t("editLocked")}
        </p>
      ) : null}
    </section>
  );
}

function GuardianDetails({ status }: Readonly<{ status: StatusResponse }>) {
  const t = useTranslations("enrollmentStatus");
  return (
    <DataGrid>
      <DataField label={t("nameLabel")}>
        {status.guardian_first_name} {status.guardian_last_name}
      </DataField>
      <DataField label={t("emailLabel")}>
        <span className="break-all">{status.guardian_email}</span>
      </DataField>
      <DataField label={t("phoneLabel")}>
        {status.guardian_phone ?? t("notProvided")}
      </DataField>
      {status.additional_guardians?.map((guardian: StatusGuardian) => (
        <DataField
          key={`${guardian.first_name}-${guardian.last_name}-${guardian.email ?? ""}-${guardian.phone ?? ""}`}
          label={t("additionalGuardiansLabel")}
          fullWidth
        >
          {guardian.first_name} {guardian.last_name}
          <span className="block font-normal text-gray-600">
            {guardian.email && guardian.email.trim() !== ""
              ? guardian.email
              : t("notProvided")}
            {" · "}
            {guardian.phone && guardian.phone.trim() !== ""
              ? guardian.phone
              : t("notProvided")}
          </span>
        </DataField>
      ))}
    </DataGrid>
  );
}

function withdrawButtonLabelKey(
  withdrawingAll: boolean,
  hasMultipleChildren: boolean,
): "withdrawing" | "withdrawAllTitle" | "withdrawTitle" {
  if (withdrawingAll) return "withdrawing";
  if (hasMultipleChildren) return "withdrawAllTitle";
  return "withdrawTitle";
}

function WithdrawAllSection({
  allWithdrawn,
  hasMultipleChildren,
  justSubmitted,
  withdrawingAll,
  onWithdraw,
}: Readonly<{
  allWithdrawn: boolean;
  hasMultipleChildren: boolean;
  justSubmitted: boolean;
  withdrawingAll: boolean;
  onWithdraw: (childId?: string) => void;
}>) {
  const t = useTranslations("enrollmentStatus");
  if (allWithdrawn || justSubmitted) return null;
  const handleWithdraw = () => {
    onWithdraw();
  };

  return (
    <section className="moto-content-surface rounded-xl border p-5 shadow-sm sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex gap-3">
          <span className="border-moto-red/30 bg-moto-red/5 text-moto-red flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border shadow-sm">
            <AlertTriangle className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("manageEyebrow")}
            </p>
            <h2 className="mt-1 text-xl font-semibold text-gray-900">
              {hasMultipleChildren ? t("withdrawAllTitle") : t("withdrawTitle")}
            </h2>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
              {t("withdrawText")}
            </p>
          </div>
        </div>
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          onClick={handleWithdraw}
          disabled={withdrawingAll}
          className="w-full shrink-0 font-semibold sm:w-auto"
        >
          {t(withdrawButtonLabelKey(withdrawingAll, hasMultipleChildren))}
        </Button>
      </div>
    </section>
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
  loadError,
  replyError,
  replyErrorId,
}: {
  readonly canCreate: boolean;
  readonly editHref: string;
  readonly requests: EnrollmentChangeRequest[];
  readonly loadError: FormError | null;
  readonly replyError: FormError | null;
  readonly replyErrorId: string | null;
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
        target_school_class: t("diff.child.target_school_class"),
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
          <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border shadow-sm">
            <MotoConceptIcon concept="parentConversations" size={20} />
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

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : requests.length === 0 ? (
        <p className="moto-content-surface rounded-xl border p-4 text-sm text-gray-600 shadow-sm">
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
                className="moto-content-surface rounded-xl border p-4 shadow-sm"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-sm font-semibold text-gray-900">
                      {request.origin === "admin"
                        ? t("changeRequestAdminCorrection", {
                            date: formatDateTime(request.created_at, locale),
                          })
                        : t("changeRequestCreated", {
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
                    {replyErrorId === request.id ? (
                      <FormErrorAlert message={replyError} />
                    ) : null}
                    <label className="block">
                      <span className="text-sm font-semibold text-gray-700">
                        {t("changeRequestReplyLabel")}
                      </span>
                      <textarea
                        name="body"
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
    timeZone: "Europe/Berlin",
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
