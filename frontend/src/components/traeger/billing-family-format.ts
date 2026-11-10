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
 * The registers of billing, in three areas with a sidebar entry each: the
 * runs (holiday and monthly), the families with what they pay, and the prices
 * with their reductions. Each register is a page of its own, so the tab is a
 * link; a page shows the tabs of its own area only.
 */
export type BillingTab =
  | "laeufe"
  | "monatsbeitraege"
  | "preise"
  | "mensa"
  | "ermaessigungen"
  | "nachweise"
  | "kinderzahl"
  | "kostentraeger"
  | "familien"
  | "lastschrift";

type BillingArea = "laeufe" | "familien" | "preise";

interface BillingRegister {
  readonly value: BillingTab;
  readonly label: string;
  readonly path: string;
}

const BILLING_AREAS: Record<
  BillingArea,
  { readonly title: string; readonly registers: readonly BillingRegister[] }
> = {
  laeufe: {
    title: "Abrechnungsläufe",
    registers: [
      { value: "laeufe", label: "Ferien", path: "/traeger/abrechnung" },
      {
        value: "monatsbeitraege",
        label: "Monatsbeiträge",
        path: "/traeger/abrechnung/monatsbeitraege",
      },
    ],
  },
  familien: {
    title: "Familien und Zahlung",
    registers: [
      {
        value: "familien",
        label: "Familien",
        path: "/traeger/abrechnung/familien",
      },
      {
        value: "lastschrift",
        label: "Lastschrift",
        path: "/traeger/abrechnung/lastschrift",
      },
      {
        value: "kinderzahl",
        label: "Kinderzahl",
        path: "/traeger/abrechnung/kinderzahl",
      },
      {
        value: "nachweise",
        label: "Nachweise",
        path: "/traeger/abrechnung/nachweise",
      },
      {
        value: "kostentraeger",
        label: "Kostenübernahmen",
        path: "/traeger/abrechnung/kostentraeger",
      },
    ],
  },
  preise: {
    title: "Preise und Ermäßigungen",
    registers: [
      {
        value: "preise",
        label: "Preisliste",
        path: "/traeger/abrechnung/preise",
      },
      { value: "mensa", label: "Mensa", path: "/traeger/abrechnung/mensa" },
      {
        value: "ermaessigungen",
        label: "Ermäßigungen",
        path: "/traeger/abrechnung/ermaessigungen",
      },
    ],
  },
};

function billingArea(tab: BillingTab): BillingArea {
  const found = (Object.keys(BILLING_AREAS) as BillingArea[]).find((area) =>
    BILLING_AREAS[area].registers.some((register) => register.value === tab),
  );
  return found ?? "laeufe";
}

/** The page title of a register: the name of its area in the sidebar. */
export function billingAreaTitle(tab: BillingTab): string {
  return BILLING_AREAS[billingArea(tab)].title;
}

export function billingTabs(
  active: BillingTab,
): NonNullable<TenantPageProps["tabs"]> {
  const area = BILLING_AREAS[billingArea(active)];
  return {
    value: active,
    onChange: () => undefined,
    label: `Bereiche: ${area.title}`,
    items: area.registers.map((register) => ({
      value: register.value,
      label: register.label,
      href: traegerPath(register.path),
    })),
  };
}

/**
 * What an invoice still asks for, as a pill. An invoice without storno or
 * credit is „Ausgestellt“, not „Offen“: moto does not see payments.
 */
export const INVOICE_STATUS_LABELS: Record<BillingInvoiceStatus, string> = {
  open: "Ausgestellt",
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
