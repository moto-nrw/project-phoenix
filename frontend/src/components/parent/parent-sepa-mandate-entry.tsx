"use client";

// Einstieg zum Lastschrift-Mandat an einem Vertrag in „Dokumente“ (#4696).
// Der Knopf erscheint nur, wenn der Träger das Mandat in der App annimmt;
// ein gültiges Mandat zeigt sich als grüne Marke daneben.

import { useTranslations } from "next-intl";
import { useEffect, useState } from "react";

import { ButtonLink } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { createLogger } from "~/lib/logger";
import { parentPath } from "~/lib/parent-url";
import {
  fetchSepaMandate,
  type SepaMandateView,
} from "~/lib/parent-sepa-mandate-api";

const logger = createLogger({ component: "ParentSepaMandateEntry" });

export function ParentSepaMandateEntry({
  organizationId,
  contractId,
}: Readonly<{ organizationId: string; contractId: string }>) {
  const t = useTranslations("parentSepaMandate");
  const [view, setView] = useState<SepaMandateView | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const next = await fetchSepaMandate(organizationId, contractId);
        if (!cancelled) setView(next);
      } catch (err) {
        // Silent on purpose: the entry is optional, and the page behind it
        // shows the error with a retry when the parents open it.
        logger.warn("parent_sepa_mandate_entry_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      }
    };
    void load();
    return () => {
      cancelled = true;
    };
  }, [organizationId, contractId]);

  if (!view?.enabled) return null;
  const active = view.mandate !== null && view.mandate.revokedOn === null;
  return (
    <>
      <ButtonLink
        href={parentPath(
          `/documents/vertrag/${encodeURIComponent(organizationId)}/${encodeURIComponent(contractId)}?lastschrift=1`,
        )}
        variant="outline"
        size="md"
      >
        {t("inboxButton")}
      </ButtonLink>
      {active ? <StatusBadge tone="green" label={t("inboxActive")} /> : null}
    </>
  );
}
