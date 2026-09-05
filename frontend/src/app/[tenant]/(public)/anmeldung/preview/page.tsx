"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Eye, Info } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { EnrollmentForm } from "~/components/enrollment/enrollment-form";
import {
  PublicEnrollmentBackLink,
  PublicEnrollmentBrand,
  PublicEnrollmentLocaleSwitcher,
  PublicEnrollmentPageShell,
  PublicEnrollmentSteps,
  PublicInfoCard,
} from "~/components/enrollment/public-enrollment-shell";
import { useTenant } from "~/lib/tenant-context";
import { isSupportedGradeLevelMax } from "~/lib/grade-level";
import {
  fetchEnrollmentPreviewBootstrap,
  schemaToPublicFormSchema,
  type FormSchema,
  type PublicFormSchema,
} from "~/lib/enrollment-form-schema-api";

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
  const resolvedGradeLevelMax = tenant?.gradeLevelMax;
  const gradeLevelMax = isSupportedGradeLevelMax(resolvedGradeLevelMax)
    ? resolvedGradeLevelMax
    : null;
  const [schema, setSchema] = useState<FormSchema | null>(null);
  const [assignedPhaseCount, setAssignedPhaseCount] = useState(0);
  const [activeAssignedPhaseCount, setActiveAssignedPhaseCount] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const previewProfileFetcher = useCallback(() => Promise.resolve(null), []);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      if (!schemaId) {
        if (!isBasePreview) {
          setError("Keine Formularvorlage ausgewählt.");
          setLoading(false);
          return;
        }
      }
      setLoading(true);
      setError(null);
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
        setError(
          err instanceof Error
            ? err.message
            : "Die Vorschau konnte nicht geladen werden.",
        );
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, [isBasePreview, schemaId]);

  const previewSchema = useMemo<PublicFormSchema | null>(() => {
    if (!schema) return null;
    return schemaToPublicFormSchema(schema);
  }, [schema]);

  const previewStatus = loading
    ? {
        label: "Vorschau wird geladen",
        hint: "Die Formularvorschau wird geladen.",
        className: "bg-gray-100 text-gray-600",
        dotClassName: "bg-gray-300",
      }
    : activeAssignedPhaseCount > 0
      ? {
          label: "Aktiv in Anmeldephase",
          hint: `Diese Vorlage wird in ${activeAssignedPhaseCount} aktiver Anmeldephase verwendet.`,
          className: "bg-moto-green/10 text-moto-green-strong",
          dotClassName: "bg-moto-green",
        }
      : assignedPhaseCount > 0
        ? {
            label: "In Phase verwendet",
            hint: `Diese Vorlage ist in ${assignedPhaseCount} Anmeldephase ausgewählt.`,
            className: "bg-moto-green/10 text-moto-green-strong",
            dotClassName: "bg-moto-green",
          }
        : {
            label: "Nicht live",
            hint: "Diese Formularvorschau ist nicht an eine aktive Anmeldephase gebunden.",
            className: "bg-gray-100 text-gray-600",
            dotClassName: "bg-gray-300",
          };
  const previewTitle = loading
    ? "Formularvorschau wird geladen"
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
          <div className="moto-content-surface rounded-2xl border p-5 shadow-sm sm:p-8">
            <PublicEnrollmentBackLink href="/enrollment-form">
              Zurück zu Anmeldeformularen
            </PublicEnrollmentBackLink>
            <p className="text-moto-blue mt-6 text-sm font-semibold tracking-wide uppercase">
              Formularvorschau
            </p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight text-gray-900 sm:text-4xl">
              {previewTitle}
            </h1>
            <span
              className={`mt-4 inline-flex items-center gap-2 rounded-full px-3 py-1 text-sm font-medium ${previewStatus.className}`}
            >
              <span
                className={`h-2 w-2 rounded-full ${previewStatus.dotClassName}`}
                aria-hidden="true"
              />
              {previewStatus.label}
            </span>
            <p className="mt-4 max-w-3xl text-base leading-7 text-gray-600">
              Diese Ansicht zeigt nur das Formular mit Basisfeldern und
              Zusatzfragen. Zeitraum und Betreuungsangebote gehören zur
              Elternansicht einer Anmeldephase.
            </p>
          </div>

          {loading ? (
            <div className="moto-content-surface rounded-2xl border p-6 text-sm font-medium text-gray-600 shadow-sm">
              Vorschau wird geladen…
            </div>
          ) : error || gradeLevelMax === null ? (
            <div className="moto-content-surface border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-6 text-sm font-medium shadow-sm">
              {error ?? "Die Klassenstufen-Konfiguration ist nicht verfügbar."}
            </div>
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
          <section className="moto-content-surface rounded-2xl border p-5 shadow-sm">
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
            icon={<MotoConceptIcon concept="permissions" size={22} />}
            title="Sichere Vorschau"
          >
            Der Absenden-Button ist in dieser Ansicht deaktiviert.
          </PublicInfoCard>
        </aside>
      </div>
    </PublicEnrollmentPageShell>
  );
}
