"use client";

// Ob die Familie eines Vertrags ein gültiges SEPA-Mandat hat (#4695), mit dem
// Weg zur Mandatsseite. Nur Anzeige: Erfassen, Widerruf und Verlauf liegen auf
// der eigenen Seite, damit Vertrags- und Empfängerseite die Formulare nicht
// laden. Eine Rolle ohne „Abrechnung“ bekommt 403; dann bleibt die Karte weg.

import { useEffect, useState } from "react";
import { ButtonLink } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { type SepaMandate, fetchSepaMandates } from "~/lib/traeger-sepa-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "SepaMandateStatusCard" });

type State =
  | { readonly kind: "loading" }
  | { readonly kind: "hidden" }
  | { readonly kind: "failed" }
  | { readonly kind: "loaded"; readonly inForce: SepaMandate | undefined };

/** The address of the mandate page of a contract's family. */
export function sepaMandatePath(contractId: string): string {
  return traegerPath(
    `/traeger/abrechnung/lastschrift/${encodeURIComponent(contractId)}`,
  );
}

export function SepaMandateStatusCard({
  contractId,
}: Readonly<{ contractId: string }>) {
  const [state, setState] = useState<State>({ kind: "loading" });

  useEffect(() => {
    let current = true;
    setState({ kind: "loading" });
    fetchSepaMandates(contractId)
      .then((mandates) => {
        if (!current) return;
        setState({
          kind: "loaded",
          inForce: mandates.find((mandate) => !mandate.revokedOn),
        });
      })
      .catch((err: unknown) => {
        if (!current) return;
        if (err instanceof ApiError && err.status === 403) {
          setState({ kind: "hidden" });
          return;
        }
        // The page around it still works; the card says it could not load.
        logger.error("sepa_mandate_status_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setState({ kind: "failed" });
      });
    return () => {
      current = false;
    };
  }, [contractId]);

  if (state.kind === "hidden") return null;
  const inForce = state.kind === "loaded" ? state.inForce : undefined;

  return (
    <SectionCard
      title="SEPA-Lastschriftmandat"
      titleBadge={
        state.kind === "loaded" ? (
          inForce ? (
            <StatusBadge tone="green" label="Gültig" />
          ) : (
            <StatusBadge tone="orange" label="Kein gültiges Mandat" />
          )
        ) : null
      }
      description={
        state.kind === "failed"
          ? "Der Stand des Mandats konnte nicht geladen werden."
          : inForce
            ? `Mandatsreferenz ${inForce.reference} · Konto ${inForce.ibanMasked}`
            : "Ohne Mandat bekommt die Familie eine Rechnung zum Überweisen."
      }
      loading={state.kind === "loading"}
      actions={
        <ButtonLink
          href={sepaMandatePath(contractId)}
          variant="outline"
          size="md"
        >
          {inForce ? "Mandat ansehen" : "Mandat erfassen"}
        </ButtonLink>
      }
    />
  );
}
