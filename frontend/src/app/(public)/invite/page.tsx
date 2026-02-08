"use client";

import { Suspense, useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { InvitationAcceptForm } from "~/components/auth/invitation-accept-form";
import { validateInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";
import { Loading } from "~/components/ui/loading";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InvitePage" });

function LoadingState() {
  return <Loading fullPage={false} />;
}

function InvitationContent() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token");
  const [invitation, setInvitation] = useState<InvitationValidation | null>(
    null,
  );
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);

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
    return <Loading fullPage={false} />;
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
      <div className="w-full max-w-2xl rounded-2xl border border-gray-200/50 bg-white/90 p-8 shadow-sm backdrop-blur-sm">
        <div className="mb-8 flex flex-col items-center gap-3 text-center">
          <Image
            src="/images/moto_transparent.png"
            alt="moto Logo"
            width={120}
            height={40}
          />
          <h1 className="text-2xl font-semibold text-gray-900">
            Willkommen bei moto
          </h1>
          <p className="text-sm text-gray-600">
            Bitte bestätige deine Einladung und lege dein persönliches Passwort
            fest.
          </p>
        </div>

        {error && (
          <div className="space-y-4">
            <div className="rounded-xl border border-red-200/50 bg-red-50/50 p-4">
              <div className="flex items-start gap-3">
                <svg
                  className="mt-0.5 h-5 w-5 flex-shrink-0 text-red-600"
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
                <p className="text-sm text-red-700">{error}</p>
              </div>
            </div>
            <div className="text-center text-sm text-gray-600">
              <p>
                Du kannst eine neue Einladung bei deinem Administrator anfordern
                oder dich über&nbsp;
                <Link
                  href="/"
                  className="font-medium text-gray-900 underline hover:text-gray-700"
                >
                  die Startseite
                </Link>
                &nbsp;anmelden.
              </p>
            </div>
          </div>
        )}

        {!error && invitation && token && (
          <InvitationAcceptForm token={token} invitation={invitation} />
        )}
      </div>
    </div>
  );
}

export default function InvitePage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <InvitationContent />
    </Suspense>
  );
}
