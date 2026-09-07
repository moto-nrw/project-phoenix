import { proxyPost } from "~/lib/route-proxy.server";

export const POST = proxyPost(() => "/api/staff/time-tracking/export/sftp");
