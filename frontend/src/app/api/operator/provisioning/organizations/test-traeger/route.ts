import { proxyPost } from "~/lib/operator/route-wrapper.server";

// Testsystem für einen neuen Träger (#4658): Test-Träger mit Test-Schulen.
export const POST = proxyPost("/operator/organizations/test-traeger");
