"use client";

// Neues Betreuungsangebot einer Anmeldephase des Trägers (#4704).

import { useParams, useRouter } from "next/navigation";
import { CatalogOfferingForm } from "~/components/traeger/catalog-offering-form";
import { useTraegerCatalogPhase } from "~/components/traeger/use-traeger-catalog";
import { TenantPage } from "~/components/ui/tenant-page";
import { useToast } from "~/contexts/ToastContext";
import { useInvalidateFamily } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { createTraegerOffering } from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";

export default function NewTraegerCatalogOfferingPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const { data: phase, loadError } = useTraegerCatalogPhase(id);
  const invalidate = useInvalidateFamily();
  const { success } = useToast();
  const phaseHref = traegerPath(`/traeger/anmeldephasen/${id}`);

  return (
    <TenantPage
      title="Neues Betreuungsangebot"
      info="Die Schulen bekommen das Angebot, wenn Sie die Anmeldephase freigeben und übertragen."
      back
      backHref={phaseHref}
      backLabel={
        phase ? `Zurück zu „${phase.draft.name}“` : "Zurück zur Anmeldephase"
      }
      error={loadError}
      loading={!loadError && !phase}
    >
      {phase ? (
        <CatalogOfferingForm
          saveLabel="Angebot anlegen"
          onSave={(draft) => createTraegerOffering(id, draft)}
          onCancel={() => router.push(phaseHref)}
          onSaved={() => {
            void invalidate(traegerKeys.catalogFamily);
            success(
              "Das Angebot ist angelegt. Die Schulen bekommen es, wenn Sie übertragen.",
            );
            router.push(phaseHref);
          }}
        />
      ) : null}
    </TenantPage>
  );
}
