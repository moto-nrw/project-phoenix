import { proxyPut } from "~/lib/route-proxy.server";

// Die ersten Schritte für die Person aus- oder einblenden (#3748).
export const PUT = proxyPut("/api/staff-onboarding/dismissal");
