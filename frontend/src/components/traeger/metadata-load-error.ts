import type { FormErrorDetail } from "~/components/ui/form-error";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";

/** A metadata refresh must not unmount a loaded form, except on a refusal. */
export function metadataLoadError(
  error: FormErrorDetail | null,
  hasData: boolean,
  failure: unknown,
) {
  const refused =
    failure instanceof ApiError &&
    [401, 403, 404].includes(failure.status ?? 0);
  return error && hasData && !refused
    ? { ...error, action: errorAlertActions(error), keepContent: true }
    : error;
}
