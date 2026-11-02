"use client";

// Neuer Ferienzeitraum des Trägers (#4659). Er entsteht als Entwurf; das
// Veröffentlichen folgt auf seiner eigenen Seite.

import { useRouter } from "next/navigation";
import { HolidayPeriodForm } from "~/components/traeger/holiday-period-form";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { TenantPage } from "~/components/ui/tenant-page";
import { useInvalidateFamily } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { traegerPath } from "~/lib/traeger-url";

export default function NewTraegerHolidayPeriodPage() {
  const router = useRouter();
  const invalidate = useInvalidateFamily();
  const { schools, error } = useTraegerSchools();
  const listHref = traegerPath("/traeger/ferien");

  return (
    <TenantPage
      title="Neuer Ferienzeitraum"
      info="Er wird als Entwurf gespeichert. Für die Anmeldung freigegeben ist er erst nach dem Veröffentlichen."
      back
      backHref={listHref}
      backLabel="Zurück zu den Ferien"
      error={error}
      loading={!error && !schools}
    >
      {schools ? (
        <HolidayPeriodForm
          schools={schools}
          onCancel={() => router.push(listHref)}
          onSaved={(period) => {
            void invalidate(traegerKeys.periodsFamily);
            router.replace(traegerPath(`/traeger/ferien/${period.id}`));
          }}
        />
      ) : null}
    </TenantPage>
  );
}
