/**
 * A school that switched remembered devices off refuses an Absprung whose
 * Träger session only a remembered device proved (#6955). The person then
 * signs in to the Träger-Portal again; that one sign-in ignores the
 * remembered device, so moto asks for the code. The mark lives in this tab
 * only and is used once.
 */
const SIGN_IN_WITH_CODE_KEY = "moto.traeger.sign-in-with-code";

export function requestTraegerSignInWithCode(): void {
  try {
    sessionStorage.setItem(SIGN_IN_WITH_CODE_KEY, "1");
  } catch {
    // Without storage the next sign-in may skip the code again; the school
    // then refuses once more and shows the same hint.
  }
}

/** Reads the mark and clears it, so only the next sign-in ignores the device. */
export function takeTraegerSignInWithCode(): boolean {
  try {
    const requested = sessionStorage.getItem(SIGN_IN_WITH_CODE_KEY) === "1";
    sessionStorage.removeItem(SIGN_IN_WITH_CODE_KEY);
    return requested;
  } catch {
    return false;
  }
}
