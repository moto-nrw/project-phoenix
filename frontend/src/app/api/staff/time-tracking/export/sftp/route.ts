import { proxyPost } from "~/lib/route-wrapper.server";

export const POST = proxyPost(() => "/api/staff/time-tracking/export/sftp");
