"use client";

import { useState } from "react";
import { useSession } from "next-auth/react";
import { useTenant } from "./tenant-provider";
import { clearSessionCache } from "~/lib/session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantSwitcher" });

interface TenantOption {
  slug: string;
  name: string;
}

/**
 * Dropdown component for switching between tenants.
 * Only shown when the user has access to multiple tenants.
 *
 * On switch:
 * 1. Calls POST /auth/switch-tenant to get new tokens
 * 2. Clears session cache
 * 3. Hard-navigates to the new subdomain (clean state)
 */
export function TenantSwitcher({ tenants }: { tenants: TenantOption[] }) {
  const { tenantSlug } = useTenant();
  const { data: session } = useSession();
  const [isSwitching, setIsSwitching] = useState(false);

  // Only show if multiple tenants available
  if (tenants.length <= 1) return null;

  const handleSwitch = async (targetSlug: string) => {
    if (targetSlug === tenantSlug || isSwitching) return;

    setIsSwitching(true);

    try {
      const token = session?.user?.token;
      if (!token) {
        logger.error("switch_tenant_no_token", {});
        return;
      }

      const response = await fetch("/api/auth/switch-tenant", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ tenant_slug: targetSlug }),
      });

      if (!response.ok) {
        logger.error("switch_tenant_failed", { status: response.status });
        return;
      }

      // Clear caches before navigating
      clearSessionCache();

      // Hard navigation to new subdomain = clean React state
      const tenantDomain = process.env.NEXT_PUBLIC_TENANT_DOMAIN ?? "localhost";
      const port = globalThis.window.location.port
        ? `:${globalThis.window.location.port}`
        : "";
      const protocol = globalThis.window.location.protocol;
      globalThis.window.location.href = `${protocol}//${targetSlug}.${tenantDomain}${port}/`;
    } catch (error) {
      logger.error("switch_tenant_error", {
        error: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setIsSwitching(false);
    }
  };

  return (
    <div className="flex items-center gap-2">
      <select
        value={tenantSlug}
        onChange={(e) => void handleSwitch(e.target.value)}
        disabled={isSwitching}
        className="rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-gray-700 shadow-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        {tenants.map((t) => (
          <option key={t.slug} value={t.slug}>
            {t.name}
          </option>
        ))}
      </select>
      {isSwitching && (
        <span className="text-xs text-gray-500">Wechseln...</span>
      )}
    </div>
  );
}
