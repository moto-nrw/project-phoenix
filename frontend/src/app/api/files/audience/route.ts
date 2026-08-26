// Dateiablage (#2596): roles and persons a folder can be shared with.
import { proxyGet } from "~/lib/route-proxy.server";

export const GET = proxyGet(() => "/api/files/audience");
