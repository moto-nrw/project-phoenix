"use client";

// Die SEPA-Lastschriftmandate aller Familien (#4695): Referenz, Konto nur
// verdeckt, Datum der Unterschrift, nächste Abbuchung und Stand, mit Verlauf
// und Widerruf. Ein neues Mandat entsteht beim Vertrag der Familie, wo auch
// der Zahlungsweg steht. Nur das Recht „Abrechnung“ sieht die Seite; jedes
// Lesen steht im Verlauf des Mandats.

import { useSession } from "next-auth/react";
import { useMemo, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import {
  SepaMandatesCard,
  useSepaMandates,
} from "~/components/traeger/sepa-mandates-card";
import { ButtonLink } from "~/components/ui/button";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { hasPermission } from "~/lib/auth-utils";
import { traegerPath } from "~/lib/traeger-url";

type Show = "in_force" | "revoked" | "all";

const SHOW_OPTIONS: { value: Show; label: string }[] = [
  { value: "in_force", label: "Gültig" },
  { value: "revoked", label: "Widerrufen" },
  { value: "all", label: "Alle" },
];

const INFO =
  "Mit einem SEPA-Mandat buchen Sie den Beitrag einer Familie ab. Ein neues Mandat erfassen Sie beim Vertrag der Familie. Mandate aus einem anderen Programm übernehmen Sie mit „Mandate importieren“. Ihre Gläubiger-ID steht unter Einstellungen → Abrechnung.";

export default function TraegerLastschriftPage() {
  const { data: session } = useSession();
  const canBill = hasPermission(session ?? null, "traeger_billing:manage");
  const { mandates, error, load } = useSepaMandates(canBill);
  const [show, setShow] = useState<Show>("in_force");
  const [search, setSearch] = useState("");

  const counts = useMemo(() => {
    const inForce = (mandates ?? []).filter((m) => !m.revokedOn).length;
    return { inForce, revoked: (mandates ?? []).length - inForce };
  }, [mandates]);
  const shown = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return (mandates ?? []).filter((mandate) => {
      const state =
        show === "all" ||
        (show === "revoked"
          ? mandate.revokedOn !== null
          : mandate.revokedOn === null);
      const text = [
        mandate.partyFirstName,
        mandate.partyLastName,
        mandate.accountHolder,
        mandate.reference,
        mandate.familyEmail,
      ]
        .join(" ")
        .toLowerCase();
      return state && (needle === "" || text.includes(needle));
    });
  }, [mandates, show, search]);
  const filters: FilterConfig[] = [
    {
      id: "show",
      label: "Stand",
      type: "buttons",
      value: show,
      onChange: (value) => setShow(value as Show),
      options: SHOW_OPTIONS,
    },
  ];

  return (
    <TenantPage
      title="Lastschrift"
      info={INFO}
      tabs={billingTabs("lastschrift")}
      actions={
        canBill ? (
          <ButtonLink
            href={traegerPath("/traeger/abrechnung/lastschrift/import")}
            variant="outline"
            size="md"
            tooltip="Öffnet den Import bestehender Mandate aus einer CSV-Datei."
          >
            Mandate importieren
          </ButtonLink>
        ) : null
      }
      error={error}
      loading={canBill && !error && mandates === null}
      stats={
        mandates ? (
          <TenantPageStats
            items={[
              { value: counts.inForce, label: "gültig" },
              { value: counts.revoked, label: "widerrufen" },
            ]}
          />
        ) : null
      }
      statsLoading={canBill && !error && mandates === null}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Familie, Kontoinhaber oder Referenz",
      }}
      filters={filters}
    >
      {mandates ? (
        <SepaMandatesCard mandates={shown} onChanged={() => void load()} />
      ) : null}
    </TenantPage>
  );
}
