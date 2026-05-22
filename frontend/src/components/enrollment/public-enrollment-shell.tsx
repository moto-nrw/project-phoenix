"use client";

import Image from "next/image";
import Link from "next/link";
import { ArrowLeft, Check, ChevronRight } from "lucide-react";
import type { TenantInfo } from "~/lib/tenant-api";

export function PublicEnrollmentBrand({
  tenant,
}: {
  readonly tenant: TenantInfo | null;
}) {
  const schoolName = tenant?.name ?? "Ihre OGS";
  const logoUrl =
    typeof tenant?.settings.logoUrl === "string" && tenant.settings.logoUrl
      ? tenant.settings.logoUrl
      : null;
  const initials = schoolName
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join("");

  return (
    <div className="flex items-center gap-3">
      <div className="moto-content-surface flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-2xl border shadow-sm">
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
      <div className="min-w-0">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          Online-Anmeldung
        </p>
        <p className="truncate text-lg font-semibold text-gray-900">
          {schoolName}
        </p>
      </div>
    </div>
  );
}

export function PublicEnrollmentSteps({
  current,
}: {
  readonly current: "phase" | "form" | "done";
}) {
  const steps = [
    { id: "phase", label: "Auswahl" },
    { id: "form", label: "Formular" },
    { id: "done", label: "Bestätigung" },
  ] as const;
  const currentIndex = steps.findIndex((step) => step.id === current);

  return (
    <ol className="flex items-center gap-2 text-xs font-semibold text-gray-500">
      {steps.map((step, index) => {
        const done = index < currentIndex || current === "done";
        const active = index === currentIndex && current !== "done";
        return (
          <li key={step.id} className="flex items-center gap-2">
            <span
              className={`flex h-8 w-8 items-center justify-center rounded-full border ${
                done
                  ? "border-[#83CD2D]/30 bg-[#83CD2D]/15 text-[#5A8E1F]"
                  : active
                    ? "border-gray-900 bg-gray-900 text-white"
                    : "border-gray-200 bg-white text-gray-400"
              }`}
            >
              {done ? <Check className="h-4 w-4" /> : index + 1}
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
              <ChevronRight className="h-4 w-4 text-gray-300" />
            )}
          </li>
        );
      })}
    </ol>
  );
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
      <ArrowLeft className="h-4 w-4" />
      {children}
    </Link>
  );
}

export function PublicEnrollmentPageShell({
  children,
}: {
  readonly children: React.ReactNode;
}) {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen min-h-screen">
      <div className="relative mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 lg:px-8">
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
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <div className="flex items-start gap-3">
        <span className="moto-content-surface flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border text-gray-600 shadow-sm">
          {icon}
        </span>
        <div>
          <h3 className="text-sm font-semibold text-gray-900">{title}</h3>
          <div className="mt-1 text-sm leading-6 text-gray-600">{children}</div>
        </div>
      </div>
    </div>
  );
}
