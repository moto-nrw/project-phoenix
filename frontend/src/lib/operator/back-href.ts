// Open-redirect guard for `?back=` query params on operator pages.
// Restricts the value to internal operator paths so a crafted link cannot
// bounce the user to an arbitrary host.
const SAFE_OPERATOR_BACK_PATH = /^\/operator\/[A-Za-z0-9/_\-?=&%.]*$/;

export function resolveOperatorBackHref(
  raw: string | null,
  fallback: string,
): string {
  if (!raw) return fallback;
  return SAFE_OPERATOR_BACK_PATH.test(raw) ? raw : fallback;
}
