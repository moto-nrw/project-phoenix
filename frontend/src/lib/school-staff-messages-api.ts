// Team-Chat im Schul-Portal (#2208): dieselben Unterhaltungen wie im
// OGS-Portal, erreicht über die school-Session und die Proxy-Routen unter
// /api/school/staff-messages (Backend: /school/staff-messages).

import { createStaffMessagesApi } from "./staff-messages-api";

export const schoolStaffMessagesApi = createStaffMessagesApi(
  "/api/school/staff-messages",
);
