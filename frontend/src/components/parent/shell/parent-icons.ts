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
 */
export {
  House,
  Users,
  ChatCircleText,
  CalendarBlank,
  DotsThree,
  Megaphone,
  ForkKnife,
  Check,
  CheckCircle,
  Question,
  Clock,
  FirstAid,
  CaretRight,
  CaretLeft,
  X,
  Bell,
  Translate,
  SignOut,
  // Nicht in der Planliste, aber von der Navigation gebraucht: "Neue
  // Anmeldung" (UserPlus) und "Konto" (UserCircle).
  UserPlus,
  UserCircle,
} from "@phosphor-icons/react";

export type { Icon, IconProps, IconWeight } from "@phosphor-icons/react";
