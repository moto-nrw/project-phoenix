import type { FormErrorDetail } from "~/components/ui/form-error";
import { errorAlertActions } from "~/components/ui/form-error-alert";
import { isRefusal } from "./use-traeger-confirmed-resource";

/** A metadata refresh must not unmount a loaded form, except on a refusal. */
export function metadataLoadError(
  error: FormErrorDetail | null,
  hasData: boolean,
  ...failures: unknown[]
) {
  const refused = failures.some(isRefusal);
  return error && hasData
    ? { ...error, action: errorAlertActions(error), keepContent: !refused }
    : error;
}
