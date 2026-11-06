"use client";

import { useEffect, useMemo, useState } from "react";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import type { SchemaTab } from "~/lib/settings-api";
import { buttonClassName } from "~/components/ui/button";
import { CopyField } from "~/components/ui/copy-field";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";

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
  const { showMessage } = useApiErrorDisplay();

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
      return `${origin}/anmeldung`;
    }
    return `${origin}/${tenantSlug}/anmeldung`;
  }, [origin, tenantSlug]);

  if (!enabled || !enrollUrl) {
    return null;
  }

  return (
    <SectionCard
      // Geschwister im selben Container sind die SettingsCategory-Bloecke
      // mit h3, und auf Mobile ist der Tab-Titel im MobileBackHeader h2.
      headingLevel={3}
      title="Anmeldelink für Eltern"
      concept={enrollmentsConcept}
      description="Teilen Sie diesen Link mit Eltern, damit sie ihre Kinder anmelden können. Jeder mit dem Link kann ihn öffnen. Eltern brauchen dafür kein Konto."
    >
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <CopyField
          value={enrollUrl}
          monospace
          className="flex-1"
          onCopyFailed={() =>
            // Kein API-Fehler: der Browser hat das Kopieren verweigert. Der
            // Link steht daneben und lässt sich von Hand markieren.
            showMessage(
              "Der Link konnte nicht kopiert werden. Bitte markieren und kopieren Sie ihn selbst.",
            )
          }
        />
        <a
          href={enrollUrl}
          target="_blank"
          rel="noopener noreferrer"
          className={buttonClassName({
            variant: "outline",
            size: "md",
            className: "shrink-0 self-start sm:self-auto",
          })}
        >
          Öffnen
        </a>
      </div>
    </SectionCard>
  );
}
