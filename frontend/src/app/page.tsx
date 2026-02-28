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

function TenantCardSkeleton() {
  return (
    <div className="animate-pulse rounded-xl border border-gray-200 bg-white/60 p-5">
      <div className="mb-2 h-5 w-2/3 rounded bg-gray-200" />
      <div className="h-4 w-1/2 rounded bg-gray-100" />
    </div>
  );
}

function TenantCard({ tenant }: { readonly tenant: TenantInfo }) {
  const port = typeof window !== "undefined" ? window.location.port : "3000";
  const host = `${tenant.subdomain}.localhost${port ? `:${port}` : ""}`;
  const href = `http://${host}/`;

  return (
    <a
      href={href}
      className="group block rounded-xl border border-gray-200 bg-white/60 p-5 transition-all duration-300 hover:border-gray-300 hover:bg-white/80 hover:shadow-lg active:scale-95"
    >
      <p className="text-lg font-semibold text-gray-900 transition-colors group-hover:text-gray-700">
        {tenant.name}
      </p>
      <p className="mt-1 text-sm text-gray-500">{host}</p>
    </a>
  );
}

export default function RootPage() {
  const [tenants, setTenants] = useState<TenantInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [isFallback, setIsFallback] = useState(false);

  useEffect(() => {
    listAllTenants()
      .then((result) => {
        if (result.length > 0) {
          setTenants(result);
        } else {
          logger.warn("tenant_list_empty_or_failed, using fallback");
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

  return (
    <div className="flex min-h-screen flex-col items-center justify-center p-4">
      <div className="mx-auto w-full max-w-lg rounded-2xl bg-white/80 p-10 text-center shadow-xl backdrop-blur-md">
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
        <h1 className="mb-2 bg-gradient-to-r from-[#5080d8] to-[#83cd2d] bg-clip-text text-3xl font-bold text-transparent">
          Schule auswählen
        </h1>
        <p className="mb-8 text-sm text-gray-500">Entwicklungsumgebung</p>

        {/* Tenant list */}
        <div className="space-y-3">
          {loading ? (
            <>
              <TenantCardSkeleton />
              <TenantCardSkeleton />
            </>
          ) : (
            tenants.map((tenant) => (
              <TenantCard key={tenant.slug} tenant={tenant} />
            ))
          )}
        </div>

        {/* Fallback notice */}
        {isFallback && !loading && (
          <p className="mt-4 text-xs text-gray-400">
            Backend nicht erreichbar — statische Links werden angezeigt.
          </p>
        )}
      </div>
    </div>
  );
}
