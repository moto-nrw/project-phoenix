"use client";

import { useCallback, useState } from "react";
import { getRelativeTime } from "~/lib/format-utils";
import { createLogger } from "~/lib/logger";
import { ConfirmationModal } from "~/components/ui/modal";

const logger = createLogger({ component: "SoftDeleteShared" });

export interface SoftDeletable {
  readonly id: string;
  readonly name: string;
  readonly deletedAt?: string | null;
}

interface UseSoftDeletableOptions<T extends SoftDeletable> {
  readonly softDeleteFn: (id: string) => Promise<unknown>;
  readonly restoreFn: (id: string) => Promise<unknown>;
  readonly mutateList: () => Promise<unknown>;
  readonly errorMessages: {
    readonly softDelete: string;
    readonly restore: string;
  };
  readonly logEventPrefix: string;
  readonly onAfterSoftDelete?: (target: T) => void | Promise<void>;
  readonly onAfterRestore?: (target: T) => void | Promise<void>;
}

export interface SoftDeletableState<T extends SoftDeletable> {
  readonly deleteTarget: T | null;
  readonly setDeleteTarget: (target: T | null) => void;
  readonly deleteConfirmInput: string;
  readonly setDeleteConfirmInput: (value: string) => void;
  readonly restoreTarget: T | null;
  readonly setRestoreTarget: (target: T | null) => void;
  readonly isProcessing: boolean;
  readonly softDeleteError: string;
  readonly showTrash: boolean;
  readonly setShowTrash: (value: boolean) => void;
  readonly handleSoftDelete: () => Promise<void>;
  readonly handleRestore: () => Promise<void>;
}

export function useSoftDeletable<T extends SoftDeletable>(
  options: UseSoftDeletableOptions<T>,
): SoftDeletableState<T> {
  const {
    softDeleteFn,
    restoreFn,
    mutateList,
    errorMessages,
    logEventPrefix,
    onAfterSoftDelete,
    onAfterRestore,
  } = options;

  const [deleteTarget, setDeleteTargetRaw] = useState<T | null>(null);
  const [deleteConfirmInput, setDeleteConfirmInput] = useState("");
  const [restoreTarget, setRestoreTargetRaw] = useState<T | null>(null);
  const [isProcessing, setIsProcessing] = useState(false);
  const [softDeleteError, setSoftDeleteError] = useState("");
  const [showTrash, setShowTrash] = useState(false);

  const setDeleteTarget = useCallback((target: T | null) => {
    setDeleteTargetRaw(target);
    setDeleteConfirmInput("");
    setSoftDeleteError("");
  }, []);

  const setRestoreTarget = useCallback((target: T | null) => {
    setRestoreTargetRaw(target);
    setSoftDeleteError("");
  }, []);

  const execute = useCallback(
    async (
      target: T | null,
      action: (id: string) => Promise<unknown>,
      errorMsg: string,
      logEvent: string,
      onSuccess: () => void | Promise<void>,
    ) => {
      if (!target) return;
      setIsProcessing(true);
      setSoftDeleteError("");
      try {
        await action(target.id);
        await onSuccess();
        await mutateList().catch(() => {});
      } catch (error) {
        setSoftDeleteError(errorMsg);
        logger.error(logEvent, {
          error: error instanceof Error ? error.message : String(error),
        });
      } finally {
        setIsProcessing(false);
      }
    },
    [mutateList],
  );

  const handleSoftDelete = useCallback(async () => {
    const target = deleteTarget;
    await execute(
      target,
      softDeleteFn,
      errorMessages.softDelete,
      `${logEventPrefix}_soft_delete_failed`,
      async () => {
        setDeleteTarget(null);
        if (target && onAfterSoftDelete) {
          await onAfterSoftDelete(target);
        }
      },
    );
  }, [
    deleteTarget,
    execute,
    softDeleteFn,
    errorMessages.softDelete,
    logEventPrefix,
    setDeleteTarget,
    onAfterSoftDelete,
  ]);

  const handleRestore = useCallback(async () => {
    const target = restoreTarget;
    await execute(
      target,
      restoreFn,
      errorMessages.restore,
      `${logEventPrefix}_restore_failed`,
      async () => {
        setRestoreTarget(null);
        setShowTrash(false);
        if (target && onAfterRestore) {
          await onAfterRestore(target);
        }
      },
    );
  }, [
    restoreTarget,
    execute,
    restoreFn,
    errorMessages.restore,
    logEventPrefix,
    setRestoreTarget,
    onAfterRestore,
  ]);

  return {
    deleteTarget,
    setDeleteTarget,
    deleteConfirmInput,
    setDeleteConfirmInput,
    restoreTarget,
    setRestoreTarget,
    isProcessing,
    softDeleteError,
    showTrash,
    setShowTrash,
    handleSoftDelete,
    handleRestore,
  };
}

