import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import { shiftTypeService, ShiftTypeApiError } from "./shift-type-api";

const backendShiftType = {
  id: 12,
  name: "Betreuung",
  color: "#83CD2D",
  description: "Zeit am Kind",
  is_active: true,
};

const mappedShiftType = {
  id: "12",
  name: "Betreuung",
  color: "#83CD2D",
  description: "Zeit am Kind",
  isActive: true,
};

describe("shiftTypeService requests", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("maps a list response and hits the correct endpoint", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [backendShiftType] }, { status: 200 }),
    );

    await expect(shiftTypeService.getShiftTypes()).resolves.toEqual([
      mappedShiftType,
    ]);
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shift-types");
  });

  it("maps a nullable list response to an empty list", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: null }, { status: 200 }),
    );

    await expect(shiftTypeService.getShiftTypes()).resolves.toEqual([]);
  });

  it("defaults a missing description to an empty string", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { data: [{ ...backendShiftType, description: undefined }] },
        { status: 200 },
      ),
    );

    await expect(shiftTypeService.getShiftTypes()).resolves.toEqual([
      { ...mappedShiftType, description: "" },
    ]);
  });

  it("serializes the create payload into the backend body", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendShiftType }, { status: 201 }),
    );

    await expect(
      shiftTypeService.createShiftType({
        name: "Betreuung",
        color: "#83CD2D",
        description: "Zeit am Kind",
        isActive: true,
      }),
    ).resolves.toEqual(mappedShiftType);

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shift-types", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "Betreuung",
        color: "#83CD2D",
        description: "Zeit am Kind",
        is_active: true,
      }),
    });
  });

  it("serializes the update payload and targets the id endpoint", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendShiftType }, { status: 200 }),
    );

    await expect(
      shiftTypeService.updateShiftType("12", {
        name: "Betreuung",
        color: "#83CD2D",
        description: "",
        isActive: false,
      }),
    ).resolves.toEqual(mappedShiftType);

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shift-types/12", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "Betreuung",
        color: "#83CD2D",
        description: "",
        is_active: false,
      }),
    });
  });

  it("includes category_ids (as numbers) when the mapping is managed (#1837)", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendShiftType }, { status: 200 }),
    );

    await shiftTypeService.updateShiftType("12", {
      name: "Betreuung",
      color: "#83CD2D",
      description: "",
      isActive: true,
      categoryIds: ["3", "7"],
    });

    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shift-types/12", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name: "Betreuung",
        color: "#83CD2D",
        description: "",
        is_active: true,
        category_ids: [3, 7],
      }),
    });
  });

  it("sends an empty category_ids array to clear the mapping", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendShiftType }, { status: 200 }),
    );

    await shiftTypeService.updateShiftType("12", {
      name: "Betreuung",
      color: "#83CD2D",
      description: "",
      isActive: true,
      categoryIds: [],
    });

    const body = JSON.parse(
      (mockSessionFetch.mock.calls[0]![1] as RequestInit).body as string,
    ) as Record<string, unknown>;
    expect(body.category_ids).toEqual([]);
  });

  it("omits category_ids entirely when categoryIds is undefined", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: backendShiftType }, { status: 201 }),
    );

    await shiftTypeService.createShiftType({
      name: "Betreuung",
      color: "#83CD2D",
      description: "",
      isActive: true,
    });

    const body = JSON.parse(
      (mockSessionFetch.mock.calls[0]![1] as RequestInit).body as string,
    ) as Record<string, unknown>;
    expect(body).not.toHaveProperty("category_ids");
  });

  it("returns cleanly after a successful delete", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(
      shiftTypeService.deleteShiftType("12"),
    ).resolves.toBeUndefined();
    expect(mockSessionFetch).toHaveBeenCalledWith("/api/staff/shift-types/12", {
      method: "DELETE",
    });
  });

  it("seeds the defaults and returns the full list", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [backendShiftType] }, { status: 200 }),
    );

    await expect(shiftTypeService.createDefaults()).resolves.toEqual([
      mappedShiftType,
    ]);
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/staff/shift-types/defaults",
      { method: "POST" },
    );
  });
});

describe("shiftTypeService errors", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("preserves status and backend detail from JSON error bodies", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "name already exists" }), {
        status: 409,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      shiftTypeService.createShiftType({
        name: "Betreuung",
        color: "#83CD2D",
        description: "",
        isActive: true,
      }),
    ).rejects.toMatchObject({ status: 409, detail: "name already exists" });
  });

  it("reads text error bodies for non-JSON responses", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("database timeout", { status: 500 }),
    );

    await expect(shiftTypeService.getShiftTypes()).rejects.toMatchObject({
      status: 500,
      detail: "database timeout",
    });
  });

  it("falls back to the default message when JSON is malformed", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("{", {
        status: 400,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(
      shiftTypeService.updateShiftType("12", {
        name: "Betreuung",
        color: "#83CD2D",
        description: "",
        isActive: true,
      }),
    ).rejects.toMatchObject({
      status: 400,
      detail: "Schichtart konnte nicht gespeichert werden",
    });
  });

  it("falls back when JSON error bodies omit the detail field", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ message: "wrong shape" }), {
        status: 422,
        headers: { "content-type": "application/json" },
      }),
    );

    await expect(shiftTypeService.getShiftTypes()).rejects.toMatchObject({
      status: 422,
      detail: "Schichtarten konnten nicht geladen werden",
    });
  });

  it("uses the delete fallback message on delete failures", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response("", { status: 403 }));

    await expect(shiftTypeService.deleteShiftType("12")).rejects.toMatchObject({
      status: 403,
      detail: "Schichtart konnte nicht gelöscht werden",
    });
  });

  it("surfaces load failures from the defaults endpoint", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("boom", { status: 500 }),
    );

    await expect(shiftTypeService.createDefaults()).rejects.toBeInstanceOf(
      ShiftTypeApiError,
    );
  });

  it("exposes a readable message on the error instance", () => {
    const err = new ShiftTypeApiError(404, "not found");
    expect(err.name).toBe("ShiftTypeApiError");
    expect(err.message).toBe("HTTP 404: not found");
  });
});
