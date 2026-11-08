"use client";

// Die Bestätigungsschreiben eines freigegebenen Laufs (#4669): je Familie
// ein Schreiben, das zugleich ihre Rechnung ist, im Briefkopf des Trägers.
// Vor dem Erstellen ein Knopf, danach was in allen Schreiben steht. Die
// Schreiben ändern sich nie.

import dynamic from "next/dynamic";
import { type ReactNode, useState } from "react";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import type { BillingLetters } from "~/lib/traeger-billing-api";
import type { InvoicePayment } from "~/lib/traeger-billing-terms-api";
import { invoiceDueDatesText } from "./billing-payment-format";
import type { LetterSource } from "./billing-letters-modal";
import { traegerPath } from "~/lib/traeger-url";

// Loaded when someone opens it: a run creates its letters once.
const BillingLettersModal = dynamic(
  () =>
    import("./billing-letters-modal").then(
      (module) => module.BillingLettersModal<unknown>,
    ),
  { ssr: false },
);

/**
 * The step before the letters of a run exist (#4669, #4684): what the letters
 * are for, where letterhead and bank account come from (children) and the
 * button that opens the creation with its preview. A source other than the
 * holiday run's creates the letters of that run (#4684).
 */
export function LettersCreateCard<T = BillingLetters>({
  title,
  description,
  runId,
  payments,
  source,
  onCreated,
  children,
}: Readonly<{
  title: string;
  description: string;
  runId: string;
  payments: readonly (InvoicePayment | null | undefined)[];
  source?: LetterSource<T>;
  onCreated: (letters: T) => void;
  children: ReactNode;
}>) {
  const [creating, setCreating] = useState(false);
  return (
    <SectionCard
      title={title}
      description={description}
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
      <p className="text-sm text-gray-700">{children}</p>
      {creating ? (
        <BillingLettersModal
          runId={runId}
          payments={payments}
          isOpen
          source={source as LetterSource<unknown> | undefined}
          onClose={() => setCreating(false)}
          onCreated={(created) => {
            setCreating(false);
            onCreated(created as T);
          }}
        />
      ) : null}
    </SectionCard>
  );
}

/** The links to letterhead and bank account the letters read. */
export function LetterheadLink() {
  return (
    <NavigationLink
      href={traegerPath("/traeger/einstellungen/briefkopf")}
      className="font-medium underline"
    >
      Briefkopf und Vorlagen
    </NavigationLink>
  );
}

export function BillingSettingsLink() {
  return (
    <NavigationLink
      href={traegerPath("/traeger/einstellungen/abrechnung")}
      className="font-medium underline"
    >
      Abrechnungsvorgaben
    </NavigationLink>
  );
}

/** What every letter of a run says: payment date, creation and the note. */
export function LettersFacts({
  paymentDueOn,
  payments,
  createdAt,
  notes,
  notesLabel,
  emptyNotes,
  children,
}: Readonly<{
  paymentDueOn: string;
  payments: readonly (InvoicePayment | null | undefined)[];
  createdAt: string;
  notes: string;
  notesLabel: string;
  emptyNotes: string;
  children?: ReactNode;
}>) {
  const deadlines = invoiceDueDatesText(payments);
  const hasLegacy = payments.some((payment) => !payment?.dueOn);
  return (
    <DataGrid columns={2}>
      {deadlines ? (
        <DataField label="Fälligkeiten">{deadlines}</DataField>
      ) : null}
      {hasLegacy ? (
        <DataField
          label={deadlines ? "Zahlen bis (ältere Rechnungen)" : "Zahlen bis"}
        >
          {formatDate(paymentDueOn)}
        </DataField>
      ) : null}
      <DataField label="Erstellt am">{formatBerlinDate(createdAt)}</DataField>
      <DataField label={notesLabel}>
        {notes ? (
          <span className="whitespace-pre-line">{notes}</span>
        ) : (
          emptyNotes
        )}
      </DataField>
      {children}
    </DataGrid>
  );
}

/**
 * Titel, Satz und Vorlage der Schreiben je Dokumentart, die die Freigabe
 * festgehalten hat (#4688). Ohne Art bleibt es das Bestätigungsschreiben.
 */
const LETTER_KINDS: Readonly<
  Record<string, { title: string; description: string; template: string }>
> = {
  confirmation: {
    title: "Bestätigungsschreiben",
    description:
      "Jede Familie bekommt ein Schreiben, das ihre Anmeldung bestätigt und zugleich ihre Rechnung ist.",
    template: "bestaetigung",
  },
  invoice: {
    title: "Rechnungen",
    description: "Jede Familie bekommt ihre Rechnung als Schreiben.",
    template: "rechnung",
  },
  fee_notice: {
    title: "Gebührenbescheide",
    description:
      "Jede Familie bekommt einen Gebührenbescheid mit Ihrer Rechtsbehelfsbelehrung.",
    template: "bescheid",
  },
};

export function BillingLettersCard({
  runId,
  payments,
  letters,
  onCreated,
  documentKind,
}: Readonly<{
  runId: string;
  payments: readonly (InvoicePayment | null | undefined)[];
  letters: BillingLetters | null;
  onCreated: (letters: BillingLetters) => void;
  /** Die Dokumentart, die die Freigabe festgehalten hat (#4688). */
  documentKind?: string;
}>) {
  const kind =
    LETTER_KINDS[documentKind ?? "confirmation"] ?? LETTER_KINDS.confirmation!;
  if (!letters) {
    return (
      <LettersCreateCard
        title={kind.title}
        description={kind.description}
        runId={runId}
        payments={payments}
        onCreated={onCreated}
      >
        Logo, Absender und Fußzeile kommen aus <LetterheadLink />. Die
        Bankverbindung kommt aus den <BillingSettingsLink />, der Text aus der{" "}
        <NavigationLink
          href={traegerPath(
            `/traeger/einstellungen/briefkopf/vorlagen/${kind.template}`,
          )}
          className="font-medium underline"
        >
          Vorlage
        </NavigationLink>
        . Ohne Bankverbindung lassen sich keine Schreiben erstellen; sie legt
        die Verwaltung fest.
      </LettersCreateCard>
    );
  }

  return (
    <SectionCard
      title={kind.title}
      description="Die Schreiben sind erstellt und lassen sich nicht mehr ändern. Das Schreiben einer Familie laden Sie über das Menü ihrer Rechnung herunter."
    >
      <LettersFacts
        paymentDueOn={letters.paymentDueOn}
        payments={payments}
        createdAt={letters.createdAt}
        notes={letters.notes}
        notesLabel="Hinweise für die Eltern"
        emptyNotes="Keine"
      >
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
        <DataField label="Vorlage">
          {letters.templateVersion > 0
            ? `Version ${letters.templateVersion}`
            : "Standardvorlage"}
        </DataField>
      </LettersFacts>
    </SectionCard>
  );
}