export function DeletedEntityCard({
  name,
  subtitle,
  deletedAt,
  onRestore,
  extraSubtitle,
  restoreDisabled = false,
  restoreDisabledReason,
}: {
  readonly name: string;
  readonly subtitle: string;
  readonly deletedAt: string | null | undefined;
  readonly onRestore: () => void;
  readonly extraSubtitle?: string | null;
  readonly restoreDisabled?: boolean;
  readonly restoreDisabledReason?: string;
}) {
  return (
    <div className="border-moto-red/10 bg-moto-red-soft/50 rounded-2xl border p-5 shadow-sm">
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <h3 className="text-base font-semibold text-gray-900">{name}</h3>
          <div className="mt-0.5 flex items-center gap-2 text-sm text-gray-500">
            <span className="font-mono">{subtitle}</span>
            {extraSubtitle && (
              <>
                <span className="text-gray-300">·</span>
                <span>{extraSubtitle}</span>
              </>
            )}
          </div>
        </div>
        <span className="bg-moto-red/15 text-moto-red-strong rounded-full px-2 py-0.5 text-xs font-medium">
          Gelöscht
        </span>
      </div>
      <div className="mt-3 flex items-center justify-between gap-3">
        <p className="text-xs text-gray-400">
          Gelöscht {deletedAt ? getRelativeTime(deletedAt) : ""}
        </p>
        <div className="flex flex-col items-end gap-1">
          <button
            type="button"
            onClick={onRestore}
            disabled={restoreDisabled}
            title={restoreDisabled ? restoreDisabledReason : undefined}
            className="bg-moto-green/15 text-moto-green-strong hover:bg-moto-green/25 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-400 disabled:hover:bg-gray-100"
          >
            Wiederherstellen
          </button>
          {restoreDisabled && restoreDisabledReason && (
            <p className="max-w-[16rem] text-right text-[11px] text-gray-500">
              {restoreDisabledReason}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

export function SoftDeleteConfirmationModal<T extends SoftDeletable>({
  target,
  entityLabel,
  entityArticleAccusative,
  nameLabel,
  warningTitle,
  warningBullets,
  inputId,
  confirmInput,
  onConfirmInputChange,
  errorMessage,
  isProcessing,
  onCancel,
  onConfirm,
  confirmDisabled = false,
  confirmDisabledReason,
}: {
  readonly target: T;
  readonly entityLabel: string;
  readonly entityArticleAccusative: string;
  readonly nameLabel: string;
  readonly warningTitle: string;
  readonly warningBullets: readonly string[];
  readonly inputId: string;
  readonly confirmInput: string;
  readonly onConfirmInputChange: (value: string) => void;
  readonly errorMessage: string;
  readonly isProcessing: boolean;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
  readonly confirmDisabled?: boolean;
  readonly confirmDisabledReason?: string;
}) {
  return (
    <ConfirmationModal
      isOpen
      onClose={onCancel}
      onConfirm={onConfirm}
      title={`${entityLabel} löschen`}
      confirmText="Löschen"
      isConfirmLoading={isProcessing}
      isConfirmDisabled={confirmInput !== target.name || confirmDisabled}
      isDismissDisabled={isProcessing}
      isBackdropDismissDisabled
      confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      loadingText="Wird gelöscht..."
    >
      <p className="text-sm text-gray-600">
        Möchten Sie {entityArticleAccusative}{" "}
        <span className="font-medium">{target.name}</span> wirklich löschen?
      </p>
      {warningBullets.length > 0 && (
        <div className="bg-moto-amber-soft text-moto-amber-strong mt-3 rounded-lg px-3 py-2 text-sm">
          <p className="font-medium">{warningTitle}</p>
          <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
            {warningBullets.map((bullet) => (
              <li key={bullet}>{bullet}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="mt-4">
        <label
          htmlFor={inputId}
          className="block text-sm font-medium text-gray-700"
        >
          {nameLabel}
        </label>
        <p className="mb-1 text-sm font-medium text-gray-900">{target.name}</p>
        <input
          id={inputId}
          type="text"
          value={confirmInput}
          onChange={(event) => onConfirmInputChange(event.target.value)}
          placeholder={target.name}
          className="focus:ring-moto-red w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
          autoComplete="off"
        />
      </div>

      {confirmDisabled && confirmDisabledReason && (
        <div className="bg-moto-red-soft text-moto-red mt-3 rounded-lg px-3 py-2 text-sm">
          {confirmDisabledReason}
        </div>
      )}

      {errorMessage && (
        <div className="bg-moto-red-soft text-moto-red mt-3 rounded-lg px-3 py-2 text-sm">
          {errorMessage}
        </div>
      )}
    </ConfirmationModal>
  );
}

export function RestoreConfirmationModal<T extends SoftDeletable>({
  target,
  setTarget,
  entityLabel,
  entityArticleAccusative,
  entityPronounNominative,
  entityPossessiveAccusative,
  extraMessage,
  onConfirm,
  isProcessing,
  errorMessage,
  confirmDisabled = false,
  confirmDisabledReason,
}: {
  readonly target: T | null;
  readonly setTarget: (t: T | null) => void;
  readonly entityLabel: string;
  readonly entityArticleAccusative: string;
  readonly entityPronounNominative: string;
  readonly entityPossessiveAccusative: string;
  readonly extraMessage?: string;
  readonly onConfirm: () => void;
  readonly isProcessing: boolean;
  readonly errorMessage?: string;
  readonly confirmDisabled?: boolean;
  readonly confirmDisabledReason?: string;
}) {
  return (
    <ConfirmationModal
      isOpen={target !== null}
      onClose={() => setTarget(null)}
      onConfirm={onConfirm}
      title={`${entityLabel} wiederherstellen`}
      confirmText="Wiederherstellen"
      isConfirmLoading={isProcessing}
      isConfirmDisabled={confirmDisabled}
    >
      <p className="text-sm text-gray-600">
        Möchten Sie {entityArticleAccusative} &quot;{target?.name}&quot;
        wiederherstellen? {entityPronounNominative} wird in{" "}
        {entityPossessiveAccusative} vorherigen Zustand zurückversetzt.
      </p>
      {extraMessage && (
        <p className="mt-2 text-xs text-gray-500">{extraMessage}</p>
      )}
      {confirmDisabled && confirmDisabledReason && (
        <div className="bg-moto-red-soft text-moto-red mt-3 rounded-lg px-3 py-2 text-sm">
          {confirmDisabledReason}
        </div>
      )}
      {errorMessage && (
        <div className="bg-moto-red-soft text-moto-red mt-3 rounded-lg px-3 py-2 text-sm">
          {errorMessage}
        </div>
      )}
    </ConfirmationModal>
  );
}
