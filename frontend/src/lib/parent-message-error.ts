import { ParentApiError } from "./parent-api";

type MessageErrorKey =
  "sendError" | "sessionExpired" | "permissionDenied" | "invalidRequest";

export function parentMessageError(
  error: unknown,
  translate: (key: MessageErrorKey) => string,
): string {
  if (error instanceof ParentApiError) {
    if (error.status === 401) return translate("sessionExpired");
    if (error.status === 403) return translate("permissionDenied");
    if (error.status === 400) return translate("invalidRequest");
  }
  return translate("sendError");
}
