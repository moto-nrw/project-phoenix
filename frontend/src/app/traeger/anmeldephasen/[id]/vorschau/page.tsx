"use client";

// Vorschau einer Anmeldephase des Trägers und „Freigeben und übertragen“
// (#4704, ADR 0195): erst hier ändert sich etwas in den Schulen.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  RolloutResultCard,
  SchoolPlanCard,
} from "~/components/traeger/catalog-preview";
import { useTraegerCatalogPhase } from "~/components/traeger/use-traeger-catalog";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { useInvalidateFamily } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  type TraegerCatalogPreview,
  type TraegerRollout,
  previewTraegerPhase,
  rolloutTraegerPhase,
} from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCatalogPreviewPage" });

const PREVIEW_STALE = "enrollment.traeger_preview_stale";

export default function TraegerCatalogPreviewPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const { data: phase } = useTraegerCatalogPhase(id);
  const { schools, error: schoolsError } = useTraegerSchools();
  const [preview, setPreview] = useState<TraegerCatalogPreview | null>(null);
  const [rollout, setRollout] = useState<TraegerRollout | null>(null);
  const [loading, setLoading] = useState(true);
  const [rollingOut, setRollingOut] = useState(false);
  const previewError = useApiLoadError();
  const { show: showPreviewError, clear: clearPreviewError } = previewError;
  const { show } = useApiErrorDisplay();
  const { success } = useToast();
  const invalidate = useInvalidateFamily();
  const phaseHref = traegerPath(`/traeger/anmeldephasen/${id}`);

  const loadPreview = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    clearPreviewError();
    try {
      setPreview(await previewTraegerPhase(id));
    } catch (err) {
      logger.error("traeger_catalog_preview_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setPreview(null);
      void showPreviewError(err, {
        object: "die Vorschau",
        retry: () => void loadPreview(),
      });
    } finally {
      setLoading(false);
    }
  }, [id, clearPreviewError, showPreviewError]);

  useEffect(() => {
    void loadPreview();
  }, [loadPreview]);

  const release = async () => {
    if (!preview) return;
    setRollingOut(true);
    try {
      const result = await rolloutTraegerPhase(id, preview.fingerprint);
      setRollout(result);
      void invalidate(traegerKeys.catalogFamily);
      success("Freigegeben und übertragen.");
    } catch (err) {
      logger.error("traeger_catalog_rollout_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void show(err, { object: "die Anmeldephase" });
      // Etwas hat sich seit der Vorschau geändert: die neue Vorschau zeigt
      // es, bevor das Büro noch einmal freigibt.
      if (err instanceof ApiError && err.code === PREVIEW_STALE) {
        void loadPreview();
      }
    } finally {
      setRollingOut(false);
    }
  };

  const ready = preview?.schools.filter(
    (school) => school.outcome === "ready",
  ).length;

  return (
    <TenantPage
      title={phase ? `Vorschau: ${phase.draft.name}` : "Vorschau"}
      back
      backHref={phaseHref}
      backLabel="Zurück zur Anmeldephase"
      info="Die Vorschau zeigt je Schule, was das Übertragen ändert. Erst mit „Freigeben und übertragen“ ändert sich etwas in den Schulen. Neue Preise gelten ab Monatsbeginn; ausgestellte Verträge und Rechnungen bleiben, wie sie sind."
      stats={
        preview && !rollout ? (
          <span>
            {preview.schools.length === 1
              ? "1 Schule"
              : `${preview.schools.length} Schulen`}
            {" · "}
            {ready === 1 ? "1 übernimmt" : `${ready} übernehmen`}
          </span>
        ) : null
      }
      statsLoading={loading && !rollout}
      actions={
        rollout ? (
          <ButtonLink
            href={phaseHref}
            variant="primary"
            size="md"
            tooltip="Öffnet die Anmeldephase mit dem neuen Stand."
          >
            Zur Anmeldephase
          </ButtonLink>
        ) : (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => void loadPreview()}
              disabled={loading || rollingOut}
              tooltip="Berechnet die Vorschau mit dem aktuellen Stand neu, ohne etwas zu ändern."
            >
              Vorschau neu laden
            </Button>
            <Button
              type="button"
              variant="success"
              size="md"
              onClick={() => void release()}
              disabled={!preview || loading || preview.schools.length === 0}
              disabledReason={
                preview && !loading && preview.schools.length === 0
                  ? "Wählen Sie zuerst in der Anmeldephase die Schulen aus."
                  : undefined
              }
              tooltip="Überträgt die Anmeldephase sofort an die Schulen der Vorschau."
              isLoading={rollingOut}
              loadingText="Wird übertragen…"
            >
              Freigeben und übertragen
            </Button>
          </>
        )
      }
      error={previewError.error ?? schoolsError}
      loading={(loading && !preview && !rollout) || !schools}
    >
      {schools && rollout ? (
        <RolloutResultCard rollout={rollout} schools={schools} />
      ) : null}
      {schools && preview && !rollout ? (
        preview.schools.length === 0 ? (
          <SectionCard>
            <EmptyState
              title="Keine Schule bekommt etwas."
              description="Wählen Sie in der Anmeldephase bei „Schulen“ aus, wer sie bekommt."
            />
          </SectionCard>
        ) : (
          preview.schools.map((plan) => (
            <SchoolPlanCard key={plan.schoolId} plan={plan} schools={schools} />
          ))
        )
      ) : null}
    </TenantPage>
  );
}
