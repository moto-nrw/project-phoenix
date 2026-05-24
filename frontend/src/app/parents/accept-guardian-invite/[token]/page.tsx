import Image from "next/image";
import Link from "next/link";
import { AuthShell } from "~/components/auth/auth-shell";
import { GuardianInvitationAcceptForm } from "~/components/auth/guardian-invitation-accept-form";
import {
  validateGuardianInvitation,
  type GuardianInvitationValidation,
} from "~/lib/guardian-invitation-api";
import { loginImageSrc } from "~/lib/tenant-api";
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
  // Server-side fetch bypasses the route handler because this page runs on the server.
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
        school_name?: string;
        tenant_slug?: string;
        school_logo_url?: string;
      };
      email?: string;
      first_name?: string;
      last_name?: string;
      expires_at?: string;
      school_name?: string;
      tenant_slug?: string;
      school_logo_url?: string;
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
        schoolName: payload.school_name,
        tenantSlug: payload.tenant_slug,
        schoolLogoUrl: payload.school_logo_url,
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

void validateGuardianInvitation;

export default async function AcceptGuardianInvitePage({ params }: PageProps) {
  const { token } = await params;
  const { invitation, errorMessage } = token
    ? await fetchInvitationServer(token)
    : {
        invitation: null,
        errorMessage: "Kein Einladungstoken angegeben.",
      };

  const schoolName = invitation?.schoolName?.trim() || "deiner Schule";
  const schoolLogoUrl = normalizeSchoolLogoUrl(
    invitation?.schoolLogoUrl?.trim() || null,
  );

  return (
    <AuthShell
      eyebrow="Eltern-Portal"
      eyebrowClassName="text-[#83CD2D]"
      title="Willkommen"
      subtitle={`Bestätige deine Einladung für ${schoolName} und lege dein persönliches Passwort fest.`}
      variant="parents"
      brand={
        <SchoolBrandMark
          schoolName={schoolName}
          schoolLogoUrl={schoolLogoUrl}
        />
      }
      formMaxWidth="max-w-[32rem]"
    >
      {errorMessage && (
        <div className="space-y-4">
          <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4">
            <p className="text-sm text-[#CC2626]">{errorMessage}</p>
          </div>
          <div className="text-center text-sm text-gray-600">
            <p>
              Du kannst eine neue Einladung bei der Schule anfordern oder dich
              über{" "}
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

      {!errorMessage && invitation && token && (
        <GuardianInvitationAcceptForm token={token} invitation={invitation} />
      )}
    </AuthShell>
  );
}

function SchoolBrandMark({
  schoolName,
  schoolLogoUrl,
}: Readonly<{
  schoolName: string;
  schoolLogoUrl: string | null;
}>) {
  const initials = schoolName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");

  return (
    <div className="flex h-20 min-w-20 items-center justify-center overflow-hidden rounded-2xl border border-gray-200 bg-white px-4 shadow-sm">
      {schoolLogoUrl ? (
        <Image
          src={schoolLogoUrl}
          alt={`${schoolName} Logo`}
          width={160}
          height={80}
          className="max-h-16 w-auto object-contain"
          priority
          unoptimized
        />
      ) : (
        <span className="text-lg font-semibold text-gray-900">
          {initials || "OGS"}
        </span>
      )}
    </div>
  );
}

function normalizeSchoolLogoUrl(url: string | null): string | null {
  if (!url) {
    return null;
  }
  if (url.startsWith("/uploads/login-images/")) {
    return loginImageSrc(url);
  }
  return url;
}
