// Dateiablage (#2596): folder overview + folder create proxy. Authority
// (visibility, files:manage) is decided in the backend.
import { proxyGet, proxyPost } from "~/lib/route-proxy.server";

export const GET = proxyGet(() => "/api/files/folders");
export const POST = proxyPost(() => "/api/files/folders");
