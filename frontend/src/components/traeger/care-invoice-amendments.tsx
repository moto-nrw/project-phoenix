"use client";

// Eine Rechnung der Prüfphase, wie die Freigabe sie ausstellt, und der
// Verlauf ihrer Änderungen (#6197). Die berechneten Positionen bleiben
// stehen; eine Änderung des Betrags oder der Positionen ist eine eigene
// Zeile mit Grund. Nur die Seite einer Rechnung lädt diese Datei.

import { Button } from "~/components/ui/button";
import { InsetPanel } from "~/components/ui/inset-panel";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatBerlinDate, formatBerlinTime } from "~/lib/date-helpers";
import {
  type CareAmendment,
  type CareInvoice,
  amendmentApplied,
  careContributionCents,
  careCorrectionLines,
} from "~/lib/traeger-care-billing-api";
import { WEEKDAY_SHORT } from "./care-billing-format";
import { COST_SHARE_KIND_LABELS } from "./care-cost-share-format";
import { mealPositionLabel } from "./care-meal-format";
import { MealPositionLine } from "./care-run-meals";
import { formatEuro } from "./contribution-format";
import type { CareAmendmentDraft } from "./care-invoice-amendment-modal";
import { InvoiceLine } from "./invoice-line";

/** "08.11.2026, 16:20" in Berlin time. */
function formatInstant(value: string): string {
  if (!value) return "";
  return `${formatBerlinDate(value)}, ${formatBerlinTime(value)}`;
}

/** The positions an applied change took off. */
function removedKeys(invoice: CareInvoice): Set<string> {
  return new Set(
    (invoice.amendments ?? [])
      .filter(
        (amendment) =>
          amendment.kind === "remove_position" && amendmentApplied(amendment),
      )
      .map((amendment) => amendment.positionKey),
  );
}

function changed(invoice: CareInvoice, kind: "recipient" | "address"): boolean {
  return (invoice.amendments ?? []).some(
    (amendment) => amendment.kind === kind && amendmentApplied(amendment),
  );
}

/** The recipient the invoice goes to, marked when the review changed it. */
function RecipientPanel({ invoice }: Readonly<{ invoice: CareInvoice }>) {
  const { recipient } = invoice;
  const name = [
    `${recipient.firstName} ${recipient.lastName}`.trim(),
    recipient.secondName,
  ]
    .filter(Boolean)
    .join(" und ");
  return (
    <InsetPanel padding="md">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-0.5 text-sm">
          <p className="font-semibold text-gray-900">{name || "–"}</p>
          <p className="break-words text-gray-600">
            {recipient.email || "Ohne E-Mail"}
          </p>
          {recipient.address ? (
            <p className="text-gray-600">
              {recipient.address.street}, {recipient.address.postalCode}{" "}
              {recipient.address.city}
            </p>
          ) : (
            <p className="text-gray-500">Ohne Anschrift</p>
          )}
        </div>
        <div className="flex flex-wrap gap-1.5">
          {changed(invoice, "recipient") ? (
            <StatusBadge tone="blue" label="Empfänger geändert" />
          ) : null}
          {changed(invoice, "address") ? (
            <StatusBadge tone="blue" label="Anschrift geändert" />
          ) : null}
        </div>
      </div>
    </InsetPanel>
  );
}

/**
 * The invoice as the release issues it: recipient, every computed line,
 * the correction lines and the total. On a preview each position has the
 * menu to take it off.
 */
