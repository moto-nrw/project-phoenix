"use client";

// Die SEPA-Lastschriftmandate aller Familien (#4695): Referenz, Konto nur
// verdeckt, Datum der Unterschrift, nächste Abbuchung und Stand, mit Verlauf
// und Widerruf. Ein neues Mandat entsteht beim Vertrag der Familie, wo auch
// der Zahlungsweg steht. Nur das Recht „Abrechnung“ sieht die Seite; jedes
// Lesen steht im Verlauf des Mandats.

import { useSession } from "next-auth/react";
import { useMemo, useState } from "react";
import {
  billingAreaTitle,
  billingTabs,
} from "~/components/traeger/billing-family-format";
import {
  SepaMandatesCard,
  useSepaMandates,
} from "~/components/traeger/sepa-mandates-card";
import type { FilterConfig } from "~/components/ui/page-header/types";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { ListSkeleton } from "~/components/ui/page-skeletons";
import { hasPermission } from "~/lib/auth-utils";

type Show = "in_force" | "revoked" | "all";

/** How the card names what the filter shows. */
const SCOPE_LABELS: Record<Show, string> = {
  in_force: "Gültige",
  revoked: "Widerrufene",
  all: "Alle",
};

const SHOW_OPTIONS: { value: Show; label: string }[] = [
  { value: "in_force", label: "Gültig" },
  { value: "revoked", label: "Widerrufen" },
  { value: "all", label: "Alle" },
];

const INFO =
  "Mit einem SEPA-Mandat buchen Sie den Beitrag einer Familie ab. Ein neues Mandat erfassen Sie beim Vertrag der Familie. Ihre Gläubiger-ID steht unter Einstellungen → Abrechnungsvorgaben.";

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
      title={billingAreaTitle("lastschrift")}
      info={INFO}
      tabs={billingTabs("lastschrift")}
      error={error}
      // A list loads in the shape of its rows.
      loading={
        canBill && !error && mandates === null ? (
          <ListSkeleton rows={6} avatar={false} />
        ) : (
          false
        )
      }
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
      // Every choice that hides mandates shows as active, the default
      // „Gültig“ too: on a phone the filters sit behind one button.
      activeFilters={
        show === "all"
          ? []
          : [
              {
                id: "show",
                label:
                  SHOW_OPTIONS.find((option) => option.value === show)?.label ??
                  "",
                onRemove: () => setShow("all"),
              },
            ]
      }
      onClearAllFilters={() => setShow("all")}
      // The selected segment already shows the filter on a wide screen; a
      // chip row would show it twice. A phone shows the count on its button.
      activeFilterDisplay="count"
    >
      {mandates ? (
        <SepaMandatesCard
          mandates={shown}
          scopeLabel={SCOPE_LABELS[show]}
          onChanged={() => void load()}
        />
      ) : null}
    </TenantPage>
  );
}
