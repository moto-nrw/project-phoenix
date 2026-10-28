import { roomService } from "~/lib/api";
import { roomKeys, type RoomFilters } from "./rooms-keys";
export const rooms = {
  ...roomKeys,
  fetcher: (params: RoomFilters = {}) =>
    roomService.getRooms(params, { strict: true }),
  detailFetcher: (id: string) => roomService.getRoom(id),
} as const;
