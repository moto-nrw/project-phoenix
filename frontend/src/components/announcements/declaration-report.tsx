"use client";

import { sortDeclarationChildren } from "~/components/announcements/declaration-status-panel";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { formatBerlinDate, formatChatDateTime } from "~/lib/date-helpers";
import { formatBytes } from "~/lib/files-api";
import type {
  Announcement,
  DeclarationAction,
  DeclarationChildState,
  DeclarationStatus,
} from "~/lib/parent-announcements-api";

/**
 * The printable report of an Erklärung (#3430): settings, every version with
 * its full text and checksums, the state per child and the full history.
 * Built for paper first. Plain headings, lists and tables, no buttons, no
 * cards: it is printed or saved as PDF through the browser.
 *
 * Plain `<table>` elements on purpose: the kit `DataTable` is an interactive
 * list (sorting, stacked mobile rows rendered twice), which neither prints
 * as one table nor belongs on paper.
 */

const STATE_LABEL: Record<DeclarationChildState, string> = {
  open: "Offen",
  partial: "Teilweise abgegeben",
  agreed: "Zugestimmt",
  declined: "Abgelehnt",
  acknowledged: "Zur Kenntnis genommen",
  revoked: "Widerrufen",
  no_signer: "Niemand kann abgeben",
  expired: "Frist abgelaufen",
};

const ACTION_LABEL: Record<DeclarationAction, string> = {
  agreed: "Zugestimmt",
  declined: "Abgelehnt",
  acknowledged: "Zur Kenntnis genommen",
  revoked: "Widerrufen",
};

const ROLE_LABEL: Record<string, string> = {
  primary_guardian: "Hauptberechtigt",
  legal_guardian: "Erziehungsberechtigt",
  co_guardian: "Mitberechtigt",
};

const METHOD_LABEL: Record<string, string> = {
  simple_electronic: "Einfache elektronische Erklärung",
};

function dateTime(iso: string): string {
  return `${formatChatDateTime(iso)} Uhr`;
}

function name(first: string, last: string): string {
  return `${first} ${last}`.trim() || "Unbekannt";
}

const TH = "border-b border-gray-300 px-2 py-1.5 text-left font-semibold";
const TD = "border-b border-gray-200 px-2 py-1.5 align-top";
const HASH = "font-mono text-xs break-all";

