import Image from "next/image";
import Link from "next/link";
import { GuardianInvitationAcceptForm } from "~/components/auth/guardian-invitation-accept-form";
import {
  validateGuardianInvitation,
  type GuardianInvitationValidation,
} from "~/lib/guardian-invitation-api";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AcceptGuardianInvitePage" });

interface PageProps {
  params: Promise<{ token: string }>;
}

interface ValidationOutcome {
  invitation: GuardianInvitationValidation | null;
  errorMessage: string | null;
}

async function fetchInvitationServer(
  token: string,
): Promise<ValidationOutcome> {
  // Server-side fetch — bypasses /api/guardian-invitations route handler
  // because the page itself runs on the server. Uses the same backend URL.
  try {
    const response = await fetch(
      `${getServerApiUrl()}/auth/guardian-invitations/${encodeURIComponent(token)}`,
      { cache: "no-store" },
    );
    if (!response.ok) {
      if (response.status === 410) {
        return {
          invitation: null,
          errorMessage:
            "Diese Einladung ist abgelaufen oder wurde bereits verwendet.",
        };
      }
      if (response.status === 404) {
        return {
          invitation: null,
          errorMessage: "Wir konnten diese Einladung nicht finden.",
        };
      }
      return {
        invitation: null,
        errorMessage: "Beim Laden der Einladung ist ein Fehler aufgetreten.",
      };
    }

    const raw = (await response.json()) as {
      data?: {
        email: string;
        first_name?: string;
        last_name?: string;
        expires_at: string;
      };
      email?: string;
      first_name?: string;
      last_name?: string;
      expires_at?: string;
    };

    const payload = raw.data ?? raw;
    if (!payload?.email || !payload?.expires_at) {
      return {
        invitation: null,
        errorMessage:
          "Die Einladungsdaten sind unvollständig. Bitte fordere eine neue Einladung an.",
      };
    }

    return {
      invitation: {
        email: payload.email,
        firstName: payload.first_name,
        lastName: payload.last_name,
        expiresAt: payload.expires_at,
      },
      errorMessage: null,
    };
  } catch (error) {
    logger.error("guardian_invitation_validation_failed_server", {
      error: error instanceof Error ? error.message : String(error),
    });
    return {
      invitation: null,
      errorMessage: "Beim Laden der Einladung ist ein Fehler aufgetreten.",
    };
  }
}

// Suppress the unused-import warning when the route runs server-only.
// validateGuardianInvitation is exported for client-side reuse but we
// validate server-side here so the form renders without a flash of loading.
void validateGuardianInvitation;

export default async function AcceptGuardianInvitePage({ params }: PageProps) {
  const { token } = await params;
  const { invitation, errorMessage } = token
    ? await fetchInvitationServer(token)
    : {
        invitation: null,
        errorMessage: "Kein Einladungstoken angegeben.",
      };

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
            Willkommen im Eltern-Portal
          </h1>
          <p className="text-sm text-gray-600">
            Bitte bestätige deine Einladung und lege dein persönliches Passwort
            fest.
          </p>
        </div>

        {errorMessage && (
          <div className="space-y-4">
            <div className="rounded-xl border border-red-200/50 bg-red-50/50 p-4">
              <p className="text-sm text-red-700">{errorMessage}</p>
            </div>
            <div className="text-center text-sm text-gray-600">
              <p>
                Du kannst eine neue Einladung bei der Schule anfordern oder dich
                über&nbsp;
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

        {!errorMessage && invitation && token && (
          <GuardianInvitationAcceptForm token={token} invitation={invitation} />
        )}
      </div>
    </div>
  );
}
