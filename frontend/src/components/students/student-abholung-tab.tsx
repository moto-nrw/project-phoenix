"use client";

import PickupScheduleManager from "./pickup-schedule-manager";

interface StudentAbholungTabProps {
  studentId: string;
  isSick?: boolean;
  isExcused?: boolean;
  onUpdate?: () => void;
}

export function StudentAbholungTab({
  studentId,
  isSick,
  isExcused,
  onUpdate,
}: StudentAbholungTabProps) {
  return (
    <PickupScheduleManager
      studentId={studentId}
      isSick={isSick}
      isExcused={isExcused}
      onUpdate={onUpdate}
    />
  );
}
