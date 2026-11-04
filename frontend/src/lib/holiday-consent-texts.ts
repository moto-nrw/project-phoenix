// The German texts of the fixed declarations of the holiday registration
// (#5626), shared by the parents' form and the Träger portal. Its own
// module, so the Träger pages do not pull in the parents' API client.

/**
 * The German texts of the fixed declarations of one version (#5626). They
 * bind; a translation only helps reading. A text the version does not have
 * is empty.
 */
export interface HolidayConsentTexts {
  readonly version: number;
  readonly privacyConsent: string;
  readonly healthConsent: string;
  readonly bindingConfirmed: string;
  readonly custodyConfirmed: string;
  readonly declarationName: string;
}

/** The consent texts as the backend sends them. */
export interface BackendConsentTexts {
  version: number;
  privacy_consent: string;
  health_consent: string;
  binding_confirmed: string;
  custody_confirmed: string;
  declaration_name: string;
}

/** Maps the consent texts of the backend; missing texts stay empty. */
export function consentTextsFromBackend(
  texts: BackendConsentTexts | null | undefined,
): HolidayConsentTexts {
  return {
    version: texts?.version ?? 0,
    privacyConsent: texts?.privacy_consent ?? "",
    healthConsent: texts?.health_consent ?? "",
    bindingConfirmed: texts?.binding_confirmed ?? "",
    custodyConfirmed: texts?.custody_confirmed ?? "",
    declarationName: texts?.declaration_name ?? "",
  };
}
