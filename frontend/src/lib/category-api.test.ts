import { beforeEach, describe, expect, it, vi } from "vitest";

const mockSessionFetch = vi.hoisted(() => vi.fn());

vi.mock("./session-cache", () => ({
  sessionFetch: mockSessionFetch,
}));

import { ApiError } from "./api-error";
import { categoryService } from "./category-api";

// The proxy routes already map to the frontend shape, so the client passes
// `data` through untouched.
const category = {
  id: "7",
  name: "Essen",
  description: "Mittagessen",
  color: "#F78C10",
  isSystem: false,
  usageCount: 3,
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-01T10:00:00Z",
};

const payload = {
  name: "Essen",
  description: "Mittagessen",
  color: "#F78C10",
};

describe("categoryService requests", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("always asks for archived and system rows with usage counts", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: [category] }, { status: 200 }),
    );

    await expect(categoryService.getManagedCategories()).resolves.toEqual([
      category,
    ]);
    // Archived rows are what the restore action needs; system rows are shown
    // read-only; the usage counts drive the "wird verwendet" hint. The
    // pickers use the plain list instead, which skips the extra aggregate
    // server-side.
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/activities/categories?include_archived=true&include_system=true&with_usage=true",
    );
  });

  it("maps a nullable list to an empty list", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: null }, { status: 200 }),
    );

    await expect(categoryService.getManagedCategories()).resolves.toEqual([]);
  });

  it("creates a category", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: category }, { status: 200 }),
    );

    await expect(categoryService.createCategory(payload)).resolves.toEqual(
      category,
    );
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/activities/categories",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(payload),
      }),
    );
  });

  it("updates a category", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: category }, { status: 200 }),
    );

    await expect(categoryService.updateCategory("7", payload)).resolves.toEqual(
      category,
    );
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/activities/categories/7",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(payload),
      }),
    );
  });

  it("archives via DELETE and resolves without a payload", async () => {
    mockSessionFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(categoryService.archiveCategory("7")).resolves.toBeUndefined();
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/activities/categories/7",
      { method: "DELETE" },
    );
  });

  it("restores a category", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ data: category }, { status: 200 }),
    );

    await expect(categoryService.restoreCategory("7")).resolves.toEqual(
      category,
    );
    expect(mockSessionFetch).toHaveBeenCalledWith(
      "/api/activities/categories/7/restore",
      { method: "POST" },
    );
  });
});

describe("categoryService error handling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps a backend diagnostic separate from the operation message", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json(
        { detail: "Eine Kategorie mit diesem Namen existiert bereits" },
        { status: 409 },
      ),
    );

    await expect(categoryService.createCategory(payload)).rejects.toMatchObject(
      {
        status: 409,
        message: "Kategorie konnte nicht angelegt werden",
        detail: "Eine Kategorie mit diesem Namen existiert bereits",
      },
    );
  });

  it("classifies a plain text error body by its status", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      new Response("upstream exploded", { status: 500 }),
    );

    await expect(categoryService.getManagedCategories()).rejects.toMatchObject({
      status: 500,
      code: "general.server",
    });
  });

  it("falls back to the operation message when the body is unusable", async () => {
    // Content-Type claims JSON but the body is not parseable — the fallback
    // message is what the user would otherwise never get.
    mockSessionFetch.mockResolvedValueOnce(
      new Response("<html>", {
        status: 502,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(
      categoryService.updateCategory("7", payload),
    ).rejects.toMatchObject({
      status: 502,
      message: "Kategorie konnte nicht gespeichert werden",
    });
  });

  it("reports a failed archive", async () => {
    mockSessionFetch.mockResolvedValueOnce(Response.json({}, { status: 403 }));

    await expect(categoryService.archiveCategory("7")).rejects.toMatchObject({
      status: 403,
      message: "Kategorie konnte nicht archiviert werden",
    });
  });

  it("reports a failed restore", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ detail: "" }, { status: 409 }),
    );

    await expect(categoryService.restoreCategory("7")).rejects.toMatchObject({
      status: 409,
      message: "Kategorie konnte nicht wiederhergestellt werden",
    });
  });

  it("throws an ApiError so callers can narrow", async () => {
    mockSessionFetch.mockResolvedValueOnce(
      Response.json({ detail: "nope" }, { status: 400 }),
    );

    await expect(categoryService.getManagedCategories()).rejects.toBeInstanceOf(
      ApiError,
    );
  });
});
