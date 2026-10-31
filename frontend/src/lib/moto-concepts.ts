// Registry of every concept: concept name → symbol, tone and label.
//
// The definitions live one per module in `~/lib/concepts/<key>`. A page that
// renders a fixed concept imports that module directly, so its bundle carries
// only that glyph instead of all of them (about 73 kB, see #5135). Import this
// registry only where the concept key is known at runtime (data-driven
// navigation, status mappings); it pulls in every glyph.
import { dashboardConcept } from "~/lib/concepts/dashboard";
import { childrenConcept } from "~/lib/concepts/children";
import { groupsConcept } from "~/lib/concepts/groups";
import { roomsConcept } from "~/lib/concepts/rooms";
import { staffConcept } from "~/lib/concepts/staff";
import { parentsConcept } from "~/lib/concepts/parents";
import { activitiesConcept } from "~/lib/concepts/activities";
import { supervisionConcept } from "~/lib/concepts/supervision";
import { responsibilityConcept } from "~/lib/concepts/responsibility";
import { organizationsConcept } from "~/lib/concepts/organizations";
import { schoolsConcept } from "~/lib/concepts/schools";
import { classDayConcept } from "~/lib/concepts/classDay";
import { peopleConcept } from "~/lib/concepts/people";
import { operatorsConcept } from "~/lib/concepts/operators";
import { presentConcept } from "~/lib/concepts/present";
import { transitConcept } from "~/lib/concepts/transit";
import { schoolyardConcept } from "~/lib/concepts/schoolyard";
import { sickConcept } from "~/lib/concepts/sick";
import { excusedConcept } from "~/lib/concepts/excused";
import { homeConcept } from "~/lib/concepts/home";
import { atSchoolConcept } from "~/lib/concepts/atSchool";
import { classTripConcept } from "~/lib/concepts/classTrip";
import { freeRoomsConcept } from "~/lib/concepts/freeRooms";
import { utilizationConcept } from "~/lib/concepts/utilization";
import { emergencyConcept } from "~/lib/concepts/emergency";
import { notArrivalConcept } from "~/lib/concepts/notArrival";
import { unknownConcept } from "~/lib/concepts/unknown";
import { calendarConcept } from "~/lib/concepts/calendar";
import { carePlanConcept } from "~/lib/concepts/carePlan";
import { careTimesConcept } from "~/lib/concepts/careTimes";
import { staffPlanConcept } from "~/lib/concepts/staffPlan";
import { substitutionConcept } from "~/lib/concepts/substitution";
import { groupAccessConcept } from "~/lib/concepts/groupAccess";
import { timeTrackingConcept } from "~/lib/concepts/timeTracking";
import { dayReportConcept } from "~/lib/concepts/dayReport";
import { calendarPeriodsConcept } from "~/lib/concepts/calendarPeriods";
import { closingDaysConcept } from "~/lib/concepts/closingDays";
import { payrollConcept } from "~/lib/concepts/payroll";
import { contributionsConcept } from "~/lib/concepts/contributions";
import { billingConcept } from "~/lib/concepts/billing";
import { messagesConcept } from "~/lib/concepts/messages";
import { parentMessagesConcept } from "~/lib/concepts/parentMessages";
import { parentConversationsConcept } from "~/lib/concepts/parentConversations";
import { newsConcept } from "~/lib/concepts/news";
import { announcementsConcept } from "~/lib/concepts/announcements";
import { pollsConcept } from "~/lib/concepts/polls";
import { confirmationsConcept } from "~/lib/concepts/confirmations";
import { feedbackConcept } from "~/lib/concepts/feedback";
import { mealPlanConcept } from "~/lib/concepts/mealPlan";
import { databaseConcept } from "~/lib/concepts/database";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { reportsConcept } from "~/lib/concepts/reports";
import { holidayWeeksConcept } from "~/lib/concepts/holidayWeeks";
import { weekProtocolsConcept } from "~/lib/concepts/weekProtocols";
import { listsConcept } from "~/lib/concepts/lists";
import { devicesConcept } from "~/lib/concepts/devices";
import { infoDisplaysConcept } from "~/lib/concepts/infoDisplays";
import { accountsConcept } from "~/lib/concepts/accounts";
import { rolesConcept } from "~/lib/concepts/roles";
import { permissionsConcept } from "~/lib/concepts/permissions";
import { passkeysConcept } from "~/lib/concepts/passkeys";
import { exportsConcept } from "~/lib/concepts/exports";
import { gradeTransitionsConcept } from "~/lib/concepts/gradeTransitions";
import { pickupConcept } from "~/lib/concepts/pickup";
import { transportConcept } from "~/lib/concepts/transport";
import { birthdaysConcept } from "~/lib/concepts/birthdays";
import { changeHistoryConcept } from "~/lib/concepts/changeHistory";
import { requestsConcept } from "~/lib/concepts/requests";
import { documentsConcept } from "~/lib/concepts/documents";
import { filesConcept } from "~/lib/concepts/files";
import { rfidConcept } from "~/lib/concepts/rfid";
import { helpConcept } from "~/lib/concepts/help";
import { settingsConcept } from "~/lib/concepts/settings";
import { notificationsConcept } from "~/lib/concepts/notifications";
import { languageConcept } from "~/lib/concepts/language";
import { logoutConcept } from "~/lib/concepts/logout";
import { moreConcept } from "~/lib/concepts/more";
import type { MotoConceptDefinition } from "~/lib/concepts/define";

