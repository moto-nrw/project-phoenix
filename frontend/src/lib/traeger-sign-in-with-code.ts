/**
 * A school that switched remembered devices off refuses an Absprung whose
 * Träger session only a remembered device proved (#6955). The person then
 * signs in to the Träger-Portal again; that sign-in ignores the remembered
 * device, so moto asks for the code. The mark lives in this tab and holds
 * until a sign-in actually sent it.
 *
 * The login page reads the mark through useSyncExternalStore: the server
 * snapshot is false, the browser reads sessionStorage, and nothing is
 * removed during render. The submit handler clears it once a sign-in that
 * carried the flag got its answer.
 */
const SIGN_IN_WITH_CODE_KEY = "moto.traeger.sign-in-with-code";
const CHANGE_EVENT = "moto:traeger-sign-in-with-code";

function notify(): void {
  globalThis.dispatchEvent?.(new Event(CHANGE_EVENT));
}

export function requestTraegerSignInWithCode(): void {
  try {
    sessionStorage.setItem(SIGN_IN_WITH_CODE_KEY, "1");
  } catch {
    // Without storage the next sign-in may skip the code again; the school
    // then refuses once more and shows the same hint.
  }
  notify();
}

/** Whether the next Träger sign-in in this tab must ignore the device. */
export function isTraegerSignInWithCodeRequested(): boolean {
  try {
    return sessionStorage.getItem(SIGN_IN_WITH_CODE_KEY) === "1";
  } catch {
    return false;
  }
}

/** Ends the request after a sign-in sent it; the next one may use the device. */
export function clearTraegerSignInWithCode(): void {
  try {
    sessionStorage.removeItem(SIGN_IN_WITH_CODE_KEY);
  } catch {
    // Unreadable storage holds no request to clear.
  }
  notify();
}

/** useSyncExternalStore subscription: this tab's changes and other tabs'. */
export function subscribeTraegerSignInWithCode(
  onChange: () => void,
): () => void {
  globalThis.addEventListener(CHANGE_EVENT, onChange);
  globalThis.addEventListener("storage", onChange);
  return () => {
    globalThis.removeEventListener(CHANGE_EVENT, onChange);
    globalThis.removeEventListener("storage", onChange);
  };
}

/** The server knows no tab: it never renders the hint. */
export function serverTraegerSignInWithCode(): boolean {
  return false;
}
