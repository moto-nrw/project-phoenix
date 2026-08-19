import {
  AddressBookIcon,
  ArrowsLeftRightIcon,
  BellIcon,
  BookOpenIcon,
  BriefcaseIcon,
  BuildingsIcon,
  BusIcon,
  CakeIcon,
  CalendarCheckIcon,
  CalendarDotsIcon,
  CalendarIcon,
  CalendarPlusIcon,
  CalendarSlashIcon,
  CalendarXIcon,
  ChalkboardTeacherIcon,
  ChatCircleTextIcon,
  ChartBarIcon,
  ChatsCircleIcon,
  ClipboardTextIcon,
  ClockIcon,
  ClockCounterClockwiseIcon,
  ContactlessPaymentIcon,
  DatabaseIcon,
  DevicesIcon,
  DoorIcon,
  DoorOpenIcon,
  DotsThreeIcon,
  EnvelopeIcon,
  EnvelopeOpenIcon,
  EyeIcon,
  FileArrowDownIcon,
  FingerprintSimpleIcon,
  FirstAidKitIcon,
  ForkKnifeIcon,
  GearIcon,
  HouseIcon,
  HouseLineIcon,
  IdentificationCardIcon,
  ListBulletsIcon,
  ListChecksIcon,
  LockKeyIcon,
  MegaphoneIcon,
  MonitorIcon,
  NotePencilIcon,
  PersonSimpleWalkIcon,
  PresentationChartIcon,
  QuestionIcon,
  ShieldCheckIcon,
  ShieldWarningIcon,
  SignOutIcon,
  SunIcon,
  TranslateIcon,
  TrayIcon,
  TrendUpIcon,
  UserCheckIcon,
  UserCircleGearIcon,
  UserGearIcon,
  UserIcon,
  UserPlusIcon,
  UserSwitchIcon,
  UsersIcon,
  UsersFourIcon,
  UsersThreeIcon,
  VanIcon,
} from "@phosphor-icons/react/ssr";
import type { Icon as PhosphorIcon, IconProps } from "@phosphor-icons/react";
import type { MotoDuotoneTone } from "~/lib/location-helper";

export type MotoConceptKind = "core" | "status" | "function";
export type MotoConceptSection =
  | "people"
  | "presence"
  | "planning"
  | "communication"
  | "administration"
  | "system";

export interface MotoConceptDefinition {
  readonly label: string;
  readonly icon: PhosphorIcon;
  readonly tone: MotoDuotoneTone;
  readonly kind: MotoConceptKind;
  readonly section: MotoConceptSection;
  readonly weight: IconProps["weight"];
}

