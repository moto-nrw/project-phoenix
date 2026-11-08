import type {
  TraegerContact,
  TraegerWithdrawalContact,
} from "~/lib/traeger-api";

/** Where a value of the withdrawal notice comes from (#6200). */
export type WithdrawalReachSource = "own" | "contact" | "missing";

export interface WithdrawalReachValue {
  readonly value: string;
  readonly source: WithdrawalReachSource;
}

export interface WithdrawalReach {
  readonly phone: WithdrawalReachValue;
  readonly email: WithdrawalReachValue;
}

function pick(own: string, contact: string): WithdrawalReachValue {
  if (own.trim() !== "") return { value: own.trim(), source: "own" };
  if (contact.trim() !== "")
    return { value: contact.trim(), source: "contact" };
  return { value: "", source: "missing" };
}

/**
 * The phone and e-mail the withdrawal notice names, as the backend picks
 * them (`withdrawalAddressee` in the care contract): the Träger's own for
 * withdrawals, else the contact person's, field by field.
 */
export function withdrawalReach(
  contact: TraegerContact,
  own: TraegerWithdrawalContact,
): WithdrawalReach {
  return {
    phone: pick(own.phone, contact.phone),
    email: pick(own.email, contact.email),
  };
}

/**
 * The addressee in the brackets of the notice, one part per entry: the
 * letterhead's lines (name first, then the address), phone and e-mail.
 */
export function withdrawalAddressee(
  letterheadText: string,
  reach: WithdrawalReach,
): string[] {
  const lines = letterheadText
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
  const parts = lines.length > 0 ? lines : ["dem Träger"];
  if (reach.phone.value !== "") parts.push(`Telefon: ${reach.phone.value}`);
  if (reach.email.value !== "") parts.push(`E-Mail: ${reach.email.value}`);
  return parts;
}
