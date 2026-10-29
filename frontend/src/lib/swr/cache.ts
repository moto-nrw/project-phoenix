/**
 * Provider access only. Cache reads and writes go through the
 * generation-locked hooks; `useSWRConfig` stays inside ~/lib/swr (#5007).
 */
export { SWRConfig } from "swr";
