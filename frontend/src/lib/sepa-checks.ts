// Shape checks of SEPA codes the forms run before they send an IBAN or a
// creditor identifier (#4695, #4696). The backend checks the same again;
// these only point at the field early. Shared by the Träger portal and the
// parents portal.

/** Removes white space and upper-cases what people type. */
export function compactCode(value: string): string {
  return value.replace(/\s+/g, "").toUpperCase();
}

/** ISO 7064 MOD 97-10 remainder; letters count as two digits (A = 10). */
function mod97(value: string): number {
  let remainder = 0;
  for (const char of value) {
    const digit = /\d/.test(char)
      ? Number(char)
      : char.charCodeAt(0) - "A".charCodeAt(0) + 10;
    remainder = Number(`${remainder}${digit}`) % 97;
  }
  return remainder;
}

/** Shape and check digits of an IBAN; the backend also checks the length. */
export function ibanLooksValid(raw: string): boolean {
  const iban = compactCode(raw);
  if (!/^[A-Z]{2}\d{2}[A-Z0-9]{11,30}$/.test(iban)) return false;
  if (iban.startsWith("DE") && !/^DE\d{20}$/.test(iban)) return false;
  return mod97(iban.slice(4) + iban.slice(0, 4)) === 1;
}

/** Shape and check digits of a SEPA creditor identifier. */
export function creditorIdLooksValid(raw: string): boolean {
  const id = compactCode(raw);
  if (!/^[A-Z]{2}\d{2}[A-Z0-9]{3}[A-Z0-9]{1,28}$/.test(id)) return false;
  if (id.startsWith("DE") && !/^DE\d{2}[A-Z0-9]{3}\d{11}$/.test(id)) {
    return false;
  }
  return mod97(id.slice(7) + id.slice(0, 4)) === 1;
}
