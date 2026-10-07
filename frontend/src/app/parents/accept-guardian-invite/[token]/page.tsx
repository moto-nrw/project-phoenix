import Image from "next/image";
import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { AuthShell } from "~/components/auth/auth-shell";
import { buildParentAuthShellCopy } from "~/components/auth/parent-auth-shell-copy";
import {
  GuardianInvitationAcceptForm,
  GuardianInvitationLoadError,
  type GuardianInvitationLoadFailure,
} from "~/components/auth/guardian-invitation-accept-form";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
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
  failure: GuardianInvitationLoadFailure | null;
}

/**
 * The failure as code and status for the shared error path (#2518). An
 * uncoded 410 or 404 from this endpoint still means the invitation.
 */
async function loadFailure(
  response: Response,
): Promise<GuardianInvitationLoadFailure> {
  let body: { code?: unknown; instance?: unknown } | undefined;
  try {
    body = (await response.json()) as typeof body;
  } catch {
    // No JSON envelope: status alone decides.
    body = undefined;
  }
  const wireCode =
    typeof body?.code === "string" && body.code ? body.code : undefined;
  const code =
    wireCode ??
    (response.status === 410
      ? "identity.invitation_expired"
      : response.status === 404
        ? "identity.invitation_not_found"
        : undefined);
  return {
    status: response.status,
    code,
    requestId: typeof body?.instance === "string" ? body.instance : undefined,
  };
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
      return { invitation: null, failure: await loadFailure(response) };
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
      logger.error("guardian_invitation_validation_incomplete", {
        has_email: Boolean(payload?.email),
        has_expiry: Boolean(payload?.expires_at),
      });
      return {
        invitation: null,
        failure: { status: 500, code: "general.server" },
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
      failure: null,
    };
  } catch (error) {
    logger.error("guardian_invitation_validation_failed_server", {
      error: error instanceof Error ? error.message : String(error),
    });
    return {
      invitation: null,
      failure: { status: 503, code: "general.unavailable" },
    };
  }
}

void validateGuardianInvitation;

export default async function AcceptGuardianInvitePage({ params }: PageProps) {
  const t = await getTranslations("guardianInvite");
  const tAuthShell = await getTranslations("parentAuthShell");
  const { token } = await params;
  const { invitation, failure } = token
    ? await fetchInvitationServer(token)
    : {
        invitation: null,
        failure: { code: "identity.invitation_not_found" },
      };

  const schoolName = invitation?.schoolName?.trim() || t("fallbackSchool");
  const schoolLogoUrl = normalizeSchoolLogoUrl(
    invitation?.schoolLogoUrl?.trim() || null,
  );
  const testimonialPanelCopy = buildParentAuthShellCopy(tAuthShell);

  return (
    <AuthShell
      eyebrow={t("eyebrow")}
      eyebrowClassName="text-moto-green"
      title={t("title")}
      subtitle={t("subtitle", { school: schoolName })}
      variant="parents"
      brand={
        <SchoolBrandMark
          schoolName={schoolName}
          schoolLogoUrl={schoolLogoUrl}
        />
      }
      formMaxWidth="max-w-[32rem]"
      footer={<LanguageSwitcher />}
      testimonialPanelCopy={testimonialPanelCopy}
    >
      {failure && (
        <div className="space-y-4">
          <GuardianInvitationLoadError failure={failure} />
          <div className="text-center text-sm text-gray-600">
            <p>
              {t("errorHelpBefore")}{" "}
              <Link
                href="/"
                className="font-medium text-gray-900 underline hover:text-gray-700"
              >
                {t("startPage")}
              </Link>{" "}
              {t("errorHelpAfter")}
            </p>
          </div>
        </div>
      )}

      {!failure && invitation && token && (
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
    <div className="moto-content-surface flex h-20 min-w-20 items-center justify-center overflow-hidden rounded-2xl border px-4 shadow-sm">
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
