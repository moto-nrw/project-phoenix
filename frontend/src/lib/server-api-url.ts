import { env } from "~/env";

/**
 * Returns the API base URL for server-side requests.
 * Uses API_URL on the internal Docker network.
 */
export function getServerApiUrl(): string {
  return env.API_URL;
}
