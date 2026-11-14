"use client";

import {
  isOpenChangeRequestStatus,
  isTerminalChildStatus,
} from "~/lib/enrollment-status";
import { ENROLLMENT_CHANGE_REQUEST_STATUS_META } from "~/components/enrollment/enrollment-change-request-status";
import { EmptyState } from "~/components/ui/empty-state";
import { Alert } from "~/components/ui/alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { usePathname } from "next/navigation";
import dynamic from "next/dynamic";
import { AlertTriangle, Check, Clock, Pencil } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useLocale } from "next-intl";
import {
  confirmRenewal,
  fetchStatus,
  type RenewalConfirmation,
  getEnrollmentFormLabels,
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
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
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
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { ConfirmationModal } from "~/components/ui/modal";
import { ParentFamilyChildrenCard } from "~/components/parent/parent-family-children-card";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { BackLink } from "~/components/ui/back-link";
import { EnrollmentChangeRequestDiff } from "~/components/enrollment/enrollment-change-request-diff";
import type {
  EnrollmentChangeRequestDiffCopy,
  EnrollmentFormLabels,
} from "~/lib/enrollment-change-request-diff";
import {
  localizeFormFields,
  translatedText,
} from "~/lib/enrollment-translations";
import { StatusBadge } from "~/components/ui/status-badge";
import type { RenewalMode } from "~/components/enrollment/enrollment-renewal-banners";
import { ApiError } from "~/lib/api-error";
import { ChildStatusBadge } from "~/components/enrollment/child-status-badge";
import { PublicEnrollmentContentSkeleton } from "~/components/enrollment/public-enrollment-shell";
import { EnrollmentSignatureStatus } from "~/components/enrollment/enrollment-signature-status";
import { childrenConcept } from "~/lib/concepts/children";
import { parentConversationsConcept } from "~/lib/concepts/parentConversations";
import { IconTile } from "~/components/ui/icon-tile";
import {
  CareFacilityTermScope,
  useCareTermTranslations,
  useProvideCareFacilityTerm,
} from "~/lib/care-facility-term";

const logger = createLogger({ component: "EnrollmentStatusView" });

// Only a renewal shows its terms (#6456): the consent fields, the legal text
// dialog and its Markdown load when one is on the page, not with every
// status page.
const RenewalBanners = dynamic(
  () =>
    import("~/components/enrollment/enrollment-renewal-banners").then(
      (module) => module.RenewalBanners,
    ),
  { ssr: false },
);

interface Props {
  readonly token: string;
  readonly justSubmitted?: boolean;
  readonly duplicateWarning?: boolean;
  /**
   * Start page of the parents app, set only by the parents-portal route. The
   * public status page on the school host has no page above it.
   */
  readonly parentAppHref?: string;
}

/**
 * The status page of one enrollment. Its school comes from the status itself
 * (#6042): on the parents host the page has no school context, so the fixed
 * texts take the care-facility term the status names.
 */
export function EnrollmentStatusView(props: Props) {
  return (
    <CareFacilityTermScope>
      <EnrollmentStatusViewContent {...props} />
    </CareFacilityTermScope>
  );
}

function EnrollmentStatusViewContent({
  token,
  justSubmitted = false,
  duplicateWarning = false,
  parentAppHref,
}: Props) {
  const t = useCareTermTranslations("enrollmentStatus");
  // Loading depends on these texts, not on the asynchronously loaded care term.
  const loadErrorObject = t("errorObject");
  const requestsLoadErrorObject = t("errorObjectChangeRequests");
  const toast = useToast();
  const [status, setStatus] = useState<StatusResponse | null>(null);
  useProvideCareFacilityTerm(status?.care_facility_term);
  const [changeRequests, setChangeRequests] = useState<
    EnrollmentChangeRequest[]
  >([]);
  const [loading, setLoading] = useState(true);
  const [requestsReloadFailed, setRequestsReloadFailed] = useState(false);
  const [notFound, setNotFound] = useState(false);
  const latestRequest = useRef(0);
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
  const editingRef = useRef(editing);
  useLayoutEffect(() => {
    editingRef.current = editing;
  });

  // Puts a loaded status on the page; the first load and the reload after a
  // write share it.
  const applyStatus = useCallback(
    (result: StatusResponse, requestChanges: EnrollmentChangeRequest[]) => {
      setStatus(result);
      setChangeRequests(requestChanges);
      // A parent may have reopened the editor while the reload was pending.
      if (!editingRef.current) {
        setEditFirstName(result.guardian_first_name);
        setEditLastName(result.guardian_last_name);
        setEditPhone(result.guardian_phone ?? "");
      }
    },
    [],
  );

  const load = useCallback(async () => {
    const request = ++latestRequest.current;
    setLoading(true);
    clearLoadError();
    clearRequestsLoadError();
    setRequestsReloadFailed(false);
    setNotFound(false);
    try {
      const [result, requestChanges] = await Promise.all([
        fetchStatus(token),
        // A failed list must not read as "no change requests yet": the
        // panel shows the load error instead of its empty text.
        listEnrollmentChangeRequests(token).catch((err: unknown) => {
          if (request !== latestRequest.current) return null;
          logger.warn("change_requests_load_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          void showRequestsLoadError(err, {
            object: requestsLoadErrorObject,
            retry: () => void loadRef.current(),
          });
          return null;
        }),
      ]);
      if (request !== latestRequest.current) return;
      if (!result) {
        setNotFound(true);
        setChangeRequests([]);
        return;
      }
      applyStatus(result, requestChanges ?? []);
    } catch (err) {
      if (request !== latestRequest.current) return;
      logger.error("status_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: loadErrorObject,
        retry: () => void loadRef.current(),
      });
    } finally {
      if (request === latestRequest.current) setLoading(false);
    }
  }, [
    token,
    applyStatus,
    loadErrorObject,
    requestsLoadErrorObject,
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

  const invalidateRequest = useCallback(() => {
    latestRequest.current++;
  }, []);
  useEffect(() => {
    void load();
    return invalidateRequest;
  }, [load, invalidateRequest]);

  // The reload after a confirmed write (#6652) only reads and sets: the shown
  // status stays while it runs, and a failure rejects, so the hook reports it
  // as a load error that says the change is saved.
  const readStatus = useCallback(async () => {
    const request = ++latestRequest.current;
    try {
      const [statusResult, requestChanges] = await Promise.allSettled([
        fetchStatus(token),
        listEnrollmentChangeRequests(token),
      ]);
      if (request !== latestRequest.current) return;
      setLoading(false);
      setRequestsReloadFailed(false);
      if (statusResult.status === "rejected") throw statusResult.reason;
      const result = statusResult.value;
      if (!result) {
        setNotFound(true);
        setChangeRequests([]);
        return;
      }
      clearLoadError();
      clearRequestsLoadError();
      applyStatus(
        result,
        requestChanges.status === "fulfilled" ? requestChanges.value : [],
      );
      if (requestChanges.status === "rejected") {
        setRequestsReloadFailed(true);
        throw requestChanges.reason;
      }
    } catch (error) {
      if (request !== latestRequest.current) return;
      throw error;
    }
  }, [token, applyStatus, clearLoadError, clearRequestsLoadError]);
  const afterWrite = useRefreshAfterWrite(t("reloadObject"), readStatus, {
    savedSuffix: t("savedButStale"),
  });

  const replyToChangeRequest = async (changeRequestId: string) => {
    const body = (replyDrafts[changeRequestId] ?? "").trim();
    if (!body) return;
    setReplyingChangeRequest(changeRequestId);
    replyError.clear();
    setReplyErrorId(changeRequestId);
    try {
      await replyEnrollmentChangeRequest(token, changeRequestId, body);
    } catch (err) {
      logger.error("change_request_reply_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void replyError.show(err, {
        object: t("errorObjectReply"),
        // Sends the answer as it reads now, not the failed draft.
        retry: () => void replyRef.current(changeRequestId),
      });
      return;
    } finally {
      setReplyingChangeRequest(null);
    }
    setReplyDrafts((prev) => ({ ...prev, [changeRequestId]: "" }));
    toast.success(t("changeRequestReplySaved"));
    await afterWrite.refresh();
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
    } catch (err) {
      logger.error("status_edit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void editError.show(err, {
        object: t("errorObjectEdit"),
        retry: () => void saveEditRef.current(),
      });
      return;
    } finally {
      setSavingEdit(false);
    }
    toast.success(t("editSaved"));
    setEditing(false);
    await afterWrite.refresh();
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

  const handleConfirmRenewal = async (confirmation: RenewalConfirmation) => {
    if (!status) return;
    setConfirmingRenewal(true);
    let confirmed: number;
    try {
      confirmed = await confirmRenewal(token, confirmation);
    } catch (err) {
      logger.error("status_confirm_renewal_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The school published new terms while the page was open: the page
      // loads them, the parents read and tick them again (#6456).
      const changed =
        err instanceof ApiError &&
        err.code === "enrollment.renewal_terms_changed";
      void showActionError(err, {
        object: t("errorObject"),
        retry: changed
          ? undefined
          : () => void confirmRenewalRef.current(confirmation),
      });
      if (changed) await load();
      return;
    } finally {
      setConfirmingRenewal(false);
    }
    if (confirmed === 0) {
      toast.success(t("termsConfirmed"));
    } else {
      toast.success(
        confirmed === 1
          ? t("renewalConfirmedOne")
          : t("renewalConfirmedMany", { count: confirmed }),
      );
    }
    await afterWrite.refresh();
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
    } catch (err) {
      logger.error("status_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The error stays in the open dialog; its confirm button tries again,
      // so a retry still passes the confirmation.
      void withdrawError.show(err, { object: t("errorObject") });
      return;
    } finally {
      setWithdrawingChild(null);
    }
    setWithdrawTarget(null);
    toast.success(childID ? t("withdrawnChild") : t("withdrawnAll"));
    await afterWrite.refresh();
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
      <SectionCard>
        <EmptyState
          tone="error"
          icon={<AlertTriangle aria-hidden="true" />}
          titleAs="h1"
          title={t("invalidTitle")}
          description={t("invalidDescription")}
          className="py-4"
        />
      </SectionCard>
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
          reload: requestsReloadFailed ? null : afterWrite.error,
          requests: requestsReloadFailed
            ? (afterWrite.error ?? requestsLoadError)
            : requestsLoadError,
          edit: editError.error,
          editField: editError.fieldError,
          reply: replyError.error,
          replyId: replyErrorId,
        }}
        justSubmitted={justSubmitted}
        parentAppHref={parentAppHref}
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
  /** The reload after a confirmed write (#6652). */
  readonly reload: FormError | null;
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
  readonly parentAppHref: string | undefined;
  readonly replyDrafts: Record<string, string>;
  readonly replyingChangeRequest: string | null;
  readonly savingEdit: boolean;
  readonly status: StatusResponse;
  readonly token: string;
  readonly withdrawingChild: string | null;
  readonly onChangeRequestReply: (changeRequestId: string) => Promise<void>;
  readonly onConfirmRenewal: (
    confirmation: RenewalConfirmation,
  ) => Promise<void>;
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
  parentAppHref,
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
  const t = useCareTermTranslations("enrollmentStatus");
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
    isOpenChangeRequestStatus(request.status),
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
  const renewalMode: RenewalMode | null =
    pendingRenewalCount > 0
      ? "opt_in"
      : autoRenewedCount > 0
        ? "opt_out"
        : // A submitted, reviewed or waitlisted renewal still awaits a decision; its
          // terms stay open until the parents accept them (#6456).
          (status.renewal_terms?.blocks.length ?? 0) > 0 &&
            !status.renewal_terms?.confirmed_at &&
            status.children.some(
              (child) =>
                child.status === "submitted" ||
                child.status === "under_review" ||
                child.status === "waitlisted",
            )
          ? "submitted"
          : null;
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
        <Alert type="warning" message={t("duplicateWarning")} />
      ) : null}
      <EnrollmentStatusHero
        // Back to the parents app only for families who can use it: without
        // an account the link would end at a login that cannot work (#3742).
        backHref={
          parentAppHref &&
          parentAccess !== "invitation" &&
          parentAccess !== "contact_ogs"
            ? parentAppHref
            : null
        }
        allEditable={allEditable}
        canRequestChange={canRequestChange}
        editHref={editHref}
        justSubmitted={justSubmitted}
        submittedDate={submittedDate}
      />
      <EnrollmentStatusSummary status={status} submittedDate={submittedDate} />

      <LoadErrorAlert error={errors.load} />
      <LoadErrorAlert error={errors.reload} />

      {allLocked ? (
        <SectionCard title={t("lockedAllTitle")} bodyClassName="mt-2 space-y-2">
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
              <ButtonLink
                href={parentsHref}
                className="w-full sm:w-auto"
                tooltip={t("buttonHints.parentApp")}
              >
                {t("lockedAllAction")}
              </ButtonLink>
            </>
          )}
        </SectionCard>
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
          token={token}
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

      {renewalMode === null ? null : (
        <RenewalBanners
          key={status.renewal_terms?.terms_fingerprint}
          adjustHref={canRequestChange ? adjustHref : null}
          confirmingRenewal={confirmingRenewal}
          mode={renewalMode}
          terms={status.renewal_terms}
          withdrawingAll={withdrawingChild === "__all__"}
          onConfirmRenewal={onConfirmRenewal}
          onWithdraw={onWithdraw}
        />
      )}
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
      {/* Kinder in der Familie (#6335): only where the school asks. */}
      <ParentFamilyChildrenCard source={{ kind: "enrollment", token }} />
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
        editHref={editHref}
      />
      {status.signature ? (
        <EnrollmentSignatureStatus
          token={token}
          signature={status.signature}
          editable={allEditable}
        />
      ) : null}
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
  readonly backHref: string | null;
  readonly allEditable: boolean;
  readonly canRequestChange: boolean;
  readonly editHref: string;
  readonly justSubmitted: boolean;
  readonly submittedDate: string;
}

