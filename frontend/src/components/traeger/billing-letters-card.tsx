"use client";

// Die Bestätigungsschreiben eines freigegebenen Laufs (#4669): je Familie
// ein Schreiben, das zugleich ihre Rechnung ist, im Briefkopf des Trägers.
// Vor dem Erstellen ein Knopf, danach was in allen Schreiben steht. Die
// Schreiben ändern sich nie.

import dynamic from "next/dynamic";
import { useState } from "react";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import type { BillingLetters } from "~/lib/traeger-billing-api";
import { traegerPath } from "~/lib/traeger-url";

// Loaded when someone opens it: a run creates its letters once.
const BillingLettersModal = dynamic(
  () =>
    import("./billing-letters-modal").then(
      (module) => module.BillingLettersModal,
    ),
  { ssr: false },
);

export function BillingLettersCard({
  runId,
  letters,
  onCreated,
}: Readonly<{
  runId: string;
  letters: BillingLetters | null;
  onCreated: (letters: BillingLetters) => void;
}>) {
  const [creating, setCreating] = useState(false);

  if (!letters) {
    return (
      <SectionCard
        title="Bestätigungsschreiben"
        description="Jede Familie bekommt ein Schreiben, das ihre Anmeldung bestätigt und zugleich ihre Rechnung ist."
        actions={
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => setCreating(true)}
          >
            Schreiben erstellen
          </Button>
        }
      >
        <p className="text-sm text-gray-700">
          Logo, Absender, Fußzeile und Bankverbindung kommen aus dem{" "}
          <NavigationLink
            href={traegerPath("/traeger/briefkopf")}
            className="font-medium underline"
          >
            Briefkopf
          </NavigationLink>
          . Ohne Bankverbindung lassen sich keine Schreiben erstellen; sie legt
          die Verwaltung fest.
        </p>
        {creating ? (
          <BillingLettersModal
            runId={runId}
            isOpen
            onClose={() => setCreating(false)}
            onCreated={(created) => {
              setCreating(false);
              onCreated(created);
            }}
          />
        ) : null}
      </SectionCard>
    );
  }

  return (
    <SectionCard
      title="Bestätigungsschreiben"
      description="Die Schreiben sind erstellt und lassen sich nicht mehr ändern. Das Schreiben einer Familie laden Sie über das Menü ihrer Rechnung herunter."
    >
      <DataGrid columns={2}>
        <DataField label="Zahlen bis">
          {formatDate(letters.paymentDueOn)}
        </DataField>
        <DataField label="Erstellt am">
          {formatBerlinDate(letters.createdAt)}
        </DataField>
        <DataField label="Hinweise für die Eltern">
          {letters.notes ? (
            <span className="whitespace-pre-line">{letters.notes}</span>
          ) : (
            "Keine"
          )}
        </DataField>
        <DataField label="Notfallnummern">
          {letters.emergencyContacts.length > 0 ? (
            <ul className="space-y-1">
              {letters.emergencyContacts.map((contact) => (
                <li key={`${contact.name}-${contact.phone}`}>
                  {contact.name}: {contact.phone}
                </li>
              ))}
            </ul>
          ) : (
            "Keine"
          )}
        </DataField>
      </DataGrid>
    </SectionCard>
  );
}