export function CareAmendedInvoiceCard({
  invoice,
  editable,
  onDraft,
}: Readonly<{
  invoice: CareInvoice;
  /** The review allows removing positions on this invoice. */
  editable: boolean;
  onDraft: (draft: CareAmendmentDraft) => void;
}>) {
  const removed = removedKeys(invoice);
  const corrections = careCorrectionLines(invoice);
  const issued = invoice.number !== "";
  return (
    <SectionCard
      title={
        issued
          ? `Rechnung ${invoice.number}`
          : "So wird die Rechnung ausgestellt"
      }
      description={
        issued
          ? "Ausgestellt. Die Rechnung ändert sich nicht mehr."
          : "Vorschau mit allen Änderungen. Die Nummer gibt erst die Freigabe."
      }
      overflow="visible"
    >
      <div className="space-y-4">
        <RecipientPanel invoice={invoice} />
        <ul className="divide-y divide-gray-100">
          {invoice.positions.map((position) => {
            const gone = removed.has(position.key);
            return (
              <li
                key={position.key || position.careOfferingId}
                className="flex items-center gap-2"
              >
                <div
                  className={`min-w-0 flex-1 ${gone ? "text-gray-500 line-through" : ""}`}
                >
                  <ul>
                    <InvoiceLine
                      title={position.moduleName}
                      detail={position.weekdays
                        .map((day) => WEEKDAY_SHORT[day.weekday])
                        .join(", ")}
                      amount={formatEuro(position.amountCents)}
                    />
                  </ul>
                </div>
                {gone ? <StatusBadge tone="gray" label="Entfällt" /> : null}
                {editable && !gone && position.key ? (
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${position.moduleName}`}
                    triggerSize="sm"
                    items={[
                      {
                        label: "Position entfernen",
                        description: "Mit Grund, die Zeile bleibt sichtbar",
                        onClick: () =>
                          onDraft({
                            kind: "remove_position",
                            positionKey: position.key,
                            label: position.moduleName,
                            amountCents:
                              position.payableCents ?? position.amountCents,
                          }),
                      },
                    ]}
                  />
                ) : null}
              </li>
            );
          })}
          {invoice.reductions.map((reduction, index) => (
            <InvoiceLine
              key={`${reduction.kind}-${index}`}
              title={reduction.name}
              detail="Ermäßigung"
              amount={formatEuro(reduction.amountCents)}
              amountClassName="text-moto-green-strong"
            />
          ))}
          {invoice.mealPositions.map((position) => {
            const key = position.key;
            const gone = removed.has(key ?? "");
            const label = mealPositionLabel(
              position.mealMonth,
              position.mode,
              position.quantity,
            );
            return (
              <li
                key={key ?? position.mealMonth}
                className="flex items-center gap-2"
              >
                <div
                  className={`min-w-0 flex-1 ${gone ? "text-gray-500 line-through" : ""}`}
                >
                  <ul>
                    <MealPositionLine position={position} />
                  </ul>
                </div>
                {gone ? <StatusBadge tone="gray" label="Entfällt" /> : null}
                {editable && !gone && key ? (
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${label}`}
                    triggerSize="sm"
                    items={[
                      {
                        label: "Position entfernen",
                        description: "Mit Grund, die Zeile bleibt sichtbar",
                        onClick: () =>
                          onDraft({
                            kind: "remove_position",
                            positionKey: key,
                            label,
                            amountCents: position.amountCents,
                          }),
                      },
                    ]}
                  />
                ) : null}
              </li>
            );
          })}
          {invoice.costShares.length > 0 ? (
            <InvoiceLine
              title="Monatsbeitrag"
              detail="vor den Kostenträgern"
              amount={formatEuro(careContributionCents(invoice))}
            />
          ) : null}
          {invoice.costShares.map((share) => (
            <InvoiceLine
              key={`${share.kind}-${share.costBearerId}`}
              title={share.costBearerName}
              detail={`übernimmt · ${COST_SHARE_KIND_LABELS[share.kind]}`}
              amount={formatEuro(-share.amountCents)}
              amountClassName="text-moto-blue-strong"
            />
          ))}
          {corrections.map((line) => (
            <InvoiceLine
              key={`correction-${line.id}`}
              title={
                <span className="inline-flex flex-wrap items-center gap-1.5">
                  {line.kind === "amount" ? "Korrektur" : line.label}
                  <StatusBadge tone="blue" size="xs" label="Änderung" />
                </span>
              }
              detail={`Grund: ${line.reason}`}
              amount={formatEuro(line.amountCents)}
              amountClassName="font-medium text-gray-900"
            />
          ))}
        </ul>
        <div className="flex items-baseline justify-between border-t border-gray-200 pt-3">
          <p className="text-sm font-semibold text-gray-900">
            {invoice.costShares.length > 0 ? "Anteil der Familie" : "Summe"}
          </p>
          <p className="text-lg font-semibold text-gray-900 tabular-nums">
            {formatEuro(invoice.totalCents)}
          </p>
        </div>
      </div>
    </SectionCard>
  );
}

const KIND_LABELS: Record<CareAmendment["kind"], string> = {
  amount: "Betrag geändert",
  add_position: "Position hinzugefügt",
  remove_position: "Position entfernt",
  recipient: "Empfänger geändert",
  address: "Anschrift geändert",
};

const CONFLICT_TEXTS: Record<NonNullable<CareAmendment["conflict"]>, string> = {
  basis_changed: "Der Wert davor hat sich geändert. Die Änderung gilt nicht.",
  position_missing:
    "Diese Position gibt es nicht mehr. Die Änderung gilt nicht.",
  negative_total: "Die Rechnung käme unter 0 €. Die Änderung gilt nicht.",
};

