import { proxyFile } from "~/lib/traeger/route-wrapper.server";

// CSV-Export des Zugriffsprotokolls (#4655).
export const GET = proxyFile("/traeger/access-log/export");
