import type { FormErrorDetail } from "~/components/ui/form-error";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";

/**
 * Whether a failure is a refusal (401, 403, 404): the session, the right or
 * the object is gone, so nothing loaded before may stay on screen.
 */
export function isRefusal(...failures: unknown[]): boolean {
  return failures.some(
    (failure) =>
      failure instanceof ApiError &&
      [401, 403, 404].includes(failure.status ?? 0),
  );
}

/** A metadata refresh must not unmount a loaded form, except on a refusal. */
export function metadataLoadError(
  error: FormErrorDetail | null,
  hasData: boolean,
  ...failures: unknown[]
) {
  const refused = isRefusal(...failures);
  return error && hasData
    ? { ...error, action: errorAlertActions(error), keepContent: !refused }
    : error;
}
