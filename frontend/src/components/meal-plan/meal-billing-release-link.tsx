"use client";

// Weg vom Speiseplan zur Mensa-Freigabe (#4686). Nur wer den Speiseplan
// pflegt und Listen mit Namen sieht, gibt einen Mensa-Monat frei.

import { useSession } from "next-auth/react";
import { ButtonLink } from "~/components/ui/button";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useMealRegistrationEnabled } from "~/lib/tenant-context";
import { useTenantAwarePath } from "~/lib/tenant-path";

export function MealBillingReleaseLink() {
  const { data: session, status } = useSession();
  const registrationEnabled = useMealRegistrationEnabled();
  const tenantPath = useTenantAwarePath();
  const allowed =
    status === "authenticated" &&
    registrationEnabled &&
    (isAdmin(session) ||
      (hasPermission(session, "meals:manage") &&
        hasPermission(session, "meals:orders")));
  if (!allowed) return null;
  return (
    <ButtonLink
      href={tenantPath("/meal-plan/freigabe")}
      variant="outline"
      size="md"
      tooltip="Öffnet die Monatszählung, die Sie für die Abrechnung freigeben."
    >
      Mensa-Freigabe
    </ButtonLink>
  );
}
