import { proxyPost } from "~/lib/route-proxy.server";

// Schließt die Einrichtung für die ganze Schule ab (#2832).
export const POST = proxyPost("/api/school-setup/complete");
