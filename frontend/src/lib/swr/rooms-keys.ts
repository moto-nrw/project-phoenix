import type { roomService } from "~/lib/api";
import { resourceKey } from "./resource-key";
export type RoomFilters = NonNullable<
  Parameters<typeof roomService.getRooms>[0]
>;
export const roomKeys = {
  family: "rooms-",
  key: (params: RoomFilters = {}) => resourceKey("rooms-list-", params),
  detailKey: (id: string) => resourceKey("rooms-detail-", { id }),
} as const;
