/** An IBAN in groups of four, as people read it from a letter. */
export function formatIBAN(iban: string): string {
  return iban.replace(/\s+/g, "").replace(/(.{4})(?=.)/g, "$1 ");
}
