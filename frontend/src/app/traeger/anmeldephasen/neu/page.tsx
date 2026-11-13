"use client";

// Neue Anmeldephase des Trägers (#4704). Sie bleibt beim Träger, bis das
// Büro sie auf ihrer Seite freigibt und überträgt.

import { useRouter } from "next/navigation";
import { CatalogPhaseForm } from "~/components/traeger/catalog-phase-form";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { TenantPage } from "~/components/ui/tenant-page";
import { useInvalidateFamily } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { createTraegerPhase } from "~/lib/traeger-catalog-api";
import { traegerPath } from "~/lib/traeger-url";

export default function NewTraegerCatalogPhasePage() {
  const router = useRouter();
  const invalidate = useInvalidateFamily();
  const { schools, error } = useTraegerSchools();
  const listHref = traegerPath("/traeger/anmeldephasen");

  return (
    <TenantPage
      title="Neue Anmeldephase"
      info="Die Schulen sehen die Anmeldephase erst, wenn Sie sie freigeben und übertragen."
      back
      backHref={listHref}
      backLabel="Zurück zu den Anmeldephasen"
      error={error}
      loading={!error && !schools}
    >
      {schools ? (
        <CatalogPhaseForm
          schools={schools}
          saveLabel="Anmeldephase anlegen"
          onSave={createTraegerPhase}
          onCancel={() => router.push(listHref)}
          onSaved={(phase) => {
            void invalidate(traegerKeys.catalogFamily);
            router.replace(traegerPath(`/traeger/anmeldephasen/${phase.id}`));
          }}
        />
      ) : null}
    </TenantPage>
  );
}
