"use client";

import { useCallback, useMemo, useSyncExternalStore } from "react";
import { useSession } from "next-auth/react";

/** How a collection page shows its entries (#3834). */
export type CollectionView = "tiles" | "table";

/** One column a table offers; `defaultVisible` is its state before any choice. */
export interface CollectionColumnDefault {
  readonly id: string;
  readonly defaultVisible: boolean;
}

interface StoredCollectionView {
  view?: CollectionView;
  /** Only the columns the user switched; every other column keeps its default. */
  columns?: Record<string, boolean>;
}

const STORAGE_PREFIX = "collection-view";
// Same-document writes don't fire the cross-tab "storage" event, so writes
// broadcast this custom event to wake up every mounted consumer.
const LOCAL_CHANGE_EVENT = "collection-view-change";

// Fallback when localStorage is not writable (private mode, blocked site
// data, full quota): the choice still holds for the lifetime of the document.
const memoryValues = new Map<string, string>();

function readRaw(key: string): string | null {
  const memory = memoryValues.get(key);
  if (memory !== undefined) return memory;
  try {
    return globalThis.localStorage.getItem(key);
  } catch {
    return null;
  }
}

function parseStored(raw: string | null): StoredCollectionView {
  if (!raw) return {};
  try {
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) return {};
    const candidate = parsed as StoredCollectionView;
    const view =
      candidate.view === "tiles" || candidate.view === "table"
        ? candidate.view
        : undefined;
    const columns: Record<string, boolean> = {};
    if (typeof candidate.columns === "object" && candidate.columns !== null) {
      for (const [id, visible] of Object.entries(candidate.columns)) {
        if (typeof visible === "boolean") columns[id] = visible;
      }
    }
    return { view, columns };
  } catch {
    return {};
  }
}

function writeStored(key: string, value: StoredCollectionView) {
  const raw = JSON.stringify(value);
  try {
    globalThis.localStorage.setItem(key, raw);
    memoryValues.delete(key);
  } catch {
    memoryValues.set(key, raw);
  }
  globalThis.dispatchEvent(new Event(LOCAL_CHANGE_EVENT));
}

function subscribe(onStoreChange: () => void) {
  globalThis.addEventListener("storage", onStoreChange);
  globalThis.addEventListener(LOCAL_CHANGE_EVENT, onStoreChange);
  return () => {
    globalThis.removeEventListener("storage", onStoreChange);
    globalThis.removeEventListener(LOCAL_CHANGE_EVENT, onStoreChange);
  };
}

const getServerSnapshot = () => null;

/**
 * Storage key per page, school and account: two staff members sharing one
 * tablet keep their own view, and one person's choice at school A does not
 * leak into school B. Mirrors the Kindersuche filter key.
 */
function buildStorageKey(
  pageKey: string,
  user:
    | { id?: string | null; email?: string | null; tenantId?: number | null }
    | undefined,
): string | null {
  if (typeof window === "undefined") return null;
  const tenantKey =
    user?.tenantId !== undefined && user.tenantId !== null
      ? `tenant-${user.tenantId}`
      : `host-${window.location.host}`;
  const accountKey = user?.id ?? user?.email ?? "anonymous";
  return `${STORAGE_PREFIX}:${pageKey}:${tenantKey}:${accountKey}`;
}

export interface CollectionViewPreference {
  readonly view: CollectionView;
  readonly setView: (view: CollectionView) => void;
  /** Table columns currently switched off. */
  readonly hiddenColumns: ReadonlySet<string>;
  readonly setColumnVisible: (id: string, visible: boolean) => void;
}

/**
 * The remembered tiles/table choice and table columns of one collection page
 * (#3834). Stored per device, school and account in localStorage; without a
 * stored choice the page opens with tiles and each column's default.
 */
export function useCollectionView(
  pageKey: string,
  columns: readonly CollectionColumnDefault[],
): CollectionViewPreference {
  const { data: session } = useSession();
  const storageKey = buildStorageKey(pageKey, session?.user);

  const getSnapshot = useCallback(
    () => (storageKey ? readRaw(storageKey) : null),
    [storageKey],
  );
  const raw = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
  const stored = useMemo(() => parseStored(raw), [raw]);

  const hiddenColumns = useMemo(() => {
    const hidden = new Set<string>();
    for (const column of columns) {
      const visible = stored.columns?.[column.id] ?? column.defaultVisible;
      if (!visible) hidden.add(column.id);
    }
    return hidden;
  }, [columns, stored]);

  const setView = useCallback(
    (view: CollectionView) => {
      if (!storageKey) return;
      writeStored(storageKey, { ...parseStored(readRaw(storageKey)), view });
    },
    [storageKey],
  );

  const setColumnVisible = useCallback(
    (id: string, visible: boolean) => {
      if (!storageKey) return;
      const current = parseStored(readRaw(storageKey));
      writeStored(storageKey, {
        ...current,
        columns: { ...current.columns, [id]: visible },
      });
    },
    [storageKey],
  );

  return {
    view: stored.view ?? "tiles",
    setView,
    hiddenColumns,
    setColumnVisible,
  };
}
