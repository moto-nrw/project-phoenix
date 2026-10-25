import Image from "next/image";
import type { TraegerBankAccount, TraegerContact } from "~/lib/traeger-api";
import { formatIBAN } from "./traeger-bank-account-format";

const COLOR_PATTERN = /^#[0-9a-fA-F]{6}$/;

interface TraegerLetterheadPreviewProps {
  readonly logoSrc: string | null;
  readonly letterheadText: string;
  readonly footerText: string;
  readonly color: string;
  readonly contact: TraegerContact;
  readonly defaultAccount: TraegerBankAccount | null;
}

function lines(text: string): string[] {
  return text
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

/**
 * The head and foot of a letter of the Träger as documents will print them
 * (#4653): logo, sender lines and contact person at the top, a line in the
 * Träger's colour, the footer with the default bank account at the bottom.
 * A display only: the values are changed in the cards below.
 */
export function TraegerLetterheadPreview({
  logoSrc,
  letterheadText,
  footerText,
  color,
  contact,
  defaultAccount,
}: TraegerLetterheadPreviewProps) {
  // The colour is the Träger's own value, not a brand colour of moto;
  // without a valid one the preview stays gray.
  const accent = COLOR_PATTERN.test(color) ? color : undefined;
  const sender = lines(letterheadText);
  const footer = lines(footerText);
  const contactLine = [contact.name, contact.phone, contact.email]
    .filter((value) => value !== "")
    .join(" · ");

  return (
    <figure
      aria-label="Vorschau des Briefkopfs"
      className="mx-auto w-full max-w-2xl rounded-md border border-gray-200 bg-white p-6 text-sm text-gray-800 shadow-sm sm:p-10"
    >
      <div className="flex flex-wrap items-start justify-between gap-6">
        <div className="flex min-h-16 items-center">
          {logoSrc ? (
            <Image
              src={logoSrc}
              alt="Logo des Trägers"
              width={240}
              height={80}
              className="max-h-20 w-auto object-contain"
              unoptimized
            />
          ) : (
            <span className="text-gray-400">Kein Logo</span>
          )}
        </div>
        <address className="text-right not-italic">
          {sender.length > 0 ? (
            sender.map((line, index) => (
              <p
                key={`${index}-${line}`}
                className={index === 0 ? "font-semibold" : undefined}
              >
                {line}
              </p>
            ))
          ) : (
            <p className="text-gray-400">Noch kein Absender</p>
          )}
        </address>
      </div>
      <div
        className="mt-6 h-1 rounded-full bg-gray-900"
        style={accent ? { backgroundColor: accent } : undefined}
        aria-hidden="true"
      />
      <p className="mt-4 text-gray-600">
        <span
          className="font-medium text-gray-900"
          style={accent ? { color: accent } : undefined}
        >
          Ansprechpartner:
        </span>{" "}
        {contactLine || (
          <span className="text-gray-400">noch nicht eingetragen</span>
        )}
      </p>
      <div className="mt-8 space-y-2" aria-hidden="true">
        <div className="h-2 w-11/12 rounded bg-gray-100" />
        <div className="h-2 w-10/12 rounded bg-gray-100" />
        <div className="h-2 w-8/12 rounded bg-gray-100" />
      </div>
      <figcaption className="mt-10 border-t border-gray-200 pt-3 text-sm text-gray-600">
        {footer.map((line, index) => (
          <p key={`${index}-${line}`}>{line}</p>
        ))}
        {defaultAccount ? (
          <p className={footer.length > 0 ? "mt-1" : undefined}>
            Bankverbindung: {defaultAccount.accountHolder} · IBAN{" "}
            {formatIBAN(defaultAccount.iban)}
            {defaultAccount.bic ? ` · BIC ${defaultAccount.bic}` : ""}
            {defaultAccount.bankName ? ` · ${defaultAccount.bankName}` : ""}
          </p>
        ) : null}
        {footer.length === 0 && !defaultAccount ? (
          <p className="text-gray-400">Noch keine Fußzeile</p>
        ) : null}
      </figcaption>
    </figure>
  );
}
