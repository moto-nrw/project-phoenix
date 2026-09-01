"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import {
  AuthShell,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { InvitationAcceptForm } from "~/components/auth/invitation-accept-form";
import { validateInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";
import { useTenantSafe } from "~/lib/tenant-context";
import { loginImageSrc } from "~/lib/tenant-api";

const logger = createLogger({ component: "InvitationPageContent" });

/**
 * Shared invitation page content used by both:
 * - `/invite?token=...` (root-level, no tenant context)
 * - `/[tenant]/(public)/invite?token=...` (tenant-scoped)
 *
 * Validates the invitation token and renders the accept form.
 */
export function InvitationPageContent({
  token,
  redirectToPath,
}: {
  token: string | null;
  /** Post-accept redirect override — see InvitationAcceptForm (#2207). */
  redirectToPath?: string;
}) {
  const [invitation, setInvitation] = useState<InvitationValidation | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const tenantContext = useTenantSafe();
  const tenant = tenantContext?.tenant;
  const brand = tenant?.settings?.loginImageUrl ? (
    <Image
      src={loginImageSrc(tenant.settings.loginImageUrl)}
      alt={`${tenant.name} Logo`}
      width={180}
      height={104}
      className="max-h-[104px] w-auto object-contain"
      priority
      unoptimized
    />
  ) : null;

  useEffect(() => {
    let cancelled = false;
    async function fetchInvitation() {
      if (!token) {
        setError("Kein Einladungstoken angegeben.");
        setIsLoading(false);
        return;
      }
      setIsLoading(true);
      setError(null);
      try {
        const result = await validateInvitation(token);
        if (!cancelled) {
          setInvitation(result);
        }
      } catch (err) {
        if (cancelled) return;
        const apiError = err as ApiError | undefined;
        const status = apiError?.status;
        if (status === 410 || status === 404) {
          logger.warn("invitation_validation_failed", {
            error: err instanceof Error ? err.message : String(err),
            status,
          });
        } else {
          logger.error("invitation_validation_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
        }
        if (status === 410) {
          setError(
            "Diese Einladung ist abgelaufen oder wurde bereits verwendet.",
          );
        } else if (status === 404) {
          setError("Wir konnten diese Einladung nicht finden.");
        } else {
          setError(
            apiError?.message ??
              "Beim Laden der Einladung ist ein Fehler aufgetreten.",
          );
        }
        setInvitation(null);
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    void fetchInvitation();
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (isLoading) {
    return (
      <AuthShell
        eyebrow="Einladung"
        eyebrowClassName="text-moto-green"
        title="Konto einrichten"
        subtitle="Wir prüfen deine Einladung."
        variant="tenant"
        brand={brand}
        formMaxWidth="max-w-[32rem]"
      >
        <Loading fullPage={false} />
      </AuthShell>
    );
  }

  return (
    <AuthShell
      eyebrow="Einladung"
      eyebrowClassName="text-moto-green"
      title="Konto einrichten"
      subtitle="Bestätige deine Einladung und lege dein persönliches Passwort fest."
      variant="tenant"
      brand={brand}
      formMaxWidth="max-w-[34rem]"
    >
      {error && (
        <div className="space-y-4">
          <div className="border-moto-red/20 bg-moto-red-soft rounded-xl border p-4">
            <div className="flex items-start gap-3">
              <svg
                role="img"
                aria-label="Fehler"
                className="text-moto-red mt-0.5 h-5 w-5 flex-shrink-0"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <p className="text-moto-red-strong text-sm">{error}</p>
            </div>
          </div>
          <Link href="/" className={authPrimaryButtonClassName}>
            Zur Anmeldung
          </Link>
        </div>
      )}

      {!error && invitation && token && (
        <InvitationAcceptForm
          token={token}
          invitation={invitation}
          redirectToPath={redirectToPath}
        />
      )}
    </AuthShell>
  );
}
