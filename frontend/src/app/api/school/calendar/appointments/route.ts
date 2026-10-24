import { proxyPost } from "~/lib/school/route-wrapper.server";

// Schultermin einer Lehrkraft anlegen (#4798).
export const POST = proxyPost("/school/calendar/appointments");
