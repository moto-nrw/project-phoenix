"use client";

// Eine Anmeldephase des Trägers (#4704): Ergebnis je Schule, Angebote und
// die Angaben der Phase. „Vorschau ansehen“ führt zum Übertragen.

import { useParams } from "next/navigation";
import { useState } from "react";
import {
  hasPendingRollout,
  phaseStatusBadge,
  serviceRange,
} from "~/components/traeger/catalog-format";
import {
  CatalogOfferingsCard,
  CatalogSchoolsCard,
} from "~/components/traeger/catalog-phase-cards";
import { CatalogPhaseForm } from "~/components/traeger/catalog-phase-form";
import { metadataLoadError } from "~/components/traeger/metadata-load-error";
import { useTraegerCatalogPhase } from "~/components/traeger/use-traeger-catalog";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import {
  type TraegerPhaseDetail,
  updateTraegerPhase,
  withdrawTraegerPhase,
} from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCatalogPhasePage" });

export default function TraegerCatalogPhasePage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const {
    data: phase,
    loadError: phaseError,
    error: phaseFailure,
    mutate,
  } = useTraegerCatalogPhase(id);
  const {
    schools,
    error: schoolsError,
    fetchError: schoolsFailure,
  } = useTraegerSchools();
  const invalidateList = useTenantMutateMatching([traegerKeys.catalogFamily]);
  const [withdrawOpen, setWithdrawOpen] = useState(false);
  const [withdrawing, setWithdrawing] = useState(false);
  const [formKey, setFormKey] = useState(0);
  const { show } = useApiErrorDisplay();
  const { success } = useToast();
  const listHref = traegerPath("/traeger/anmeldephasen");
  const previewHref = traegerPath(`/traeger/anmeldephasen/${id}/vorschau`);

  const saved = (next: TraegerPhaseDetail) => {
    void mutate(next, { revalidate: false });
    void invalidateList({ except: [traegerKeys.catalogPhase(id)] });
  };

  const confirmWithdraw = async () => {
    setWithdrawing(true);
    try {
      saved(await withdrawTraegerPhase(id));
      setWithdrawOpen(false);
      success(
        "Die Anmeldephase ist zurückgezogen. Übertragen Sie jetzt, damit sie aus den Schulen verschwindet.",
      );
    } catch (err) {
      logger.error("traeger_catalog_phase_withdraw_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void show(err, { object: "die Anmeldephase" });
    } finally {
      setWithdrawing(false);
    }
  };

  const loadError = phaseError ?? schoolsError;
  const shownError = metadataLoadError(
    loadError,
    Boolean(phase && schools),
    phaseFailure,
    schoolsFailure,
  );
  const withdrawn = Boolean(phase?.withdrawnAt);
  const pending =
    phase && schools
      ? hasPendingRollout(
          phase,
          schools.map((school) => school.id),
        )
      : false;

  return (
    <TenantPage
      title={phase?.draft.name ?? "Anmeldephase"}
      back
      backHref={listHref}
      backLabel="Zurück zu den Anmeldephasen"
      info="Jede Schule bekommt eine eigene Kopie dieser Anmeldephase. Die Schule darf sie ändern oder ausschalten; ihre Änderungen bleiben. Entfernen können Sie die Kopie nur hier mit „Zurückziehen“."
      stats={
        phase ? (
          <span className="flex flex-wrap items-center gap-2">
            {serviceRange(
              phase.draft.serviceStartDate,
              phase.draft.serviceEndDate,
            )}
            <StatusBadge
              {...(pending && !withdrawn
                ? { label: "Änderungen nicht übertragen", tone: "warning" }
                : phaseStatusBadge(phase))}
            />
          </span>
        ) : null
      }
      statsLoading={!phase && !loadError}
      actions={
        phase ? (
          <>
            <ButtonLink
              href={previewHref}
              variant="primary"
              size="md"
              tooltip="Zeigt, was die Schulen bekommen, noch ohne etwas zu übertragen."
            >
              Vorschau ansehen
            </ButtonLink>
            {withdrawn ? null : (
              <OverflowMenu
                ariaLabel="Weitere Aktionen für die Anmeldephase"
                items={[
                  {
                    label: "Zurückziehen",
                    description: "Aus allen Schulen entfernen",
                    destructive: true,
                    onClick: () => setWithdrawOpen(true),
                  },
                ]}
              />
            )}
          </>
        ) : null
      }
      error={shownError}
      loading={!loadError && !(phase && schools)}
    >
      {phase && schools ? (
        <>
          {pending ? (
            <Alert
              type="warning"
              message={
                withdrawn
                  ? "Die Anmeldephase ist zurückgezogen. In den Schulen ist sie erst weg, wenn Sie übertragen. Öffnen Sie dazu „Vorschau ansehen“."
                  : "Die Schulen haben noch nicht den aktuellen Stand. Öffnen Sie „Vorschau ansehen“ und übertragen Sie danach."
              }
            />
          ) : null}
          <CatalogSchoolsCard phase={phase} schools={schools} />
          <CatalogOfferingsCard
            phase={phase}
            editable={!withdrawn}
            onWithdrawn={() => void mutate()}
          />
          {withdrawn ? null : (
            <CatalogPhaseForm
              key={`${phase.id}-${formKey}`}
              phase={phase}
              schools={schools}
              saveLabel="Speichern"
              onSave={(input) => updateTraegerPhase(id, input)}
              onSaved={(next) => {
                saved(next);
                success(
                  "Gespeichert. Die Schulen bekommen die Änderung, wenn Sie übertragen.",
                );
              }}
              onCancel={() => setFormKey((key) => key + 1)}
            />
          )}
          <ConfirmationModal
            isOpen={withdrawOpen}
            onClose={() => setWithdrawOpen(false)}
            onConfirm={() => void confirmWithdraw()}
            title="Anmeldephase zurückziehen?"
            confirmText="Zurückziehen"
            confirmVariant="danger"
            isConfirmLoading={withdrawing}
            loadingText="Wird zurückgezogen…"
          >
            <p className="text-sm text-gray-700">
              Die Anmeldephase und ihre Angebote verschwinden aus allen Schulen,
              sobald Sie das nächste Mal übertragen. Gibt es dort schon
              Anmeldungen oder Buchungen, schaltet moto sie nur aus. Das lässt
              sich nicht rückgängig machen.
            </p>
          </ConfirmationModal>
        </>
      ) : null}
    </TenantPage>
  );
}
