"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Eye, Info } from "lucide-react";
import { StatusBadge } from "~/components/ui/status-badge";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import {
  PublicEnrollmentBrand,
  PublicEnrollmentLocaleSwitcher,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
  PublicInfoCard,
} from "~/components/enrollment/public-enrollment-shell";
import { BackLink } from "~/components/ui/back-link";
import { useTenant } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  fetchEnrollmentPreviewBootstrap,
  schemaToPublicFormSchema,
  type FormSchema,
  type PublicFormSchema,
} from "~/lib/enrollment-form-schema-api";
import { useApiLoadError } from "~/contexts/ToastContext";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { permissionsConcept } from "~/lib/concepts/permissions";
import { FormSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";

export default function EnrollmentPreviewPage() {
  return (
    <Suspense fallback={null}>
      <EnrollmentPreviewPageContent />
    </Suspense>
  );
}

function EnrollmentPreviewPageContent() {
  const searchParams = useSearchParams();
  const schemaId = searchParams.get("schemaId");
  const isBasePreview = searchParams.get("base") === "1";
  const { tenant } = useTenant();
  const tenantPath = useTenantAwarePath();
  const resolvedGradeLevelMax = tenant?.gradeLevelMax;
  const gradeLevelMax = isSupportedGradeLevelMax(resolvedGradeLevelMax)
    ? resolvedGradeLevelMax
    : null;
  const [schema, setSchema] = useState<FormSchema | null>(null);
  const [assignedPhaseCount, setAssignedPhaseCount] = useState(0);
  const [activeAssignedPhaseCount, setActiveAssignedPhaseCount] = useState(0);
  const [loading, setLoading] = useState(true);
  // Without a template in the address there is nothing to load: a fixed
  // hint, not an API error.
  const [missingSchema, setMissingSchema] = useState(false);
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [loadAttempt, setLoadAttempt] = useState(0);
  const previewProfileFetcher = useCallback(() => Promise.resolve(null), []);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (!schemaId) {
        if (!isBasePreview) {
          setMissingSchema(true);
          setLoading(false);
          return;
        }
      }
      setLoading(true);
      setMissingSchema(false);
      clearLoadError();
      try {
        const result = await fetchEnrollmentPreviewBootstrap({
          schemaId,
          base: isBasePreview,
        });
        if (cancelled) return;
        setSchema(result.schema);
        setAssignedPhaseCount(result.assigned_phase_count);
        setActiveAssignedPhaseCount(result.active_assigned_phase_count);
      } catch (err) {
        if (cancelled) return;
        void showLoadError(err, {
          object: "die Vorschau",
          retry: () => setLoadAttempt((n) => n + 1),
        });
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [isBasePreview, schemaId, loadAttempt, clearLoadError, showLoadError]);

  const previewSchema = useMemo<PublicFormSchema | null>(() => {
    if (!schema) return null;
    return schemaToPublicFormSchema(schema);
  }, [schema]);

  const previewStatus = loading
    ? {
        label: "Vorschau wird geladen",
        hint: "Das Formular erscheint gleich.",
        tone: "gray" as const,
      }
    : activeAssignedPhaseCount > 0
      ? {
          label: "Aktiv in Anmeldephase",
          hint: `Diese Vorlage wird in ${activeAssignedPhaseCount} aktiver Anmeldephase verwendet.`,
          tone: "green" as const,
        }
      : assignedPhaseCount > 0
        ? {
            label: "In Phase verwendet",
            hint: `Diese Vorlage ist in ${assignedPhaseCount} Anmeldephase ausgewählt.`,
            tone: "green" as const,
          }
        : {
            label: "Nicht live",
            hint: "Diese Formularvorschau ist nicht an eine aktive Anmeldephase gebunden.",
            tone: "gray" as const,
          };
  const previewTitle = loading
    ? "Formularvorschau"
    : (schema?.name ?? "Basisformular");

  return (
    <PublicEnrollmentPageShell withInlineSwitcher>
      <div className="mb-8 flex flex-wrap items-center justify-between gap-4">
        <PublicEnrollmentBrand tenant={tenant} />
        <div className="flex items-center gap-3">
          <PublicEnrollmentSteps current="form" />
          <PublicEnrollmentLocaleSwitcher />
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <section className="space-y-5">
          <div className="moto-content-surface rounded-2xl border p-5 sm:p-8">
            <BackLink
              href={tenantPath("/enrollment-form")}
              label="Zurück zu den Anmeldeformularen"
            />
            <p className="text-moto-blue mt-6 text-sm font-semibold tracking-wide uppercase">
              Formularvorschau
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
              {previewTitle}
            </h1>
            <div className="mt-4">
              <StatusBadge
                tone={previewStatus.tone}
                label={previewStatus.label}
              />
            </div>
            <p className="mt-4 max-w-3xl text-base leading-7 text-gray-600">
              Diese Ansicht zeigt nur das Formular mit Basisfeldern und
              Zusatzfragen. Zeitraum und Betreuungsangebote gehören zur
              Elternansicht einer Anmeldephase.
            </p>
          </div>

          {loading ? (
            <SkeletonRegion label="Vorschau wird geladen">
              <FormSkeleton />
            </SkeletonRegion>
          ) : loadError || missingSchema || gradeLevelMax === null ? (
            <LoadErrorAlert
              error={
                loadError ??
                (missingSchema
                  ? "Keine Formularvorlage ausgewählt."
                  : "Die Klassenstufen sind für diese Schule nicht eingerichtet.")
              }
            />
          ) : (
            <EnrollmentForm
              gradeLevelMax={gradeLevelMax}
              localizedCopy
              onSubmitted={() => undefined}
              profileFetcher={previewProfileFetcher}
              previewMode
              previewSchema={previewSchema}
              skipCaptcha
            />
          )}
        </section>

        <aside className="space-y-4 lg:sticky lg:top-8 lg:self-start">
          <section className="moto-content-surface rounded-2xl border p-5">
            <p className="text-sm font-semibold tracking-wide text-gray-500 uppercase">
              Vorschau
            </p>
            <h2 className="mt-2 text-xl font-semibold text-gray-900">
              {previewStatus.label}
            </h2>
            <p className="mt-3 text-sm leading-6 text-gray-600">
              {previewStatus.hint} Diese Seite dient nur zur Prüfung des
              Formulars.
            </p>
          </section>

          <PublicInfoCard
            icon={<Eye className="h-5 w-5" />}
            title="Direktes Formular"
          >
            Du siehst hier das Formular selbst, nicht die öffentliche
            Auswahlseite für Eltern.
          </PublicInfoCard>
          <PublicInfoCard
            icon={<Info className="h-5 w-5" />}
            title="Ohne Phase"
          >
            Zeitraum und Betreuungsangebote werden erst in der
            Live-Elternansicht oder in einer Phasenvorschau sichtbar.
          </PublicInfoCard>
          <PublicInfoCard
            icon={<MotoConceptIcon concept={permissionsConcept} size={22} />}
            title="Sichere Vorschau"
          >
            Der Absenden-Button ist in dieser Ansicht deaktiviert.
          </PublicInfoCard>
        </aside>
      </div>
    </PublicEnrollmentPageShell>
  );
}
