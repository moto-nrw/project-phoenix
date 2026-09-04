/**
 * Returns the API base URL for server-side requests.
 * Uses API_URL on the internal Docker network.
 */
export function getServerApiUrl(): string {
  const apiUrl = process.env.API_URL;
  if (!apiUrl) throw new Error("API_URL is not set");
  return apiUrl;
}
