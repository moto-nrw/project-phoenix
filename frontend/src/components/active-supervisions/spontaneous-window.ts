function padClockPart(value: number): string {
  return value.toString().padStart(2, "0");
}

function formatLocalDate(date: Date): string {
  return [
    date.getFullYear(),
    padClockPart(date.getMonth() + 1),
    padClockPart(date.getDate()),
  ].join("-");
}

function formatClock(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `${padClockPart(hours)}:${padClockPart(minutes)}`;
}

/**
 * The default plan window for a spontaneously started activity: from the
 * current local wall clock (capped at 23:30) one hour ahead, never past
 * 23:59 of the same day.
 */
export function spontaneousActivityWindow(now: Date): {
  date: string;
  startTime: string;
  endTime: string;
} {
  const currentMinutes = now.getHours() * 60 + now.getMinutes();
  const startMinutes = Math.min(currentMinutes, 23 * 60 + 30);
  const endMinutes = Math.min(startMinutes + 60, 23 * 60 + 59);
  return {
    date: formatLocalDate(now),
    startTime: formatClock(startMinutes),
    endTime: formatClock(endMinutes),
  };
}
