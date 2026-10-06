"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import Image from "next/image";
import Link from "next/link";
import {
  AuthShell,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { InvitationAcceptForm } from "~/components/auth/invitation-accept-form";
import { InvitationOwnerAcceptForm } from "~/components/auth/invitation-owner-accept-form";
import { validateInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { useApiLoadError } from "~/contexts/ToastContext";
import { errorStatus } from "~/lib/expected-failure";
import { createLogger } from "~/lib/logger";
import { useTenantSafe } from "~/lib/tenant-context";
import { loginImageSrc } from "~/lib/tenant-api";

const logger = createLogger({ component: "InvitationPageContent" });

/** Ohne Token im Link gibt es nichts zu laden. */
const MISSING_TOKEN_TEXT =
  "Der Link ist unvollständig. Bitte öffnen Sie den Link aus der E-Mail erneut.";

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
  // Ladefehler vor Ort mit Katalogtext je Code (#2517): abgelaufen oder
  // unbekannt hat eigene Texte, ein Serverfehler bietet Wiederholen.
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [isLoading, setIsLoading] = useState(Boolean(token));
  const [attempt, setAttempt] = useState(0);
  const retryRef = useRef<() => void>(() => undefined);
  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  useLayoutEffect(() => {
    retryRef.current = retry;
  });
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
        setIsLoading(false);
        return;
      }
      setIsLoading(true);
      clearLoadError();
      try {
        const result = await validateInvitation(token);
        if (!cancelled) {
          setInvitation(result);
        }
      } catch (err) {
        if (cancelled) return;
        // Ein abgelaufener oder unbekannter Link ist kein Defekt.
        const status = errorStatus(err);
        const context = {
          error: err instanceof Error ? err.message : String(err),
          status,
        };
        if (status === 410 || status === 404) {
          logger.warn("invitation_validation_failed", context);
        } else {
          logger.error("invitation_validation_failed", context);
        }
        setInvitation(null);
        // Bis der Katalogtext da ist, bleibt die Ladeanzeige stehen.
        await showLoadError(err, {
          object: "die Einladung",
          retry: () => retryRef.current(),
        });
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
  }, [token, attempt, clearLoadError, showLoadError]);

  const error = token ? loadError : MISSING_TOKEN_TEXT;

  if (isLoading) {
    return (
      <AuthShell
        eyebrow="Einladung"
        eyebrowClassName="text-moto-green"
        title="Konto einrichten"
        subtitle="Wir prüfen Ihre Einladung."
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
      title={
        invitation?.requiresAccountLogin
          ? "Schule hinzufügen"
          : "Konto einrichten"
      }
      subtitle={
        invitation?.requiresAccountLogin
          ? "Nutzen Sie Ihr bestehendes Konto. Ihr Passwort bleibt unverändert."
          : "Nehmen Sie die Einladung an und wählen Sie Ihr Passwort."
      }
      variant="tenant"
      brand={brand}
      formMaxWidth="max-w-[34rem]"
    >
      {error && (
        <div className="space-y-4">
          <LoadErrorAlert error={error} />
          <Link href="/" className={authPrimaryButtonClassName}>
            Zur Anmeldung
          </Link>
        </div>
      )}

      {!error &&
        invitation &&
        token &&
        (invitation.requiresAccountLogin ? (
          <InvitationOwnerAcceptForm
            token={token}
            invitation={invitation}
            redirectToPath={redirectToPath}
          />
        ) : (
          <InvitationAcceptForm
            token={token}
            invitation={invitation}
            redirectToPath={redirectToPath}
          />
        ))}
    </AuthShell>
  );
}