/** "77,50 € → 70,00 €", the old and the new value of a change. */
function changeText(amendment: CareAmendment): string {
  const { before, after } = amendment;
  switch (amendment.kind) {
    case "amount":
      return `${formatEuro(before.totalCents ?? 0)} → ${formatEuro(after.totalCents ?? 0)}`;
    case "add_position":
      return `${amendment.label}: ${formatEuro(amendment.amountCents)}`;
    case "remove_position":
      return `${before.label || amendment.label}: ${formatEuro(amendment.amountCents)}`;
    case "recipient": {
      const name = (value: CareAmendment["before"]) =>
        value.recipient
          ? `${value.recipient.firstName} ${value.recipient.lastName}`.trim()
          : "–";
      return `${name(before)} → ${name(after)}`;
    }
    case "address": {
      const place = (value: CareAmendment["before"]) =>
        value.address
          ? `${value.address.street}, ${value.address.city}`
          : "ohne Anschrift";
      return `${place(before)} → ${place(after)}`;
    }
  }
}

/**
 * Every change of the invoice in the order it was made, also the ones taken
 * back. A change in force can be revoked; the revocation is a new entry.
 */
export function CareAmendmentHistoryCard({
  invoice,
  editable,
  onDraft,
}: Readonly<{
  invoice: CareInvoice;
  editable: boolean;
  onDraft: (draft: CareAmendmentDraft) => void;
}>) {
  const entries = [...(invoice.amendments ?? [])].reverse();
  return (
    <SectionCard
      title="Verlauf der Änderungen"
      titleBadge={
        entries.length > 0 ? (
          <span className="text-sm font-normal text-gray-600 tabular-nums">
            {entries.length}
          </span>
        ) : undefined
      }
      description="Jede Änderung bleibt hier stehen, mit Grund und Zeit. Zurücknehmen legt einen neuen Eintrag an."
    >
      {entries.length === 0 ? (
        <p className="text-sm text-gray-600">
          Noch keine Änderung. Die Rechnung gilt, wie sie berechnet ist.
        </p>
      ) : (
        <ResponsiveActionList>
          {entries.map((amendment) => {
            const active = amendment.revocation === null;
            return (
              <li
                key={`${amendment.carriedFromNumber === null ? "run" : "carried"}-${amendment.id}`}
                className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
              >
                <div className="min-w-0 space-y-1">
                  <p className="flex flex-wrap items-center gap-1.5 text-sm">
                    <span
                      className={`font-semibold ${active ? "text-gray-900" : "text-gray-500 line-through"}`}
                    >
                      {KIND_LABELS[amendment.kind]}
                    </span>
                    {amendment.carriedFromNumber !== null ? (
                      <StatusBadge
                        tone="purple"
                        size="xs"
                        label={`Übernommen aus ${amendment.carriedFromNumber}`}
                      />
                    ) : null}
                    {amendment.revocation ? (
                      <StatusBadge
                        tone="gray"
                        size="xs"
                        label="Zurückgenommen"
                      />
                    ) : null}
                    {active && amendment.conflict ? (
                      <StatusBadge
                        tone="warning"
                        size="xs"
                        label="Passt nicht mehr"
                      />
                    ) : null}
                  </p>
                  <p
                    className={`text-sm tabular-nums ${active ? "text-gray-700" : "text-gray-500"}`}
                  >
                    {changeText(amendment)}
                  </p>
                  <p className="text-xs break-words text-gray-600">
                    Grund: {amendment.reason} ·{" "}
                    {formatInstant(amendment.amendedAt)}
                  </p>
                  {active && amendment.conflict ? (
                    <p className="text-moto-orange-strong text-xs font-medium">
                      {CONFLICT_TEXTS[amendment.conflict]}
                    </p>
                  ) : null}
                  {amendment.revocation ? (
                    <p className="text-xs break-words text-gray-600">
                      Zurückgenommen: {amendment.revocation.reason} ·{" "}
                      {formatInstant(amendment.revocation.revokedAt)}
                    </p>
                  ) : null}
                </div>
                {editable && active ? (
                  <div className="flex w-full min-[480px]:w-auto sm:justify-end">
                    <Button
                      type="button"
                      variant="surface"
                      size="compact"
                      aria-label={`${KIND_LABELS[amendment.kind]} zurücknehmen`}
                      onClick={() => onDraft({ kind: "revoke", amendment })}
                    >
                      Zurücknehmen
                    </Button>
                  </div>
                ) : null}
              </li>
            );
          })}
        </ResponsiveActionList>
      )}
    </SectionCard>
  );
}
