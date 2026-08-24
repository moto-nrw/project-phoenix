"use client";

import { use, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { MobileBackButton } from "~/components/ui/mobile-back-button";
import { DetailSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { RolloverForm } from "~/components/enrollment/rollover-form";
import { getPhase, type Phase } from "~/lib/enrollment-phase-api";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";
import { createLogger } from "~/lib/logger";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantMutate } from "~/lib/swr";

const logger = createLogger({ component: "MobileRolloverPage" });

interface PageProps {
  readonly params: Promise<{ phaseId: string }>;
}

export default function MobileRolloverPage({ params }: PageProps) {
  const { phaseId } = use(params);
  const { isReady } = useRequireAdmin();
  const tenantPath = useTenantAwarePath();
  const tenantMutate = useTenantMutate();
  const [phase, setPhase] = useState<Phase | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isReady) return;
    getPhase(phaseId)
      .then(setPhase)
      .catch((err: unknown) => {
        const message =
          err instanceof Error
            ? err.message
            : "Phase konnte nicht geladen werden";
        logger.error("rollover_phase_load_failed", { error: message });
        setError(message);
      });
  }, [isReady, phaseId]);

  if (!isReady || (phase === null && error === null)) {
    return (
      <SkeletonRegion label="Anschlussphase wird geladen">
        <DetailSkeleton sections={2} fieldsPerSection={3} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="w-full space-y-4">
      <MobileBackButton
        href={tenantPath("/dashboard")}
        ariaLabel="Zurück zur Übersicht"
      />
      {error ? <Alert type="error" message={error} /> : null}
      {phase ? (
        <RolloverForm
          source={phase}
          onCancel={() => (globalThis.location.href = tenantPath("/dashboard"))}
          onSuccess={() => {
            void tenantMutate("enrollment-phase-expiry-warnings");
            globalThis.location.href = tenantPath("/dashboard");
          }}
        />
      ) : null}
    </div>
  );
}
