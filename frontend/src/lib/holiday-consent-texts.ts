// The German texts of the fixed declarations of the holiday registration
// (#5626), shared by the parents' form and the Träger portal. Its own
// module, so the Träger pages do not pull in the parents' API client.

/** Days the consent texts before version 4 promise: three months (#6691). */
export const HOLIDAY_LEGACY_RETENTION_DAYS = 92;

/**
 * Days after the holidays the health information of every registration
 * goes, whatever the registration itself keeps (#6691).
 */
export const HOLIDAY_HEALTH_RETENTION_DAYS = 92;

/**
 * The German texts of the fixed declarations of one version (#5626). They
 * bind; a translation only helps reading. A text the version does not have
 * is empty.
 */
export interface HolidayConsentTexts {
  readonly version: number;
  /**
   * Days after the holidays the privacy text promises to delete the
   * registration (#6691); 92 for the versions with the fixed three months.
   */
  readonly retentionDays: number;
  readonly privacyConsent: string;
  readonly healthConsent: string;
  readonly bindingConfirmed: string;
  readonly custodyConfirmed: string;
  readonly declarationName: string;
}

/** The consent texts as the backend sends them. */
export interface BackendConsentTexts {
  version: number;
  retention_days?: number;
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
    retentionDays: texts?.retention_days ?? HOLIDAY_LEGACY_RETENTION_DAYS,
    privacyConsent: texts?.privacy_consent ?? "",
    healthConsent: texts?.health_consent ?? "",
    bindingConfirmed: texts?.binding_confirmed ?? "",
    custodyConfirmed: texts?.custody_confirmed ?? "",
    declarationName: texts?.declaration_name ?? "",
  };
}
