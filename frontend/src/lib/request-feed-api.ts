import { fetchWithAuth } from "~/lib/fetch-with-auth";

interface Envelope<T> {
  readonly data?: T;
}

export interface RequestFeedStatus {
  readonly active: boolean;
}

export interface RequestFeedLink {
  readonly url: string;
}

function unwrap<T>(value: T | Envelope<T>): T {
  if (value && typeof value === "object" && "data" in value) {
    return (value as Envelope<T>).data as T;
  }
  return value as T;
}

async function request<T>(method: "GET" | "POST", suffix = ""): Promise<T> {
  const response = await fetchWithAuth(
    `/api/students/change-requests/rss-feed${suffix}`,
    { method, cache: "no-store" },
  );
  if (!response.ok) {
    throw new Error(`Request feed failed: ${response.status}`);
  }
  return unwrap((await response.json()) as T | Envelope<T>);
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
