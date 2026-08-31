"use client";

import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { StudentConsent, StudentConsentKey } from "~/lib/student-helpers";

const CONSENT_LABELS: Record<StudentConsentKey, string> = {
  agb: "Allgemeine Geschäftsbedingungen (AGB)",
  data_processing: "Datenschutz zur Kenntnis genommen",
  email_contact: "Kontakt per E-Mail erlaubt",
  photo: "Foto-Einwilligung",
};

export function StudentConsentsReadOnly({
  consents,
}: Readonly<{ consents?: readonly StudentConsent[] }>) {
  const visibleConsents =
    consents?.filter((consent) => consent.state !== "not_recorded") ?? [];

  if (visibleConsents.length === 0) return null;

  return (
    <SectionCard
      id="student-consents"
      title="Einwilligungen und Bestätigungen"
      description="Nur zur Information. Eltern können die Foto-Einwilligung im Elternportal widerrufen."
    >
      <ul className="divide-y divide-gray-100">
        {visibleConsents.map((consent) => {
          const withdrawn = consent.state === "withdrawn";
          return (
            <li
              key={consent.key}
              className="flex flex-col gap-3 py-4 first:pt-0 last:pb-0 sm:flex-row sm:items-center sm:justify-between"
            >
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">
                  {CONSENT_LABELS[consent.key]}
                </p>
                {consent.changed_at ? (
                  <p className="mt-1 text-sm text-gray-600">
                    {withdrawn ? "Widerrufen" : "Hinterlegt"} am{" "}
                    {formatBerlinDate(consent.changed_at, "de-DE")}
                  </p>
                ) : null}
              </div>
              <div className="flex items-start sm:items-end">
                <StatusBadge
                  label={withdrawn ? "Widerrufen" : "Hinterlegt"}
                  tone={withdrawn ? "red" : "green"}
                />
              </div>
            </li>
          );
        })}
      </ul>
    </SectionCard>
  );
}
