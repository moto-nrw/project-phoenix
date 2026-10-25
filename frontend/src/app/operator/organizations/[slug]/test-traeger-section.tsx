"use client";

import { useCallback, useState } from "react";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";

const logger = createLogger({ component: "TestTraegerSection" });

/**
 * The test system of a test Träger (#4658): what it is, how it leaves, and
 * the sample children of its test schools. Creating them again adds only the
 * children that are missing, so it also repairs a creation that stopped
 * halfway.
 */
export function TestTraegerSection({
  organizationId,
  organizationActive,
  onChanged,
}: {
  readonly organizationId: string;
  readonly organizationActive: boolean;
  readonly onChanged: () => Promise<unknown>;
}) {
  const [running, setRunning] = useState(false);
  const { show: showError } = useApiErrorDisplay();
  const { success: toastSuccess } = useToast();

  const handleSampleChildren = useCallback(async () => {
    setRunning(true);
    try {
      const result =
        await operatorProvisioningService.createSampleChildren(organizationId);
      toastSuccess(
        result.createdCount > 0
          ? `${result.createdCount} Beispielkinder sind angelegt.`
          : "Alle Beispielkinder sind schon da.",
      );
      await onChanged();
    } catch (error) {
      logger.warn("test_traeger_sample_children_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      void showError(error, { object: "das Anlegen der Beispielkinder" });
    } finally {
      setRunning(false);
    }
  }, [onChanged, organizationId, showError, toastSuccess]);

  return (
    <SectionCard
      title="Testsystem"
      description="Dieser Träger ist ein Test-Träger. Seine Test-Schulen enthalten nur erfundene Beispielkinder. „Beispielkinder anlegen“ ergänzt fehlende Kinder; ein Kind mit geändertem Namen oder geänderter Klasse gilt dabei als fehlend. Zum Löschen deaktivieren Sie den Träger und erfassen im Reiter „Löschauftrag“ den Auftrag."
      action={
        organizationActive ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            isLoading={running}
            loadingText="Wird angelegt..."
            onClick={() => void handleSampleChildren()}
          >
            Beispielkinder anlegen
          </Button>
        ) : undefined
      }
    />
  );
}
