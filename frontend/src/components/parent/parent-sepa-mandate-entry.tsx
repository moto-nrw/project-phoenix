"use client";

// Einstieg zum Lastschrift-Mandat an einem Vertrag in „Dokumente“ (#4696).
// Der Knopf erscheint nur, wenn der Träger das Mandat in der App annimmt;
// ein gültiges Mandat zeigt sich als grüne Marke daneben.

import { useTranslations } from "next-intl";

import { ButtonLink } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { parentPath } from "~/lib/parent-url";
import type { ParentDocument } from "~/lib/parent-api";

export function ParentSepaMandateEntry({
  organizationId,
  contractId,
  status,
}: Readonly<{
  organizationId: string;
  contractId: string;
  status: ParentDocument["sepa_mandate"];
}>) {
  const t = useTranslations("parentSepaMandate");
  if (!status?.enabled) return null;
  return (
    <>
      <ButtonLink
        href={parentPath(
          `/documents/vertrag/${encodeURIComponent(organizationId)}/${encodeURIComponent(contractId)}?lastschrift=1`,
        )}
        variant="outline"
        size="md"
        tooltip={t("inboxButtonTooltip")}
      >
        {t("inboxButton")}
      </ButtonLink>
      {status.collection_eligible ? (
        <StatusBadge tone="green" label={t("inboxActive")} />
      ) : null}
    </>
  );
}
