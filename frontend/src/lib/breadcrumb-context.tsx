"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import type { ReactNode } from "react";

export interface BreadcrumbData {
  studentName?: string;
  roomName?: string;
  activityName?: string;
  referrerPage?: string;
  activeSupervisionName?: string;
  ogsGroupName?: string;
  pageTitle?: string;
}

interface BreadcrumbContextValue {
  breadcrumb: BreadcrumbData;
  setBreadcrumb: (data: BreadcrumbData) => void;
}

const BreadcrumbContext = createContext<BreadcrumbContextValue | null>(null);

export function BreadcrumbProvider({
  children,
}: {
  readonly children: ReactNode;
}) {
  const [breadcrumb, setBreadcrumb] = useState<BreadcrumbData>({});

  const value = useMemo(() => ({ breadcrumb, setBreadcrumb }), [breadcrumb]);

  return (
    <BreadcrumbContext.Provider value={value}>
      {children}
    </BreadcrumbContext.Provider>
  );
}

export function useBreadcrumb(): BreadcrumbContextValue {
  const ctx = useContext(BreadcrumbContext);
  if (!ctx) {
    throw new Error("useBreadcrumb must be used within a BreadcrumbProvider");
  }
  return ctx;
}

/**
 * Sets breadcrumb data on mount and clears it on unmount.
 * Pages call this to communicate breadcrumb info to the persistent Header.
 */
/**
 * Convenience hook for student history pages.
 * Reads the group/room name from localStorage based on the referrer
 * and sets the breadcrumb accordingly.
 */
export function useStudentHistoryBreadcrumb(opts: {
  studentName?: string;
  referrer: string;
}): void {
  const breadcrumbGroupName =
    opts.referrer.startsWith("/ogs-groups") && globalThis.window !== undefined
      ? localStorage.getItem("sidebar-last-group-name")
      : undefined;
  const breadcrumbRoomName =
    opts.referrer.startsWith("/active-supervisions") &&
    globalThis.window !== undefined
      ? localStorage.getItem("sidebar-last-room-name")
      : undefined;

  useSetBreadcrumb({
    studentName: opts.studentName,
    referrerPage: opts.referrer,
    ogsGroupName: breadcrumbGroupName ?? undefined,
    activeSupervisionName: breadcrumbRoomName ?? undefined,
  });
}

export function useSetBreadcrumb(data: BreadcrumbData): void {
  const { setBreadcrumb } = useBreadcrumb();

  const stableKey = JSON.stringify(data);

  const clear = useCallback(() => {
    setBreadcrumb({});
  }, [setBreadcrumb]);

  useEffect(() => {
    setBreadcrumb(JSON.parse(stableKey) as BreadcrumbData);
    return clear;
    // eslint-disable-next-line react-hooks/exhaustive-deps -- stableKey captures all data changes
  }, [stableKey, clear]);
}
