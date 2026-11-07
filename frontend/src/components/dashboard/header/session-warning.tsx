// Session expiry warning components for header
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import { Alert } from "~/components/ui/alert";

/**
 * Warning icon SVG
 */
function WarningIcon({ className }: Readonly<{ className?: string }>) {
  return (
    <svg
      className={className ?? "h-5 w-5"}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>
  );
}

/**
 * Desktop session expiry warning with message
 */
function SessionExpiryWarningDesktop() {
  return (
    <Alert
      type="warning"
      size="compact"
      announce="off"
      message="Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an."
    />
  );
}

/**
 * Mobile session expiry warning (icon only)
 */
function SessionExpiryWarningMobile() {
  return (
    <div className="text-moto-orange-strong p-2">
      <WarningIcon />
    </div>
  );
}

/**
 * Conditional session warning for header sections
 */
interface SessionWarningProps {
  readonly isExpired: boolean;
  readonly variant: "desktop" | "mobile";
}

export function SessionWarning({ isExpired, variant }: SessionWarningProps) {
  if (!isExpired) return null;

  return variant === "desktop" ? (
    <SessionExpiryWarningDesktop />
  ) : (
    <SessionExpiryWarningMobile />
  );
}
