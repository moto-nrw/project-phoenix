/**
 * Opens „Passwort vergessen?“ on the Träger login page: the way a reset link
 * without token points to a new link.
 */
export const TRAEGER_FORGOT_PASSWORD_PARAM = "passwort-vergessen";

/**
 * Asks the Träger login page for a sign-in with code (#6955): a school whose
 * remembered devices are off refused a switch of a session that came from
 * the Träger-Portal. The page ends a running Träger session and ignores the
 * remembered device once.
 */
export const TRAEGER_SIGN_IN_WITH_CODE_PARAM = "mit-code";
