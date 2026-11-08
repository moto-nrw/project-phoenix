/** Display helpers of the invoices and amounts per family (#4691). */

import type { StatusBadgeTone } from "~/components/ui/status-badge";
import type { TenantPageProps } from "~/components/ui/tenant-page";
import type {
  BillingFamily,
  BillingInvoiceStatus,
  BillingPayment,
} from "~/lib/traeger-billing-families-api";
import { traegerPath } from "~/lib/traeger-url";

/**
 * The registers of the page „Abrechnung“: the holiday runs, the monthly
 * contribution with its price list (#4684) and the families. Each register
 * is a page of its own, so the tab is a link.
 */
export type BillingTab =
  | "laeufe"
  | "monatsbeitraege"
  | "jahresbescheide"
  | "preise"
  | "mensa"
  | "ermaessigungen"
  | "nachweise"
  | "kinderzahl"
  | "kostentraeger"
  | "familien";

export function billingTabs(
  active: BillingTab,
): NonNullable<TenantPageProps["tabs"]> {
  return {
    value: active,
    // Die Reiter sind Links; die Navigation übernimmt der Link selbst.
    onChange: () => undefined,
    label: "Bereiche der Abrechnung",
    items: [
      {
        value: "laeufe",
        label: "Ferien",
        href: traegerPath("/traeger/abrechnung"),
      },
      {
        value: "monatsbeitraege",
        label: "Monatsbeiträge",
        href: traegerPath("/traeger/abrechnung/monatsbeitraege"),
      },
      {
        value: "jahresbescheide",
        label: "Jahresbescheide",
        href: traegerPath("/traeger/abrechnung/jahresbescheide"),
      },
      {
        value: "preise",
        label: "Preisliste",
        href: traegerPath("/traeger/abrechnung/preise"),
      },
      {
        value: "mensa",
        label: "Mensa",
        href: traegerPath("/traeger/abrechnung/mensa"),
      },
      {
        value: "ermaessigungen",
        label: "Ermäßigungen",
        href: traegerPath("/traeger/abrechnung/ermaessigungen"),
      },
      {
        value: "nachweise",
        label: "Nachweise",
        href: traegerPath("/traeger/abrechnung/nachweise"),
      },
      {
        value: "kinderzahl",
        label: "Kinderzahl",
        href: traegerPath("/traeger/abrechnung/kinderzahl"),
      },
      {
        value: "kostentraeger",
        label: "Kostenträger",
        href: traegerPath("/traeger/abrechnung/kostentraeger"),
      },
      {
        value: "familien",
        label: "Familien",
        href: traegerPath("/traeger/abrechnung/familien"),
      },
    ],
  };
}

/** What an invoice still asks for, as a pill. */
export const INVOICE_STATUS_LABELS: Record<BillingInvoiceStatus, string> = {
  open: "Offen",
  partly_credited: "Teilgutschrift",
  credited: "Gutgeschrieben",
  cancelled: "Storniert",
  nothing_due: "Nichts zu zahlen",
};

export const INVOICE_STATUS_TONES: Record<
  BillingInvoiceStatus,
  StatusBadgeTone
> = {
  open: "orange",
  partly_credited: "blue",
  credited: "gray",
  cancelled: "red",
  nothing_due: "green",
};

/** Who pays an invoice, in one short phrase. */
export function paymentLabel(payment: BillingPayment): string {
  switch (payment.kind) {
    case "cost_bearer":
      return payment.costBearerName
        ? `Kostenübernahme: ${payment.costBearerName}`
        : "Kostenübernahme";
    case "exempt":
      return "Beitragsfrei";
    default:
      return "Familie zahlt selbst";
  }
}

export const PAYMENT_TONES: Record<BillingPayment["kind"], StatusBadgeTone> = {
  family: "gray",
  cost_bearer: "blue",
  exempt: "green",
};

export function familyName(family: BillingFamily): string {
  return `${family.recipientFirstName} ${family.recipientLastName}`.trim();
}

/** Sorts and searches by the last name first. */
export function familySortName(family: BillingFamily): string {
  return `${family.recipientLastName} ${family.recipientFirstName}`.toLocaleLowerCase(
    "de",
  );
}

/** Whether the search text names this family: name, address or number. */
export function familyMatches(family: BillingFamily, search: string): boolean {
  const needle = search.trim().toLocaleLowerCase("de");
  if (!needle) return true;
  return [
    familyName(family),
    family.recipientEmail,
    ...family.debtorNumbers,
  ].some((value) => value.toLocaleLowerCase("de").includes(needle));
}
