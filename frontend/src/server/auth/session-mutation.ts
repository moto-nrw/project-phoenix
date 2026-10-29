import "server-only";
import { AsyncLocalStorage } from "node:async_hooks";

const mutation = new AsyncLocalStorage<boolean>();

/** Only an in-process BFF mutation may introduce newly minted session tokens. */
export const isServerSessionMutation = () => mutation.getStore() === true;
export function withServerSessionMutation<T>(action: () => Promise<T>) {
  return mutation.run(true, action);
}
