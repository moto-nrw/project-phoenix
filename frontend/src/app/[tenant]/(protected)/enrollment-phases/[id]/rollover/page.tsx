"use client";

import { use, useEffect, useState } from "react";
import { TenantPage } from "~/components/ui/tenant-page";
import { RolloverForm } from "~/components/enrollment/rollover-form";
import { getPhase, type Phase } from "~/lib/enrollment-phase-api";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "MobileRolloverPage" });

interface PageProps {
  readonly params: Promise<{ id: string }>;
}

export default function MobileRolloverPage({ params }: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequireAdmin();
  const tenantPath = useTenantAwarePath();
  const tenantMutate = useTenantMutate();
  const [phase, setPhase] = useState<Phase | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isReady) return;
    getPhase(id)
      .then(setPhase)
      .catch((err: unknown) => {
        const message =
          err instanceof Error
            ? err.message
            : "Phase konnte nicht geladen werden";
        logger.error("rollover_phase_load_failed", { error: message });
        setError(message);
      });
  }, [isReady, id]);

  if (!isReady || (phase === null && error === null)) {
    return (
      <TenantPage
        title="Anschlussphase erstellen"
        back
        backHref="/enrollment-phases"
        backLabel="Zurück zu den Anmeldephasen"
        statsLoading
        loading
      />
    );
  }

  // Titel, Statuszeile und Zurück-Knopf trägt das Formular selbst.
  if (!phase) {
    return (
      <TenantPage
        title="Anschlussphase erstellen"
        back
        backHref="/enrollment-phases"
        backLabel="Zurück zu den Anmeldephasen"
        error={error}
      />
    );
  }

  return (
    <RolloverForm
      variant="page"
      source={phase}
      onCancel={() => (globalThis.location.href = tenantPath("/dashboard"))}
      onSuccess={() => {
        void tenantMutate("enrollment-phase-expiry-warnings");
        globalThis.location.href = tenantPath("/dashboard");
      }}
    />
  );
}
