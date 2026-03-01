/**
 * Root page for the bare domain (no subdomain).
 * Shown when a user visits e.g. localhost:3000 without a subdomain.
 * In production, this redirects to the main school subdomain.
 * During development, it shows a tenant selector.
 */
"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import { listAllTenants } from "~/lib/tenant-api";
import type { TenantInfo } from "~/lib/tenant-api";
import { createLogger } from "~/lib/logger";
import { env } from "~/env";

const logger = createLogger({ component: "RootPage" });

/** Hardcoded fallback tenants when the backend is unreachable */
const FALLBACK_TENANTS: TenantInfo[] = [
  {
    tenantId: 0,
    slug: "school-a",
    name: "Testschule A",
    subdomain: "school-a",
    organizationId: 0,
    organizationName: "",
    settings: {},
  },
  {
    tenantId: 0,
    slug: "school-b",
    name: "Testschule B",
    subdomain: "school-b",
    organizationId: 0,
    organizationName: "",
    settings: {},
  },
];

export default function RootPage() {
  const [tenants, setTenants] = useState<TenantInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [isFallback, setIsFallback] = useState(false);
  const [selectedSlug, setSelectedSlug] = useState("");

  useEffect(() => {
    listAllTenants()
      .then((result) => {
        if (result.length > 0) {
          setTenants(result);
        } else {
          logger.warn("tenant_list_empty_or_failed");
          setTenants(FALLBACK_TENANTS);
          setIsFallback(true);
        }
      })
      .catch((error: unknown) => {
        logger.error("tenant_list_fetch_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
        setTenants(FALLBACK_TENANTS);
        setIsFallback(true);
      })
      .finally(() => setLoading(false));
  }, []);

  const handleNavigate = () => {
    const tenant = tenants.find((t) => t.slug === selectedSlug);
    if (!tenant) return;

    const tenantDomain = env.NEXT_PUBLIC_TENANT_DOMAIN ?? "localhost";
    const port = typeof window !== "undefined" ? window.location.port : "3000";
    const protocol =
      typeof window !== "undefined" ? window.location.protocol : "http:";
    const portSuffix = port ? `:${port}` : "";
    const host = `${tenant.subdomain}.${tenantDomain}${portSuffix}`;

    window.location.href = `${protocol}//${host}/`;
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto w-full max-w-2xl rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md">
        {/* Logo */}
        <div className="mb-8 flex justify-center">
          <Image
            src="/images/moto_transparent.png"
            alt="MOTO Logo"
            width={200}
            height={80}
            priority
          />
        </div>

        {/* Heading */}
        <h1 className="mb-2 bg-gradient-to-r from-[#5080d8] to-[#83cd2d] bg-clip-text text-4xl font-bold text-transparent md:text-5xl">
          Willkommen bei moto!
        </h1>
        <p className="mb-10 text-xl text-gray-700">
          Wählen Sie Ihre Einrichtung aus
        </p>

        {/* Tenant selector */}
        <div className="space-y-6">
          <div className="text-left">
            <label
              htmlFor="tenant-select"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Einrichtung
            </label>
            {loading ? (
              <div className="h-10 w-full animate-pulse rounded-lg bg-gray-200" />
            ) : (
              <div className="relative">
                <select
                  id="tenant-select"
                  value={selectedSlug}
                  onChange={(e) => setSelectedSlug(e.target.value)}
                  className="w-full appearance-none rounded-lg border-0 bg-white px-3 py-2.5 pr-10 text-gray-900 ring-1 ring-gray-200 transition-all focus:ring-2 focus:ring-gray-900 focus:outline-none"
                >
                  <option value="">Bitte auswählen...</option>
                  {tenants.map((tenant) => (
                    <option key={tenant.slug} value={tenant.slug}>
                      {tenant.name}
                    </option>
                  ))}
                </select>
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                  <svg
                    className="h-4 w-4 text-gray-500"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 9l-7 7-7-7"
                    />
                  </svg>
                </div>
              </div>
            )}
          </div>

          <div className="flex justify-center">
            <button
              type="button"
              disabled={!selectedSlug}
              onClick={handleNavigate}
              className="group relative overflow-hidden rounded-xl bg-gray-900 px-8 py-2.5 text-sm font-semibold text-white transition-all duration-200 hover:bg-gray-800 focus:ring-2 focus:ring-gray-900 focus:ring-offset-2 focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Weiter
            </button>
          </div>

          <p className="text-sm text-gray-500">
            Noch nicht registriert?{" "}
            <a
              href="mailto:kontakt@moto.nrw"
              className="font-medium text-gray-700 transition-colors hover:text-gray-900 hover:underline"
            >
              Kontaktieren Sie uns
            </a>
          </p>
        </div>

        {/* Fallback notice */}
        {isFallback && !loading && (
          <p className="mt-6 text-xs text-gray-400">
            Backend nicht erreichbar — statische Links werden angezeigt.
          </p>
        )}
      </div>
    </div>
  );
}
