import { proxyGet, proxyPost } from "~/lib/school/route-wrapper.server";

// Verlauf lesen (markiert als gelesen) und Nachricht senden (#2208).
export const GET = proxyGet(
  (params) => `/school/staff-messages/threads/${params.threadID as string}`,
);
export const POST = proxyPost(
  (params) => `/school/staff-messages/threads/${params.threadID as string}`,
);
