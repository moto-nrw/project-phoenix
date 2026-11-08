import type { ErrorCode } from "~/lib/error-codes.generated";

/** A login the backend throttled (429). Its answer carries only the class code
 * general.unavailable, whose text asks to try again at once. */
export const LOGIN_RATE_LIMITED = "rate_limited";

/** The `code` of the CredentialsSignin a refused portal login throws. */
export type LoginRefusalCode = ErrorCode | typeof LOGIN_RATE_LIMITED;
