import { useMemo } from "react";
import { useSession } from "next-auth/react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import type { TenantPageProps } from "~/components/ui/tenant-page";
import { hasEffectiveAdminScope, isCaregiver } from "~/lib/auth-utils";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { useTenantAwarePath } from "~/lib/tenant-path";
import {
  useChildSearchOnlyNavigation,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";
import {
  ALL_CHILDREN_SCOPE,
  MY_GROUPS_SCOPE,
  activeChildScope,
  buildChildScopeTabs,
  roomIdFromScope,
  type ScopeRoom,
} from "./child-scope-tabs";
import { groupsConcept } from "~/lib/concepts/groups";
import { supervisionConcept } from "~/lib/concepts/supervision";

const EMPTY_IDS: readonly string[] = [];
const EMPTY_ROOMS: readonly ScopeRoom[] = [];
// Die Menüeinträge sind Links; nach dem Schließen des Menüs ist nichts zu tun.
const noop = () => undefined;

interface ChildScopeOptions {
  readonly selectedGroupIds: readonly string[];
  /** Der wirksame Raumfilter: leer an einem anderen Tag als heute. */
  readonly roomId: string;
  readonly isToday: boolean;
  readonly setGroupIds: (groupIds: string[]) => void;
  readonly setRoom: (roomId: string, roomName: string) => void;
  readonly clearRoom: () => void;
}

/**
 * „Meine Gruppe“ und „Aktuelle Aufsicht“ als Teil von „Alle Kinder“, wenn die
 * Schule sie aus der Navigation genommen hat (#1321).
 *
 * Liefert die Reiter für die Kopfkarte und die Menüeinträge, über die
 * Gruppenseite und Aufsichtsseite erreichbar bleiben: Gruppe übergeben,
 * Blöcke beenden oder Kinder in der Anwesenheitsliste abhaken ist keine
 * Suche und bleibt deshalb dort. Ohne die Einstellung liefert der Hook
 * nichts, und die Seite sieht aus wie vorher.
 */
export function useChildScopeTabs({
  selectedGroupIds,
  roomId,
  isToday,
  setGroupIds,
  setRoom,
  clearRoom,
}: ChildScopeOptions): {
  readonly tabs: TenantPageProps["tabs"];
  readonly menuItems: readonly OverflowMenuItem[];
} {
  const enabled = useChildSearchOnlyNavigation();
  const openCareGroupMode = useOpenCareGroupMode();
  const isBinaryMode = usePresenceMode() === "binary";
  const { userContext } = useUserContext();
  const { overviewEnabled, ownSupervisedRooms } = useOptionalSupervision();
  const { data: session } = useSession();
  const tenantPath = useTenantAwarePath();

  // Gruppen gibt es bei offener Betreuung nicht; Räume nur im detaillierten
  // Modus, und der Raumfilter kennt nur den heutigen Tag.
  const ownGroupIds =
    enabled && !openCareGroupMode
      ? (userContext?.educationalGroupIds ?? EMPTY_IDS)
      : EMPTY_IDS;
  const rooms = useMemo<readonly ScopeRoom[]>(
    () =>
      enabled && !isBinaryMode && isToday
        ? (ownSupervisedRooms ?? EMPTY_ROOMS)
        : EMPTY_ROOMS,
    [enabled, isBinaryMode, isToday, ownSupervisedRooms],
  );

  const items = useMemo(
    () => buildChildScopeTabs({ ownGroupIds, rooms }),
    [ownGroupIds, rooms],
  );

  const tabs: TenantPageProps["tabs"] = items
    ? {
        label: "Welche Kinder",
        items,
        value: activeChildScope({ selectedGroupIds, roomId, ownGroupIds }),
        onChange: (scope) => {
          if (scope === ALL_CHILDREN_SCOPE) {
            setGroupIds([]);
            clearRoom();
            return;
          }
          if (scope === MY_GROUPS_SCOPE) {
            clearRoom();
            setGroupIds([...ownGroupIds]);
            return;
          }
          const scopeRoomId = roomIdFromScope(scope);
          const room = rooms.find((candidate) => candidate.id === scopeRoomId);
          if (!room) return;
          setGroupIds([]);
          setRoom(room.id, room.name);
        },
      }
    : undefined;

  // Dieselben Bedingungen wie die früheren Einträge in der Seitenleiste, die
  // auch ohne eigene Gruppe oder laufende Aufsicht dastanden: Betreuungsrolle
  // oder schulweite Übersicht, Gruppen nur mit festen Gruppen, die Aufsicht
  // nicht im binären Modus. Die Aufsicht braucht den Eintrag gerade dann, wenn
  // noch nichts läuft: dort startet man den nächsten geplanten Termin.
  const showStaffEntries = isCaregiver(session) || overviewEnabled;
  const showGroupEntry = showStaffEntries || hasEffectiveAdminScope(session);
  const menuItems: OverflowMenuItem[] = [];
  if (enabled && showGroupEntry && !openCareGroupMode) {
    menuItems.push({
      label: "Zur Gruppenseite",
      icon: <MotoConceptIcon concept={groupsConcept} size={16} />,
      href: tenantPath("/ogs-groups"),
      onClick: noop,
    });
  }
  if (enabled && showStaffEntries && !isBinaryMode) {
    menuItems.push({
      label: "Zur Aufsicht",
      icon: <MotoConceptIcon concept={supervisionConcept} size={16} />,
      href: tenantPath("/active-supervisions"),
      onClick: noop,
    });
  }

  return { tabs, menuItems };
}
