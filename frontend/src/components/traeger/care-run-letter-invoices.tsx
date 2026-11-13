"use client";

// Die Rechnungen eines freigegebenen Monatslaufs auf der Seite „Schreiben“
// (#4684). Nach der Freigabe trennt die Zeile „ausgestellt“ (Nummer und
// Datum) von „an die Familie“ (Stand der Mail). Schreiben, Senden und
// Stornieren stehen im Menü der Zeile. Nur diese Seite lädt den Code dafür.

import { Eye, Send, Undo2 } from "lucide-react";
import type { DataTableColumn } from "~/components/ui/data-table";
import {
  type OverflowMenuItem,
  OverflowMenu,
} from "~/components/ui/page-header/OverflowMenu";
import type {
  CareInvoice,
  CareLetterMailing,
} from "~/lib/traeger-care-billing-api";
import { canSendMail } from "./billing-mail-status";
import { type CareInvoiceRow, CareRunInvoices } from "./care-run-invoices";
import { LetterSendCell } from "./letter-send-cell";

/** What the run does with the letters of its invoices, from the page. */
export interface CareInvoiceLetterActions {
  /** Per invoice and kind, as `GET …/letters` lists them. */
  readonly letters: readonly CareLetterMailing[];
  /** The letters exist; before that a row has nothing to show or send. */
  readonly created: boolean;
  /** The row whose mail is being sent, as `kind-invoiceId`. */
  readonly sendingKey: string | null;
  readonly onShowLetter: (letter: CareLetterMailing) => void;
  readonly onSend: (letter: CareLetterMailing) => void;
  readonly onCancel: (invoice: CareLetterMailing) => void;
}

/** „An die Familie“: what reached the family, apart from what is issued. */
function SendCell({
  row,
  created,
}: Readonly<{ row: CareInvoiceRow; created: boolean }>) {
  const { letter } = row;
  if (row.storno && letter && !letter.hasLetter) {
    return (
      <p className="text-sm text-gray-600">
        Kein Schreiben. Die Familie hatte die Rechnung noch nicht.
      </p>
    );
  }
  if (!created || !letter?.hasLetter) {
    return <p className="text-sm text-gray-600">Noch kein Schreiben</p>;
  }
  if (!row.storno && letter.cancelled && letter.sends === 0) {
    return (
      <p className="text-sm text-gray-600">
        {letter.printed
          ? "Zum Drucken heruntergeladen. Geben Sie der Familie das Storno-Schreiben."
          : "Nicht gesendet. Die Familie bekommt nichts."}
      </p>
    );
  }
  return (
    <LetterSendCell
      letter={letter}
      printInstruction="Bitte drucken Sie das Schreiben."
    />
  );
}

function rowMenuItems(
  row: CareInvoiceRow,
  actions: CareInvoiceLetterActions,
): OverflowMenuItem[] {
  const { letter } = row;
  if (!letter) return [];
  const items: OverflowMenuItem[] = [];
  const live = row.storno || !letter.cancelled;
  if (actions.created && letter.hasLetter) {
    items.push({
      label: "Schreiben ansehen",
      icon: <Eye className="size-4" aria-hidden />,
      description: "So, wie die Familie es bekommt",
      onClick: () => actions.onShowLetter(letter),
    });
    if (live && canSendMail(letter) && actions.sendingKey === null) {
      items.push({
        label: letter.sends === 0 ? "Senden" : "Erneut senden",
        icon: <Send className="size-4" aria-hidden />,
        description:
          letter.sends === 0
            ? letter.inInbox
              ? "Per E-Mail und in die Eltern-App"
              : "Nur per E-Mail, nicht in die Eltern-App"
            : "Noch einmal an dieselbe Adresse",
        onClick: () => actions.onSend(letter),
      });
    }
  }
  if (!row.storno && !letter.cancelled) {
    items.push({
      label: "Stornieren",
      icon: <Undo2 className="size-4" aria-hidden />,
      description: "Hebt die Rechnung auf, moto fragt vorher nach",
      destructive: true,
      onClick: () => actions.onCancel(letter),
    });
  }
  return items;
}

function letterColumns(
  actions: CareInvoiceLetterActions,
): DataTableColumn<CareInvoiceRow>[] {
  return [
    {
      key: "send",
      header: "An die Familie",
      stacked: "field",
      className: "w-48 align-top",
      render: (row) => <SendCell row={row} created={actions.created} />,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      className: "align-top",
      render: (row) => (
        <OverflowMenu
          ariaLabel={`Aktionen für ${row.storno ? "den Storno" : "die Rechnung"} von ${row.invoice.childFirstName} ${row.invoice.childLastName}`}
          items={rowMenuItems(row, actions)}
        />
      ),
    },
  ];
}

/** The invoices of a released run with their letter, mail and Storno. */
export function CareRunLetterInvoices({
  invoices,
  month,
  letterActions,
}: Readonly<{
  invoices: readonly CareInvoice[];
  month: string;
  letterActions: CareInvoiceLetterActions;
}>) {
  return (
    <CareRunInvoices
      invoices={invoices}
      month={month}
      released
      letters={letterActions.letters}
      letterColumns={letterColumns(letterActions)}
    />
  );
}