export const MOTO_CONCEPTS = {
  dashboard: concept("Home", HouseLineIcon, "neutral", "function", "system"),
  children: concept("Kinder", UserIcon, "greenVivid", "core", "people"),
  groups: concept("Gruppen", UsersThreeIcon, "greenDeep", "core", "people"),
  rooms: concept("Räume", DoorIcon, "navy", "core", "people"),
  staff: concept(
    "Mitarbeiter",
    IdentificationCardIcon,
    "orange",
    "core",
    "people",
  ),
  parents: concept("Eltern", UsersIcon, "blue", "core", "people"),
  activities: concept("Aktivitäten", ListChecksIcon, "coral", "core", "people"),
  supervision: concept("Aufsicht", EyeIcon, "purple", "core", "people"),
  // Primary responsibility for a slot, as opposed to merely supervising it.
  // Carries the ShieldCheck the ResponsibilityBadge used before the concept
  // system existed; "function" kind keeps it out of the unique-primary-color
  // rule that only governs core and status concepts.
  responsibility: concept(
    "Zuständig",
    ShieldCheckIcon,
    "teal",
    "function",
    "people",
  ),
  organizations: concept(
    "Träger",
    BuildingsIcon,
    "petrol",
    "core",
    "administration",
  ),
  schools: concept(
    "Schulen",
    ChalkboardTeacherIcon,
    "gold",
    "core",
    "administration",
  ),
  people: concept("Personen", AddressBookIcon, "teal", "core", "people"),
  operators: concept("Operatoren", UsersFourIcon, "mint", "core", "people"),

  present: concept(
    "Anwesend",
    UserCheckIcon,
    "greenDeep",
    "status",
    "presence",
  ),
  transit: concept(
    "Unterwegs",
    PersonSimpleWalkIcon,
    "magenta",
    "status",
    "presence",
  ),
  // Orange, matching LOCATION_COLORS.SCHOOLYARD (#F78C10) — the badge for
  // this very status. Every other status concept sits in the hue family of
  // its badge; Schulhof was the one that did not, so the dashboard tile and
  // the student-list pill showed the same status in two colors. Amber is
  // reserved for LOCATION_COLORS.WARNING now.
  schoolyard: concept("Schulhof", SunIcon, "orange", "status", "presence"),
  sick: concept("Krank", FirstAidKitIcon, "red", "status", "presence"),
  excused: concept(
    "Entschuldigt",
    ShieldCheckIcon,
    "purple",
    "status",
    "presence",
  ),
  home: concept("Zuhause", HouseIcon, "neutral", "status", "presence"),
  classTrip: concept("Klassenfahrt", BusIcon, "cyan", "status", "presence"),
  freeRooms: concept("Freie Räume", DoorOpenIcon, "mint", "status", "presence"),
  // Gold, not orange: orange belongs to the Schulhof status above, and the
  // unique-primary-color rule for status concepts allows only one owner.
  // Auslastung is a metric rather than a presence state, so it has no badge
  // hue of its own to match.
  utilization: concept(
    "Auslastung",
    ChartBarIcon,
    "gold",
    "status",
    "presence",
  ),
  emergency: concept(
    "Notfall",
    ShieldWarningIcon,
    "wine",
    "status",
    "presence",
  ),
  notArrival: concept(
    "Kommt heute nicht",
    CalendarXIcon,
    "navy",
    "status",
    "presence",
  ),
  unknown: concept("Unbekannt", QuestionIcon, "stone", "status", "presence"),

  calendar: concept("Kalender", CalendarIcon, "indigo", "function", "planning"),
  carePlan: concept(
    "Betreuungsplan",
    CalendarDotsIcon,
    "indigo",
    "function",
    "planning",
  ),
  careTimes: concept(
    "Betreuungszeiten",
    ClockIcon,
    "indigo",
    "function",
    "planning",
  ),
  staffPlan: concept(
    "Dienstplan",
    ClipboardTextIcon,
    "indigo",
    "function",
    "planning",
  ),
  substitution: concept(
    "Vertretung",
    ArrowsLeftRightIcon,
    "indigo",
    "function",
    "planning",
  ),
  groupAccess: concept(
    "Gruppenzugriff",
    UserSwitchIcon,
    "purple",
    "function",
    "planning",
  ),
  timeTracking: concept(
    "Zeiterfassung",
    ClockIcon,
    "timeTracking",
    "function",
    "planning",
  ),
  dayReport: concept(
    "Tagesauswertung",
    CalendarCheckIcon,
    "greenDeep",
    "function",
    "planning",
  ),
  calendarPeriods: concept(
    "Kalenderzeiträume",
    CalendarPlusIcon,
    "indigo",
    "function",
    "planning",
  ),
  closingDays: concept(
    "Schließtage",
    CalendarSlashIcon,
    "stone",
    "function",
    "planning",
  ),
  payroll: concept(
    "Abrechnung",
    BriefcaseIcon,
    "neutral",
    "function",
    "planning",
  ),

  messages: concept(
    "Nachrichten",
    ChatsCircleIcon,
    "blue",
    "function",
    "communication",
  ),
  parentMessages: concept(
    "Elternmitteilungen",
    EnvelopeIcon,
    "blue",
    "function",
    "communication",
  ),
  parentConversations: concept(
    "Nachrichten mit Eltern",
    ChatCircleTextIcon,
    "blue",
    "function",
    "communication",
  ),
  news: concept(
    "Elternbriefe",
    EnvelopeOpenIcon,
    "blue",
    "function",
    "communication",
  ),
  announcements: concept(
    "Ankündigungen",
    MegaphoneIcon,
    "amber",
    "function",
    "communication",
  ),
  polls: concept(
    "Umfragen",
    QuestionIcon,
    "purple",
    "function",
    "communication",
  ),
  confirmations: concept(
    "Bestätigungen",
    ListChecksIcon,
    "indigo",
    "function",
    "communication",
  ),
  notes: concept(
    "Notizen",
    NotePencilIcon,
    "coral",
    "function",
    "communication",
  ),
  mealPlan: concept(
    "Essensplan",
    ForkKnifeIcon,
    "greenDeep",
    "function",
    "communication",
  ),

  database: concept(
    "Datenverwaltung",
    DatabaseIcon,
    "neutral",
    "function",
    "administration",
  ),
  enrollments: concept(
    "Anmeldungen",
    UserPlusIcon,
    "greenDeep",
    "function",
    "administration",
  ),
  reports: concept(
    "Berichte",
    PresentationChartIcon,
    "indigo",
    "function",
    "administration",
  ),
  lists: concept(
    "Tageslisten",
    ListBulletsIcon,
    "neutral",
    "function",
    "administration",
  ),
  devices: concept("Geräte", DevicesIcon, "neutral", "function", "system"),
  infoDisplays: concept(
    "Info-Displays",
    MonitorIcon,
    "blue",
    "function",
    "system",
  ),
  accounts: concept(
    "Konten",
    UserCircleGearIcon,
    "neutral",
    "function",
    "system",
  ),
  roles: concept("Rollen", UserGearIcon, "purple", "function", "system"),
  permissions: concept(
    "Berechtigungen",
    LockKeyIcon,
    "purple",
    "function",
    "system",
  ),
  passkeys: concept(
    "Passkeys",
    FingerprintSimpleIcon,
    "purple",
    "function",
    "system",
    "regular",
  ),
  exports: concept(
    "Exporte",
    FileArrowDownIcon,
    "neutral",
    "function",
    "system",
  ),
  gradeTransitions: concept(
    "Jahrgangswechsel",
    TrendUpIcon,
    "gold",
    "function",
    "administration",
  ),
  pickup: concept("Abholung", ClockIcon, "gold", "function", "planning"),
  transport: concept("Fahrdienst", VanIcon, "cyan", "function", "planning"),
  birthdays: concept("Geburtstage", CakeIcon, "magenta", "function", "people"),
  changeHistory: concept(
    "Änderungsverlauf",
    ClockCounterClockwiseIcon,
    "stone",
    "function",
    "administration",
  ),
  // Anfragen-Modul (#2429): eingereichte Wünsche von Eltern und
  // Mitarbeitenden, über die die OGS entscheidet.
  requests: concept("Anfragen", TrayIcon, "blue", "function", "communication"),
  rfid: concept(
    "RFID",
    ContactlessPaymentIcon,
    "neutral",
    "function",
    "system",
  ),
  help: concept("Hilfe", BookOpenIcon, "greenDeep", "function", "system"),
  settings: concept("Einstellungen", GearIcon, "neutral", "function", "system"),
  notifications: concept(
    "Benachrichtigungen",
    BellIcon,
    "neutral",
    "function",
    "system",
  ),
  // Die drei Huellen-Konzepte der Eltern-App. Sie stehen in der Navigation
  // neben den Zielen und muessen deshalb dieselbe Duotone-Sprache sprechen.
  language: concept("Sprache", TranslateIcon, "petrol", "function", "system"),
  logout: concept("Abmelden", SignOutIcon, "neutral", "function", "system"),
  more: concept("Mehr", DotsThreeIcon, "stone", "function", "system"),
} as const satisfies Record<string, MotoConceptDefinition>;

export type MotoConceptKey = keyof typeof MOTO_CONCEPTS;

function concept(
  label: string,
  icon: PhosphorIcon,
  tone: MotoDuotoneTone,
  kind: MotoConceptKind,
  section: MotoConceptSection,
  weight: IconProps["weight"] = "duotone",
): MotoConceptDefinition {
  return { label, icon, tone, kind, section, weight };
}
