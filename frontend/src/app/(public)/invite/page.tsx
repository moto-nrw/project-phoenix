"use client";

import { Suspense, useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Alert } from "~/components/ui";
import { InvitationAcceptForm } from "~/components/auth/invitation-accept-form";
import { validateInvitation } from "~/lib/invitation-api";
import type { InvitationValidation } from "~/lib/invitation-helpers";
import type { ApiError } from "~/lib/auth-api";

import { Loading } from "~/components/ui/loading";
function LoadingState() {
  return (
    <Loading fullPage={false} />
  );
}

function InvitationContent() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token");
  const [invitation, setInvitation] = useState<InvitationValidation | null>(null);
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
        if (apiError?.status === 410) {
          setError("Diese Einladung ist abgelaufen oder wurde bereits verwendet.");
        } else if (apiError?.status === 404) {
          setError("Wir konnten diese Einladung nicht finden.");
        } else {
          setError(apiError?.message ?? "Beim Laden der Einladung ist ein Fehler aufgetreten.");
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
      <Loading fullPage={false} />
    );
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-gradient-to-br from-slate-50 via-white to-slate-100 p-4">
      <div className="w-full max-w-2xl rounded-3xl bg-white/90 p-8 shadow-2xl backdrop-blur-xl">
        <div className="mb-6 flex flex-col items-center gap-4 text-center">
          <Image src="/images/moto_transparent.png" alt="moto Logo" width={140} height={48} className="opacity-70" />
          <h1 className="text-3xl font-semibold text-gray-900">Willkommen bei moto</h1>
          <p className="text-sm text-gray-600">
            Bitte bestätige deine Einladung und lege dein persönliches Passwort fest.
          </p>
        </div>

        {error && (
          <div className="space-y-4">
            <Alert type="error" message={error} />
            <div className="text-center text-sm text-gray-600">
              <p>
                Du kannst eine neue Einladung bei deinem Administrator anfordern oder dich über&nbsp;
                <Link href="/" className="font-medium text-[#5080d8] hover:underline">
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
