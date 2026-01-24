/**
 * Tests for database service factory
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import type { EntityConfig, PaginatedResponse } from "./types";
import { createCrudService, createExtendedService } from "./service-factory";

// Mock global fetch
const mockFetch = vi.fn();
vi.stubGlobal("fetch", mockFetch);

// Mock console methods
const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
const consoleWarnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});

interface TestEntity {
  id: string;
  name: string;
  active: boolean;
}

// Basic test config
const createTestConfig = (
  overrides: Partial<EntityConfig<TestEntity>> = {},
): EntityConfig<TestEntity> => ({
  name: { singular: "Item", plural: "Items" },
  theme: {
    name: "students",
    colors: {
      primary: "#000",
      secondary: "#fff",
      background: "#eee",
      text: "#333",
      border: "#ccc",
    },
    gradients: { header: "linear-gradient(to right, #000, #333)" },
    icons: { main: "/icons/test.svg" },
  },
  api: { basePath: "/api/items" },
  form: { sections: [] },
  detail: { sections: [] },
  list: {
    title: "Items",
    description: "List of items",
    searchPlaceholder: "Search...",
    item: { title: (e) => e.name },
  },
  ...overrides,
});

// Helper to create mock response
const createMockResponse = (
  data: unknown,
  options: {
    ok?: boolean;
    status?: number;
    contentType?: string;
    contentLength?: string | null;
  } = {},
) => {
  const { ok = true, status = 200, contentType = "application/json" } = options;

  return {
    ok,
    status,
    text: () => Promise.resolve(JSON.stringify(data)),
    json: () => Promise.resolve(data),
    headers: {
      get: (name: string) => {
        if (name === "content-type") return contentType;
        if (name === "content-length")
          return options.contentLength !== undefined
            ? options.contentLength
            : "100";
        return null;
      },
    },
  };
};

describe("createCrudService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe("getList", () => {
    it("should fetch list without filters", async () => {
      const paginatedResponse: PaginatedResponse<TestEntity> = {
        data: [{ id: "1", name: "Test", active: true }],
        pagination: {
          current_page: 1,
          page_size: 10,
          total_pages: 1,
          total_records: 1,
        },
      };

      mockFetch.mockResolvedValueOnce(createMockResponse(paginatedResponse));

      const service = createCrudService(createTestConfig());
      const result = await service.getList();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items",
        expect.objectContaining({
          credentials: "include",
        }),
      );
      expect(result.data).toHaveLength(1);
      expect(result.pagination.total_records).toBe(1);
    });

    it("should fetch list with string filters", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({
          data: [],
          pagination: {
            current_page: 1,
            page_size: 10,
            total_pages: 1,
            total_records: 0,
          },
        }),
      );

      const service = createCrudService(createTestConfig());
      await service.getList({ search: "test", status: "active" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items?search=test&status=active",
        expect.anything(),
      );
    });

    it("should handle object filters by JSON stringifying", async () => {
      mockFetch.mockResolvedValueOnce(createMockResponse({ data: [] }));

      const service = createCrudService(createTestConfig());
      await service.getList({ filter: { key: "value" } });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items?filter=%7B%22key%22%3A%22value%22%7D",
        expect.anything(),
      );
    });

    it("should handle boolean filters", async () => {
      mockFetch.mockResolvedValueOnce(createMockResponse({ data: [] }));

      const service = createCrudService(createTestConfig());
      await service.getList({ active: true, archived: false });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items?active=true&archived=false",
        expect.anything(),
      );
    });

    it("should handle number filters", async () => {
      mockFetch.mockResolvedValueOnce(createMockResponse({ data: [] }));

      const service = createCrudService(createTestConfig());
      await service.getList({ page: 1, limit: 20 });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items?page=1&limit=20",
        expect.anything(),
      );
    });

    it("should skip null and undefined filter values", async () => {
      mockFetch.mockResolvedValueOnce(createMockResponse({ data: [] }));

      const service = createCrudService(createTestConfig());
      await service.getList({
        valid: "value",
        nullVal: null,
        undefinedVal: undefined,
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items?valid=value",
        expect.anything(),
      );
    });

    it("should handle direct array response", async () => {
      const items = [
        { id: "1", name: "One", active: true },
        { id: "2", name: "Two", active: false },
      ];
      mockFetch.mockResolvedValueOnce(createMockResponse(items));

      const service = createCrudService(createTestConfig());
      const result = await service.getList();

      expect(result.data).toHaveLength(2);
      expect(result.pagination.current_page).toBe(1);
      expect(result.pagination.total_records).toBe(2);
    });

    it("should handle API wrapper response", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({
          success: true,
          data: {
            data: [{ id: "1", name: "Test", active: true }],
            pagination: {
              current_page: 2,
              page_size: 5,
              total_pages: 4,
              total_records: 20,
            },
          },
        }),
      );

      const service = createCrudService(createTestConfig());
      const result = await service.getList();

      expect(result.pagination.current_page).toBe(2);
      expect(result.pagination.total_records).toBe(20);
    });

    it("should handle wrapped response with data array but no pagination", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({
          data: [{ id: "1", name: "Test", active: true }],
        }),
      );

      const service = createCrudService(createTestConfig());
      const result = await service.getList();

      expect(result.data).toHaveLength(1);
      expect(result.pagination.current_page).toBe(1);
      expect(result.pagination.total_records).toBe(1);
    });

    it("should use mapResponse when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({
          data: [{ id: "1", title: "Test", is_active: true }],
        }),
      );

      const config = createTestConfig({
        service: {
          mapResponse: (data: unknown) => {
            const d = data as { id: string; title: string; is_active: boolean };
            return { id: d.id, name: d.title, active: d.is_active };
          },
        },
      });

      const service = createCrudService(config);
      const result = await service.getList();

      expect(result.data[0].name).toBe("Test");
      expect(result.data[0].active).toBe(true);
    });

    it("should handle unexpected response structure with warning", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ unexpected: "structure" }),
      );

      const service = createCrudService(createTestConfig());
      const result = await service.getList();

      expect(consoleWarnSpy).toHaveBeenCalledWith(
        "Unexpected response structure:",
        expect.anything(),
      );
      expect(result.data).toEqual([]);
    });

    it("should use custom list endpoint when provided", async () => {
      mockFetch.mockResolvedValueOnce(createMockResponse({ data: [] }));

      const config = createTestConfig({
        api: {
          basePath: "/api/items",
          endpoints: { list: "/api/items/all" },
        },
      });

      const service = createCrudService(config);
      await service.getList();

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/all",
        expect.anything(),
      );
    });

    it("should throw on API error", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ error: "Not found" }, { ok: false, status: 404 }),
      );

      const service = createCrudService(createTestConfig());

      await expect(service.getList()).rejects.toThrow("API error: 404");
      expect(consoleErrorSpy).toHaveBeenCalled();
    });
  });

  describe("getOne", () => {
    it("should fetch single item by id", async () => {
      const item = { id: "123", name: "Test Item", active: true };
      mockFetch.mockResolvedValueOnce(createMockResponse(item));

      const service = createCrudService(createTestConfig());
      const result = await service.getOne("123");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/123",
        expect.objectContaining({ credentials: "include" }),
      );
      expect(result.id).toBe("123");
    });

    it("should extract data from wrapped response", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({
          data: { id: "123", name: "Test", active: true },
        }),
      );

      const service = createCrudService(createTestConfig());
      const result = await service.getOne("123");

      expect(result.name).toBe("Test");
    });

    it("should use mapResponse when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", title: "Test", is_active: true }),
      );

      const config = createTestConfig({
        service: {
          mapResponse: (data: unknown) => {
            const d = data as { id: string; title: string; is_active: boolean };
            return { id: d.id, name: d.title, active: d.is_active };
          },
        },
      });

      const service = createCrudService(config);
      const result = await service.getOne("1");

      expect(result.name).toBe("Test");
    });

    it("should use custom getOne method when provided", async () => {
      const customGetOne = vi.fn().mockResolvedValue({
        id: "custom",
        name: "Custom",
        active: true,
      });

      const config = createTestConfig({
        service: {
          customMethods: {
            getOne: customGetOne,
          },
        },
      });

      const service = createCrudService(config);
      const result = await service.getOne("custom");

      expect(customGetOne).toHaveBeenCalledWith("custom");
      expect(result.id).toBe("custom");
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should use custom get endpoint when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const config = createTestConfig({
        api: {
          basePath: "/api/items",
          endpoints: { get: "/api/items/detail/{id}" },
        },
      });

      const service = createCrudService(config);
      await service.getOne("123");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/detail/123",
        expect.anything(),
      );
    });

    it("should throw on error", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ error: "Not found" }, { ok: false, status: 404 }),
      );

      const service = createCrudService(createTestConfig());

      await expect(service.getOne("999")).rejects.toThrow("API error: 404");
    });
  });

  describe("create", () => {
    it("should create item with POST request", async () => {
      const newItem = { name: "New Item", active: true };
      const createdItem = { id: "1", ...newItem };

      mockFetch.mockResolvedValueOnce(createMockResponse(createdItem));

      const service = createCrudService(createTestConfig());
      const result = await service.create(newItem);

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify(newItem),
        }),
      );
      expect(result.id).toBe("1");
    });

    it("should use custom create method when provided", async () => {
      const customCreate = vi.fn().mockResolvedValue({
        id: "custom",
        name: "Custom",
        active: true,
      });

      const config = createTestConfig({
        service: { create: customCreate },
      });

      const service = createCrudService(config);
      const result = await service.create({ name: "Test" });

      expect(customCreate).toHaveBeenCalledWith({ name: "Test" }, undefined);
      expect(result.id).toBe("custom");
    });

    it("should apply beforeCreate hook", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Modified", active: true }),
      );

      const beforeCreate = vi
        .fn()
        .mockResolvedValue({ name: "Modified", active: true });

      const config = createTestConfig({
        hooks: { beforeCreate },
      });

      const service = createCrudService(config);
      await service.create({ name: "Original" });

      expect(beforeCreate).toHaveBeenCalledWith({ name: "Original" });
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items",
        expect.objectContaining({
          body: JSON.stringify({ name: "Modified", active: true }),
        }),
      );
    });

    it("should apply afterCreate hook", async () => {
      const createdItem = { id: "1", name: "Test", active: true };
      mockFetch.mockResolvedValueOnce(createMockResponse(createdItem));

      const afterCreate = vi.fn().mockResolvedValue(undefined);

      const config = createTestConfig({
        hooks: { afterCreate },
      });

      const service = createCrudService(config);
      await service.create({ name: "Test" });

      expect(afterCreate).toHaveBeenCalledWith(createdItem);
    });

    it("should apply afterCreate hook when using custom create", async () => {
      const customCreate = vi.fn().mockResolvedValue({
        id: "1",
        name: "Custom",
        active: true,
      });
      const afterCreate = vi.fn().mockResolvedValue(undefined);

      const config = createTestConfig({
        service: { create: customCreate },
        hooks: { afterCreate },
      });

      const service = createCrudService(config);
      await service.create({ name: "Test" });

      expect(afterCreate).toHaveBeenCalled();
    });

    it("should use mapRequest when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const config = createTestConfig({
        service: {
          mapRequest: (data: Partial<TestEntity>) => ({
            title: data.name,
            is_active: data.active,
          }),
        },
      });

      const service = createCrudService(config);
      await service.create({ name: "Test", active: true });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items",
        expect.objectContaining({
          body: JSON.stringify({ title: "Test", is_active: true }),
        }),
      );
    });

    it("should use mapResponse on the result", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", title: "Created", is_active: false }),
      );

      const config = createTestConfig({
        service: {
          mapResponse: (data: unknown) => {
            const d = data as { id: string; title: string; is_active: boolean };
            return { id: d.id, name: d.title, active: d.is_active };
          },
        },
      });

      const service = createCrudService(config);
      const result = await service.create({ name: "Test" });

      expect(result.name).toBe("Created");
      expect(result.active).toBe(false);
    });

    it("should extract data from wrapped response", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({
          data: { id: "1", name: "Created", active: true },
        }),
      );

      const service = createCrudService(createTestConfig());
      const result = await service.create({ name: "Test" });

      expect(result.name).toBe("Created");
    });

    it("should use custom create endpoint when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const config = createTestConfig({
        api: {
          basePath: "/api/items",
          endpoints: { create: "/api/items/new" },
        },
      });

      const service = createCrudService(config);
      await service.create({ name: "Test" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/new",
        expect.objectContaining({ method: "POST" }),
      );
    });
  });

  describe("update", () => {
    it("should update item with PUT request", async () => {
      const updatedItem = { id: "1", name: "Updated", active: false };
      mockFetch.mockResolvedValueOnce(createMockResponse(updatedItem));

      const service = createCrudService(createTestConfig());
      const result = await service.update("1", { name: "Updated" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/1",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ name: "Updated" }),
        }),
      );
      expect(result.name).toBe("Updated");
    });

    it("should use custom update method when provided", async () => {
      const customUpdate = vi.fn().mockResolvedValue({
        id: "1",
        name: "Custom Updated",
        active: true,
      });

      const config = createTestConfig({
        service: { update: customUpdate },
      });

      const service = createCrudService(config);
      const result = await service.update("1", { name: "Test" });

      expect(customUpdate).toHaveBeenCalledWith(
        "1",
        { name: "Test" },
        undefined,
      );
      expect(result.name).toBe("Custom Updated");
    });

    it("should apply beforeUpdate hook", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Modified", active: true }),
      );

      const beforeUpdate = vi
        .fn()
        .mockResolvedValue({ name: "Modified", active: true });

      const config = createTestConfig({
        hooks: { beforeUpdate },
      });

      const service = createCrudService(config);
      await service.update("1", { name: "Original" });

      expect(beforeUpdate).toHaveBeenCalledWith("1", { name: "Original" });
    });

    it("should apply afterUpdate hook", async () => {
      const updatedItem = { id: "1", name: "Updated", active: true };
      mockFetch.mockResolvedValueOnce(createMockResponse(updatedItem));

      const afterUpdate = vi.fn().mockResolvedValue(undefined);

      const config = createTestConfig({
        hooks: { afterUpdate },
      });

      const service = createCrudService(config);
      await service.update("1", { name: "Updated" });

      expect(afterUpdate).toHaveBeenCalledWith(updatedItem);
    });

    it("should apply afterUpdate hook when using custom update", async () => {
      const customUpdate = vi.fn().mockResolvedValue({
        id: "1",
        name: "Custom",
        active: true,
      });
      const afterUpdate = vi.fn().mockResolvedValue(undefined);

      const config = createTestConfig({
        service: { update: customUpdate },
        hooks: { afterUpdate },
      });

      const service = createCrudService(config);
      await service.update("1", { name: "Test" });

      expect(afterUpdate).toHaveBeenCalled();
    });

    it("should use mapRequest when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const config = createTestConfig({
        service: {
          mapRequest: (data: Partial<TestEntity>) => ({
            title: data.name,
          }),
        },
      });

      const service = createCrudService(config);
      await service.update("1", { name: "Updated" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/1",
        expect.objectContaining({
          body: JSON.stringify({ title: "Updated" }),
        }),
      );
    });

    it("should use custom update endpoint when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const config = createTestConfig({
        api: {
          basePath: "/api/items",
          endpoints: { update: "/api/items/modify/{id}" },
        },
      });

      const service = createCrudService(config);
      await service.update("123", { name: "Test" });

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/modify/123",
        expect.objectContaining({ method: "PUT" }),
      );
    });
  });

  describe("delete", () => {
    it("should delete item with DELETE request", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { status: 204, contentLength: "0" }),
      );

      const service = createCrudService(createTestConfig());
      await service.delete("1");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/1",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("should apply beforeDelete hook and proceed when returns true", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { status: 204, contentLength: "0" }),
      );

      const beforeDelete = vi.fn().mockResolvedValue(true);

      const config = createTestConfig({
        hooks: { beforeDelete },
      });

      const service = createCrudService(config);
      await service.delete("1");

      expect(beforeDelete).toHaveBeenCalledWith("1");
      expect(mockFetch).toHaveBeenCalled();
    });

    it("should cancel delete when beforeDelete returns false", async () => {
      const beforeDelete = vi.fn().mockResolvedValue(false);

      const config = createTestConfig({
        hooks: { beforeDelete },
      });

      const service = createCrudService(config);

      await expect(service.delete("1")).rejects.toThrow(
        "Delete operation cancelled",
      );
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("should apply afterDelete hook", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { status: 204, contentLength: "0" }),
      );

      const afterDelete = vi.fn().mockResolvedValue(undefined);

      const config = createTestConfig({
        hooks: { afterDelete },
      });

      const service = createCrudService(config);
      await service.delete("1");

      expect(afterDelete).toHaveBeenCalledWith("1");
    });

    it("should use custom delete endpoint when provided", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { status: 204, contentLength: "0" }),
      );

      const config = createTestConfig({
        api: {
          basePath: "/api/items",
          endpoints: { delete: "/api/items/remove/{id}" },
        },
      });

      const service = createCrudService(config);
      await service.delete("123");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/items/remove/123",
        expect.objectContaining({ method: "DELETE" }),
      );
    });

    it("should handle empty response from delete", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { status: 204, contentLength: "0" }),
      );

      const service = createCrudService(createTestConfig());
      await expect(service.delete("1")).resolves.toBeUndefined();
    });
  });

  describe("fetchWithAuth helper", () => {
    it("should include credentials and Content-Type header", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const service = createCrudService(createTestConfig());
      await service.getOne("1");

      expect(mockFetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          credentials: "include",
          headers: expect.any(Headers),
        }),
      );

      const callHeaders = mockFetch.mock.calls[0][1].headers as Headers;
      expect(callHeaders.get("Content-Type")).toBe("application/json");
    });

    it("should merge custom headers with defaults", async () => {
      // Test through create which passes custom body
      mockFetch.mockResolvedValueOnce(
        createMockResponse({ id: "1", name: "Test", active: true }),
      );

      const service = createCrudService(createTestConfig());
      await service.create({ name: "Test" });

      const call = mockFetch.mock.calls[0];
      expect(call[1].headers.get("Content-Type")).toBe("application/json");
    });

    it("should handle 204 No Content response", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { status: 204, contentLength: "0" }),
      );

      const service = createCrudService(createTestConfig());
      await service.delete("1");

      // Should not throw
    });

    it("should handle response with no JSON content type", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(null, { contentType: "text/plain" }),
      );

      const service = createCrudService(createTestConfig());
      await service.delete("1");

      // Should handle gracefully
    });

    it("should log API errors with status and body", async () => {
      mockFetch.mockResolvedValueOnce(
        createMockResponse(
          { error: "Bad request" },
          { ok: false, status: 400 },
        ),
      );

      const service = createCrudService(createTestConfig());

      await expect(service.getOne("1")).rejects.toThrow();
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        "API error: 400",
        expect.any(String),
      );
    });
  });
});

describe("createExtendedService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should return base service when no custom methods", async () => {
    mockFetch.mockResolvedValueOnce(
      createMockResponse({
        data: [{ id: "1", name: "Test", active: true }],
        pagination: {
          current_page: 1,
          page_size: 10,
          total_pages: 1,
          total_records: 1,
        },
      }),
    );

    const service = createExtendedService(createTestConfig());
    const result = await service.getList();

    expect(result.data).toHaveLength(1);
  });

  it("should add custom methods to the service", async () => {
    const customMethod = vi.fn().mockResolvedValue({ custom: true });

    const config = createTestConfig({
      service: {
        customMethods: {
          customAction: customMethod,
        },
      },
    });

    const service = createExtendedService(config) as ReturnType<
      typeof createCrudService<TestEntity>
    > & {
      customAction: typeof customMethod;
    };

    const result = await service.customAction("id", { data: "test" });

    expect(customMethod).toHaveBeenCalledWith("id", { data: "test" });
    expect(result).toEqual({ custom: true });
  });

  it("should preserve all CRUD methods when adding custom methods", async () => {
    mockFetch.mockResolvedValue(
      createMockResponse({
        data: [{ id: "1", name: "Test", active: true }],
        pagination: {
          current_page: 1,
          page_size: 10,
          total_pages: 1,
          total_records: 1,
        },
      }),
    );

    const customMethod = vi.fn().mockResolvedValue({ custom: true });

    const config = createTestConfig({
      service: {
        customMethods: {
          customAction: customMethod,
        },
      },
    });

    const service = createExtendedService(config);

    // All CRUD methods should still work
    expect(typeof service.getList).toBe("function");
    expect(typeof service.getOne).toBe("function");
    expect(typeof service.create).toBe("function");
    expect(typeof service.update).toBe("function");
    expect(typeof service.delete).toBe("function");
  });
});