function EnrollmentStatusHero({
  backHref,
  allEditable,
  canRequestChange,
  editHref,
  justSubmitted,
  submittedDate,
}: EnrollmentStatusHeroProps) {
  const t = useCareTermTranslations("enrollmentStatus");
  const statusIconTone = justSubmitted ? "green" : "neutral";

  return (
    <SectionCard padding="none">
      <div className="grid lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="p-5 max-sm:p-4">
          {backHref ? (
            <BackLink
              href={backHref}
              label={t("backToParentApp")}
              className="mb-4"
            />
          ) : null}
          <IconTile
            size="hero"
            tone={statusIconTone}
            icon={
              justSubmitted ? (
                <Check aria-hidden="true" />
              ) : (
                <Clock aria-hidden="true" />
              )
            }
          />
          <h1 className="mt-6 max-w-2xl text-3xl font-bold tracking-tight text-wrap text-gray-900 sm:text-4xl">
            {justSubmitted ? t("submittedTitle") : t("statusTitle")}
          </h1>
          <p className="mt-4 max-w-2xl text-base leading-7 text-gray-600">
            {justSubmitted
              ? t("submittedDescription")
              : t("statusDescription", { date: submittedDate })}
          </p>
          {allEditable || canRequestChange ? (
            <div className="mt-6">
              <ButtonLink
                href={editHref}
                variant="primary"
                size="md"
                className="gap-2"
                tooltip={
                  allEditable
                    ? t("buttonHints.editFull")
                    : t("buttonHints.requestChange")
                }
              >
                <Pencil className="h-4 w-4" aria-hidden="true" />
                {allEditable ? t("editFull") : t("requestChange")}
              </ButtonLink>
            </div>
          ) : null}
        </div>
        <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 max-sm:p-4 lg:border-t-0 lg:border-l">
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
    </SectionCard>
  );
}

