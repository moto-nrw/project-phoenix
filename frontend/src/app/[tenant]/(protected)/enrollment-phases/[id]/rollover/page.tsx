"use client";

import { use, useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { BackButton } from "~/components/ui/back-button";
import { DetailSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
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
      <SkeletonRegion label="Anschlussphase wird geladen">
        <DetailSkeleton sections={2} fieldsPerSection={3} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="w-full space-y-4">
      <BackButton referrer="/enrollment-phases" />
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
