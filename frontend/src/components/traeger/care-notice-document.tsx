"use client";

// Seite 1 eines Jahresbescheids (#6219), so aufgebaut wie der Bescheid der
// Gemeinde Hardt (MK-007): Kopf mit Absender, Anschrift und Auskunft rechts,
// Bescheid-Nr. groß, dann „I. Berechnung“ je Grundlage mit bisher, neu und
// Zu-/Abgang, „II. Abrechnung“ mit den Raten und dem Zahlungsweg, zuletzt
// die Rechtsbehelfsbelehrung des Trägers. Seite 2 von MK-007 liegt nicht vor
// und wird nicht nachgebaut.

import { DataField } from "~/components/ui/data-field";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import type {
  CareNotice,
  CareNoticeInstallment,
  CareNoticeLetter,
  CareNoticeLine,
} from "~/lib/traeger-care-notices-api";
import { formatMonth } from "./care-billing-format";
import {
  NOTICE_KIND_LABELS,
  NOTICE_KIND_TONES,
  formatDelta,
  noticePeriod,
} from "./care-notice-format";
import { formatEuro } from "./contribution-format";

function lineColumns(change: boolean): DataTableColumn<CareNoticeLine>[] {
  const columns: DataTableColumn<CareNoticeLine>[] = [
    {
      key: "basis",
      header: "Berechnungsgrundlage",
      render: (line) => (
        <div className="min-w-0">
          <p className="font-medium text-gray-900">
            {line.kind === "cost_share" ? `Anteil ${line.label}` : line.label}
          </p>
          <p className="text-xs text-gray-600">
            {[
              line.detail,
              `Zeitraum ${noticePeriod(line.periodFrom, line.periodUntil)}`,
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
        </div>
      ),
      stacked: "title",
    },
  ];
  if (change) {
    columns.push({
      key: "monthly_before",
      header: "Monat bisher",
      align: "right",
      className: "tabular-nums text-gray-600",
      render: (line) => formatEuro(line.monthlyPreviousCents),
    });
  }
  columns.push(
    {
      key: "monthly",
      header: change ? "Monat neu" : "Betrag im Monat",
      align: "right",
      className: "tabular-nums",
      render: (line) => formatEuro(line.monthlyCents),
      stackedRender: (line) =>
        `${formatEuro(line.monthlyCents)} im Monat · ${noticePeriod(line.periodFrom, line.periodUntil)}`,
      stackedLabel: change,
    },
    {
      key: "quantity",
      header: "Anzahl",
      align: "right",
      className: "tabular-nums",
      render: (line) => line.quantity,
      stacked: change ? "field" : "hidden",
    },
  );
  if (change) {
    columns.push({
      key: "annual_before",
      header: "Zeitraum bisher",
      align: "right",
      className: "tabular-nums text-gray-600",
      render: (line) => formatEuro(line.annualPreviousCents),
    });
  }
  columns.push(
    {
      key: "annual",
      header: change ? "Zeitraum neu" : "Betrag im Zeitraum",
      align: "right",
      className: "tabular-nums font-medium text-gray-900",
      render: (line) => formatEuro(line.annualCents),
      stacked: change ? "field" : "meta",
    },
    {
      key: "delta",
      header: "Zu-/Abgang",
      align: "right",
      className: "tabular-nums",
      render: (line) => formatDelta(line.deltaCents),
      stacked: change ? "field" : "hidden",
    },
  );
  return columns;
}

function installmentColumns(
  change: boolean,
): DataTableColumn<CareNoticeInstallment>[] {
  const columns: DataTableColumn<CareNoticeInstallment>[] = [
    {
      key: "month",
      header: "Rate für",
      render: (rate) => (
        <span className="font-medium text-gray-900">
          {formatMonth(rate.month)}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "due",
      header: "Fällig",
      render: (rate) =>
        rate.immediatelyDue ? (
          <StatusBadge label="sofort fällig" tone="orange" size="xs" />
        ) : (
          <span className="tabular-nums">am {formatDate(rate.dueOn)}</span>
        ),
      stacked: "field",
      stackedLabel: false,
    },
  ];
  if (change) {
    columns.push({
      key: "before",
      header: "bisher",
      align: "right",
      className: "tabular-nums text-gray-600",
      render: (rate) => formatEuro(rate.previousCents),
    });
  }
  columns.push({
    key: "amount",
    header: change ? "neu" : "Betrag",
    align: "right",
    className: "tabular-nums font-medium text-gray-900",
    render: (rate) => formatEuro(rate.amountCents),
    stacked: change ? "field" : "meta",
  });
  if (change) {
    columns.push({
      key: "delta",
      header: "Zu-/Abgang",
      align: "right",
      className: "tabular-nums",
      render: (rate) => formatDelta(rate.deltaCents),
    });
  }
  return columns;
}

/** What the family does with the rates, by the payment method. */
function paymentText(notice: CareNotice): string {
  return notice.paymentMethod === "direct_debit"
    ? "Die fälligen Raten werden per SEPA-Lastschrift eingezogen. Bitte nicht überweisen."
    : `Bitte überweisen Sie jede Rate bis zum Fälligkeitstag. Geben Sie dabei die Bescheid-Nr.${notice.number ? ` ${notice.number}` : ""} an.`;
}

function RecipientBlock({ letter }: Readonly<{ letter: CareNoticeLetter }>) {
  const { recipient } = letter.notice;
  const sender = letter.senderText.split("\n")[0]?.trim() ?? "";
  return (
    <address className="not-italic">
      {sender ? (
        <p className="mb-2 border-b border-gray-200 pb-1 text-xs text-gray-500">
          {sender}
        </p>
      ) : null}
      <p className="text-sm text-gray-900">
        {recipient.firstName} {recipient.lastName}
      </p>
      {recipient.secondName ? (
        <p className="text-sm text-gray-900">{recipient.secondName}</p>
      ) : null}
      {recipient.address ? (
        <>
          <p className="text-sm text-gray-900">{recipient.address.street}</p>
          <p className="text-sm text-gray-900">
            {recipient.address.postalCode} {recipient.address.city}
          </p>
        </>
      ) : (
        <p className="text-sm text-gray-600">{recipient.email}</p>
      )}
    </address>
  );
}

/** Page 1 of a Jahresbescheid with the letterhead of the Träger. */
export function CareNoticeDocument({
  letter,
}: Readonly<{ letter: CareNoticeLetter }>) {
  const { notice } = letter;
  const change = notice.kind === "change";
  const draft = notice.number === "";
  return (
    <SectionCard
      as="article"
      bodyClassName="space-y-8 sm:p-5"
      className="mx-auto w-full max-w-4xl"
    >
      <header className="grid gap-6 sm:grid-cols-[1fr_minmax(0,18rem)]">
        <RecipientBlock letter={letter} />
        <div className="space-y-4">
          {letter.logoDataUrl ? (
            // A data URL of the Träger's own logo: next/image cannot optimize it.
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={letter.logoDataUrl}
              alt="Logo des Trägers"
              className="ml-auto h-14 w-auto object-contain"
            />
          ) : null}
          <dl className="space-y-1">
            <DataField inline label="Datum">
              {notice.issuedOn
                ? formatDate(notice.issuedOn)
                : "bei Ausstellung"}
            </DataField>
            <DataField inline label="Seite">
              1
            </DataField>
            {letter.contactName ? (
              <DataField inline label="Auskunft">
                {letter.contactName}
                {letter.contactPhone ? (
                  <span className="block text-gray-600">
                    Tel. {letter.contactPhone}
                  </span>
                ) : null}
                {letter.contactEmail ? (
                  <span className="block text-gray-600">
                    {letter.contactEmail}
                  </span>
                ) : null}
              </DataField>
            ) : null}
          </dl>
          <InsetPanel padding="md">
            <p className="text-xs text-gray-600">Bescheid-Nr.</p>
            <p className="text-xl font-semibold tracking-tight text-gray-900 tabular-nums">
              {draft ? "wird vergeben" : notice.number}
            </p>
            <p className="mt-1 text-xs text-gray-600">
              Bitte bei allen Zahlungen und Rückfragen angeben.
            </p>
          </InsetPanel>
        </div>
      </header>

      <div className="space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-xl font-semibold text-gray-900">
            {change ? "Änderungsbescheid" : "Gebührenbescheid"} für die
            Betreuung {notice.year}
          </h2>
          <StatusBadge
            label={NOTICE_KIND_LABELS[notice.kind]}
            tone={NOTICE_KIND_TONES[notice.kind]}
          />
        </div>
        <p className="text-sm text-gray-700">
          {notice.childFirstName} {notice.childLastName} · {notice.schoolName} ·
          Zeitraum {noticePeriod(notice.periodFrom, notice.periodUntil)}
        </p>
        {notice.amends ? (
          <p className="text-sm text-gray-700">
            Ändert den Bescheid Nr. {notice.amends.number}
            {notice.amends.issuedOn
              ? ` vom ${formatDate(notice.amends.issuedOn)}`
              : ""}
            .
          </p>
        ) : null}
      </div>

      <section className="space-y-3" aria-labelledby="notice-calculation">
        <SectionLabel as="h3" id="notice-calculation">
          I. Berechnung
        </SectionLabel>
        <DataTable
          columns={lineColumns(change)}
          rows={[...notice.lines]}
          stackedDetailKey="monthly"
          getRowKey={(line) =>
            `${line.kind}-${line.label}-${line.detail}-${line.periodFrom}`
          }
          stackedOnMobile
          stackedLayout={change ? "lines" : "row"}
          dense
        />
        <dl className="ml-auto max-w-sm space-y-1 [&>div:last-child]:border-t [&>div:last-child]:border-gray-200 [&>div:last-child]:pt-2">
          {change ? (
            <>
              <DataField inline label="Bisher festgesetzt">
                <span className="tabular-nums">
                  {formatEuro(notice.previousCents)}
                </span>
              </DataField>
              <DataField inline label="Zu-/Abgang">
                <span className="tabular-nums">
                  {formatDelta(notice.deltaCents)}
                </span>
              </DataField>
            </>
          ) : null}
          <DataField inline label="Gesamtbetrag">
            <span className="text-base font-semibold tabular-nums">
              {formatEuro(notice.totalCents)}
            </span>
          </DataField>
        </dl>
      </section>

      <section className="space-y-3" aria-labelledby="notice-payment">
        <SectionLabel as="h3" id="notice-payment">
          II. Abrechnung
        </SectionLabel>
        <p className="text-sm text-gray-700">
          Der Gesamtbetrag ist in Monatsraten zum Monatsersten fällig. Raten für
          Monate, die schon begonnen haben, sind sofort fällig.{" "}
          {paymentText(notice)}
        </p>
        <DataTable
          columns={installmentColumns(change)}
          rows={[...notice.installments]}
          stackedDetailKey="due"
          getRowKey={(rate) => rate.month}
          stackedOnMobile
          stackedLayout={change ? "lines" : "row"}
          dense
        />
      </section>

      <section className="space-y-2" aria-labelledby="notice-legal-remedy">
        <SectionLabel as="h3" id="notice-legal-remedy">
          Rechtsbehelfsbelehrung
        </SectionLabel>
        {notice.legalRemedy ? (
          <p className="text-sm whitespace-pre-line text-gray-700">
            {notice.legalRemedy}
          </p>
        ) : (
          <p className="text-sm text-gray-600">
            Die Vorlage „Bescheid“ hat noch keine Rechtsbehelfsbelehrung.
          </p>
        )}
      </section>

      {letter.footerText ? (
        <footer className="border-t border-gray-200 pt-4 text-xs whitespace-pre-line text-gray-500">
          {letter.footerText}
        </footer>
      ) : null}
    </SectionCard>
  );
}