export type { MotoConceptDefinition } from "~/lib/concepts/define";

const registry = {
  dashboard: dashboardConcept,
  children: childrenConcept,
  groups: groupsConcept,
  rooms: roomsConcept,
  staff: staffConcept,
  parents: parentsConcept,
  activities: activitiesConcept,
  supervision: supervisionConcept,
  responsibility: responsibilityConcept,
  organizations: organizationsConcept,
  schools: schoolsConcept,
  classDay: classDayConcept,
  people: peopleConcept,
  operators: operatorsConcept,
  present: presentConcept,
  transit: transitConcept,
  schoolyard: schoolyardConcept,
  sick: sickConcept,
  excused: excusedConcept,
  home: homeConcept,
  atSchool: atSchoolConcept,
  classTrip: classTripConcept,
  freeRooms: freeRoomsConcept,
  utilization: utilizationConcept,
  emergency: emergencyConcept,
  notArrival: notArrivalConcept,
  unknown: unknownConcept,
  calendar: calendarConcept,
  carePlan: carePlanConcept,
  careTimes: careTimesConcept,
  staffPlan: staffPlanConcept,
  substitution: substitutionConcept,
  groupAccess: groupAccessConcept,
  timeTracking: timeTrackingConcept,
  dayReport: dayReportConcept,
  calendarPeriods: calendarPeriodsConcept,
  closingDays: closingDaysConcept,
  payroll: payrollConcept,
  contributions: contributionsConcept,
  billing: billingConcept,
  messages: messagesConcept,
  parentMessages: parentMessagesConcept,
  parentConversations: parentConversationsConcept,
  news: newsConcept,
  announcements: announcementsConcept,
  polls: pollsConcept,
  confirmations: confirmationsConcept,
  feedback: feedbackConcept,
  mealPlan: mealPlanConcept,
  database: databaseConcept,
  enrollments: enrollmentsConcept,
  reports: reportsConcept,
  holidayWeeks: holidayWeeksConcept,
  weekProtocols: weekProtocolsConcept,
  lists: listsConcept,
  devices: devicesConcept,
  infoDisplays: infoDisplaysConcept,
  accounts: accountsConcept,
  roles: rolesConcept,
  permissions: permissionsConcept,
  passkeys: passkeysConcept,
  exports: exportsConcept,
  gradeTransitions: gradeTransitionsConcept,
  pickup: pickupConcept,
  transport: transportConcept,
  birthdays: birthdaysConcept,
  changeHistory: changeHistoryConcept,
  requests: requestsConcept,
  documents: documentsConcept,
  files: filesConcept,
  rfid: rfidConcept,
  help: helpConcept,
  settings: settingsConcept,
  notifications: notificationsConcept,
  language: languageConcept,
  logout: logoutConcept,
  more: moreConcept,
} as const;

/** Every concept under its own key; a mismatched key fails to compile. */
export const MOTO_CONCEPTS: {
  readonly [K in keyof typeof registry]: MotoConceptDefinition<K>;
} = registry;

export type MotoConceptKey = keyof typeof MOTO_CONCEPTS;

/** A concept definition, as the concept components take it. */
export type MotoConcept = MotoConceptDefinition<MotoConceptKey>;
