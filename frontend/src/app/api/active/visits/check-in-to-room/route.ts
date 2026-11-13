import { proxyPost } from "~/lib/route-wrapper.server";

// Check children in and place them in a released room (#6953). The backend
// decides the release, the rights and the room session.
export const POST = proxyPost("/api/active/visits/check-in-to-room");
