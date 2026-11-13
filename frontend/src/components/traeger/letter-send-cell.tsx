import { StatusBadge } from "~/components/ui/status-badge";
import {
  type LetterMailState,
  mailBadge,
  mailStatusDetail,
} from "./billing-mail-status";

/** Delivery status of an existing letter, shared by invoices and notices. */
export function LetterSendCell({
  letter,
  printInstruction,
}: Readonly<{
  letter: LetterMailState & { readonly inInbox: boolean };
  printInstruction: string;
}>) {
  const detail = mailStatusDetail(letter);
  return (
    <div className="flex min-w-0 flex-col items-start gap-1">
      <StatusBadge {...mailBadge(letter)} />
      {detail ? <p className="text-sm text-gray-600">{detail}</p> : null}
      {!letter.recipientEmail.trim() ? (
        <p className="text-sm text-gray-600">{printInstruction}</p>
      ) : null}
      {!letter.inInbox ? (
        <p className="text-sm text-gray-600">
          Nicht in der Eltern-App: Das Kind hat noch keinen Schülerdatensatz.
        </p>
      ) : null}
    </div>
  );
}
