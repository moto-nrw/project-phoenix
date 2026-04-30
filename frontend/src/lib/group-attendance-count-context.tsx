"use client";

import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";

interface GroupAttendanceCount {
  readonly present: number;
  readonly total: number;
}

interface GroupAttendanceCountContextValue {
  readonly counts: Readonly<Record<string, GroupAttendanceCount>>;
  readonly setGroupAttendanceCount: (
    groupId: string,
    count: GroupAttendanceCount,
  ) => void;
}

const GroupAttendanceCountContext =
  createContext<GroupAttendanceCountContextValue | null>(null);

const EMPTY_GROUP_ATTENDANCE_COUNTS: GroupAttendanceCountContextValue = {
  counts: {},
  setGroupAttendanceCount: () => {},
};

export function GroupAttendanceCountProvider({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const [counts, setCounts] = useState<Record<string, GroupAttendanceCount>>(
    {},
  );

  const setGroupAttendanceCount = useCallback(
    (groupId: string, count: GroupAttendanceCount) => {
      setCounts((prev) => {
        const existing = prev[groupId];
        if (
          existing &&
          existing.present === count.present &&
          existing.total === count.total
        ) {
          return prev;
        }

        return {
          ...prev,
          [groupId]: count,
        };
      });
    },
    [],
  );

  const value = useMemo(
    () => ({ counts, setGroupAttendanceCount }),
    [counts, setGroupAttendanceCount],
  );

  return (
    <GroupAttendanceCountContext.Provider value={value}>
      {children}
    </GroupAttendanceCountContext.Provider>
  );
}

export function useGroupAttendanceCounts() {
  const context = useContext(GroupAttendanceCountContext);
  return context ?? EMPTY_GROUP_ATTENDANCE_COUNTS;
}
