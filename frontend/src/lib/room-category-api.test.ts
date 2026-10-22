import { beforeEach, describe, expect, it, vi } from "vitest";

import { MOTO_COLOR_PALETTE } from "./location-helper";

const sessionFetch = vi.hoisted(() => vi.fn());
vi.mock("./session-cache", () => ({ sessionFetch }));

import {
  roomCategoryOptions,
  roomCategoryService,
  suggestedRoomCategoryColor,
} from "./room-category-api";

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

describe("room category API", () => {
  beforeEach(() => {
    sessionFetch.mockReset();
  });

  it("maps the backend list to string ids and camel case", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({
        data: [{ id: 7, name: "Sport", color: "#827710", room_count: 3 }],
      }),
    );

    await expect(roomCategoryService.list()).resolves.toEqual([
      { id: "7", name: "Sport", color: "#827710", roomCount: 3 },
    ]);
    expect(sessionFetch).toHaveBeenCalledWith("/api/room-categories");
  });

  it("offers the categories by name for the room form", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse({
        data: [{ id: 7, name: "Sport", color: "#827710", room_count: 0 }],
      }),
    );

    await expect(roomCategoryOptions()).resolves.toEqual([
      { value: "Sport", label: "Sport" },
    ]);
  });

  it("raises the catalog error of a refused delete", async () => {
    sessionFetch.mockResolvedValue(
      jsonResponse(
        {
          type: "about:blank",
          title: "Conflict",
          status: 409,
          detail: "room category is still used by rooms",
          code: "rooms.category_in_use",
        },
        409,
      ),
    );

    await expect(roomCategoryService.remove("7")).rejects.toMatchObject({
      code: "rooms.category_in_use",
    });
    expect(sessionFetch).toHaveBeenCalledWith("/api/room-categories/7", {
      method: "DELETE",
    });
  });

  it("suggests the first palette color no category carries", () => {
    const category = (color: string) => ({
      id: color,
      name: color,
      color,
      roomCount: 0,
    });

    expect(suggestedRoomCategoryColor([])).toBe(MOTO_COLOR_PALETTE.wine.base);
    expect(
      suggestedRoomCategoryColor([category(MOTO_COLOR_PALETTE.wine.base)]),
    ).toBe(MOTO_COLOR_PALETTE.gold.base);
  });
});
