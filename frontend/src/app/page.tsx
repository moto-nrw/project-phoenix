/**
 * Root page for the bare domain (no subdomain).
 * Shown when a user visits e.g. localhost:3000 without a subdomain.
 * In production, this redirects to the main school subdomain.
 * During development, it shows a tenant selector.
 */
"use client";

import { useState, useEffect } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import {
  AuthShell,
  MotoIconBrand,
  authPrimaryButtonClassName,
} from "~/components/auth/auth-shell";
import { CustomSelect } from "~/components/ui/custom-select";
import { listAllTenants } from "~/lib/tenant-api";
import type { TenantListResult, TenantSummary } from "~/lib/tenant-api";
import { createLogger } from "~/lib/logger";
import { env } from "~/env";

const logger = createLogger({ component: "RootPage" });

export default function RootPage() {
  const [tenants, setTenants] = useState<TenantSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [listStatus, setListStatus] =
    useState<TenantListResult["status"]>("ok");
  const [selectedSlug, setSelectedSlug] = useState("");
  const [isNavigating, setIsNavigating] = useState(false);

  useEffect(() => {
    void listAllTenants()
      .then((result) => {
        setListStatus(result.status);
        if (result.tenants.length > 0) {
          setTenants(result.tenants);
        } else if (result.status === "error") {
          logger.warn("tenant_list_fetch_failed");
        } else {
          logger.warn("tenant_list_empty");
        }
      })
      .finally(() => setLoading(false));
  }, []);

  const handleNavigate = () => {
    const tenant = tenants.find((t) => t.slug === selectedSlug);
    if (!tenant) return;

    setIsNavigating(true);

    const tenantDomain = env.NEXT_PUBLIC_TENANT_DOMAIN;
    const portSuffix = window.location.port ? `:${window.location.port}` : "";
    const host = `${tenant.subdomain}.${tenantDomain}${portSuffix}`;

    window.location.href = `${window.location.protocol}//${host}/`;
  };

  return (
    <AuthShell
      eyebrow="Einrichtung auswählen"
      eyebrowClassName="tracking-[0.08em] text-[#83CD2D]"
      title="Willkommen"
      subtitle="Wählen Sie Ihre Einrichtung aus."
      variant="tenant-select"
      brand={<MotoIconBrand />}
      footer={
        <p className="text-sm text-gray-500">
          Noch nicht registriert?{" "}
          <Link
            href="/start"
            className="font-medium text-gray-700 transition-colors hover:text-gray-900 hover:underline"
          >
            Los geht&apos;s
          </Link>
        </p>
      }
    >
      <div className="space-y-6">
        {!loading && listStatus === "error" ? (
          <p className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            Backend nicht erreichbar. Bitte versuchen Sie es später erneut.
          </p>
        ) : !loading && tenants.length === 0 ? (
          <p className="rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-sm text-gray-600">
            Aktuell sind keine Einrichtungen verfügbar.
          </p>
        ) : (
          <>
            <div className="text-left">
              <label
                id="tenant-select-label"
                htmlFor="tenant-select"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Einrichtung
              </label>
              <CustomSelect
                id="tenant-select"
                ariaLabelledBy="tenant-select-label"
                value={selectedSlug}
                disabled={loading || isNavigating}
                onChange={setSelectedSlug}
                placeholder={loading ? "Laden..." : "Bitte auswählen..."}
                options={[
                  {
                    value: "",
                    label: loading ? "Laden..." : "Bitte auswählen...",
                  },
                  ...tenants.map((tenant) => ({
                    value: tenant.slug,
                    label: tenant.name,
                  })),
                ]}
              />
            </div>

            <button
              type="button"
              disabled={!selectedSlug || isNavigating}
              onClick={handleNavigate}
              className={authPrimaryButtonClassName}
            >
              {isNavigating ? "OGS wird geladen..." : "Weiter"}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </button>
          </>
        )}
      </div>
    </AuthShell>
  );
}
