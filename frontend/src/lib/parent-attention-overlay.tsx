"use client";

import {
  createContext,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import type { ParentAttention } from "~/lib/parent-attention";
import { useSWRCacheKey } from "~/lib/swr";

export interface ParentAttentionSelection {
  readonly studentId: string;
  readonly studentName: string;
  readonly attention: ParentAttention;
}

const OverlayContext = createContext<{
  readonly selection: ParentAttentionSelection | null;
  readonly select: (selection: ParentAttentionSelection) => void;
} | null>(null);

/** Only the selection crosses the route boundary, never the dialog's code. */
export function ParentAttentionOverlayProvider({
  children,
}: Readonly<{ children: ReactNode }>) {
  const cacheKey = useSWRCacheKey();
  const scope = cacheKey("parent-attention-overlay");
  const [snapshot, setSnapshot] = useState<{
    readonly scope: typeof scope;
    readonly selection: ParentAttentionSelection;
  } | null>(null);
  const value = useMemo(
    () => ({
      selection: snapshot?.scope === scope ? snapshot.selection : null,
      select: (selection: ParentAttentionSelection) =>
        setSnapshot({ scope, selection }),
    }),
    [scope, snapshot],
  );
  return (
    <OverlayContext.Provider value={value}>{children}</OverlayContext.Provider>
  );
}

export function useParentAttentionOverlay() {
  const context = useContext(OverlayContext);
  if (!context) throw new Error("Parent attention overlay provider missing");
  return context;
}