export function DeclarationReport({
  announcement,
  status,
  generatedAt,
}: Readonly<{
  announcement: Announcement;
  status: DeclarationStatus;
  /** When the report was built; shown so a printout carries its moment. */
  generatedAt: string;
}>) {
  const consent = status.kind === "consent";
  const s = status.summary;

  return (
    <article className="space-y-6 text-sm leading-6 text-gray-900 print:text-[11pt]">
      <header className="space-y-1">
        <h2 className="hidden text-xl font-semibold print:block">
          Nachweisbericht: {announcement.title}
        </h2>
        <p className="text-gray-600">
          Erstellt am {dateTime(generatedAt)}. Stand der Erklärung zu diesem
          Zeitpunkt.
        </p>
      </header>

      {!status.integrity_ok && (
        <p className="text-moto-red-strong border-moto-red/30 rounded-lg border p-3 font-semibold">
          Achtung: Mindestens ein Eintrag passt nicht mehr zu seiner Prüfsumme.
          Bitte melden Sie das dem moto-Team.
        </p>
      )}

      <section className="space-y-2">
        <h3 className="text-base font-semibold">Einstellungen</h3>
        <DataGrid>
          <DataField label="Art">
            {consent ? "Zustimmen oder ablehnen" : "Nur zur Kenntnis nehmen"}
          </DataField>
          <DataField label="Wer muss abgeben?">
            {status.signers === "all"
              ? "Alle sorgeberechtigten Personen"
              : "Eine sorgeberechtigte Person genügt"}
          </DataField>
          {consent && (
            <DataField label="Widerruf">
              {status.revocable
                ? "Erlaubt, auch nach der Frist"
                : "Nicht erlaubt"}
            </DataField>
          )}
          <DataField label="Passwort vor der Abgabe">
            {status.requires_password ? "Wird abgefragt" : "Nein"}
          </DataField>
          <DataField label="Frist">
            {status.deadline
              ? `Bis ${formatBerlinDate(status.deadline)}`
              : "Keine Frist"}
          </DataField>
          <DataField label="Verfahren">
            Einfache elektronische Erklärung im angemeldeten Eltern-Konto
          </DataField>
        </DataGrid>
      </section>

      <section className="space-y-2">
        <h3 className="text-base font-semibold">Stand</h3>
        <p>
          {s.children_total} {s.children_total === 1 ? "Kind" : "Kinder"}{" "}
          erreicht.{" "}
          {consent
            ? `${s.agreed} zugestimmt, ${s.declined} abgelehnt, ${s.revoked} widerrufen`
            : `${s.acknowledged} zur Kenntnis genommen`}
          , {s.partial} teilweise, {s.open} offen, {s.expired} mit abgelaufener
          Frist, {s.no_signer} ohne Person, die abgeben kann.
        </p>
      </section>

      <section className="space-y-4">
        <h3 className="text-base font-semibold">Fassungen</h3>
        {status.versions.length === 0 ? (
          <p className="text-gray-600">
            Noch keine Fassung. Sie entsteht beim Veröffentlichen.
          </p>
        ) : (
          status.versions.map((version) => (
            <div
              key={version.id}
              className="break-inside-avoid-page space-y-2 border-t border-gray-200 pt-3"
            >
              <p className="font-semibold">
                Fassung {version.version_no}
                {version.id === status.current_version?.id ? " (aktuell)" : ""},
                veröffentlicht am {dateTime(version.published_at)}
              </p>
              {!version.integrity_ok && (
                <p className="text-moto-red-strong font-semibold">
                  Prüfung fehlgeschlagen: Der gespeicherte Text passt nicht mehr
                  zu seiner Prüfsumme.
                </p>
              )}
              <p className="font-medium">{version.title}</p>
              <p className="whitespace-pre-line">{version.body}</p>
              <p>
                <span className="text-gray-600">Prüfsumme (SHA-256): </span>
                <span className={HASH}>{version.content_hash}</span>
              </p>
              {version.attachments.length > 0 && (
                <div>
                  <p className="text-gray-600">Dateien:</p>
                  <ul className="list-disc space-y-1 pl-5">
                    {version.attachments.map((file) => (
                      <li key={`${file.filename}-${file.sha256}`}>
                        {file.filename} ({formatBytes(file.size_bytes)})
                        <br />
                        <span className="text-gray-600">SHA-256: </span>
                        <span className={HASH}>{file.sha256}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          ))
        )}
      </section>

      <section className="space-y-2">
        <h3 className="text-base font-semibold">Stand je Kind</h3>
        {status.children.length === 0 ? (
          <p className="text-gray-600">Die Erklärung erreicht kein Kind.</p>
        ) : (
          <div className="overflow-x-auto print:overflow-visible">
            <table className="w-full border-collapse text-left">
              <thead>
                <tr>
                  <th className={TH}>Kind</th>
                  <th className={TH}>Klasse</th>
                  <th className={TH}>Stand</th>
                  <th className={TH}>Personen</th>
                </tr>
              </thead>
              <tbody>
                {sortDeclarationChildren(status.children).map((child) => (
                  <tr key={child.student_id} className="break-inside-avoid">
                    <td className={TD}>
                      {name(child.first_name, child.last_name)}
                    </td>
                    <td className={TD}>{child.school_class || "–"}</td>
                    <td className={TD}>
                      {STATE_LABEL[child.state] ?? child.state}
                    </td>
                    <td className={TD}>
                      {child.signers.length === 0
                        ? "–"
                        : child.signers.map((signer) => (
                            <span key={signer.account_id} className="block">
                              {name(signer.first_name, signer.last_name)}:{" "}
                              {signer.action
                                ? `${ACTION_LABEL[signer.action]}${
                                    signer.submitted_at
                                      ? ` am ${dateTime(signer.submitted_at)}`
                                      : ""
                                  }`
                                : "noch nichts abgegeben"}
                            </span>
                          ))}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <section className="space-y-2">
        <h3 className="text-base font-semibold">Verlauf</h3>
        <p className="text-gray-600">
          Jede Abgabe, auch zu früheren Fassungen, neueste zuerst.
        </p>
        {status.submissions.length === 0 ? (
          <p className="text-gray-600">Noch keine Abgaben.</p>
        ) : (
          <div className="overflow-x-auto print:overflow-visible">
            <table className="w-full border-collapse text-left">
              <thead>
                <tr>
                  <th className={TH}>Zeitpunkt</th>
                  <th className={TH}>Kind</th>
                  <th className={TH}>Person</th>
                  <th className={TH}>Aktion</th>
                  <th className={TH}>Fassung</th>
                  <th className={TH}>Verfahren</th>
                  <th className={TH}>Prüfsummen</th>
                </tr>
              </thead>
              <tbody>
                {status.submissions.map((entry) => (
                  <tr key={entry.id} className="break-inside-avoid">
                    <td className={TD}>{dateTime(entry.submitted_at)}</td>
                    <td className={TD}>
                      {name(entry.student_first_name, entry.student_last_name)}
                    </td>
                    <td className={TD}>
                      {entry.signer_name}
                      {ROLE_LABEL[entry.guardian_role] && (
                        <span className="block text-gray-600">
                          {ROLE_LABEL[entry.guardian_role]}
                        </span>
                      )}
                    </td>
                    <td className={TD}>
                      {ACTION_LABEL[entry.action] ?? entry.action}
                    </td>
                    <td className={TD}>{entry.version_no}</td>
                    <td className={TD}>
                      {METHOD_LABEL[entry.method] ?? entry.method}
                      <span className="block text-gray-600">
                        {entry.password_confirmed
                          ? "Passwort bestätigt"
                          : "Ohne Passwort"}
                      </span>
                    </td>
                    <td className={TD}>
                      <span className="block text-gray-600">Fassung:</span>
                      <span className={`block ${HASH}`}>
                        {entry.content_hash}
                      </span>
                      <span className="block text-gray-600">Eintrag:</span>
                      <span className={`block ${HASH}`}>
                        {entry.record_hash}
                      </span>
                      <span
                        className={`block font-semibold ${entry.integrity_ok ? "text-moto-green-strong" : "text-moto-red-strong"}`}
                      >
                        {entry.integrity_ok
                          ? "Prüfung in Ordnung"
                          : "Prüfung fehlgeschlagen"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>

      <footer className="border-t border-gray-200 pt-3 text-gray-600">
        Dieser Bericht belegt einfache elektronische Erklärungen im
        Eltern-Portal von moto. Er ersetzt keine gesetzlich vorgeschriebene
        Unterschrift auf Papier.
      </footer>
    </article>
  );
}
