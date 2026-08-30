function padClockPart(value: number): string {
  return value.toString().padStart(2, "0");
}

function formatClock(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `${padClockPart(hours)}:${padClockPart(minutes)}`;
}

const BERLIN_DATE_TIME_FORMATTER = new Intl.DateTimeFormat("en-GB", {
  timeZone: "Europe/Berlin",
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

/**
 * The default plan window for a spontaneously started activity: from the
 * current Berlin wall clock (capped at 23:30) one hour ahead, never past
 * 23:59 of the same day.
 */
export function spontaneousActivityWindow(now: Date): {
  date: string;
  startTime: string;
  endTime: string;
} {
  const parts = BERLIN_DATE_TIME_FORMATTER.formatToParts(now);
  const value = (type: string) =>
    parts.find((part) => part.type === type)?.value ?? "";
  const currentMinutes = Number(value("hour")) * 60 + Number(value("minute"));
  const startMinutes = Math.min(currentMinutes, 23 * 60 + 30);
  const endMinutes = Math.min(startMinutes + 60, 23 * 60 + 59);
  return {
    date: `${value("year")}-${value("month")}-${value("day")}`,
    startTime: formatClock(startMinutes),
    endTime: formatClock(endMinutes),
  };
}
