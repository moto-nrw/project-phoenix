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
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import { useSidebarMemory } from "~/lib/sidebar-memory";
import type { HelpTopicId } from "~/lib/help-topics";

interface BreadcrumbData {
  studentName?: string;
  staffName?: string;
  /** Name des Raums auf der Raumseite /rooms/[id] (#3115). */
  roomName?: string;
  /** Titel der Mitteilung auf /parent-announcements/[id] (#3115). */
  announcementTitle?: string;
  referrerPage?: string;
  activeSupervisionName?: string;
  /** Offener Raum ohne eigene Aufsicht: steht unter „Räume“ (#4372). */
  activeSupervisionIsSharedRoom?: boolean;
  ogsGroupName?: string;
  pageTitle?: string;
  helpTopic?: HelpTopicId;
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
 * Convenience hook for student history pages.
 * Reads the remembered group/room name based on the referrer
 * and sets the breadcrumb accordingly.
 */
export function useStudentHistoryBreadcrumb(opts: {
  studentName?: string;
  referrer: string;
}): void {
  const memory = useSidebarMemory();
  const groupNameKey = memory.key("sidebar-last-group-name");
  const roomNameKey = memory.key("sidebar-last-room-name");
  const breadcrumbGroupName = useLocalStorageValue(
    groupNameKey ?? "",
    groupNameKey !== null && opts.referrer.startsWith("/ogs-groups"),
  );
  const breadcrumbRoomName = useLocalStorageValue(
    roomNameKey ?? "",
    roomNameKey !== null && opts.referrer.startsWith("/active-supervisions"),
  );

  useSetBreadcrumb({
    studentName: opts.studentName,
    referrerPage: opts.referrer,
    ogsGroupName: breadcrumbGroupName ?? undefined,
    activeSupervisionName: breadcrumbRoomName ?? undefined,
  });
}

/**
 * Sets breadcrumb data on mount and clears it on unmount.
 * Pages call this to communicate breadcrumb info to the persistent Header.
 */
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
