/**
 * Root page for the bare domain (no subdomain).
 * Shown when a user visits e.g. localhost:3000 without a subdomain.
 * In production, this redirects to the main school subdomain.
 * During development, it shows a tenant selector.
 */
"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useState, useEffect } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { AuthShell, MotoIconBrand } from "~/components/auth/auth-shell";
import { CustomSelect } from "~/components/ui/custom-select";
import { listAllTenants } from "~/lib/tenant-api";
import type { TenantListResult, TenantSummary } from "~/lib/tenant-api";
import { createLogger } from "~/lib/logger";
import { clientEnv } from "~/env.client";
import { Button } from "~/components/ui/button";

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

    const tenantDomain = clientEnv.NEXT_PUBLIC_TENANT_DOMAIN;
    const portSuffix = window.location.port ? `:${window.location.port}` : "";
    const host = `${tenant.subdomain}.${tenantDomain}${portSuffix}`;

    window.location.href = `${window.location.protocol}//${host}/`;
  };

  const placeholder = loading ? "Laden..." : "Bitte auswählen...";

  return (
    <AuthShell
      eyebrow="Einrichtung auswählen"
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
          <LoadErrorAlert error="Backend nicht erreichbar. Bitte versuchen Sie es später erneut." />
        ) : !loading && tenants.length === 0 ? (
          <EmptyState
            variant="compact"
            description="Aktuell sind keine Einrichtungen verfügbar."
          />
        ) : (
          <>
            <div className="text-left">
              <CustomSelect
                id="tenant-select"
                label="Einrichtung"
                value={selectedSlug}
                disabled={loading || isNavigating}
                onChange={setSelectedSlug}
                placeholder={placeholder}
                options={[
                  {
                    value: "",
                    label: placeholder,
                  },
                  ...tenants.map((tenant) => ({
                    value: tenant.slug,
                    label: tenant.name,
                  })),
                ]}
              />
            </div>

            <Button
              type="button"
              disabled={!selectedSlug || isNavigating}
              isLoading={isNavigating}
              loadingText="OGS wird geladen..."
              onClick={handleNavigate}
              variant="primary"
              size="field"
              className="w-full gap-2"
            >
              Weiter
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </Button>
          </>
        )}
      </div>
    </AuthShell>
  );
}
