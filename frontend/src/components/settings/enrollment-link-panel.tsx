"use client";

import { useEffect, useMemo, useState } from "react";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import type { SchemaTab } from "~/lib/settings-api";
import { createLogger } from "~/lib/logger";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";

const logger = createLogger({ component: "EnrollmentLinkPanel" });

interface Props {
  readonly tab: SchemaTab;
}

/**
 * EnrollmentLinkPanel surfaces the public parent-enrollment URL at the top
 * of the enrollment settings tab. Hidden when enrollment.enabled is false
 * so admins aren't tempted to share a link that 404s.
 *
 * URL composition mirrors useTenantRouter: subdomain mode keeps the host
 * as `{slug}.{baseDomain}` and uses bare paths; path mode prefixes the
 * tenant slug. We compute both with `window.location` so we don't have to
 * import the env or care which TENANT_DOMAIN is configured.
 */
export function EnrollmentLinkPanel({ tab }: Props) {
  const tenantSlug = useTenantSlugSafe();
  const [origin, setOrigin] = useState<string>("");
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">(
    "idle",
  );

  // Window access is client-only; defer until mount.
  useEffect(() => {
    if (typeof window !== "undefined") {
      setOrigin(window.location.origin);
    }
  }, []);

  const enabled = useMemo(() => {
    for (const cat of tab.categories) {
      for (const item of cat.items) {
        if (item.key === "enrollment.enabled") {
          return item.value === true;
        }
      }
    }
    return false;
  }, [tab]);

  const enrollUrl = useMemo(() => {
    if (!origin || !tenantSlug) return "";
    // Detect subdomain mode by checking whether the tenant slug is the
    // first label of the current host. Path mode otherwise.
    const inSubdomainMode =
      typeof window !== "undefined" &&
      window.location.hostname.startsWith(`${tenantSlug}.`);
    if (inSubdomainMode) {
      return `${origin}/enroll`;
    }
    return `${origin}/${tenantSlug}/enroll`;
  }, [origin, tenantSlug]);

  if (!enabled || !enrollUrl) {
    return null;
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(enrollUrl);
      setCopyState("copied");
      window.setTimeout(() => setCopyState("idle"), 2500);
    } catch (err) {
      logger.error("enrollment_link_copy_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setCopyState("error");
      window.setTimeout(() => setCopyState("idle"), 2500);
    }
  };

  return (
    <section className="moto-content-surface rounded-2xl border p-4 sm:p-6">
      <div className="flex flex-col gap-2">
        <ConceptSectionHeader
          // Geschwister im selben Container sind die SettingsCategory-Bloecke
          // mit h3, und auf Mobile ist der Tab-Titel im MobileBackHeader h2.
          level={3}
          title="Anmeldelink für Eltern"
          concept="enrollments"
          subtitle="Teilen Sie diesen Link mit Eltern, damit sie ihre Kinder anmelden können. Der Link ist öffentlich; ein Login ist nicht nötig."
        />
        <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center">
          <code className="moto-content-surface flex-1 truncate rounded-lg border px-3 py-2 font-mono text-xs text-gray-800">
            {enrollUrl}
          </code>
          <button
            type="button"
            onClick={() => void handleCopy()}
            className={`shrink-0 rounded-lg px-3 py-2 text-xs font-medium transition-colors ${
              copyState === "copied"
                ? "bg-moto-green text-gray-950"
                : copyState === "error"
                  ? "bg-moto-red text-white"
                  : "bg-gray-900 text-white hover:bg-gray-800"
            }`}
          >
            {copyState === "copied"
              ? "Kopiert"
              : copyState === "error"
                ? "Fehler"
                : "Kopieren"}
          </button>
          <a
            href={enrollUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="moto-content-surface shrink-0 rounded-lg border px-3 py-2 text-center text-xs font-medium text-gray-700 hover:bg-gray-50"
          >
            Öffnen
          </a>
        </div>
      </div>
    </section>
  );
}