function EnrollmentStatusSummary({
  status,
  submittedDate,
}: Readonly<{ status: StatusResponse; submittedDate: string }>) {
  const t = useCareTermTranslations("enrollmentStatus");
  return (
    <section className="grid gap-3 sm:grid-cols-3">
      <StatCard
        variant="inline"
        icon={<Clock aria-hidden="true" />}
        label={t("receivedLabel")}
        value={submittedDate}
      />
      <StatCard
        variant="inline"
        icon={<MotoConceptIcon concept={childrenConcept} size={18} />}
        label={t("childrenLabel")}
        value={status.children.length}
      />
      <StatCard
        variant="inline"
        icon={
          <MotoConceptIcon concept={parentConversationsConcept} size={18} />
        }
        label={t("contactLabel")}
        value={
          <span className="block truncate" title={status.guardian_email}>
            {status.guardian_email}
          </span>
        }
      />
    </section>
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
  const t = useCareTermTranslations("enrollmentStatus");
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
  const t = useCareTermTranslations("enrollmentStatus");
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
  const t = useCareTermTranslations("enrollmentStatus");
  const handleWithdraw = () => {
    onWithdraw(child.id);
  };

  return (
    <li className="moto-content-surface rounded-xl border p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border">
            <MotoConceptIcon concept={childrenConcept} size={20} />
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
                  <ButtonLink
                    href={parentsHref}
                    size="sm"
                    tooltip={t("buttonHints.parentApp")}
                  >
                    {t("lockedAllAction")}
                  </ButtonLink>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
        <ChildStatusBadge
          status={child.status}
          label={t(`status.${child.status}`)}
        />
      </div>
      <div className="mt-4 flex flex-col gap-3 border-t border-gray-100 pt-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-gray-500">
          {t(childStatusTextKey(child, canWithdraw))}
        </p>
        {canWithdraw ? (
          <Button
            type="button"
            onClick={handleWithdraw}
            disabled={isWithdrawing}
            variant="surface"
            size="md"
            className="w-full sm:w-auto"
            tooltip={t("buttonHints.withdrawChild")}
          >
            {isWithdrawing ? t("withdrawing") : t("withdrawChild")}
          </Button>
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
  const t = useCareTermTranslations("enrollmentStatus");
  return (
    <section className="moto-content-surface space-y-4 rounded-xl border p-5 sm:p-6">
      <h2 className="text-xl font-semibold text-gray-900">
        {t("childrenLabel")}
      </h2>
      <ul className="space-y-3">
        {enrollments.map((child) => {
          const canWithdraw =
            hasMultipleChildren &&
            !justSubmitted &&
            !isTerminalChildStatus(child.status);
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
  /** The full form; a signed enrollment is edited only there (#4786). */
  readonly editHref: string;
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
  editHref,
}: GuardianSectionProps) {
  const t = useCareTermTranslations("enrollmentStatus");
  const handleCancel = () => {
    onCancelEdit();
    setEditFirstName(status.guardian_first_name);
    setEditLastName(status.guardian_last_name);
    setEditPhone(status.guardian_phone ?? "");
  };

  return (
    <section className="moto-content-surface space-y-4 rounded-xl border p-5 sm:p-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <h2 className="text-xl font-semibold text-gray-900">
          {t("guardianTitle")}
        </h2>
        {allEditable && !editing && status.signature_required ? (
          <ButtonLink
            href={editHref}
            variant="outline"
            size="md"
            className="w-full gap-2 sm:w-auto"
            tooltip={t("buttonHints.editFull")}
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            {t("edit")}
          </ButtonLink>
        ) : null}
        {allEditable && !editing && !status.signature_required ? (
          <Button
            type="button"
            onClick={() => setEditing(true)}
            variant="outline"
            size="md"
            className="w-full gap-2 sm:w-auto"
            tooltip={t("buttonHints.editGuardian")}
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            {t("edit")}
          </Button>
        ) : null}
      </div>
      {!editing ? (
        <GuardianDetails status={status} />
      ) : (
        <form onSubmit={onEdit} className="space-y-4 text-sm">
          <FormErrorAlert message={editError} />
          <div className="grid gap-4 sm:grid-cols-2">
            <Input
              id="status-edit-guardian_first_name"
              type="text"
              name="guardian_first_name"
              label={t("firstNameLabel")}
              value={editFirstName}
              onChange={(event) => setEditFirstName(event.target.value)}
              error={editFieldError("guardian_first_name")}
              required
            />
            <Input
              id="status-edit-guardian_last_name"
              type="text"
              name="guardian_last_name"
              label={t("lastNameLabel")}
              value={editLastName}
              onChange={(event) => setEditLastName(event.target.value)}
              error={editFieldError("guardian_last_name")}
              required
            />
          </div>
          <Input
            id="status-edit-guardian_phone"
            type="tel"
            name="guardian_phone"
            label={t("phoneOptionalLabel")}
            value={editPhone}
            onChange={(event) => setEditPhone(event.target.value)}
            error={editFieldError("guardian_phone")}
          />
          <p className="text-sm text-gray-500">{t("emailImmutable")}</p>
          <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
            <Button
              type="submit"
              disabled={savingEdit}
              size="md"
              tooltip={t("buttonHints.saveGuardian")}
            >
              {savingEdit ? t("saving") : t("save")}
            </Button>
            <Button
              type="button"
              onClick={handleCancel}
              disabled={savingEdit}
              variant="surface"
              size="md"
              noTooltip="Abbrechen verwirft nur die Eingaben im Formular."
            >
              {t("cancel")}
            </Button>
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
  const t = useCareTermTranslations("enrollmentStatus");
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
  const t = useCareTermTranslations("enrollmentStatus");
  if (allWithdrawn || justSubmitted) return null;
  const handleWithdraw = () => {
    onWithdraw();
  };

  return (
    <section className="moto-content-surface rounded-xl border p-5 sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex gap-3">
          <IconTile tone="red" icon={<AlertTriangle aria-hidden="true" />} />
          <div>
            <h2 className="text-xl font-semibold text-gray-900">
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
          className="w-full shrink-0 sm:w-auto"
          tooltip={
            hasMultipleChildren
              ? t("buttonHints.withdrawAll")
              : t("buttonHints.withdrawOne")
          }
        >
          {t(withdrawButtonLabelKey(withdrawingAll, hasMultipleChildren))}
        </Button>
      </div>
    </section>
  );
}

function ChangeRequestsPanel({
  token,
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
  readonly token: string;
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
  const t = useCareTermTranslations("enrollmentStatus");
  const locale = useLocale();
  const formLabels = useLocalizedFormLabels(token, requests.length > 0, locale);
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
      consentLabels: {
        agb: t("diff.consent.agb"),
        data_processing: t("diff.consent.data_processing"),
        email_contact: t("diff.consent.email_contact"),
        photo: t("diff.consent.photo"),
      },
      childSnapshotLabels: {
        custom_data: t("diff.child.custom_data"),
        date_of_birth: t("diff.child.date_of_birth"),
        first_name: t("diff.child.first_name"),
        holiday_care_wish: t("diff.child.holiday_care_wish"),
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
    <section className="moto-content-surface space-y-4 rounded-xl border p-5 sm:p-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex gap-3">
          <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border">
            <MotoConceptIcon concept={parentConversationsConcept} size={20} />
          </span>
          <div>
            <h2 className="text-xl font-semibold text-gray-900">
              {t("changeRequestsTitle")}
            </h2>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
              {t("changeRequestsDescription")}
            </p>
          </div>
        </div>
        {canCreate ? (
          <ButtonLink
            href={editHref}
            variant="primary"
            size="md"
            className="w-full shrink-0 gap-2 sm:w-auto"
            tooltip={t("buttonHints.requestChange")}
          >
            <Pencil className="h-4 w-4" aria-hidden="true" />
            {t("requestChange")}
          </ButtonLink>
        ) : null}
      </div>

      {loadError ? (
        <LoadErrorAlert error={loadError} />
      ) : requests.length === 0 ? (
        <p className="moto-content-surface rounded-xl border p-4 text-sm text-gray-600">
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
                className="moto-content-surface rounded-xl border p-4"
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
                      <InsetPanel as="li" padding="compact" key={message.id}>
                        <p className="text-xs font-semibold text-gray-500">
                          {t(`changeRequestAuthor.${message.author_type}`)} ·{" "}
                          {formatDateTime(message.created_at, locale)}
                        </p>
                        <p className="mt-1 text-sm leading-6 whitespace-pre-wrap text-gray-700">
                          {message.body}
                        </p>
                      </InsetPanel>
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
                    emptyMessage={t("changeRequestDiffEmpty")}
                    copy={diffCopy}
                    formLabels={formLabels}
                  />
                </div>

                {request.admin_decision_note ? (
                  <InsetPanel
                    padding="compact"
                    className="mt-3 text-sm leading-6 text-gray-700"
                  >
                    <span className="font-semibold">
                      {t("changeRequestDecisionNote")}{" "}
                    </span>
                    {request.admin_decision_note}
                  </InsetPanel>
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
                    <Textarea
                      id={`change-request-reply-${request.id}`}
                      name="body"
                      label={t("changeRequestReplyLabel")}
                      value={replyDrafts[request.id] ?? ""}
                      onChange={(event) =>
                        onReplyDraftChange(request.id, event.target.value)
                      }
                      rows={3}
                      required
                    />
                    <Button
                      type="submit"
                      disabled={replyingId === request.id}
                      size="md"
                      tooltip={t("buttonHints.replySubmit")}
                    >
                      {replyingId === request.id
                        ? t("changeRequestReplySending")
                        : t("changeRequestReplySubmit")}
                    </Button>
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

// The questions of the form behind the status link name the rows of a change
// request (#6142), in the reader's language where the school translated them.
function useLocalizedFormLabels(
  token: string,
  enabled: boolean,
  locale: string,
): EnrollmentFormLabels | null {
  const [labels, setLabels] = useState<EnrollmentFormLabels | null>(null);
  useLayoutEffect(() => {
    setLabels(null);
  }, [token, enabled]);
  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    getEnrollmentFormLabels(token)
      .then((loaded) => {
        if (!cancelled) setLabels(loaded);
      })
      .catch((err: unknown) => {
        // Stays silent on purpose: every changed value is still shown, only
        // named by its stored key, and the family has nothing to do.
        logger.warn("enrollment_form_labels_failed", {
          error: err instanceof Error ? err.message : "unknown",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [token, enabled]);
  return useMemo(
    () =>
      labels
        ? {
            fields: localizeFormFields(labels.fields, locale),
            legal_blocks: labels.legal_blocks.map((block) => ({
              ...block,
              title: translatedText(block, "title", block.title, locale),
            })),
          }
        : null,
    [labels, locale],
  );
}

function ChangeRequestPill({
  status,
}: {
  readonly status: EnrollmentChangeRequest["status"];
}) {
  const t = useCareTermTranslations("enrollmentStatus");
  return (
    <StatusBadge
      tone={ENROLLMENT_CHANGE_REQUEST_STATUS_META[status].tone}
      label={t(`changeRequestStatus.${status}`)}
    />
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
