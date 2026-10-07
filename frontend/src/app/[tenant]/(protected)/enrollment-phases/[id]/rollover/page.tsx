"use client";

import { use, useEffect, useState } from "react";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiLoadError } from "~/contexts/ToastContext";
import { RolloverForm } from "~/components/enrollment/rollover-form";
import { getPhase, type Phase } from "~/lib/enrollment-phase-api";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "MobileRolloverPage" });

interface PageProps {
  readonly params: Promise<{ id: string }>;
}

export default function MobileRolloverPage({ params }: PageProps) {
  const { id } = use(params);
  const { isReady } = useRequirePermission("config:manage");
  const tenantPath = useTenantAwarePath();
  const tenantMutate = useTenantMutate();
  const [phase, setPhase] = useState<Phase | null>(null);
  const phaseLoad = useApiLoadError();
  const showLoadError = phaseLoad.show;
  const clearLoadError = phaseLoad.clear;
  // Bumped by „Wiederholen“ to load the phase again.
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (!isReady) return;
    let cancelled = false;
    clearLoadError();
    getPhase(id)
      .then((loaded) => {
        if (!cancelled) setPhase(loaded);
      })
      .catch(async (err: unknown) => {
        if (cancelled) return;
        logger.error("rollover_phase_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showLoadError(err, {
          object: "die Anmeldephase",
          retry: () => setAttempt((current) => current + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [isReady, id, attempt, showLoadError, clearLoadError]);

  if (!isReady || (phase === null && !phaseLoad.error)) {
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
        error={phaseLoad.error}
      />
    );
  }

  return (
    <RolloverForm
      variant="page"
      source={phase}
      onCancel={() => (globalThis.location.href = tenantPath("/home"))}
      onSuccess={() => {
        void tenantMutate("enrollment-phase-expiry-warnings");
        globalThis.location.href = tenantPath("/home");
      }}
    />
  );
}
