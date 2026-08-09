import { proxyPost } from "@/lib/route-proxy.server";

export const POST = proxyPost("/api/students/pickup-schedules/bulk");
