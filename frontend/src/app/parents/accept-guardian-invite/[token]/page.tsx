import Image from "next/image";
import Link from "next/link";
import {
  CalendarClock,
  CheckCircle2,
  KeyRound,
  Mail,
  ShieldCheck,
  UserRound,
  type LucideIcon,
} from "lucide-react";
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

  const guardianFullName = invitation
    ? [invitation.firstName, invitation.lastName]
        .filter((value) => value && value.trim().length > 0)
        .join(" ")
        .trim()
    : "";
  const schoolName = invitation?.schoolName?.trim() || "deiner Schule";
  const schoolLogoUrl = normalizeSchoolLogoUrl(
    invitation?.schoolLogoUrl?.trim() || null,
  );

  return (
    <main className="min-h-screen overflow-x-hidden bg-gray-50 text-gray-900 lg:grid lg:grid-cols-2 lg:bg-white">
      <section className="relative flex min-h-screen items-center justify-center bg-gray-50 px-4 py-5 sm:px-6 lg:bg-white lg:px-10 lg:py-8 xl:px-20">
        <div
          className="moto-dotted-background absolute inset-0 lg:hidden"
          aria-hidden="true"
        />
        <div className="relative z-10 w-full max-w-xl rounded-2xl border border-gray-200/70 bg-white/95 p-5 shadow-xl backdrop-blur-sm sm:p-8 lg:max-w-md lg:border-0 lg:bg-transparent lg:p-0 lg:shadow-none lg:backdrop-blur-none">
          <div className="mb-7 space-y-5 text-center lg:text-left">
            <SchoolBrandHeader
              schoolName={schoolName}
              schoolLogoUrl={schoolLogoUrl}
            />
            <div>
              <h1 className="text-2xl font-semibold text-gray-900 sm:text-3xl">
                Willkommen im Eltern-Portal
              </h1>
              <p className="mt-2 max-w-md text-sm leading-6 text-gray-600">
                Bestätige deine Einladung für {schoolName} und lege dein
                persönliches Passwort fest.
              </p>
            </div>
          </div>

          {errorMessage && (
            <div className="space-y-4">
              <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4">
                <p className="text-sm text-[#CC2626]">{errorMessage}</p>
              </div>
              <div className="text-center text-sm text-gray-600 lg:text-left">
                <p>
                  Du kannst eine neue Einladung bei der Schule anfordern oder
                  dich über{" "}
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
            <GuardianInvitationAcceptForm
              token={token}
              invitation={invitation}
            />
          )}

          <PoweredByMoto />
        </div>
      </section>

      <aside className="moto-dotted-background relative hidden min-h-screen overflow-hidden border-l border-gray-200 bg-gray-50 lg:flex lg:items-center lg:justify-center lg:px-10 lg:py-10">
        <div className="pointer-events-none absolute inset-y-0 left-0 w-28 bg-gradient-to-r from-white to-transparent" />
        <div className="relative z-10 w-full max-w-lg space-y-4">
          <InviteContextCard
            icon={ShieldCheck}
            eyebrow="Elternzugang"
            title={`Ein Zugang für ${schoolName}`}
            text="Nach der Bestätigung meldest du dich im Eltern-Portal an und siehst dort Status, Rückmeldungen und neue Anmeldungen."
          />

          {invitation && (
            <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
              <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                Einladung
              </p>
              <div className="mt-4 grid gap-3">
                <ContextInfoRow
                  icon={Mail}
                  label="E-Mail"
                  value={invitation.email}
                />
                <ContextInfoRow
                  icon={UserRound}
                  label="Name"
                  value={guardianFullName || "Noch nicht gesetzt"}
                />
                <ContextInfoRow
                  icon={CalendarClock}
                  label="Gültig bis"
                  value={new Date(invitation.expiresAt).toLocaleDateString(
                    "de-DE",
                    {
                      day: "2-digit",
                      month: "2-digit",
                      year: "numeric",
                      hour: "2-digit",
                      minute: "2-digit",
                    },
                  )}
                />
              </div>
            </section>
          )}

          <InviteContextCard
            icon={KeyRound}
            eyebrow="Passwort"
            title="Sicher anlegen, später normal anmelden"
            text="Nutze gern den Vorschlag deines Passwortmanagers. Das Passwort kann eingeblendet werden, wenn du es prüfen möchtest."
          />
        </div>
      </aside>
    </main>
  );
}

function SchoolBrandHeader({
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
    <div className="flex flex-col items-center gap-3 lg:items-start">
      <div className="flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        {schoolLogoUrl ? (
          <Image
            src={schoolLogoUrl}
            alt={`${schoolName} Logo`}
            width={64}
            height={64}
            className="h-full w-full object-contain p-2"
            priority
            unoptimized
          />
        ) : (
          <span className="text-lg font-semibold text-gray-900">
            {initials || "OGS"}
          </span>
        )}
      </div>
      <div>
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Eltern-Portal
        </p>
        <p className="mt-1 text-base font-semibold text-gray-900">
          {schoolName}
        </p>
      </div>
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

function PoweredByMoto() {
  return (
    <div className="mt-6 flex items-center justify-center gap-1.5 text-xs text-gray-500 lg:justify-start">
      <span>Unterstützt von</span>
      <span className="font-semibold text-gray-900">moto</span>
      <Image
        src="/images/moto_transparent.png"
        alt=""
        width={30}
        height={16}
        className="h-4 w-auto"
      />
    </div>
  );
}

function InviteContextCard({
  icon: Icon,
  eyebrow,
  title,
  text,
}: Readonly<{
  icon: LucideIcon;
  eyebrow: string;
  title: string;
  text: string;
}>) {
  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
      <div className="flex items-start gap-4">
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-900 text-white shadow-sm">
          <Icon className="h-5 w-5" aria-hidden="true" />
        </span>
        <div>
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            {eyebrow}
          </p>
          <h2 className="mt-1 text-lg font-semibold text-gray-900">{title}</h2>
          <p className="mt-2 text-sm leading-6 text-gray-600">{text}</p>
        </div>
      </div>
    </section>
  );
}

function ContextInfoRow({
  icon: Icon,
  label,
  value,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  value: string;
}>) {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-gray-100 bg-gray-50/70 px-3 py-2.5">
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-white text-gray-500 shadow-sm">
        <Icon className="h-4 w-4" aria-hidden="true" />
      </span>
      <div className="min-w-0">
        <p className="text-xs font-medium text-gray-500 uppercase">{label}</p>
        <p className="mt-0.5 truncate text-sm font-semibold text-gray-900">
          {value}
        </p>
      </div>
      <CheckCircle2
        className="ml-auto h-4 w-4 shrink-0 text-[#83CD2D]"
        aria-hidden="true"
      />
    </div>
  );
}
