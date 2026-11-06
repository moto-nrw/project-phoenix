import { uncachedAuth } from "~/server/auth";

/**
 * Sends a tenant backend request and repeats it once with the refreshed token
 * when the backend answers 401 and the session now holds a newer token. The
 * route wrappers do the same for handlers that throw; this is for handlers
 * that hold the raw Response (files, uploads).
 */
export async function sendWithTokenRefresh(
  token: string,
  send: (bearer: string) => Promise<Response>,
): Promise<Response> {
  const response = await send(token);
  if (response.status !== 401) return response;
  const refreshed = (await uncachedAuth())?.user?.token;
  return refreshed && refreshed !== token ? send(refreshed) : response;
}
