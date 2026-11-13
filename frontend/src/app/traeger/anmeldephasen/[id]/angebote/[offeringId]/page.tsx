"use client";

// Ein Betreuungsangebot einer Anmeldephase des Trägers ändern (#4704).

import { useParams, useRouter } from "next/navigation";
import { CatalogOfferingForm } from "~/components/traeger/catalog-offering-form";
import { useTraegerCatalogPhase } from "~/components/traeger/use-traeger-catalog";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { TenantPage } from "~/components/ui/tenant-page";
import { useToast } from "~/contexts/ToastContext";
import { useInvalidateFamily } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { updateTraegerOffering } from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";

export default function TraegerCatalogOfferingPage() {
  const router = useRouter();
  const params = useParams<{ id: string; offeringId: string }>();
  const id = params?.id ?? "";
  const offeringId = params?.offeringId ?? "";
  const { data: phase, loadError } = useTraegerCatalogPhase(id);
  const invalidate = useInvalidateFamily();
  const { success } = useToast();
  const phaseHref = traegerPath(`/traeger/anmeldephasen/${id}`);
  const offering = phase?.offerings.find((entry) => entry.id === offeringId);
  const editable = Boolean(
    offering && !offering.withdrawnAt && !phase?.withdrawnAt,
  );

  return (
    <TenantPage
      title={offering?.draft.fields.name ?? "Betreuungsangebot"}
      info="Ihre Änderung erreicht die Schulen erst, wenn Sie übertragen. Hat eine Schule ein Feld selbst geändert, behält sie ihren Wert."
      back
      backHref={phaseHref}
      backLabel={
        phase ? `Zurück zu „${phase.draft.name}“` : "Zurück zur Anmeldephase"
      }
      error={loadError}
      loading={!loadError && !phase}
    >
      {phase && offering && editable ? (
        <CatalogOfferingForm
          key={offering.id}
          offering={offering}
          saveLabel="Speichern"
          onSave={(draft) => updateTraegerOffering(offering.id, draft)}
          onCancel={() => router.push(phaseHref)}
          onSaved={() => {
            void invalidate(traegerKeys.catalogFamily);
            success(
              "Gespeichert. Die Schulen bekommen die Änderung, wenn Sie übertragen.",
            );
            router.push(phaseHref);
          }}
        />
      ) : null}
      {phase && !editable ? (
        <SectionCard>
          <EmptyState
            title={
              offering
                ? "Dieses Angebot ist zurückgezogen."
                : "Dieses Angebot gibt es nicht."
            }
            description="Ein zurückgezogenes Angebot lässt sich nicht mehr ändern."
          />
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}
