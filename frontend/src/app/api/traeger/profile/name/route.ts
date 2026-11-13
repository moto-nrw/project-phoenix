import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Den eigenen Namen im Träger-Büro ändern (#4693).
export const PUT = proxyPut("/traeger/profile/name");
