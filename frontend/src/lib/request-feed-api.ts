import { apiErrorFromResponse } from "~/lib/api-error";
import { fetchWithAuth } from "~/lib/fetch-with-auth";

export interface RequestFeedStatus {
  readonly active: boolean;
}

export interface RequestFeedLink {
  readonly url: string;
}

async function request<T>(method: "GET" | "POST", suffix = ""): Promise<T> {
  const response = await fetchWithAuth(
    `/api/students/change-requests/rss-feed${suffix}`,
    { method, cache: "no-store" },
  );
  if (!response.ok) {
    throw await apiErrorFromResponse(response, "request feed failed");
  }
  return ((await response.json()) as { data: T }).data;
}

export function getRequestFeedStatus(): Promise<RequestFeedStatus> {
  return request("GET");
}

export function createRequestFeed(): Promise<RequestFeedLink> {
  return request("POST");
}

export function rotateRequestFeed(): Promise<RequestFeedLink> {
  return request("POST", "/rotate");
}
