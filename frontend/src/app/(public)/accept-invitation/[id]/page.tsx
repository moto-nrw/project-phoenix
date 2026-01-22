"use client";

import { Suspense, useCallback, useEffect, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { BetterAuthInvitationForm } from "~/components/auth/betterauth-invitation-form";
import { Loading } from "~/components/ui/loading";
import { authClient, useSession } from "~/lib/auth-client";
import { useToast } from "~/contexts/ToastContext";

interface InvitationData {
  id: string;
  email: string;
  status: string;
  organizationId: string;
  role: string;
  organizationName?: string;
}

function LoadingState() {
  return <Loading fullPage={false} />;
}

function AcceptInvitationContent() {
  const params = useParams();
  const searchParams = useSearchParams();
  const router = useRouter();
  const { success: toastSuccess, error: toastError } = useToast();
  const { data: session, isPending: isSessionLoading } = useSession();

  const invitationId = params.id as string;

  // Read pre-filled data from URL query params (set by invitation email)
  const emailFromUrl = searchParams.get("email");
  const orgNameFromUrl = searchParams.get("org");
  const roleFromUrl = searchParams.get("role");

  const [invitation, setInvitation] = useState<InvitationData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isAccepting, setIsAccepting] = useState(false);

  // Fetch invitation details (only when authenticated)
  useEffect(() => {
    let cancelled = false;

    async function fetchInvitation() {
      if (!invitationId) {
        setError("Keine Einladungs-ID angegeben.");
        setIsLoading(false);
        return;
      }

      // Only fetch if user is authenticated (API requires auth)
      if (!session?.user) {
        // Not authenticated - use URL params instead
        if (emailFromUrl) {
          // We have email from URL, can show the signup form
          setInvitation({
            id: invitationId,
            email: emailFromUrl,
            status: "pending",
            organizationId: "", // Will be set after accepting
            role: roleFromUrl ?? "member",
            organizationName: orgNameFromUrl ?? undefined,
          });
        }
        setIsLoading(false);
        return;
      }

      setIsLoading(true);
      setError(null);

      try {
        const result = await authClient.organization.getInvitation({
          query: { id: invitationId },
        });

        if (cancelled) return;

        if (result.error) {
          if (result.error.status === 404) {
            setError("Diese Einladung wurde nicht gefunden.");
          } else if (result.error.code === "NOT_AUTHENTICATED") {
            // Shouldn't happen since we check session above, but handle gracefully
            setError(null);
            if (emailFromUrl) {
              setInvitation({
                id: invitationId,
                email: emailFromUrl,
                status: "pending",
                organizationId: "",
                role: roleFromUrl ?? "member",
                organizationName: orgNameFromUrl ?? undefined,
              });
            }
          } else {
            setError(
              result.error.message ??
                "Beim Laden der Einladung ist ein Fehler aufgetreten.",
            );
          }
          if (result.error.code !== "NOT_AUTHENTICATED") {
            setInvitation(null);
          }
          return;
        }

        if (!result.data) {
          setError("Einladungsdaten nicht verfügbar.");
          setInvitation(null);
          return;
        }

        // Check invitation status (cast to string for flexible checking)
        const status = result.data.status as string;
        if (status === "accepted") {
          setError("Diese Einladung wurde bereits angenommen.");
          setInvitation(null);
          return;
        }
        if (status === "canceled" || status === "cancelled") {
          setError("Diese Einladung wurde zurückgezogen.");
          setInvitation(null);
          return;
        }
        if (status === "expired") {
          setError("Diese Einladung ist abgelaufen.");
          setInvitation(null);
          return;
        }
        if (status === "rejected") {
          setError("Diese Einladung wurde abgelehnt.");
          setInvitation(null);
          return;
        }

        setInvitation({
          id: result.data.id,
          email: result.data.email,
          status: result.data.status,
          organizationId: result.data.organizationId,
          role: result.data.role,
          organizationName: orgNameFromUrl ?? undefined,
        });
      } catch (err) {
        if (cancelled) return;

        console.error("Error fetching invitation:", err);
        setError("Beim Laden der Einladung ist ein Fehler aufgetreten.");
        setInvitation(null);
      } finally {
        if (!cancelled) {
          setIsLoading(false);
        }
      }
    }

    // Wait for session loading to complete before fetching
    if (!isSessionLoading) {
      void fetchInvitation();
    }

    return () => {
      cancelled = true;
    };
  }, [
    invitationId,
    session,
    isSessionLoading,
    emailFromUrl,
    orgNameFromUrl,
    roleFromUrl,
  ]);

  // Auto-accept if user is logged in with matching email
  const handleAutoAccept = useCallback(async () => {
    if (!invitation || !session?.user) return;

    // Check if logged in with different email
    if (session.user.email !== invitation.email) {
      return; // Will show email mismatch UI
    }

    setIsAccepting(true);

    try {
      const acceptResult = await authClient.organization.acceptInvitation({
        invitationId: invitation.id,
      });

      if (acceptResult.error) {
        toastError("Fehler beim Beitreten der Organisation.");
        console.error("Accept invitation error:", acceptResult.error);
        setIsAccepting(false);
        return;
      }

      // Set the organization as active
      const orgId = acceptResult.data?.invitation?.organizationId;
      if (orgId) {
        await authClient.organization.setActive({
          organizationId: orgId,
        });
      }

      toastSuccess("Willkommen in der Organisation!");

      setTimeout(() => {
        router.push("/dashboard");
        router.refresh();
      }, 1000);
    } catch (err) {
      console.error("Auto-accept error:", err);
      toastError("Fehler beim Annehmen der Einladung.");
      setIsAccepting(false);
    }
  }, [invitation, session, router, toastSuccess, toastError]);

  // Trigger auto-accept when session and invitation are ready
  useEffect(() => {
    if (
      !isSessionLoading &&
      !isLoading &&
      invitation &&
      session?.user?.email === invitation.email
    ) {
      void handleAutoAccept();
    }
  }, [isSessionLoading, isLoading, session, invitation, handleAutoAccept]);

  // Loading state
  if (isLoading || isSessionLoading) {
    return <Loading fullPage={false} />;
  }

  // Auto-accepting state (logged in with matching email)
  if (
    isAccepting ||
    (invitation && session?.user?.email === invitation.email)
  ) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-gray-50 p-4">
        <div className="w-full max-w-md rounded-2xl border border-gray-200/50 bg-white/90 p-8 shadow-sm backdrop-blur-sm">
          <div className="flex flex-col items-center gap-4 text-center">
            <Loading fullPage={false} />
            <p className="text-sm text-gray-600">
              Du wirst zur Organisation hinzugefügt...
            </p>
          </div>
        </div>
      </div>
    );
  }

  // No email available and not authenticated
  if (!invitation && !emailFromUrl && !session?.user && !error) {
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
              Einladung annehmen
            </h1>
          </div>
          <div className="space-y-4">
            <div className="rounded-xl border border-blue-200/50 bg-blue-50/50 p-4">
              <div className="flex items-start gap-3">
                <svg
                  className="mt-0.5 h-5 w-5 flex-shrink-0 text-blue-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
                <div className="text-sm text-blue-700">
                  <p className="font-medium">Anmeldung erforderlich</p>
                  <p className="mt-1">
                    Bitte melde dich an oder erstelle ein Konto, um diese
                    Einladung anzunehmen.
                  </p>
                </div>
              </div>
            </div>
            <div className="flex flex-col gap-3 sm:flex-row">
              <Link
                href="/"
                className="flex-1 rounded-xl bg-gray-900 py-3 text-center text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800"
              >
                Zur Anmeldung
              </Link>
              <Link
                href="/signup"
                className="flex-1 rounded-xl border border-gray-300 bg-white py-3 text-center text-sm font-semibold text-gray-700 shadow-sm transition-all duration-200 hover:bg-gray-50"
              >
                Konto erstellen
              </Link>
            </div>
          </div>
        </div>
      </div>
    );
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
            Einladung annehmen
          </h1>
          <p className="text-sm text-gray-600">
            Erstelle dein Konto, um der Organisation beizutreten.
          </p>
        </div>

        {/* Error state */}
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
                oder dich über{" "}
                <Link
                  href="/"
                  className="font-medium text-gray-900 underline hover:text-gray-700"
                >
                  die Startseite
                </Link>{" "}
                anmelden.
              </p>
            </div>
          </div>
        )}

        {/* Email mismatch state */}
        {!error &&
          invitation &&
          session?.user &&
          session.user.email !== invitation.email && (
            <div className="space-y-4">
              <div className="rounded-xl border border-yellow-200/50 bg-yellow-50/50 p-4">
                <div className="flex items-start gap-3">
                  <svg
                    className="mt-0.5 h-5 w-5 flex-shrink-0 text-yellow-600"
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
                  <div className="text-sm text-yellow-700">
                    <p className="font-medium">E-Mail-Adresse stimmt nicht</p>
                    <p className="mt-1">
                      Diese Einladung ist für{" "}
                      <span className="font-semibold">{invitation.email}</span>.
                      Du bist als{" "}
                      <span className="font-semibold">
                        {session.user.email}
                      </span>{" "}
                      angemeldet.
                    </p>
                  </div>
                </div>
              </div>
              <div className="flex flex-col gap-3 sm:flex-row">
                <button
                  onClick={async () => {
                    await authClient.signOut();
                    router.refresh();
                  }}
                  className="flex-1 rounded-xl border border-gray-300 bg-white py-3 text-sm font-semibold text-gray-700 shadow-sm transition-all duration-200 hover:bg-gray-50"
                >
                  Abmelden & neu registrieren
                </button>
                <Link
                  href="/dashboard"
                  className="flex-1 rounded-xl bg-gray-900 py-3 text-center text-sm font-semibold text-white shadow-lg transition-all duration-200 hover:bg-gray-800"
                >
                  Zum Dashboard
                </Link>
              </div>
            </div>
          )}

        {/* Signup form state (not logged in, have invitation data) */}
        {!error && invitation && !session?.user && (
          <BetterAuthInvitationForm
            invitationId={invitation.id}
            email={invitation.email}
            organizationName={invitation.organizationName}
            role={invitation.role}
          />
        )}
      </div>
    </div>
  );
}

export default function AcceptInvitationPage() {
  return (
    <Suspense fallback={<LoadingState />}>
      <AcceptInvitationContent />
    </Suspense>
  );
}
