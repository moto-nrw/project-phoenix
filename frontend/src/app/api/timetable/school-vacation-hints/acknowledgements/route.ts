import { proxyPost } from "~/lib/route-wrapper.server";

/**
 * POST /api/timetable/school-vacation-hints/acknowledgements
 * Die Schule bestätigt, dass sie in diesen Ferien Betreuung plant (#5345).
 */
export const POST = proxyPost(
  "/api/timetable/school-vacation-hints/acknowledgements",
);
