import "server-only";
import { AsyncLocalStorage } from "node:async_hooks";

const mutation = new AsyncLocalStorage<boolean>();

/** Only a trusted server action may introduce a newly minted preview token. */
export const isServerSessionMutation = () => mutation.getStore() === true;
export function withServerSessionMutation<T>(action: () => Promise<T>) {
  return mutation.run(true, action);
}
