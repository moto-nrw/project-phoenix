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
 */
export {
  HouseIcon as House,
  UsersIcon as Users,
  ChatCircleTextIcon as ChatCircleText,
  CalendarBlankIcon as CalendarBlank,
  DotsThreeIcon as DotsThree,
  MegaphoneIcon as Megaphone,
  ForkKnifeIcon as ForkKnife,
  CheckIcon as Check,
  CheckCircleIcon as CheckCircle,
  QuestionIcon as Question,
  ClockIcon as Clock,
  FirstAidIcon as FirstAid,
  CaretRightIcon as CaretRight,
  CaretLeftIcon as CaretLeft,
  XIcon as X,
  BellIcon as Bell,
  TranslateIcon as Translate,
  SignOutIcon as SignOut,
  // Nicht in der Planliste, aber von der Navigation gebraucht: "Neue
  // Anmeldung" (UserPlus) und "Konto" (UserCircle).
  UserPlusIcon as UserPlus,
  UserCircleIcon as UserCircle,
} from "@phosphor-icons/react";

export type { Icon, IconProps, IconWeight } from "@phosphor-icons/react";
