"use client";

/**
 * Einverständnisse im Eltern-Portal (#3430): für jedes eigene Kind der Stand, die
 * Knöpfe, die der Server erlaubt, und der Nachweis nach der Antwort.
 *
 * Nichts ist vorausgewählt. Jede Antwort läuft über eine Rückfrage, die Kind,
 * Aktion und Fassung wiederholt. Verlangt die Schule das Passwort, steht das
 * Feld in dieser Rückfrage. Ein falsches Passwort kommt als 403 zurück und
 * meldet deshalb niemanden ab.
 */

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { FileText } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";

import { Alert } from "~/components/ui/alert";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import NavigationLink from "~/components/ui/navigation-link";
import { ConfirmationModal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { formatBerlinDate, formatChatDateTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { parentPath } from "~/lib/parent-url";
import {
  ParentApiError,
  type ParentAnnouncement,
  type ParentDeclaration,
  type ParentDeclarationAction,
  type ParentDeclarationChild,
  type ParentDeclarationState,
  declarationProofPath,
  submitDeclaration,
} from "~/lib/parent-api";

const logger = createLogger({ component: "ParentDeclaration" });

/** True when the item is an Einverständnis with its declaration block. */
export function isDeclarationItem(
  item: ParentAnnouncement,
): item is ParentAnnouncement & { declaration: ParentDeclaration } {
  return item.delivery_mode === "declaration" && item.declaration !== undefined;
}

/**
 * True while one of the guardian's own children still needs THIS guardian's
 * action: allowed to act, nothing given yet, the deadline has not passed, and
 * the child is still open or partly done. With signers "any" a decision by
 * another guardian settles the child, so no "Antwort nötig" then, although
 * the buttons stay (a decline still wins). Same rule as the unread count.
 */
export function isOpenDeclaration(item: ParentAnnouncement): boolean {
  if (!isDeclarationItem(item) || item.declaration.closed) return false;
  return item.declaration.children.some(
    (child) =>
      child.can_submit &&
      child.my_action === null &&
      (child.state === "open" || child.state === "partial"),
  );
}

const STATE_TONE: Record<ParentDeclarationState, StatusBadgeTone> = {
  open: "orange",
  partial: "blue",
  agreed: "green",
  declined: "red",
  revoked: "red",
  no_signer: "gray",
  expired: "gray",
};

/** States in which a child needs no further submission. */
const SETTLED_STATES = new Set<ParentDeclarationState>([
  "agreed",
  "declined",
  "revoked",
]);

/** The decision actions; "revoked" gets its own button and dialog. */
const DECISION_ACTIONS: readonly ParentDeclarationAction[] = [
  "agreed",
  "declined",
];

/**
 * What a failed submit does to the dialog. The text comes from the shared
 * error path (#2518); this only decides by code and status. Password and
 * rate-limit errors keep the dialog open so the guardian can try again;
 * everything that changed the Einverständnis itself closes it and reloads.
 * `uncertain`: no clear answer came back, the backend may have saved it.
 */
export function declarationErrorOutcome(err: unknown): {
  keepDialog: boolean;
  reload: boolean;
  uncertain: boolean;
  clearPassword: boolean;
} {
  const keep = { keepDialog: true, reload: false, uncertain: false };
  const close = { keepDialog: false, reload: true, uncertain: false };
  if (err instanceof ParentApiError) {
    if (err.status === 429) return { ...keep, clearPassword: false };
    switch (err.code) {
      case "care.declaration_password_required":
        return { ...keep, clearPassword: false };
      case "care.declaration_password_incorrect":
        return { ...keep, clearPassword: true };
      case "care.declaration_version_changed":
      case "care.declaration_closed":
      case "care.declaration_action_not_allowed":
      case "care.declaration_not_permitted":
      case "care.child_care_ended":
        return { ...close, clearPassword: false };
    }
    if (err.status === 404) return { ...close, clearPassword: false };
    if (err.status >= 500) {
      return { ...close, uncertain: true, clearPassword: false };
    }
    return { ...keep, clearPassword: false };
  }
  // No answer at all (network, a proxy that never finished): the backend may
  // still have saved it. Submitting is idempotent, so reload and let the card
  // show what is really stored instead of guessing.
  return { ...close, uncertain: true, clearPassword: false };
}

/**
 * The child's state right after this guardian's action, until the reload
 * brings the server's view. A decline or a withdrawal wins over consent;
 * with "all" the child stays partial while another person is still open.
 */
function stateAfter(
  declaration: ParentDeclaration,
  child: ParentDeclarationChild,
  action: ParentDeclarationAction,
): ParentDeclarationState {
  if (action === "declined" || action === "revoked") return action;
  if (declaration.signers === "any") return action;
  const othersOpen = child.other_signers.some((other) => other.action === null);
  return othersOpen ? "partial" : action;
}

interface PendingAction {
  readonly child: ParentDeclarationChild;
  readonly action: ParentDeclarationAction;
}

export function DeclarationSection({
  item,
  onUpdated,
  onReload,
  onBusyChange,
}: Readonly<{
  item: ParentAnnouncement & { declaration: ParentDeclaration };
  onUpdated: (id: string, patch: Partial<ParentAnnouncement>) => void;
  /** Reload the feed; the server has the authoritative state. */
  onReload?: (id: string) => void;
  /** Locks the surrounding dialog while a confirmation is open or running. */
  onBusyChange?: (busy: boolean) => void;
}>) {
  const t = useTranslations("parentDeclaration");
  const locale = useLocale();
  const declaration = item.declaration;

  const [pending, setPending] = useState<PendingAction | null>(null);
  const [password, setPassword] = useState("");
  const [saving, setSaving] = useState(false);
  // Both sit inside the open letter dialog, which lies above every toast:
  // the confirmation's own error while it is open, the section's after a
  // refusal closed it.
  const dialogRef = useRef<HTMLDivElement>(null);
  const dialogError = useApiFormError(dialogRef);
  const sectionError = useApiFormError();
  const [notice, setNotice] = useState<string | null>(null);

  // A different Einverständnis starts clean. A new version of the same one keeps
  // the notice: "the text was changed" must still be visible after the
  // reload brought the new version in.
  const { clear: clearSectionError } = sectionError;
  useEffect(() => {
    setNotice(null);
    clearSectionError();
    setPending(null);
  }, [item.id, clearSectionError]);

  useEffect(() => {
    onBusyChange?.(pending !== null || saving);
  }, [pending, saving, onBusyChange]);

  const open = (
    child: ParentDeclarationChild,
    action: ParentDeclarationAction,
  ) => {
    setPassword("");
    dialogError.clear();
    setNotice(null);
    sectionError.clear();
    setPending({ child, action });
  };

  const close = () => {
    if (saving) return;
    setPending(null);
    setPassword("");
    dialogError.clear();
  };

  const submit = async () => {
    if (!pending) return;
    if (declaration.requires_password && password === "") {
      dialogError.invalid(t("errors.passwordRequired"));
      return;
    }
    const { child, action } = pending;
    setSaving(true);
    dialogError.clear();
    try {
      const result = await submitDeclaration(item.id, {
        studentId: child.student_id,
        action,
        versionId: declaration.version.id,
        password: declaration.requires_password ? password : undefined,
      });
      const submittedAt = result.submission.submitted_at;
      onUpdated(item.id, {
        read: true,
        declaration: {
          ...declaration,
          children: declaration.children.map((candidate) =>
            candidate.student_id === child.student_id
              ? {
                  ...candidate,
                  my_action: action,
                  my_submitted_at: submittedAt,
                  state: stateAfter(declaration, candidate, action),
                  allowed_actions:
                    action === "agreed" && declaration.revocable
                      ? ["revoked"]
                      : [],
                }
              : candidate,
          ),
        },
      });
      setPending(null);
      setPassword("");
      setNotice(t("saved", { name: child.first_name }));
      window.dispatchEvent(new Event("parent-news-unread-refresh"));
      onReload?.(item.id);
    } catch (err: unknown) {
      const outcome = declarationErrorOutcome(err);
      logger.warn("parent_declaration_submit_failed", {
        status: err instanceof ParentApiError ? err.status : undefined,
        code: err instanceof ParentApiError ? err.code : undefined,
      });
      if (outcome.keepDialog) {
        if (outcome.clearPassword) setPassword("");
        void dialogError.show(err, {
          object: t("errorObjectAnswer"),
          retry: () => void submitRef.current(),
        });
      } else {
        setPending(null);
        setPassword("");
        // The dialog is gone, so there is nothing to retry from here; the
        // reload shows the stored state.
        void sectionError.show(err, {
          object: t("errorObjectAnswer"),
          messageSuffix: outcome.uncertain
            ? t("errors.uncertainReload")
            : undefined,
        });
      }
      if (outcome.reload) onReload?.(item.id);
    } finally {
      setSaving(false);
    }
  };

  // The retry sends the current password and choice, not the failed ones.
  const submitRef = useRef(submit);
  useLayoutEffect(() => {
    submitRef.current = submit;
  });

  const children = declaration.children;
  if (children.length === 0) return null;

  return (
    <section aria-labelledby={`declaration-${item.id}`} className="space-y-3">
      <div className="px-1">
        <h4
          id={`declaration-${item.id}`}
          className="text-base font-semibold text-gray-950"
        >
          {t("sectionTitle")}
        </h4>
        <p className="mt-0.5 text-sm leading-6 text-gray-600">
          {declaration.signers === "all" ? t("signersAll") : t("signersAny")}{" "}
          {t("nothingPreselected")}
        </p>
        <p className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-sm text-gray-600">
          <span>
            {t("version", { version: declaration.version.version_no })}
          </span>
          {declaration.closed ? (
            <span className="text-moto-red-strong font-semibold">
              {t("closed")}
            </span>
          ) : declaration.deadline ? (
            <span>
              {t("deadline", {
                date: formatBerlinDate(declaration.deadline, locale),
              })}
            </span>
          ) : null}
          {declaration.requires_password && <span>{t("passwordNote")}</span>}
        </p>
      </div>

      {notice && <Alert type="success" message={notice} />}
      <FormErrorAlert message={sectionError.error} />

      {children.map((child) => (
        <DeclarationChildCard
          key={child.student_id}
          item={item}
          declaration={declaration}
          child={child}
          busy={saving}
          onAction={(action) => open(child, action)}
        />
      ))}

      {pending && (
        <ConfirmationModal
          isOpen
          onClose={close}
          onConfirm={() => void submit()}
          title={
            pending.action === "revoked" ? t("revokeTitle") : t("confirmTitle")
          }
          confirmText={
            pending.action === "revoked"
              ? t("revokeConfirm")
              : t(`action.${pending.action}`)
          }
          cancelText={t("cancel")}
          closeLabel={t("cancel")}
          backdropLabel={t("cancel")}
          confirmVariant={pending.action === "revoked" ? "danger" : "primary"}
          isConfirmLoading={saving}
          loadingText={t("saving")}
          isDismissDisabled={saving}
          isBackdropDismissDisabled
          mobileSheet
        >
          <div
            ref={dialogRef}
            className="space-y-3 text-base leading-7 text-gray-800"
          >
            <p>
              {pending.action === "revoked"
                ? t("revokeBody", { name: pending.child.first_name })
                : t("confirmBody", {
                    name: pending.child.first_name,
                    action: t(`done.${pending.action}`),
                  })}
            </p>
            <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 rounded-xl bg-gray-50 px-4 py-3 text-sm">
              <dt className="text-gray-500">{t("confirmChild")}</dt>
              <dd className="font-semibold text-gray-900">
                {pending.child.first_name} {pending.child.last_name}
              </dd>
              <dt className="text-gray-500">{t("confirmAction")}</dt>
              <dd className="font-semibold text-gray-900">
                {t(`done.${pending.action}`)}
              </dd>
              <dt className="text-gray-500">{t("confirmVersion")}</dt>
              <dd className="font-semibold text-gray-900">
                {t("version", { version: declaration.version.version_no })}
              </dd>
            </dl>
            <p className="text-sm text-gray-600">{t("confirmRecorded")}</p>
            {declaration.requires_password && (
              <Input
                label={t("passwordLabel")}
                name="declaration-password"
                type="password"
                autoComplete="current-password"
                value={password}
                error={dialogError.fieldError("declaration-password")}
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void submit();
                }}
              />
            )}
            <FormErrorAlert message={dialogError.error} />
          </div>
        </ConfirmationModal>
      )}
    </section>
  );
}

