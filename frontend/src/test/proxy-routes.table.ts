/**
 * Expected backend target of every pure proxy route, checked by
 * `proxy-routes.test.ts`. Key: route directory below `src/app`. Value: one
 * entry per exported method: `METHOD portal backend-path [raw]`. Dynamic
 * segments `[name]` are filled with `name/1`, so `name%2F1` proves the
 * segment is encoded.
 */
export const PROXY_ROUTES: Record<string, readonly string[]> = {
  "api/active/groups": [
    "GET tenant /api/active/groups raw",
    "POST tenant /api/active/groups raw",
  ],
  "api/active/groups/[id]/supervisors": [
    "GET tenant /api/active/groups/id%2F1/supervisors raw",
  ],
  "api/active/groups/[id]/visits": [
    "GET tenant /api/active/groups/id%2F1/visits raw",
  ],
  "api/active/groups/[id]/visits/display": [
    "GET tenant /api/active/groups/id%2F1/visits/display",
  ],
  "api/active/groups/group/[groupId]": [
    "GET tenant /api/active/groups/group/groupId%2F1 raw",
  ],
  "api/active/groups/room/[roomId]": [
    "GET tenant /api/active/groups/room/roomId%2F1 raw",
  ],
  "api/active/groups/unclaimed": [
    "GET tenant /api/active/groups/unclaimed raw",
  ],
  "api/active/supervisors/all": ["GET tenant /api/active/supervisors/all"],
  "api/active/supervisors/group/[groupId]": [
    "GET tenant /api/active/supervisors/group/groupId%2F1 raw",
  ],
  "api/active/supervisors/staff/[staffId]": [
    "GET tenant /api/active/supervisors/staff/staffId%2F1 raw",
  ],
  "api/active/supervisors/staff/[staffId]/active": [
    "GET tenant /api/active/supervisors/staff/staffId%2F1/active",
  ],
  "api/active/tracking-indicators": [
    "POST tenant /api/active/tracking-indicators",
  ],
  "api/active/visits/group/[groupId]": [
    "GET tenant /api/active/visits/group/groupId%2F1 raw",
  ],
  "api/active/visits/move-to-group": [
    "POST tenant /api/active/visits/move-to-group",
  ],
  "api/active/visits/student/[studentId]": [
    "GET tenant /api/active/visits/student/studentId%2F1 raw",
  ],
  "api/active/visits/student/[studentId]/current": [
    "GET tenant /api/active/visits/student/studentId%2F1/current raw",
  ],
  "api/active/visits/transit/assign": [
    "POST tenant /api/active/visits/transit/assign",
  ],
  "api/analytics-consent": [
    "GET tenant /analytics-consent/",
    "PUT tenant /analytics-consent/",
  ],
  "api/announcement-attachments/[announcementId]/[attachmentId]": [
    "DELETE tenant /api/announcement-attachments/announcementId%2F1/attachmentId%2F1",
  ],
  "api/caterer/schools": ["GET caterer /caterer/schools"],
  "api/caterer/schools/[schoolId]/orders": [
    "GET caterer /caterer/schools/schoolId%2F1/orders",
  ],
  "api/caterer/schools/[schoolId]/orders/[orderId]/confirm": [
    "POST caterer /caterer/schools/schoolId%2F1/orders/orderId%2F1/confirm",
  ],
  "api/class-list-entries": [
    "GET tenant /api/class-list-entries/",
    "POST tenant /api/class-list-entries/",
  ],
  "api/class-list-entries/[id]": [
    "DELETE tenant /api/class-list-entries/id%2F1",
    "PUT tenant /api/class-list-entries/id%2F1",
  ],
  "api/class-list-entries/[id]/assign": [
    "POST tenant /api/class-list-entries/id%2F1/assign",
  ],
  "api/database/counts": ["GET tenant /api/database/stats raw"],
  "api/enrollment/phases/expiry-warnings": [
    "GET tenant /api/enrollment/phases/expiry-warnings",
  ],
  "api/files/audience": ["GET tenant /api/files/audience"],
  "api/files/folders": [
    "GET tenant /api/files/folders",
    "POST tenant /api/files/folders",
  ],
  "api/files/folders/[folderId]": [
    "DELETE tenant /api/files/folders/folderId%2F1",
    "PUT tenant /api/files/folders/folderId%2F1",
  ],
  "api/files/folders/[folderId]/files/[fileId]": [
    "DELETE tenant /api/files/folders/folderId%2F1/files/fileId%2F1",
  ],
  "api/guardians": [
    "GET tenant /api/guardians raw",
    "POST tenant /api/guardians",
  ],
  "api/guardians/[id]/delete-preview": [
    "GET tenant /api/guardians/id%2F1/delete-preview",
  ],
  "api/guardians/[id]/payment": [
    "GET tenant /api/guardians/id%2F1/payment",
    "PUT tenant /api/guardians/id%2F1/payment",
  ],
  "api/guardians/[id]/payment/reveal": [
    "POST tenant /api/guardians/id%2F1/payment/reveal",
  ],
  "api/guardians/[id]/phone-numbers": [
    "GET tenant /api/guardians/id%2F1/phone-numbers",
    "POST tenant /api/guardians/id%2F1/phone-numbers",
  ],
  "api/guardians/[id]/phone-numbers/[phoneId]": [
    "DELETE tenant /api/guardians/id%2F1/phone-numbers/phoneId%2F1",
    "PUT tenant /api/guardians/id%2F1/phone-numbers/phoneId%2F1",
  ],
  "api/guardians/[id]/phone-numbers/[phoneId]/set-primary": [
    "POST tenant /api/guardians/id%2F1/phone-numbers/phoneId%2F1/set-primary",
  ],
  "api/guardians/bulk-invite": ["POST tenant /api/guardians/bulk-invite"],
  "api/guardians/invitations/pending-approval": [
    "GET tenant /api/guardians/invitations/pending-approval",
  ],
  "api/guardians/payment-overview": [
    "GET tenant /api/guardians/payment-overview",
  ],
  "api/guardians/relationships/[relationshipId]": [
    "PUT tenant /api/guardians/relationships/relationshipId%2F1",
  ],
  "api/guardians/search": ["GET tenant /api/guardians/search"],
  "api/guardians/students/[studentId]/guardians": [
    "GET tenant /api/guardians/students/studentId%2F1/guardians",
    "POST tenant /api/guardians/students/studentId%2F1/guardians",
  ],
  "api/guardians/students/[studentId]/guardians/[guardianId]": [
    "DELETE tenant /api/guardians/students/studentId%2F1/guardians/guardianId%2F1",
  ],
  "api/guardians/students/[studentId]/guardians/batch": [
    "POST tenant /api/guardians/students/studentId%2F1/guardians/batch",
  ],
  "api/guardians/students/[studentId]/invite": [
    "POST tenant /api/guardians/students/studentId%2F1/invite",
  ],
  "api/guardians/students/[studentId]/payer": [
    "PUT tenant /api/guardians/students/studentId%2F1/payer",
  ],
  "api/me": ["GET tenant /api/me"],
  "api/me/groups": ["GET tenant /api/me/groups"],
  "api/me/groups/active": ["GET tenant /api/me/groups/active"],
  "api/me/groups/activity": ["GET tenant /api/me/groups/activity"],
  "api/me/groups/supervised": ["GET tenant /api/me/groups/supervised"],
  "api/me/profile": [
    "GET tenant /api/me/profile",
    "PUT tenant /api/me/profile",
  ],
  "api/me/staff": ["GET tenant /api/me/staff"],
  "api/me/teacher": ["GET tenant /api/me/teacher"],
  "api/meal-caterer": ["GET tenant /api/meal-caterer"],
  "api/meal-caterer/invitations": ["POST tenant /api/meal-caterer/invitations"],
  "api/meal-caterer/invitations/[invitationId]": [
    "DELETE tenant /api/meal-caterer/invitations/invitationId%2F1",
  ],
  "api/meal-caterer/supply": ["DELETE tenant /api/meal-caterer/supply"],
  "api/meal-plan/orders": ["GET tenant /api/meal-plan/orders"],
  "api/meal-plan/participants": ["GET tenant /api/meal-plan/participants"],
  "api/meal-plan/participation/[studentId]": [
    "GET tenant /api/meal-plan/participation/studentId%2F1",
  ],
  "api/meal-plan/participation/[studentId]/days/[date]": [
    "DELETE tenant /api/meal-plan/participation/studentId%2F1/days/date%2F1",
    "PUT tenant /api/meal-plan/participation/studentId%2F1/days/date%2F1",
  ],
  "api/meal-plan/participation/[studentId]/schedule": [
    "PUT tenant /api/meal-plan/participation/studentId%2F1/schedule",
  ],
  "api/meal-plan/participants/week": [
    "GET tenant /api/meal-plan/participants/week",
  ],
  "api/messages/group-threads": [
    "GET tenant /api/messages/group-threads",
    "POST tenant /api/messages/group-threads",
  ],
  "api/messages/group-threads/[groupThreadId]": [
    "GET tenant /api/messages/group-threads/groupThreadId%2F1",
  ],
  "api/messages/group-threads/[groupThreadId]/close": [
    "POST tenant /api/messages/group-threads/groupThreadId%2F1/close",
  ],
  "api/messages/group-threads/[groupThreadId]/messages": [
    "POST tenant /api/messages/group-threads/groupThreadId%2F1/messages",
  ],
  "api/messages/group-threads/audience": [
    "GET tenant /api/messages/group-threads/audience",
  ],
  "api/messages/reachable-students": [
    "GET tenant /api/messages/reachable-students",
  ],
  "api/messages/students/[studentId]/guardians": [
    "GET tenant /api/messages/students/studentId%2F1/guardians",
  ],
  "api/notifications/push/public-key": [
    "GET tenant /api/notifications/push/public-key",
  ],
  "api/operator/announcements/[id]/publish": [
    "POST operator /operator/announcements/id%2F1/publish",
  ],
  "api/operator/announcements/[id]/stats": [
    "GET operator /operator/announcements/id%2F1/stats",
  ],
  "api/operator/announcements/[id]/views": [
    "GET operator /operator/announcements/id%2F1/views",
  ],
  "api/operator/billing/key-date-counts": [
    "GET operator /operator/billing/key-date-counts",
  ],
  "api/operator/billing/key-day": [
    "GET operator /operator/billing/key-day",
    "PUT operator /operator/billing/key-day",
  ],
  "api/operator/profile": [
    "GET operator /operator/profile",
    "PUT operator /operator/profile",
  ],
  "api/operator/provisioning/accounts": ["GET operator /operator/accounts"],
  "api/operator/provisioning/devices": [
    "GET operator /operator/devices",
    "POST operator /operator/devices",
  ],
  "api/operator/provisioning/devices/[id]/set-api-key": [
    "POST operator /operator/devices/id%2F1/set-api-key",
  ],
  "api/operator/provisioning/devices/[id]/transfer": [
    "POST operator /operator/devices/id%2F1/transfer",
  ],
  "api/operator/provisioning/devices/[id]/transfer-status": [
    "GET operator /operator/devices/id%2F1/transfer-status",
  ],
  "api/operator/provisioning/organizations": [
    "GET operator /operator/organizations",
    "POST operator /operator/organizations",
  ],
  "api/operator/provisioning/organizations/[id]/accounts": [
    "GET operator /operator/organizations/id%2F1/accounts",
  ],
  "api/operator/provisioning/organizations/[id]/devices": [
    "GET operator /operator/organizations/id%2F1/devices",
  ],
  "api/operator/provisioning/organizations/[id]/erasure-order": [
    "GET operator /operator/organizations/id%2F1/erasure-order",
    "POST operator /operator/organizations/id%2F1/erasure-order",
    "DELETE operator /operator/organizations/id%2F1/erasure-order",
  ],
  "api/operator/provisioning/organizations/[id]/traeger-members": [
    "GET operator /operator/organizations/id%2F1/traeger-members",
    "POST operator /operator/organizations/id%2F1/traeger-members",
  ],
  "api/operator/provisioning/organizations/[id]/traeger-members/invitations": [
    "POST operator /operator/organizations/id%2F1/traeger-members/invitations",
  ],
  "api/operator/provisioning/organizations/[id]/persons": [
    "GET operator /operator/organizations/id%2F1/persons",
  ],
  "api/operator/provisioning/organizations/[id]/restore": [
    "POST operator /operator/organizations/id%2F1/restore",
  ],
  "api/operator/provisioning/organizations/[id]/sample-children": [
    "POST operator /operator/organizations/id%2F1/sample-children",
  ],
  "api/operator/provisioning/organizations/[id]/schools": [
    "GET operator /operator/organizations/id%2F1/schools",
  ],
  "api/operator/provisioning/organizations/summaries": [
    "GET operator /operator/organizations/summaries",
  ],
  "api/operator/provisioning/organizations/test-traeger": [
    "POST operator /operator/organizations/test-traeger",
  ],
  "api/operator/provisioning/roles": ["GET operator /operator/roles"],
  "api/operator/provisioning/schools": [
    "GET operator /operator/schools",
    "POST operator /operator/schools",
  ],
  "api/operator/provisioning/schools/[id]/accounts": [
    "GET operator /operator/schools/id%2F1/accounts",
  ],
  "api/operator/provisioning/schools/[id]/create-account": [
    "POST operator /operator/schools/id%2F1/create-account",
  ],
  "api/operator/provisioning/schools/[id]/devices": [
    "GET operator /operator/schools/id%2F1/devices",
  ],
  "api/operator/provisioning/schools/[id]/invite-admin": [
    "POST operator /operator/schools/id%2F1/invite-admin",
  ],
  "api/operator/provisioning/schools/[id]/restore": [
    "POST operator /operator/schools/id%2F1/restore",
  ],
  "api/operator/provisioning/schools/[id]/settings/booking-authority-impact": [
    "GET operator /operator/schools/id%2F1/settings/booking-authority-impact",
  ],
  "api/operator/provisioning/schools/[id]/settings/schema": [
    "GET operator /operator/schools/id%2F1/settings/schema",
  ],
  "api/operator/provisioning/schools/summaries": [
    "GET operator /operator/schools/summaries",
  ],
  "api/operator/provisioning/stats": ["GET operator /operator/stats"],
  "api/operator/provisioning/unregistered-tag-scans": [
    "GET operator /operator/unregistered-tag-scans",
  ],
  "api/parent/analytics-consent": [
    "GET parent /analytics-consent/",
    "PUT parent /analytics-consent/",
  ],
  "api/parent/analytics-policy": ["GET parent /analytics-policy"],
  "api/parent/calendar": ["GET parent /parent/me/calendar"],
  "api/parent/calendar/appointments/[appointmentId]/overview": [
    "GET parent /parent/me/calendar/appointments/appointmentId%2F1/overview",
  ],
  "api/parent/calendar/feed": ["GET parent /parent/me/calendar/feed"],
  "api/parent/calendar/feed/rotate": [
    "POST parent /parent/me/calendar/feed/rotate",
  ],
  "api/parent/calendar/recipients/[recipientId]/response": [
    "POST parent /parent/me/calendar/recipients/recipientId%2F1/response",
  ],
  "api/parent/enrollments/[tenantSlug]/bootstrap/[phaseId]": [
    "GET parent /parent/enrollments/tenantSlug%2F1/bootstrap/phaseId%2F1",
  ],
  "api/parent/enrollments/[tenantSlug]/profile": [
    "GET parent /parent/enrollments/tenantSlug%2F1/profile",
  ],
  "api/parent/enrollments/[tenantSlug]/submit": [
    "POST parent /parent/enrollments/tenantSlug%2F1/submit",
  ],
  "api/parent/holiday-registrations/[token]/family": [
    "GET parent /parent/holiday-registrations/token%2F1/family",
  ],
  "api/parent/me/children": ["GET parent /parent/me/children"],
  "api/parent/me/children/[studentId]/care-offerings": [
    "GET parent /parent/me/children/studentId%2F1/care-offerings",
  ],
  "api/parent/me/children/[studentId]/care-offerings/catalog": [
    "GET parent /parent/me/children/studentId%2F1/care-offerings/catalog",
  ],
  "api/parent/me/children/[studentId]/care-offerings/requests": [
    "POST parent /parent/me/children/studentId%2F1/care-offerings/requests",
  ],
  "api/parent/me/children/[studentId]/care-offerings/requests/[requestId]": [
    "PUT parent /parent/me/children/studentId%2F1/care-offerings/requests/requestId%2F1",
  ],
  "api/parent/me/children/[studentId]/care-schedule": [
    "GET parent /parent/me/children/studentId%2F1/care-schedule",
  ],
  "api/parent/me/children/[studentId]/care-schedule/requests": [
    "POST parent /parent/me/children/studentId%2F1/care-schedule/requests",
  ],
  "api/parent/me/children/[studentId]/care-schedule/requests/[requestId]": [
    "PUT parent /parent/me/children/studentId%2F1/care-schedule/requests/requestId%2F1",
  ],
  "api/parent/me/children/[studentId]/consents": [
    "GET parent /parent/me/children/studentId%2F1/consents",
  ],
  "api/parent/me/children/[studentId]/courses": [
    "GET parent /parent/me/children/studentId%2F1/courses",
  ],
  "api/parent/me/children/[studentId]/courses/requests": [
    "POST parent /parent/me/children/studentId%2F1/courses/requests",
  ],
  "api/parent/me/children/[studentId]/courses/requests/[requestId]/withdraw": [
    "POST parent /parent/me/children/studentId%2F1/courses/requests/requestId%2F1/withdraw",
  ],
  "api/parent/me/children/[studentId]/excused-requests": [
    "GET parent /parent/me/children/studentId%2F1/excused-requests",
  ],
  "api/parent/me/children/[studentId]/excused-requests/[requestId]": [
    "PUT parent /parent/me/children/studentId%2F1/excused-requests/requestId%2F1",
  ],
  "api/parent/me/children/[studentId]/features": [
    "GET parent /parent/me/children/studentId%2F1/features",
  ],
  "api/parent/me/children/[studentId]/guardians": [
    "GET parent /parent/me/children/studentId%2F1/guardians",
    "POST parent /parent/me/children/studentId%2F1/guardians",
  ],
  "api/parent/me/children/[studentId]/guardians/[guardianProfileId]/contact": [
    "PUT parent /parent/me/children/studentId%2F1/guardians/guardianProfileId%2F1/contact",
  ],
  "api/parent/me/children/[studentId]/guardians/[guardianProfileId]/pickup": [
    "PUT parent /parent/me/children/studentId%2F1/guardians/guardianProfileId%2F1/pickup",
  ],
  "api/parent/me/children/[studentId]/guardians/[guardianProfileId]/pickup-permission-requests":
    [
      "POST parent /parent/me/children/studentId%2F1/guardians/guardianProfileId%2F1/pickup-permission-requests",
    ],
  "api/parent/me/children/[studentId]/master-data": [
    "GET parent /parent/me/children/studentId%2F1/master-data",
  ],
  "api/parent/me/children/[studentId]/master-data/requests": [
    "GET parent /parent/me/children/studentId%2F1/master-data/requests",
    "POST parent /parent/me/children/studentId%2F1/master-data/requests",
  ],
  "api/parent/me/children/[studentId]/master-data/requests/[requestId]": [
    "PUT parent /parent/me/children/studentId%2F1/master-data/requests/requestId%2F1",
  ],
  "api/parent/me/children/[studentId]/meal-plan": [
    "GET parent /parent/me/children/studentId%2F1/meal-plan",
  ],
  "api/parent/me/children/[studentId]/pickup-change-requests": [
    "GET parent /parent/me/children/studentId%2F1/pickup-change-requests",
  ],
  "api/parent/me/children/[studentId]/pickup-change-requests/[requestId]": [
    "PUT parent /parent/me/children/studentId%2F1/pickup-change-requests/requestId%2F1",
  ],
  "api/parent/me/children/[studentId]/pickup-permission-requests": [
    "GET parent /parent/me/children/studentId%2F1/pickup-permission-requests",
  ],
  "api/parent/me/children/[studentId]/pickup-signature-statement": [
    "POST parent /parent/me/children/studentId%2F1/pickup-signature-statement",
  ],
  "api/parent/me/children/[studentId]/related-accounts": [
    "GET parent /parent/me/children/studentId%2F1/related-accounts",
    "POST parent /parent/me/children/studentId%2F1/related-accounts",
  ],
  "api/parent/me/children/[studentId]/request-sharing-options": [
    "GET parent /parent/me/children/studentId%2F1/request-sharing-options",
  ],
  "api/parent/me/children/[studentId]/request-sharing/[requestType]/[requestId]":
    [
      "GET parent /parent/me/children/studentId%2F1/request-sharing/requestType%2F1/requestId%2F1",
      "PUT parent /parent/me/children/studentId%2F1/request-sharing/requestType%2F1/requestId%2F1",
    ],
  "api/parent/me/children/[studentId]/requests/[type]/[id]/events": [
    "GET parent /parent/me/children/studentId%2F1/requests/type%2F1/id%2F1/events",
  ],
  "api/parent/me/children/[studentId]/today": [
    "GET parent /parent/me/children/studentId%2F1/today",
  ],
  "api/parent/me/enrollable-schools": [
    "GET parent /parent/me/enrollable-schools",
  ],
  "api/parent/me/documents": ["GET parent /parent/me/documents"],
  "api/parent/me/enrollments": ["GET parent /parent/me/enrollments"],
  "api/parent/me/messages": ["GET parent /parent/me/messages"],
  "api/parent/me/messages/children/[studentId]": [
    "GET parent /parent/me/messages/children/studentId%2F1",
    "POST parent /parent/me/messages/children/studentId%2F1",
  ],
  "api/parent/me/messages/children/[studentId]/threads": [
    "GET parent /parent/me/messages/children/studentId%2F1/threads",
  ],
  "api/parent/me/messages/group-threads": [
    "GET parent /parent/me/messages/group-threads",
  ],
  "api/parent/me/messages/group-threads/[groupThreadId]": [
    "GET parent /parent/me/messages/group-threads/groupThreadId%2F1",
  ],
  "api/parent/me/messages/unread-count": [
    "GET parent /parent/me/messages/unread-count",
  ],
  "api/parent/me/news": ["GET parent /parent/me/news"],
  "api/parent/me/news/[announcementId]/acknowledge": [
    "POST parent /parent/me/news/announcementId%2F1/acknowledge",
  ],
  "api/parent/me/news/[announcementId]/attachments": [
    "GET parent /parent-news-attachments/announcementId%2F1",
  ],
  "api/parent/me/news/[announcementId]/declaration": [
    "POST parent /parent/me/news/announcementId%2F1/declaration",
  ],
  "api/parent/me/news/[announcementId]/declaration/proof": [
    "GET parent /parent/me/news/announcementId%2F1/declaration/proof",
  ],
  "api/parent/me/news/[announcementId]/read": [
    "POST parent /parent/me/news/announcementId%2F1/read",
  ],
  "api/parent/me/news/[announcementId]/respond": [
    "POST parent /parent/me/news/announcementId%2F1/respond",
  ],
  "api/parent/me/news/unread-count": [
    "GET parent /parent/me/news/unread-count",
  ],
  "api/parent/me/profile": [
    "GET parent /parent/me/profile",
    "PUT parent /parent/me/profile",
  ],
  "api/parent/me/push/public-key": ["GET parent /parent/me/push/public-key"],
  "api/parent/me/pwa-usage": ["POST parent /parent/me/pwa-usage"],
  "api/platform/announcements/[id]/dismiss": [
    "POST tenant /api/platform/announcements/id%2F1/dismiss",
  ],
  "api/platform/announcements/unread": [
    "GET tenant /api/platform/announcements/unread",
  ],
  "api/pwa/usage": ["POST tenant /api/pwa/usage"],
  "api/rooms/by-category": ["GET tenant /api/rooms/by-category raw"],
  "api/school-setup": ["GET tenant /api/school-setup"],
  "api/school-setup/complete": ["POST tenant /api/school-setup/complete"],
  "api/school-setup/dismissal": ["PUT tenant /api/school-setup/dismissal"],
  "api/school-setup/steps/[step]": [
    "PUT tenant /api/school-setup/steps/step%2F1",
  ],
  "api/school/analytics-consent": [
    "GET school /analytics-consent/",
    "PUT school /analytics-consent/",
  ],
  "api/school/analytics-policy": ["GET school /analytics-policy"],
  "api/school/calendar/appointments": [
    "POST school /school/calendar/appointments",
  ],
  "api/school/calendar/feed": ["GET school /school/calendar/feed"],
  "api/school/calendar/feed/rotate": [
    "POST school /school/calendar/feed/rotate",
  ],
  "api/school/calendar/my": ["GET school /school/calendar/my"],
  "api/school/calendar/recipients/[recipientId]/response": [
    "POST school /school/calendar/recipients/recipientId%2F1/response",
  ],
  "api/school/class-day": ["GET school /school/class-day/"],
  "api/school/class-day/arrival-exceptions": [
    "GET school /school/class-day/arrival-exceptions",
  ],
  "api/school/class-day/arrival-exceptions/[schoolClass]/[date]": [
    "DELETE school /school/class-day/arrival-exceptions/schoolClass%2F1/date%2F1",
    "PUT school /school/class-day/arrival-exceptions/schoolClass%2F1/date%2F1",
  ],
  "api/school/class-day/arrival-exceptions/block-start": [
    "GET school /school/class-day/arrival-exceptions/block-start",
  ],
  "api/school/class-day/class-trips/[schoolClass]": [
    "POST school /school/class-day/class-trips/schoolClass%2F1",
  ],
  "api/school/class-day/homework/[schoolClass]": [
    "POST school /school/class-day/homework/schoolClass%2F1",
  ],
  "api/school/class-day/homework/[schoolClass]/[homeworkId]": [
    "DELETE school /school/class-day/homework/schoolClass%2F1/homeworkId%2F1",
    "PUT school /school/class-day/homework/schoolClass%2F1/homeworkId%2F1",
  ],
  "api/school/class-day/handover/[schoolClass]/[date]": [
    "DELETE school /school/class-day/handover/schoolClass%2F1/date%2F1",
    "PUT school /school/class-day/handover/schoolClass%2F1/date%2F1",
  ],
  "api/school/class-day/classes": ["GET school /school/class-day/classes"],
  "api/school/class-day/day-hints/[schoolClass]/[date]": [
    "DELETE school /school/class-day/day-hints/schoolClass%2F1/date%2F1",
    "PUT school /school/class-day/day-hints/schoolClass%2F1/date%2F1",
  ],
  "api/school/class-day/day-hints/[schoolClass]/[date]/students/[studentId]": [
    "DELETE school /school/class-day/day-hints/schoolClass%2F1/date%2F1/students/studentId%2F1",
    "PUT school /school/class-day/day-hints/schoolClass%2F1/date%2F1/students/studentId%2F1",
  ],
  "api/school/class-day/morning-attendance/[schoolClass]/[date]": [
    "PUT school /school/class-day/morning-attendance/schoolClass%2F1/date%2F1",
  ],
  "api/school/class-day/school-day": [
    "GET school /school/class-day/school-day",
  ],
  "api/school/class-day/shared-notes/[schoolClass]/students/[studentId]": [
    "GET school /school/class-day/shared-notes/schoolClass%2F1/students/studentId%2F1",
  ],
  "api/school/parent-letters": [
    "GET school /school/parent-letters/",
    "POST school /school/parent-letters/",
  ],
  "api/school/parent-letters/[announcementId]": [
    "DELETE school /school/parent-letters/announcementId%2F1",
    "GET school /school/parent-letters/announcementId%2F1",
  ],
  "api/school/parent-letters/[announcementId]/declaration-status": [
    "GET school /school/parent-letters/announcementId%2F1/declaration-status",
  ],
  "api/school/parent-letters/[announcementId]/letter-status": [
    "GET school /school/parent-letters/announcementId%2F1/letter-status",
  ],
  "api/school/parent-letters/[announcementId]/publish": [
    "POST school /school/parent-letters/announcementId%2F1/publish",
  ],
  "api/school/parent-letters/[announcementId]/recipients": [
    "GET school /school/parent-letters/announcementId%2F1/recipients",
  ],
  "api/school/parent-letters/[announcementId]/remind": [
    "POST school /school/parent-letters/announcementId%2F1/remind",
  ],
  "api/school/parent-letters/[announcementId]/resend-failed": [
    "POST school /school/parent-letters/announcementId%2F1/resend-failed",
  ],
  "api/school/parent-letters/[announcementId]/stats": [
    "GET school /school/parent-letters/announcementId%2F1/stats",
  ],
  "api/school/parent-letters/[announcementId]/unpublish": [
    "POST school /school/parent-letters/announcementId%2F1/unpublish",
  ],
  "api/school/notifications/preferences": [
    "DELETE school /school/notifications/preferences/",
    "GET school /school/notifications/preferences/",
  ],
  "api/school/notifications/push/public-key": [
    "GET school /school/notifications/push/public-key",
  ],
  "api/school/notifications/test": ["POST school /school/notifications/test"],
  "api/school/staff-messages": ["GET school /school/staff-messages/"],
  "api/school/staff-messages/recipients": [
    "GET school /school/staff-messages/recipients",
  ],
  "api/school/staff-messages/threads/[threadID]": [
    "GET school /school/staff-messages/threads/threadID/1",
    "POST school /school/staff-messages/threads/threadID/1",
  ],
  "api/school/staff-messages/threads/[threadID]/leave": [
    "POST school /school/staff-messages/threads/threadID/1/leave",
  ],
  "api/school/staff-messages/threads/open": [
    "POST school /school/staff-messages/threads/open",
  ],
  "api/school/staff-messages/unread-count": [
    "GET school /school/staff-messages/unread-count",
  ],
  "api/school/staff-notices/[noticeId]/acknowledge": [
    "POST school /school/staff-notices/noticeId%2F1/acknowledge",
  ],
  "api/school/staff-notices/today": ["GET school /school/staff-notices/today"],
  "api/school/supervisions": ["GET school /school/supervisions/"],
  "api/school/supervisions/[id]/complete": [
    "POST school /school/supervisions/id/1/complete",
  ],
  "api/school/supervisions/[id]/roster": [
    "GET school /school/supervisions/id/1/roster",
  ],
  "api/school/supervisions/[id]/start": [
    "POST school /school/supervisions/id/1/start",
  ],
  "api/school/supervisions/[id]/students/[studentId]/attendance": [
    "PATCH school /school/supervisions/id/1/students/studentId/1/attendance",
  ],
  "api/school/supervisions/[id]/students/[studentId]/check-in": [
    "POST school /school/supervisions/id/1/students/studentId/1/check-in",
  ],
  "api/school/supervisions/[id]/students/[studentId]/check-out": [
    "POST school /school/supervisions/id/1/students/studentId/1/check-out",
  ],
  "api/school/supervisions/[id]/students/[studentId]/homework-feedback": [
    "PUT school /school/supervisions/id%2F1/students/studentId%2F1/homework-feedback",
  ],
  "api/school/supervisions/[id]/students/[studentId]/sheet": [
    "GET school /school/supervisions/id/1/students/studentId/1/sheet",
  ],
  "api/settings/home-layout": [
    "DELETE tenant /api/settings/home-layout",
    "GET tenant /api/settings/home-layout",
    "PUT tenant /api/settings/home-layout",
  ],
  "api/settings/home-layout/policies": [
    "PUT tenant /api/settings/home-layout/policies",
  ],
  "api/settings/payroll-status": ["GET tenant /api/settings/payroll-status"],
  "api/staff-messages/groups": ["POST tenant /api/staff-messages/groups"],
  "api/staff-messages/threads/[threadID]/leave": [
    "POST tenant /api/staff-messages/threads/threadID%2F1/leave",
  ],
  "api/staff-messages/threads/[threadID]/members": [
    "POST tenant /api/staff-messages/threads/threadID%2F1/members",
  ],
  "api/staff-messages/threads/[threadID]/members/[accountID]": [
    "DELETE tenant /api/staff-messages/threads/threadID%2F1/members/accountID%2F1",
  ],
  "api/staff-messages/threads/[threadID]/name": [
    "PUT tenant /api/staff-messages/threads/threadID%2F1/name",
  ],
  "api/staff-onboarding": ["GET tenant /api/staff-onboarding"],
  "api/staff-onboarding/dismissal": [
    "PUT tenant /api/staff-onboarding/dismissal",
  ],
  "api/staff-onboarding/steps/[step]": [
    "PUT tenant /api/staff-onboarding/steps/step%2F1",
  ],
  "api/staff/[id]/absences/rebook": [
    "POST tenant /api/staff/id%2F1/absences/rebook",
  ],
  "api/staff/[id]/documents/[documentId]": [
    "DELETE tenant /api/staff/id%2F1/documents/documentId%2F1",
  ],
  "api/staff/[id]/payroll-number": [
    "GET tenant /api/staff/id%2F1/payroll-number",
    "PUT tenant /api/staff/id%2F1/payroll-number",
  ],
  "api/staff/[id]/schedule": [
    "GET tenant /api/staff/id%2F1/schedule",
    "PUT tenant /api/staff/id%2F1/schedule",
  ],
  "api/staff/[id]/school-classes": [
    "GET tenant /api/staff/id%2F1/school-classes",
    "PUT tenant /api/staff/id%2F1/school-classes",
  ],
  "api/staff/[id]/stammdaten": ["GET tenant /api/staff/id%2F1/stammdaten"],
  "api/staff/[id]/stammdaten/[section]": [
    "PUT tenant /api/staff/id%2F1/stammdaten/section%2F1",
  ],
  "api/staff/[id]/stammdaten/bank-steuer": [
    "GET tenant /api/staff/id%2F1/stammdaten/bank-steuer",
    "PUT tenant /api/staff/id%2F1/stammdaten/bank-steuer",
  ],
  "api/staff/[id]/stammdaten/bank-steuer/reveal": [
    "POST tenant /api/staff/id%2F1/stammdaten/bank-steuer/reveal",
  ],
  "api/staff/[id]/time-tracking/comp-time-preview": [
    "GET tenant /api/staff/id%2F1/time-tracking/comp-time-preview",
  ],
  "api/staff/[id]/time-tracking/month-summary": [
    "GET tenant /api/staff/id%2F1/time-tracking/month-summary",
  ],
  "api/staff/[id]/time-tracking/schedule-targets": [
    "GET tenant /api/staff/id%2F1/time-tracking/schedule-targets",
  ],
  "api/staff/[id]/time-tracking/sessions": [
    "POST tenant /api/staff/id%2F1/time-tracking/sessions",
  ],
  "api/staff/[id]/time-tracking/sessions/[sessionId]": [
    "PUT tenant /api/staff/id%2F1/time-tracking/sessions/sessionId%2F1",
  ],
  "api/staff/[id]/time-tracking/sessions/[sessionId]/edits": [
    "GET tenant /api/staff/id%2F1/time-tracking/sessions/sessionId%2F1/edits",
  ],
  "api/staff/absence-types": [
    "GET tenant /api/absence-types",
    "POST tenant /api/absence-types",
  ],
  "api/staff/absence-types/[absenceTypeId]": [
    "PUT tenant /api/absence-types/absenceTypeId%2F1",
  ],
  "api/staff/absence-types/[absenceTypeId]/allowances/[staffId]/preview": [
    "GET tenant /api/absence-types/absenceTypeId%2F1/allowances/staffId%2F1/preview",
  ],
  "api/staff/dashboard-summary": ["GET tenant /api/staff/dashboard-summary"],
  "api/staff/documents-profile/[id]": [
    "GET tenant /api/staff/documents-profile/id%2F1",
  ],
  "api/staff/financial-profile/[id]": [
    "GET tenant /api/staff/financial-profile/id%2F1",
  ],
  "api/staff/shift-types": [
    "GET tenant /api/shift-types",
    "POST tenant /api/shift-types",
  ],
  "api/staff/shift-types/[shiftTypeId]": [
    "DELETE tenant /api/shift-types/shiftTypeId%2F1",
    "PUT tenant /api/shift-types/shiftTypeId%2F1",
  ],
  "api/staff/shift-types/defaults": ["POST tenant /api/shift-types/defaults"],
  "api/staff/shifts": [
    "GET tenant /api/staff-shifts",
    "POST tenant /api/staff-shifts",
  ],
  "api/staff/shifts/[shiftId]": [
    "DELETE tenant /api/staff-shifts/shiftId%2F1",
    "PUT tenant /api/staff-shifts/shiftId%2F1",
  ],
  "api/staff/shifts/[shiftId]/cancellation": [
    "PUT tenant /api/staff-shifts/shiftId%2F1/cancellation",
  ],
  "api/staff/shifts/[shiftId]/move": [
    "PUT tenant /api/staff-shifts/shiftId%2F1/move",
  ],
  "api/staff/shifts/overview": ["GET tenant /api/staff-shifts/overview"],
  "api/staff/shifts/series": ["POST tenant /api/staff-shifts/series"],
  "api/staff/shifts/series/[seriesId]/split": [
    "PUT tenant /api/staff-shifts/series/seriesId%2F1/split",
  ],
  "api/staff/time-tracking/audit-log": [
    "GET tenant /api/staff/time-tracking/audit-log",
  ],
  "api/staff/time-tracking/export/datev-report": [
    "GET tenant /api/staff/time-tracking/export/datev-report",
  ],
  "api/staff/time-tracking/export/sftp": [
    "POST tenant /api/staff/time-tracking/export/sftp",
  ],
  "api/staff/time-tracking/export/sftp-status": [
    "GET tenant /api/staff/time-tracking/export/sftp-status",
  ],
  "api/staff/time-tracking/overview": [
    "GET tenant /api/staff/time-tracking/overview",
  ],
  "api/students/[id]/arrival-exceptions": [
    "POST tenant /api/students/id%2F1/arrival-exceptions",
  ],
  "api/students/[id]/arrival-exceptions/[exceptionId]": [
    "DELETE tenant /api/students/id%2F1/arrival-exceptions/exceptionId%2F1",
    "PUT tenant /api/students/id%2F1/arrival-exceptions/exceptionId%2F1",
  ],
  "api/students/[id]/arrival-notes": [
    "POST tenant /api/students/id%2F1/arrival-notes",
  ],
  "api/students/[id]/arrival-notes/[noteId]": [
    "DELETE tenant /api/students/id%2F1/arrival-notes/noteId%2F1",
    "PUT tenant /api/students/id%2F1/arrival-notes/noteId%2F1",
  ],
  "api/students/[id]/arrival-schedules": [
    "GET tenant /api/students/id%2F1/arrival-schedules",
    "PUT tenant /api/students/id%2F1/arrival-schedules",
  ],
  "api/students/[id]/care-end/resume": [
    "POST tenant /api/students/id%2F1/care-end/resume",
  ],
  "api/students/[id]/care-withdrawals": [
    "GET tenant /api/students/id%2F1/care-withdrawals",
  ],
  "api/students/[id]/companions": [
    "GET tenant /api/students/id%2F1/companions",
  ],
  "api/students/[id]/departure-exceptions/[date]": [
    "DELETE tenant /api/students/id%2F1/departure-exceptions/date%2F1",
    "PUT tenant /api/students/id%2F1/departure-exceptions/date%2F1",
  ],
  "api/students/[id]/documents/[documentId]": [
    "DELETE tenant /api/students/id%2F1/documents/documentId%2F1",
  ],
  "api/students/[id]/note-subjects": [
    "GET tenant /api/students/id%2F1/note-subjects",
  ],
  "api/students/[id]/notes": [
    "GET tenant /api/students/id%2F1/notes",
    "POST tenant /api/students/id%2F1/notes",
  ],
  "api/students/[id]/notes/[noteId]": [
    "DELETE tenant /api/students/id%2F1/notes/noteId%2F1",
    "PUT tenant /api/students/id%2F1/notes/noteId%2F1",
  ],
  "api/students/[id]/pickup-exceptions": [
    "POST tenant /api/students/id%2F1/pickup-exceptions",
  ],
  "api/students/[id]/pickup-exceptions/[exceptionId]": [
    "DELETE tenant /api/students/id%2F1/pickup-exceptions/exceptionId%2F1",
    "PUT tenant /api/students/id%2F1/pickup-exceptions/exceptionId%2F1",
  ],
  "api/students/[id]/pickup-notes": [
    "POST tenant /api/students/id%2F1/pickup-notes",
    "PUT tenant /api/students/id%2F1/pickup-notes",
  ],
  "api/students/[id]/pickup-notes/[noteId]": [
    "DELETE tenant /api/students/id%2F1/pickup-notes/noteId%2F1",
    "PUT tenant /api/students/id%2F1/pickup-notes/noteId%2F1",
  ],
  "api/students/[id]/pickup-schedules": [
    "GET tenant /api/students/id%2F1/pickup-schedules",
    "PUT tenant /api/students/id%2F1/pickup-schedules",
  ],
  "api/students/[id]/pickup-schedules/apply": [
    "POST tenant /api/students/id%2F1/pickup-schedules/apply",
  ],
  "api/students/[id]/pickup-schedules/preview": [
    "POST tenant /api/students/id%2F1/pickup-schedules/preview",
  ],
  "api/students/[id]/pickup-schedules/reset-offering": [
    "POST tenant /api/students/id%2F1/pickup-schedules/reset-offering",
  ],
  "api/students/arrival-schedules/bulk": [
    "POST tenant /api/students/arrival-schedules/bulk",
  ],
  "api/students/arrival-schedules/status": [
    "POST tenant /api/students/arrival-schedules/status",
  ],
  "api/students/arrival-settings": [
    "GET tenant /api/students/arrival-settings",
  ],
  "api/students/arrival-times/bulk": [
    "POST tenant /api/students/arrival-times/bulk",
  ],
  "api/students/care-end": ["POST tenant /api/students/care-end"],
  "api/students/care-end/cancel": ["POST tenant /api/students/care-end/cancel"],
  "api/students/care-end/preview": [
    "POST tenant /api/students/care-end/preview",
  ],
  "api/students/care-withdrawals": [
    "GET tenant /api/students/care-withdrawals",
  ],
  "api/students/care-withdrawals/[completionId]/care-end": [
    "POST tenant /api/students/care-withdrawals/completionId%2F1/care-end",
  ],
  "api/students/care-withdrawals/[completionId]/care-end/preview": [
    "POST tenant /api/students/care-withdrawals/completionId%2F1/care-end/preview",
  ],
  "api/students/care-withdrawals/[completionId]/deletion-impact": [
    "GET tenant /api/students/care-withdrawals/completionId%2F1/deletion-impact",
  ],
  "api/students/change-requests/conflicts/resolve": [
    "POST tenant /api/students/change-requests/conflicts/resolve",
  ],
  "api/students/class-arrival-exceptions/[schoolClass]": [
    "GET tenant /api/students/class-arrival-exceptions/schoolClass%2F1",
  ],
  "api/students/class-arrival-exceptions/[schoolClass]/[date]": [
    "DELETE tenant /api/students/class-arrival-exceptions/schoolClass%2F1/date%2F1",
    "PUT tenant /api/students/class-arrival-exceptions/schoolClass%2F1/date%2F1",
  ],
  "api/students/class-arrival-times/[schoolClass]": [
    "GET tenant /api/students/class-arrival-times/schoolClass%2F1",
  ],
  "api/students/ended-care": ["GET tenant /api/students/ended-care"],
  "api/students/pickup-schedules/bulk": [
    "POST tenant /api/students/pickup-schedules/bulk",
  ],
  "api/students/pickup-times/bulk": [
    "POST tenant /api/students/pickup-times/bulk",
  ],
  "api/students/status-days/bulk": [
    "POST tenant /api/students/status-days/bulk",
  ],
  "api/substitutions": [
    "GET tenant /api/substitutions",
    "POST tenant /api/substitutions",
  ],
  "api/substitutions/end": ["POST tenant /api/substitutions/end"],
  "api/surveys": ["GET tenant /api/surveys", "POST tenant /api/surveys"],
  "api/surveys/[surveyId]/answers": [
    "POST tenant /api/surveys/surveyId%2F1/answers",
  ],
  "api/surveys/[surveyId]/close": [
    "POST tenant /api/surveys/surveyId%2F1/close",
  ],
  "api/surveys/[surveyId]/form": ["GET tenant /api/surveys/surveyId%2F1/form"],
  "api/surveys/[surveyId]/publish": [
    "POST tenant /api/surveys/surveyId%2F1/publish",
  ],
  "api/surveys/[surveyId]/remind": [
    "POST tenant /api/surveys/surveyId%2F1/remind",
  ],
  "api/surveys/[surveyId]/respondents": [
    "GET tenant /api/surveys/surveyId%2F1/respondents",
  ],
  "api/surveys/[surveyId]/results": [
    "GET tenant /api/surveys/surveyId%2F1/results",
  ],
  "api/surveys/to-answer": ["GET tenant /api/surveys/to-answer"],
  "api/time-tracking/[id]/edits": [
    "GET tenant /api/time-tracking/id%2F1/edits",
  ],
  "api/time-tracking/absences": [
    "GET tenant /api/time-tracking/absences",
    "POST tenant /api/time-tracking/absences",
  ],
  "api/time-tracking/absences/[id]": [
    "DELETE tenant /api/time-tracking/absences/id%2F1",
    "PUT tenant /api/time-tracking/absences/id%2F1",
  ],
  "api/time-tracking/assignments": [
    "GET tenant /api/time-tracking/assignments",
  ],
  "api/time-tracking/break/end": ["POST tenant /api/time-tracking/break/end"],
  "api/time-tracking/break/start": [
    "POST tenant /api/time-tracking/break/start",
  ],
  "api/time-tracking/breaks/[sessionId]": [
    "GET tenant /api/time-tracking/breaks/sessionId%2F1",
  ],
  "api/time-tracking/check-in": ["POST tenant /api/time-tracking/check-in"],
  "api/time-tracking/closing-days": [
    "GET tenant /api/time-tracking/closing-days",
  ],
  "api/time-tracking/config": ["GET tenant /api/time-tracking/config"],
  "api/time-tracking/absence-allowances": [
    "GET tenant /api/time-tracking/absence-allowances",
  ],
  "api/time-tracking/current": ["GET tenant /api/time-tracking/current"],
  "api/time-tracking/history": ["GET tenant /api/time-tracking/history"],
  "api/time-tracking/holidays": ["GET tenant /api/time-tracking/holidays"],
  "api/time-tracking/month-summary": [
    "GET tenant /api/time-tracking/month-summary",
  ],
  "api/time-tracking/schedule-targets": [
    "GET tenant /api/time-tracking/schedule-targets",
  ],
  "api/time-tracking/shifts": ["GET tenant /api/time-tracking/shifts"],
  "api/time-tracking/vacation/quota": [
    "GET tenant /api/time-tracking/vacation/quota",
  ],
  "api/time-tracking/vacation/request": [
    "POST tenant /api/time-tracking/vacation/request",
  ],
  "api/timetable/closing-days": [
    "GET tenant /api/timetable/closing-days",
    "POST tenant /api/timetable/closing-days",
  ],
  "api/timetable/closing-days/[id]": [
    "DELETE tenant /api/timetable/closing-days/id%2F1",
    "PUT tenant /api/timetable/closing-days/id%2F1",
  ],
  "api/timetable/conflict-acks": ["GET tenant /api/timetable/conflict-acks"],
  "api/timetable/conflict-acks/[fingerprint]": [
    "DELETE tenant /api/timetable/conflict-acks/fingerprint%2F1",
    "PUT tenant /api/timetable/conflict-acks/fingerprint%2F1",
  ],
  "api/timetable/conflict-check": ["GET tenant /api/timetable/conflicts"],
  "api/timetable/deviations/history": [
    "GET tenant /api/timetable/deviations/history",
  ],
  "api/timetable/gaps": ["GET tenant /api/timetable/gaps"],
  "api/timetable/holidays": ["GET tenant /api/timetable/holidays"],
  "api/timetable/instances": [
    "GET tenant /api/timetable/instances",
    "POST tenant /api/timetable/instances",
  ],
  "api/timetable/instances/[id]": [
    "DELETE tenant /api/timetable/instances/id%2F1",
    "PUT tenant /api/timetable/instances/id%2F1",
  ],
  "api/timetable/instances/[id]/cancel": [
    "POST tenant /api/timetable/instances/id%2F1/cancel",
  ],
  "api/timetable/instances/[id]/complete": [
    "POST tenant /api/timetable/instances/id%2F1/complete",
  ],
  "api/timetable/instances/[id]/deviations": [
    "POST tenant /api/timetable/instances/id%2F1/deviations",
  ],
  "api/timetable/instances/[id]/guardian-notice": [
    "GET tenant /api/timetable/instances/id%2F1/guardian-notice",
  ],
  "api/timetable/instances/[id]/move-staff": [
    "POST tenant /api/timetable/instances/id%2F1/move-staff",
  ],
  "api/timetable/instances/[id]/participants": [
    "GET tenant /api/timetable/instances/id%2F1/participants",
  ],
  "api/timetable/instances/[id]/reopen": [
    "POST tenant /api/timetable/instances/id%2F1/reopen",
  ],
  "api/timetable/instances/[id]/staff-pool": [
    "GET tenant /api/timetable/instances/id%2F1/staff-pool",
  ],
  "api/timetable/instances/[id]/start": [
    "POST tenant /api/timetable/instances/id%2F1/start",
  ],
  "api/timetable/instances/[id]/students/[studentId]": [
    "PATCH tenant /api/timetable/instances/id%2F1/students/studentId%2F1",
  ],
  "api/timetable/instances/[id]/students/[studentId]/correction": [
    "POST tenant /api/timetable/instances/id%2F1/students/studentId%2F1/correction",
  ],
  "api/timetable/instances/[id]/students/[studentId]/corrections": [
    "GET tenant /api/timetable/instances/id%2F1/students/studentId%2F1/corrections",
  ],
  "api/timetable/instances/bulk-cancel": [
    "POST tenant /api/timetable/instances/bulk-cancel",
  ],
  "api/timetable/instances/edited-in-window": [
    "GET tenant /api/timetable/instances/edited-in-window",
  ],
  "api/timetable/instances/re-plan-week": [
    "POST tenant /api/timetable/instances/re-plan-week",
  ],
  "api/timetable/materialize": ["POST tenant /api/timetable/materialize"],
  "api/timetable/offering-sources": [
    "GET tenant /api/timetable/offering-sources",
  ],
  "api/timetable/offering-sources/combined-counts": [
    "GET tenant /api/timetable/offering-sources/combined-counts",
  ],
  "api/timetable/offering-sources/preview": [
    "GET tenant /api/timetable/offering-sources/preview",
  ],
  "api/timetable/offering-sources/suggestion": [
    "GET tenant /api/timetable/offering-sources/suggestion",
  ],
  "api/timetable/operations/active-groups/[id]/roster": [
    "GET tenant /api/timetable/operations/active-groups/id%2F1/roster",
  ],
  "api/timetable/operations/instances/[id]/complete": [
    "POST tenant /api/timetable/operations/instances/id%2F1/complete",
  ],
  "api/timetable/operations/instances/[id]/reopen": [
    "POST tenant /api/timetable/operations/instances/id%2F1/reopen",
  ],
  "api/timetable/operations/instances/[id]/roster": [
    "GET tenant /api/timetable/operations/instances/id%2F1/roster",
  ],
  "api/timetable/operations/instances/[id]/start": [
    "POST tenant /api/timetable/operations/instances/id%2F1/start",
  ],
  "api/timetable/operations/instances/[id]/students/[studentId]/attendance": [
    "PATCH tenant /api/timetable/operations/instances/id%2F1/students/studentId%2F1/attendance",
  ],
  "api/timetable/operations/instances/[id]/students/[studentId]/check-in": [
    "POST tenant /api/timetable/operations/instances/id%2F1/students/studentId%2F1/check-in",
  ],
  "api/timetable/operations/instances/[id]/students/[studentId]/check-out": [
    "POST tenant /api/timetable/operations/instances/id%2F1/students/studentId%2F1/check-out",
  ],
  "api/timetable/operations/instances/[id]/students/[studentId]/homework-feedback":
    [
      "PUT tenant /api/timetable/operations/instances/id%2F1/students/studentId%2F1/homework-feedback",
    ],
  "api/timetable/operations/instances/[id]/students/check-in": [
    "POST tenant /api/timetable/operations/instances/id%2F1/students/check-in",
  ],
  "api/timetable/operations/planned-now": [
    "GET tenant /api/timetable/operations/planned-now",
  ],
  "api/timetable/operations/spontaneous/start": [
    "POST tenant /api/timetable/operations/spontaneous/start",
  ],
  "api/timetable/periods": [
    "GET tenant /api/timetable/periods",
    "POST tenant /api/timetable/periods",
  ],
  "api/timetable/periods/[id]": [
    "DELETE tenant /api/timetable/periods/id%2F1",
    "GET tenant /api/timetable/periods/id%2F1",
    "PUT tenant /api/timetable/periods/id%2F1",
  ],
  "api/timetable/periods/bootstrap": [
    "POST tenant /api/timetable/periods/bootstrap",
  ],
  "api/timetable/pickup-extensions": [
    "GET tenant /api/timetable/pickup-extensions/",
  ],
  "api/timetable/pickup-extensions/[id]/resolve": [
    "POST tenant /api/timetable/pickup-extensions/id%2F1/resolve",
  ],
  "api/timetable/planning-tracks": [
    "GET tenant /api/timetable/planning-tracks",
    "POST tenant /api/timetable/planning-tracks",
  ],
  "api/timetable/planning-tracks/[id]/restore": [
    "POST tenant /api/timetable/planning-tracks/id%2F1/restore",
  ],
  "api/timetable/planning-tracks/order": [
    "PUT tenant /api/timetable/planning-tracks/order",
  ],
  "api/timetable/shift-coverage": ["POST tenant /api/timetable/shift-coverage"],
  "api/timetable/student/[id]/day": [
    "GET tenant /api/timetable/student/id%2F1/day",
  ],
  "api/timetable/student/[id]/week": [
    "GET tenant /api/timetable/student/id%2F1/week",
  ],
  "api/timetable/substitutions/bulk": [
    "POST tenant /api/timetable/substitutions/bulk",
  ],
  "api/timetable/templates": [
    "GET tenant /api/timetable/templates",
    "POST tenant /api/timetable/templates",
  ],
  "api/timetable/templates/[id]": [
    "DELETE tenant /api/timetable/templates/id%2F1",
    "GET tenant /api/timetable/templates/id%2F1",
    "PUT tenant /api/timetable/templates/id%2F1",
  ],
  "api/timetable/templates/[id]/manual-roster": [
    "PUT tenant /api/timetable/templates/id%2F1/manual-roster",
  ],
  "api/timetable/templates/[id]/offering-connection": [
    "POST tenant /api/timetable/templates/id%2F1/offering-connection",
  ],
  "api/timetable/templates/[id]/offering-preview": [
    "GET tenant /api/timetable/templates/id%2F1/offering-preview",
  ],
  "api/timetable/templates/[id]/split": [
    "POST tenant /api/timetable/templates/id%2F1/split",
  ],
  "api/timetable/templates/[id]/students/[studentId]/remove-days": [
    "POST tenant /api/timetable/templates/id%2F1/students/studentId%2F1/remove-days",
  ],
  "api/timetable/templates/roster-deviations": [
    "GET tenant /api/timetable/templates/roster-deviations",
  ],
  "api/traeger/access-log": ["GET traeger /traeger/access-log"],
  "api/traeger/access-log/people": ["GET traeger /traeger/access-log/people"],
  "api/traeger/accounts": ["GET traeger /traeger/accounts/"],
  "api/traeger/accounts/[accountId]": [
    "DELETE traeger /traeger/accounts/accountId%2F1",
  ],
  "api/traeger/accounts/[accountId]/roles": [
    "PUT traeger /traeger/accounts/accountId%2F1/roles",
  ],
  "api/traeger/accounts/invitations": [
    "POST traeger /traeger/accounts/invitations",
  ],
  "api/traeger/accounts/invitations/[invitationId]": [
    "DELETE traeger /traeger/accounts/invitations/invitationId%2F1",
  ],
  "api/traeger/billing-runs": [
    "GET traeger /traeger/billing-runs",
    "POST traeger /traeger/billing-runs",
  ],
  "api/traeger/billing-runs/[id]": ["GET traeger /traeger/billing-runs/id%2F1"],
  "api/traeger/billing-runs/[id]/corrections": [
    "POST traeger /traeger/billing-runs/id%2F1/corrections",
  ],
  "api/traeger/billing-runs/[id]/documents/[documentId]/mailings": [
    "POST traeger /traeger/billing-runs/id%2F1/documents/documentId%2F1/mailings",
  ],
  "api/traeger/billing-runs/[id]/letters": [
    "POST traeger /traeger/billing-runs/id%2F1/letters",
  ],
  "api/traeger/billing-runs/[id]/mailings": [
    "GET traeger /traeger/billing-runs/id%2F1/mailings",
    "POST traeger /traeger/billing-runs/id%2F1/mailings",
  ],
  "api/traeger/billing-runs/[id]/release": [
    "POST traeger /traeger/billing-runs/id%2F1/release",
  ],
  "api/traeger/billing-runs/periods": [
    "GET traeger /traeger/billing-runs/periods",
  ],
  "api/traeger/contribution-rules": [
    "GET traeger /traeger/contribution-rules",
    "POST traeger /traeger/contribution-rules",
  ],
  "api/traeger/holiday-periods": [
    "GET traeger /traeger/holiday-periods",
    "POST traeger /traeger/holiday-periods",
  ],
  "api/traeger/holiday-periods/[id]": [
    "GET traeger /traeger/holiday-periods/id%2F1",
    "PUT traeger /traeger/holiday-periods/id%2F1",
  ],
  "api/traeger/holiday-periods/[id]/publish": [
    "POST traeger /traeger/holiday-periods/id%2F1/publish",
  ],
  "api/traeger/holiday-periods/[id]/registrations": [
    "GET traeger /traeger/holiday-periods/id%2F1/registrations",
  ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]": [
    "GET traeger /traeger/holiday-periods/id%2F1/registrations/registrationId%2F1",
  ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]/cancel": [
    "POST traeger /traeger/holiday-periods/id%2F1/registrations/registrationId%2F1/cancel",
  ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]/children/[childId]/bookings":
    [
      "PUT traeger /traeger/holiday-periods/id%2F1/registrations/registrationId%2F1/children/childId%2F1/bookings",
    ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]/confirm": [
    "POST traeger /traeger/holiday-contributions/id%2F1/registrations/registrationId%2F1/confirm",
  ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]/contribution":
    [
      "GET traeger /traeger/holiday-contributions/id%2F1/registrations/registrationId%2F1",
    ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]/freeze": [
    "POST traeger /traeger/holiday-periods/id%2F1/registrations/registrationId%2F1/freeze",
  ],
  "api/traeger/holiday-periods/[id]/registrations/[registrationId]/reopen": [
    "POST traeger /traeger/holiday-periods/id%2F1/registrations/registrationId%2F1/reopen",
  ],
  "api/traeger/master-data": ["GET traeger /traeger/master-data/"],
  "api/traeger/master-data/bank-accounts": [
    "POST traeger /traeger/master-data/bank-accounts",
  ],
  "api/traeger/master-data/contact": [
    "PUT traeger /traeger/master-data/contact",
  ],
  "api/traeger/number-ranges": ["GET traeger /traeger/number-ranges"],
  "api/traeger/number-ranges/[service]": [
    "PUT traeger /traeger/number-ranges/service%2F1",
  ],
  "api/traeger/organization": ["GET traeger /traeger/organization"],
  "api/traeger/school-sessions": ["POST traeger /traeger/school-sessions"],
  "api/traeger/schools": ["GET traeger /traeger/schools"],
  "api/traeger/settings/schema": ["GET traeger /traeger/settings/schema"],
  "api/work-time-models": [
    "GET tenant /api/work-time-models",
    "POST tenant /api/work-time-models",
  ],
  "api/work-time-models/[id]": [
    "DELETE tenant /api/work-time-models/id%2F1",
    "GET tenant /api/work-time-models/id%2F1",
    "PUT tenant /api/work-time-models/id%2F1",
  ],
};
