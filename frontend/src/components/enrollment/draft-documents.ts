"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DraftDocuments" });

/**
 * Documents an editor uploads before its form is saved ("drafts"). The
 * editor deletes every draft its save does not reference, on discard and on
 * unmount, so no file stays on the server without a form that shows it.
 * Upload controls report to the editor through this context.
 */
export interface DraftDocumentLifecycle {
  /** An upload of this editing session finished. */
  remember: (reference: string) => void;
  /** An upload starts; saving waits until every started upload ended. */
  begin: () => void;
  end: () => void;
  /**
   * Deletes the document now if it is a draft of this session; a saved
   * document stays, because older template versions still show it. Without
   * it the next save or discard removes the draft. Throws when the delete
   * fails; the draft then stays tracked.
   */
  discard?: (reference: string) => Promise<void>;
}

export const DraftDocumentContext =
  createContext<DraftDocumentLifecycle | null>(null);

/** The lifecycle of the surrounding editor. Upload controls need one. */
export function useDraftDocumentLifecycle(): DraftDocumentLifecycle {
  const lifecycle = useContext(DraftDocumentContext);
  if (!lifecycle) {
    throw new Error(
      "Upload controls need a DraftDocumentContext provider: without it, uploaded files would never be cleaned up.",
    );
  }
  return lifecycle;
}

type RemoveDocument = (
  reference: string,
  options?: { keepalive?: boolean },
) => Promise<void>;

export interface DraftDocumentSave {
  /** The save went through: the referenced drafts are part of the form now. */
  commit: () => void;
  /** The save failed. After an unmount the referenced drafts are deleted. */
  fail: () => void;
}

/** Tracks the drafts of one editor and provides its upload lifecycle. */
export function useDraftDocuments(remove: RemoveDocument) {
  const { show: showActionError } = useApiErrorDisplay();
  const draftsRef = useRef<Set<string>>(new Set());
  // Drafts a running save references; an unmount must not delete them.
  const savingRef = useRef<Set<string>>(new Set());
  const unmountedRef = useRef(false);
  const [uploadCount, setUploadCount] = useState(0);

  const forget = useCallback((references: Iterable<string>) => {
    for (const reference of references) draftsRef.current.delete(reference);
  }, []);

  const lifecycle = useMemo<DraftDocumentLifecycle>(
    () => ({
      remember: (reference) => {
        const trimmed = reference.trim();
        if (trimmed) draftsRef.current.add(trimmed);
      },
      begin: () => setUploadCount((count) => count + 1),
      end: () => setUploadCount((count) => Math.max(0, count - 1)),
      discard: async (reference) => {
        if (!draftsRef.current.has(reference)) return;
        await remove(reference);
        draftsRef.current.delete(reference);
      },
    }),
    [remove],
  );

  /** Deletes every draft except `keep`; failures stay tracked for the next try. */
  const cleanup = useCallback(
    async ({
      notify = true,
      keep = new Set<string>(),
    }: { notify?: boolean; keep?: ReadonlySet<string> } = {}) => {
      const references = Array.from(draftsRef.current).filter(
        (reference) => !keep.has(reference),
      );
      if (references.length === 0) return;
      const results = await Promise.allSettled(
        references.map((reference) => remove(reference, { keepalive: false })),
      );
      forget(
        references.filter((_, index) => results[index]?.status === "fulfilled"),
      );
      const failure = results.find(
        (result): result is PromiseRejectedResult =>
          result.status === "rejected",
      );
      if (notify && failure) {
        logger.warn("draft_legal_document_cleanup_failed", {
          error:
            failure.reason instanceof Error
              ? failure.reason.message
              : String(failure.reason),
        });
        void showActionError(failure.reason, {
          object: "eine ungespeicherte PDF-Datei",
        });
      }
    },
    [forget, remove, showActionError],
  );

  /**
   * Starts a save that references some drafts: the other drafts are deleted
   * first, the referenced ones are protected from an unmount until the save
   * commits or fails.
   */
  const startSave = useCallback(
    async (referenced: ReadonlySet<string>): Promise<DraftDocumentSave> => {
      savingRef.current = new Set(referenced);
      await cleanup({ keep: referenced });
      return {
        commit: () => {
          forget(referenced);
          savingRef.current = new Set();
        },
        fail: () => {
          if (unmountedRef.current) {
            // Nobody is left to save the form: its uploads go too.
            deleteAfterUnmount(remove, savingRef.current);
          }
          savingRef.current = new Set();
        },
      };
    },
    [cleanup, forget, remove],
  );

  useEffect(() => {
    unmountedRef.current = false;
    return () => {
      unmountedRef.current = true;
      const saving = savingRef.current;
      deleteAfterUnmount(
        remove,
        Array.from(draftsRef.current).filter((ref) => !saving.has(ref)),
      );
      draftsRef.current = new Set(
        Array.from(draftsRef.current).filter((ref) => saving.has(ref)),
      );
    };
  }, [remove]);

  const drafts = useCallback((): ReadonlySet<string> => draftsRef.current, []);

  return {
    lifecycle,
    /** True while an upload runs; saving and leaving wait for it. */
    uploading: uploadCount > 0,
    drafts,
    cleanup,
    startSave,
  };
}

function deleteAfterUnmount(
  remove: RemoveDocument,
  references: Iterable<string>,
) {
  for (const reference of references) {
    void remove(reference, { keepalive: true }).catch((error: unknown) => {
      logger.warn("draft_document_cleanup_failed_after_unmount", {
        error: error instanceof Error ? error.message : String(error),
      });
    });
  }
}
