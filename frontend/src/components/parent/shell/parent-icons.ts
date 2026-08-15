/**
 * Alle Icons der Eltern-App an einer Stelle.
 *
 * Die Eltern-App nutzt Phosphor, weil die moto-Website es nutzt und die
 * Website die einzige Designquelle ist. Personal- und Operator-Portal bleiben
 * bei lucide-react. Gewicht "regular" als Standard, "fill" fuer aktive
 * Zustaende. Kein "duotone".
 *
 * Nur ueber dieses Modul importieren, damit ein Bibliothekswechsel eine
 * Datei betrifft und nicht fuenfzig.
 *
 * Importiert werden die Namen mit Suffix "Icon" (HouseIcon statt House). Die
 * kurzen Namen existieren in 2.1.10 noch, sind dort aber als @deprecated
 * markiert; hier heissen sie kurz, damit der Aufrufcode lesbar bleibt.
 *
 * Die Liste enthaelt nur, was heute gebraucht wird. Icons auf Vorrat lehnt
 * `pnpm knip` als toten Export ab; die spaeteren Etappen tragen ihre hier ein.
 */
export {
  HouseIcon as House,
  UsersIcon as Users,
  ChatCircleTextIcon as ChatCircleText,
  CalendarBlankIcon as CalendarBlank,
  DotsThreeIcon as DotsThree,
  MegaphoneIcon as Megaphone,
  ForkKnifeIcon as ForkKnife,
  BellIcon as Bell,
  TranslateIcon as Translate,
  SignOutIcon as SignOut,
  UserPlusIcon as UserPlus,
  UserCircleIcon as UserCircle,
  CheckCircleIcon as CheckCircle,
  ClockIcon as Clock,
  ProhibitIcon as Prohibit,
  CalendarXIcon as CalendarX,
  QuestionIcon as Question,
  FirstAidIcon as FirstAid,
  CalendarCheckIcon as CalendarCheck,
  CaretRightIcon as CaretRight,
  ListChecksIcon as ListChecks,
  ExportIcon as Export,
  PlusIcon as Plus,
  CircleNotchIcon as CircleNotch,
} from "@phosphor-icons/react";

export type { Icon, IconWeight } from "@phosphor-icons/react";
