// Der Schlüssel einer Familie in der Elternabrechnung: die Mail-Adresse der
// Eltern, oder der Vertrag selbst, wenn moto keine Adresse kennt (etwa bei
// einem importierten Vertrag, ADR 0232). Der zweite ist keine Adresse und
// wird nie als eine gezeigt.

const CONTRACT_FAMILY_PREFIX = "contract:";

/** Whether the key names a contract instead of a mail address. */
function isContractFamily(key: string): boolean {
  return key.startsWith(CONTRACT_FAMILY_PREFIX);
}

/** The key as a person reads it: the address, or „Vertrag Nr. 12“. */
export function familyKeyLabel(key: string): string {
  return isContractFamily(key)
    ? `Vertrag Nr. ${key.slice(CONTRACT_FAMILY_PREFIX.length)}`
    : key;
}

/** The mail address of the family, "" when moto knows it only by its contract. */
export function familyKeyEmail(key: string): string {
  return isContractFamily(key) ? "" : key;
}
