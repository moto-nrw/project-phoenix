import { SWR_FAMILIES } from "./shell-keys";
import type { roomService } from "~/lib/api";
import { resourceKey } from "./resource-key";
export type RoomFilters = NonNullable<
  Parameters<typeof roomService.getRooms>[0]
>;
export const roomKeys = {
  family: SWR_FAMILIES.rooms,
  key: (params: RoomFilters = {}) => resourceKey("rooms-list-", params),
  // The room page's room with its history. Outside the list family; the SSE
  // hook and the room sections refresh it via the "room-detail-" prefix.
  detailKey: (id: string) => `room-detail-${encodeURIComponent(id)}`,
} as const;
