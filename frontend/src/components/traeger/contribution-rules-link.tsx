import NavigationLink from "~/components/ui/navigation-link";
import { traegerPath } from "~/lib/traeger-url";

const RULE_KINDS: readonly string[] = ["sibling", "benefits_exemption"];

/**
 * Der Weg von einer angewandten Regel zu den Regeln selbst (#5685): nur, wenn
 * unter den Posten eine Ermäßigung oder Befreiung steht.
 */
export function ContributionRulesLink({
  kinds,
  canManageBillingSettings,
}: Readonly<{
  kinds: readonly string[];
  canManageBillingSettings: boolean;
}>) {
  if (
    !canManageBillingSettings ||
    !kinds.some((kind) => RULE_KINDS.includes(kind))
  ) {
    return null;
  }
  return (
    <p className="mt-3 text-sm text-gray-700">
      Die Regeln stehen in den{" "}
      <NavigationLink
        href={traegerPath("/traeger/einstellungen/abrechnung")}
        className="font-medium underline"
      >
        Abrechnungsvorgaben
      </NavigationLink>
      .
    </p>
  );
}
