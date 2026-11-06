import type { TenantPageProps } from "~/components/ui/tenant-page";
import { traegerPath } from "~/lib/traeger-url";

type CareContractTab = "vertraege" | "bedingungen";

/**
 * Die Reiter der Verträge (#4678): die Verträge selbst und die
 * Vertragsbedingungen je Angebot. Die Reiter sind Links.
 */
export function careContractTabs(
  active: CareContractTab,
  missingConditions?: number,
): NonNullable<TenantPageProps["tabs"]> {
  return {
    value: active,
    onChange: () => undefined,
    label: "Bereiche der Verträge",
    items: [
      {
        value: "vertraege",
        label: "Verträge",
        href: traegerPath("/traeger/vertraege"),
      },
      {
        value: "bedingungen",
        label: "Vertragsbedingungen",
        badge: missingConditions || undefined,
        href: traegerPath("/traeger/vertraege/bedingungen"),
      },
    ],
  };
}
