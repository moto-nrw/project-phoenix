function parseInstant(value: string | undefined): number | null {
  if (!value) return null;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : null;
}

export function isPlannedStartExpired(
  startExpiresAt: string | undefined,
  now: Date,
): boolean {
  const expires = parseInstant(startExpiresAt);
  return expires != null && now.getTime() >= expires;
}

export function canStartPlannedInstance(
  instance: {
    canStart?: boolean;
    startAvailableAt?: string;
    startExpiresAt?: string;
  },
  now: Date,
): boolean {
  if (isPlannedStartExpired(instance.startExpiresAt, now)) {
    return false;
  }
  if (instance.canStart !== false) return true;
  const available = parseInstant(instance.startAvailableAt);
  return available != null && now.getTime() >= available;
}

export function canCompleteInstance(
  canComplete: boolean,
  completeAvailableAt: string,
  now: Date,
): boolean {
  if (canComplete) return true;
  if (!completeAvailableAt) return false;
  const available = new Date(completeAvailableAt);
  return (
    Number.isFinite(available.getTime()) && now.getTime() >= available.getTime()
  );
}
