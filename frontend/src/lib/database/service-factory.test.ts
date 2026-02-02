/**
 * Tests for CRUD Service Factory
 * Tests service creation and CRUD operations
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { createCrudService, createExtendedService } from "./service-factory";
import type { EntityConfig } from "./types";
import { databaseThemes } from "@/components/ui/database/themes";

// Mock next-auth
const mockGetSession = vi.fn();
vi.mock("next-auth/react", () => ({
  getSession: (): unknown => mockGetSession(),
}));

// Mock global fetch
global.fetch = vi.fn();

interface TestEntity {
  id: string;
  name: string;
}

describe("createCrudService", () => {
  const mockConfig: EntityConfig<TestEntity> = {
    name: {
      singular: "Test Entity",
      plural: "Test Entities",
    },
    theme: databaseThemes.students,
    api: {
      basePath: "/api/test",
    },
    form: {
      sections: [],
    },
    detail: {
      sections: [],
    },
    list: {
      title: "Test",
      description: "Test description",
      searchPlaceholder: "Search...",
      item: {
        title: () => "Test",
      },
    },
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetSession.mockResolvedValue({
      user: { token: "mock-token" },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("getList", () => {
    it("fetches list of entities with pagination", async () => {
      const mockResponse = {
        data: [
          { id: "1", name: "Test 1" },
          { id: "2", name: "Test 2" },
        ],
        pagination: {
          current_page: 1,
          page_size: 10,
          total_pages: 1,
          total_records: 2,
        },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.getList();

      expect(result.data).toHaveLength(2);
      expect(result.pagination.total_records).toBe(2);
    });

    it("handles wrapped API response", async () => {
      const mockResponse = {
        success: true,
        data: {
          data: [{ id: "1", name: "Test 1" }],
          pagination: {
            current_page: 1,
            page_size: 10,
            total_pages: 1,
            total_records: 1,
          },
        },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.getList();

      expect(result.data).toHaveLength(1);
    });

    it("handles direct array response", async () => {
      const mockResponse = [
        { id: "1", name: "Test 1" },
        { id: "2", name: "Test 2" },
      ];

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.getList();

      expect(result.data).toHaveLength(2);
      expect(result.pagination.total_records).toBe(2);
    });

    it("applies mapResponse to each item", async () => {
      const mockResponse = {
        data: [{ id: 1, name: "Test 1" }],
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const configWithMapper: EntityConfig<TestEntity> = {
        ...mockConfig,
        service: {
          mapResponse: (data: unknown) =>
            ({
              ...(data as Record<string, unknown>),
              id: String((data as { id: number }).id),
            }) as unknown as TestEntity,
        },
      };

      const service = createCrudService(configWithMapper);
      const result = await service.getList();

      expect(result.data[0]?.id).toBe("1");
    });

    it("includes filters in query string", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      await service.getList({ status: "active", page: 1 });

      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining("status=active"),
        expect.any(Object),
      );
    });

    it("handles API errors", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Internal Server Error"),
      });

      const service = createCrudService(mockConfig);

      await expect(service.getList()).rejects.toThrow("API error");
    });
  });

  describe("getOne", () => {
    it("fetches single entity", async () => {
      const mockResponse = {
        data: { id: "1", name: "Test 1" },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.getOne("1");

      expect(result.id).toBe("1");
      expect(result.name).toBe("Test 1");
    });

    it("applies mapResponse", async () => {
      const mockResponse = {
        data: { id: 1, name: "Test 1" },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const configWithMapper: EntityConfig<TestEntity> = {
        ...mockConfig,
        service: {
          mapResponse: (data: unknown) =>
            ({
              ...(data as Record<string, unknown>),
              id: String((data as { id: number }).id),
            }) as unknown as TestEntity,
        },
      };

      const service = createCrudService(configWithMapper);
      const result = await service.getOne("1");

      expect(result.id).toBe("1");
    });

    it("uses custom getOne method if provided", async () => {
      const mockGetOne = vi.fn().mockResolvedValue({ id: "1", name: "Test 1" });

      const configWithCustom: EntityConfig<TestEntity> = {
        ...mockConfig,
        service: {
          customMethods: {
            getOne: mockGetOne,
          },
        },
      };

      const service = createCrudService(configWithCustom);
      const result = await service.getOne("1");

      expect(mockGetOne).toHaveBeenCalledWith("1");
      expect(result.id).toBe("1");
    });
  });

  describe("create", () => {
    it("creates new entity", async () => {
      const mockResponse = {
        data: { id: "1", name: "New Entity" },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.create({ name: "New Entity" });

      expect(result.id).toBe("1");
      expect(result.name).toBe("New Entity");
    });

    it("applies mapRequest before sending", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: "1", name: "Test" } }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const configWithMapper: EntityConfig<TestEntity> = {
        ...mockConfig,
        service: {
          mapRequest: (data: Record<string, unknown>) => ({
            ...data,
            transformed: true,
          }),
        },
      };

      const service = createCrudService(configWithMapper);
      await service.create({ name: "Test" });

      expect(global.fetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          method: "POST",
          body: expect.stringContaining("transformed") as string,
        }),
      );
    });

    it("calls beforeCreate and afterCreate hooks", async () => {
      const beforeCreate = vi.fn((data: Record<string, unknown>) =>
        Promise.resolve(data),
      );
      const afterCreate = vi.fn((): Promise<void> => Promise.resolve());

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: "1", name: "Test" } }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const configWithHooks: EntityConfig<TestEntity> = {
        ...mockConfig,
        hooks: {
          beforeCreate,
          afterCreate,
        },
      };

      const service = createCrudService(configWithHooks);
      await service.create({ name: "Test" });

      expect(beforeCreate).toHaveBeenCalled();
      expect(afterCreate).toHaveBeenCalled();
    });

    it("uses custom create method if provided", async () => {
      const mockCreate = vi.fn().mockResolvedValue({ id: "1", name: "Test" });

      const configWithCustom: EntityConfig<TestEntity> = {
        ...mockConfig,
        service: {
          create: mockCreate,
        },
      };

      const service = createCrudService(configWithCustom);
      await service.create({ name: "Test" });

      expect(mockCreate).toHaveBeenCalled();
    });
  });

  describe("update", () => {
    it("updates existing entity", async () => {
      const mockResponse = {
        data: { id: "1", name: "Updated" },
      };

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve(mockResponse),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.update("1", { name: "Updated" });

      expect(result.id).toBe("1");
      expect(result.name).toBe("Updated");
    });

    it("calls beforeUpdate and afterUpdate hooks", async () => {
      const beforeUpdate = vi.fn((_id: string, data: Record<string, unknown>) =>
        Promise.resolve(data),
      );
      const afterUpdate = vi.fn((): Promise<void> => Promise.resolve());

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: { id: "1", name: "Updated" } }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const configWithHooks: EntityConfig<TestEntity> = {
        ...mockConfig,
        hooks: {
          beforeUpdate,
          afterUpdate,
        },
      };

      const service = createCrudService(configWithHooks);
      await service.update("1", { name: "Updated" });

      expect(beforeUpdate).toHaveBeenCalled();
      expect(afterUpdate).toHaveBeenCalled();
    });

    it("uses custom update method if provided", async () => {
      const mockUpdate = vi
        .fn()
        .mockResolvedValue({ id: "1", name: "Updated" });

      const configWithCustom: EntityConfig<TestEntity> = {
        ...mockConfig,
        service: {
          update: mockUpdate,
        },
      };

      const service = createCrudService(configWithCustom);
      await service.update("1", { name: "Updated" });

      expect(mockUpdate).toHaveBeenCalledWith(
        "1",
        { name: "Updated" },
        "mock-token",
      );
    });
  });

  describe("delete", () => {
    it("deletes entity", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        status: 204,
        headers: new Headers({
          "content-length": "0",
        }),
      });

      const service = createCrudService(mockConfig);
      await service.delete("1");

      expect(global.fetch).toHaveBeenCalledWith(
        expect.stringContaining("/api/test/1"),
        expect.objectContaining({
          method: "DELETE",
        }),
      );
    });

    it("calls beforeDelete and afterDelete hooks", async () => {
      const beforeDelete = vi.fn(
        (_id: string): Promise<boolean> => Promise.resolve(true),
      );
      const afterDelete = vi.fn(
        (_id: string): Promise<void> => Promise.resolve(),
      );

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        status: 204,
        headers: new Headers({
          "content-length": "0",
        }),
      });

      const configWithHooks: EntityConfig<TestEntity> = {
        ...mockConfig,
        hooks: {
          beforeDelete,
          afterDelete,
        },
      };

      const service = createCrudService(configWithHooks);
      await service.delete("1");

      expect(beforeDelete).toHaveBeenCalledWith("1");
      expect(afterDelete).toHaveBeenCalledWith("1");
    });

    it("cancels delete when beforeDelete returns false", async () => {
      const beforeDelete = vi.fn(
        (_id: string): Promise<boolean> => Promise.resolve(false),
      );

      const configWithHooks: EntityConfig<TestEntity> = {
        ...mockConfig,
        hooks: {
          beforeDelete,
        },
      };

      const service = createCrudService(configWithHooks);

      await expect(service.delete("1")).rejects.toThrow("cancelled");
      expect(global.fetch).not.toHaveBeenCalled();
    });
  });

  describe("authentication", () => {
    it("includes auth token in requests", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      await service.getList();

      expect(global.fetch).toHaveBeenCalledWith(
        expect.any(String),
        expect.objectContaining({
          headers: expect.any(Headers) as unknown as Headers,
        }),
      );

      const callArgs = (global.fetch as ReturnType<typeof vi.fn>).mock
        .calls[0] as [string, { headers?: Headers }] | undefined;
      const headers = callArgs?.[1]?.headers;
      expect(headers?.get("Authorization")).toBe("Bearer mock-token");
    });

    it("works without token when not authenticated", async () => {
      mockGetSession.mockResolvedValue(null);

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ data: [] }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      await service.getList();

      expect(global.fetch).toHaveBeenCalled();
    });
  });
});

describe("createExtendedService", () => {
  it("creates service with custom methods", () => {
    const mockConfig: EntityConfig<TestEntity> = {
      name: {
        singular: "Test",
        plural: "Tests",
      },
      theme: databaseThemes.students,
      api: {
        basePath: "/api/test",
      },
      form: {
        sections: [],
      },
      detail: {
        sections: [],
      },
      list: {
        title: "Test",
        description: "Test description",
        searchPlaceholder: "Search...",
        item: {
          title: () => "Test",
        },
      },
      service: {
        customMethods: {
          customMethod: vi.fn(),
        },
      },
    };

    const service = createExtendedService(mockConfig);

    expect(service).toBeDefined();
    expect("customMethod" in service).toBe(true);
  });

  it("returns base service when no custom methods", () => {
    const mockConfig: EntityConfig<TestEntity> = {
      name: {
        singular: "Test",
        plural: "Tests",
      },
      theme: databaseThemes.students,
      api: {
        basePath: "/api/test",
      },
      form: {
        sections: [],
      },
      detail: {
        sections: [],
      },
      list: {
        title: "Test",
        description: "Test description",
        searchPlaceholder: "Search...",
        item: {
          title: () => "Test",
        },
      },
    };

    const service = createExtendedService(mockConfig);

    expect(service).toBeDefined();
    expect(typeof service.getList).toBe("function");
    expect(typeof service.getOne).toBe("function");
    expect(typeof service.create).toBe("function");
    expect(typeof service.update).toBe("function");
    expect(typeof service.delete).toBe("function");
  });
});
