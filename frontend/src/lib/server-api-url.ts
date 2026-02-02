import { env } from "~/env";

/**
 * Returns the API base URL for server-side requests.
 * Uses API_URL (internal Docker network) when available,
 * falls back to NEXT_PUBLIC_API_URL for local development.
 */
export function getServerApiUrl(): string {
  return env.API_URL ?? env.NEXT_PUBLIC_API_URL;
}
