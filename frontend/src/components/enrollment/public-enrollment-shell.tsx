"use client";

import Image from "next/image";
import Link from "next/link";
import { ArrowLeft, Check, ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import { loginImageSrc, type TenantInfo } from "~/lib/tenant-api";
import { LanguageSwitcher } from "~/components/parent/language-switcher";

export function PublicEnrollmentBrand({
  tenant,
}: {
  readonly tenant: TenantInfo | null;
}) {
  const t = useTranslations("enrollmentPublic");
  const schoolName = tenant?.name ?? t("fallbackSchool");
  const logoUrl = getTenantLogoUrl(tenant);
  const initials = schoolName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");

  return (
    <div className="flex min-w-0 items-center gap-3">
      <div className="moto-content-surface flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-xl border shadow-sm sm:h-12 sm:w-12">
        {logoUrl ? (
          <Image
            src={logoUrl}
            alt={`${schoolName} Logo`}
            width={48}
            height={48}
            className="h-full w-full object-contain p-1.5"
            unoptimized
          />
        ) : (
          <span className="text-sm font-bold text-gray-900">
            {initials || "OGS"}
          </span>
        )}
      </div>
      <div className="min-w-0 flex-1">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("brandEyebrow")}
        </p>
        <p className="truncate text-lg font-semibold text-gray-900">
          {schoolName}
        </p>
      </div>
    </div>
  );
}

function getTenantLogoUrl(tenant: TenantInfo | null): string | null {
  const explicitLogo =
    typeof tenant?.settings.logoUrl === "string" && tenant.settings.logoUrl
      ? tenant.settings.logoUrl
      : null;
  if (explicitLogo) {
    return explicitLogo;
  }

  const loginImage =
    typeof tenant?.settings.loginImageUrl === "string" &&
    tenant.settings.loginImageUrl
      ? tenant.settings.loginImageUrl
      : null;
  if (!loginImage) {
    return null;
  }
  if (loginImage.startsWith("/uploads/login-images/")) {
    return loginImageSrc(loginImage);
  }
  return loginImage;
}

export function PublicEnrollmentSteps({
  current,
}: {
  readonly current: "phase" | "form" | "done";
}) {
  const t = useTranslations("enrollmentPublic");
  const steps = [
    { id: "phase", label: t("steps.phase") },
    { id: "form", label: t("steps.form") },
    { id: "done", label: t("steps.done") },
  ] as const;
  const currentIndex = steps.findIndex((step) => step.id === current);

  return (
    <ol
      className="flex shrink-0 items-center gap-1.5 text-xs font-semibold text-gray-500 sm:gap-2"
      aria-label={t("steps.aria")}
    >
      {steps.map((step, index) => {
        const done = index < currentIndex || current === "done";
        const active = index === currentIndex && current !== "done";
        return (
          <li key={step.id} className="flex items-center gap-2">
            <span
              className={`flex h-7 w-7 items-center justify-center rounded-full border sm:h-8 sm:w-8 ${
                done
                  ? "border-moto-green/30 bg-moto-green/15 text-moto-green-strong"
                  : active
                    ? "border-gray-900 bg-gray-900 text-white"
                    : "border-gray-200 bg-white text-gray-400"
              }`}
            >
              {done ? (
                <Check className="h-4 w-4" aria-hidden="true" />
              ) : (
                index + 1
              )}
            </span>
            <span
              className={
                active || done
                  ? "hidden text-gray-900 sm:inline"
                  : "hidden sm:inline"
              }
            >
              {step.label}
            </span>
            {index < steps.length - 1 && (
              <ChevronRight
                className="h-3.5 w-3.5 text-gray-300 sm:h-4 sm:w-4"
                aria-hidden="true"
              />
            )}
          </li>
        );
      })}
    </ol>
  );
}

/** Compact language switcher matching the parent portal header. Lives in the
 * enrollment page header row (next to the step indicator) on branded pages, or
 * standalone top-right on pages without a header (see PublicEnrollmentPageShell). */
export function PublicEnrollmentLocaleSwitcher() {
  return <LanguageSwitcher compact />;
}

export function PublicEnrollmentBackLink({
  href,
  children,
}: {
  readonly href: string;
  readonly children: React.ReactNode;
}) {
  return (
    <Link
      href={href}
      className="inline-flex items-center gap-2 rounded-lg px-2 py-1 text-sm font-semibold text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      {children}
    </Link>
  );
}

export function PublicEnrollmentPageShell({
  children,
  withInlineSwitcher = false,
}: {
  readonly children: React.ReactNode;
  /** When the page renders its own header (brand row), it embeds the locale
   * switcher there and sets this to true so the shell doesn't render a second
   * one. Pages without a header (e.g. the submitted confirmation) leave it
   * false and get a compact switcher in a standalone top-right corner. */
  readonly withInlineSwitcher?: boolean;
}) {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen overflow-x-hidden">
      <div className="relative mx-auto w-full max-w-7xl px-4 py-4 sm:px-6 sm:py-6 lg:px-8">
        {!withInlineSwitcher && (
          <div className="mb-4 flex justify-end">
            <PublicEnrollmentLocaleSwitcher />
          </div>
        )}
        {children}
      </div>
    </main>
  );
}

export function PublicInfoCard({
  icon,
  title,
  children,
}: {
  readonly icon: React.ReactNode;
  readonly title: string;
  readonly children: React.ReactNode;
}) {
  return (
    <div className="moto-content-surface rounded-xl border p-4 shadow-sm">
      <div className="flex items-start gap-3">
        <span
          className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border text-gray-600 shadow-sm"
          aria-hidden="true"
        >
          {icon}
        </span>
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
          <div className="mt-1 text-sm leading-6 break-words text-gray-600">
            {children}
          </div>
        </div>
      </div>
    </div>
  );
}
