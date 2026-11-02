import { proxyPost } from "~/lib/school/route-wrapper.server";

// Einen Gruppenchat verlassen (#4776). Lehrkräfte legen keine Gruppen an
// und verwalten keine; Verlassen ist ihre einzige Gruppen-Aktion.
export const POST = proxyPost(
  (params) =>
    `/school/staff-messages/threads/${params.threadID as string}/leave`,
);