function DeclarationChildCard({
  item,
  declaration,
  child,
  busy,
  onAction,
}: Readonly<{
  item: ParentAnnouncement;
  declaration: ParentDeclaration;
  child: ParentDeclarationChild;
  busy: boolean;
  onAction: (action: ParentDeclarationAction) => void;
}>) {
  const t = useTranslations("parentDeclaration");
  const locale = useLocale();
  const decisions = DECISION_ACTIONS.filter((action) =>
    child.allowed_actions.includes(action),
  );
  const canRevoke = child.allowed_actions.includes("revoked");
  // With "any", one submission settles the child. Another guardian did that
  // already: say so first, and keep the buttons as a quiet second option (a
  // decline still wins over a consent).
  const settledByOther =
    declaration.signers === "any" &&
    child.my_action === null &&
    SETTLED_STATES.has(child.state);
  const settler = settledByOther
    ? child.other_signers.find((other) => other.action !== null)
    : undefined;
  const name = `${child.first_name} ${child.last_name}`.trim();

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-center justify-between gap-3">
        <span className="min-w-0 truncate text-base font-semibold text-gray-900">
          {name}
        </span>
        <StatusBadge
          label={t(`state.${child.state}`)}
          tone={STATE_TONE[child.state] ?? "gray"}
        />
      </div>

      {!child.can_submit && (
        <p className="mt-2 text-sm leading-6 text-gray-600">
          {t("cannotSubmit", { name: child.first_name })}
        </p>
      )}

      {child.my_action && (
        <div className="mt-3 rounded-xl bg-gray-50 px-4 py-3">
          <p className="text-sm text-gray-800">
            {child.my_submitted_at
              ? t("myAction", {
                  action: t(`done.${child.my_action}`),
                  date: formatChatDateTime(child.my_submitted_at, locale),
                })
              : t("myActionNoDate", { action: t(`done.${child.my_action}`) })}
          </p>
          <NavigationLink
            href={parentPath(declarationProofPath(item.id, child.student_id))}
            className="text-moto-blue-strong focus-visible:ring-moto-blue mt-1 inline-flex min-h-11 items-center gap-2 text-sm font-semibold underline underline-offset-4 focus-visible:ring-2 focus-visible:outline-none"
          >
            <FileText className="h-4 w-4 shrink-0" aria-hidden="true" />
            {t("proof")}
          </NavigationLink>
        </div>
      )}

      {declaration.signers === "all" && child.other_signers.length > 0 && (
        <div className="mt-3">
          <p className="text-sm font-medium text-gray-700">
            {t("othersTitle")}
          </p>
          <ul className="mt-1 space-y-0.5 text-sm text-gray-600">
            {child.other_signers.map((other, index) => (
              <li
                // Other guardians carry no id in the feed; the list is fixed
                // per load, so position is stable.
                // eslint-disable-next-line react/no-array-index-key
                key={`${other.first_name}-${other.last_name}-${index}`}
              >
                {`${other.first_name} ${other.last_name}`.trim()}:{" "}
                {other.action ? t(`done.${other.action}`) : t("otherOpen")}
              </li>
            ))}
          </ul>
        </div>
      )}

      {settledByOther && (
        <p className="mt-3 rounded-xl bg-gray-50 px-4 py-3 text-sm leading-6 text-gray-800">
          {settler?.action
            ? settler.submitted_at
              ? t(`settledOnDate.${settler.action}`, {
                  name: `${settler.first_name} ${settler.last_name}`.trim(),
                  date: formatChatDateTime(settler.submitted_at, locale),
                })
              : t(`settled.${settler.action}`, {
                  name: `${settler.first_name} ${settler.last_name}`.trim(),
                })
            : t("settledUnknown", { name: child.first_name })}{" "}
          {t("oneIsEnough")}
        </p>
      )}

      {decisions.length > 0 && (
        <div className="mt-3">
          {settledByOther && (
            <p className="mb-1 text-sm leading-6 text-gray-700">
              {t("ownAnyway")}
            </p>
          )}
          <div className="space-y-1 text-sm leading-6 text-gray-700">
            {decisions.map((action) => (
              <p key={action}>
                {t(`explain.${action}`, { name: child.first_name })}
              </p>
            ))}
          </div>
          <div className="mt-3 flex flex-col gap-2 sm:flex-row">
            {decisions.map((action) => (
              <Button
                key={action}
                type="button"
                size={settledByOther ? "md" : "touch"}
                variant={
                  settledByOther || action === "declined"
                    ? "outline"
                    : "primary"
                }
                className="sm:flex-1"
                disabled={busy}
                onClick={() => onAction(action)}
              >
                {t(`action.${action}`)}
              </Button>
            ))}
          </div>
        </div>
      )}

      {canRevoke && (
        <div className="mt-3 border-t border-gray-100 pt-3">
          <p className="text-sm leading-6 text-gray-600">{t("revokeHint")}</p>
          <Button
            type="button"
            size="touch"
            variant="outline_danger"
            className="mt-2 w-full sm:w-auto"
            disabled={busy}
            onClick={() => onAction("revoked")}
          >
            {t("action.revoked")}
          </Button>
        </div>
      )}

      {child.can_submit &&
        decisions.length === 0 &&
        !child.my_action &&
        declaration.closed && (
          <p className="mt-2 text-sm leading-6 text-gray-600">
            {t("closedNoAction")}
          </p>
        )}
    </div>
  );
}
