"use client";

// Neuer Ferienzeitraum des Trägers (#4659). Er entsteht als Entwurf; das
// Veröffentlichen folgt auf seiner eigenen Seite.

import { useRouter } from "next/navigation";
import { HolidayPeriodForm } from "~/components/traeger/holiday-period-form";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { BackLink } from "~/components/ui/back-link";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { traegerPath } from "~/lib/traeger-url";

export default function NewTraegerHolidayPeriodPage() {
  const router = useRouter();
  const { schools, error } = useTraegerSchools();
  const listHref = traegerPath("/traeger/ferien");

  return (
    <div className="w-full space-y-6">
      <div className="space-y-2">
        <BackLink href={listHref} label="Zurück zu den Ferien" />
        <h1 className="text-xl font-semibold text-gray-900 sm:text-2xl">
          Neuer Ferienzeitraum
        </h1>
        <p className="text-sm text-gray-500">
          Er wird als Entwurf gespeichert. Für die Anmeldung freigegeben ist er
          erst nach dem Veröffentlichen.
        </p>
      </div>
      {error ? (
        <LoadErrorAlert error={error} />
      ) : schools ? (
        <HolidayPeriodForm
          schools={schools}
          onCancel={() => router.push(listHref)}
          onSaved={(period) =>
            router.replace(traegerPath(`/traeger/ferien/${period.id}`))
          }
        />
      ) : (
        <Loading fullPage={false} />
      )}
    </div>
  );
}
