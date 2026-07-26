/**
 * Tests for CRUD Service Factory
 * Tests service creation and CRUD operations
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  createCrudService,
  createExtendedService,
  getDeleteErrorMessage,
  MalformedCrudListResponseError,
} from "./service-factory";
import type { EntityConfig } from "./types";
import { databaseThemes } from "@/lib/database/themes";

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
    labels: {
      createModalTitle: "Test erstellen",
      editModalTitle: "Test bearbeiten",
    },
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

    it("throws a typed error for malformed list responses", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () => Promise.resolve({ unexpected: true }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);
      const caught = await service.getList().catch((error: unknown) => error);

      expect(caught).toBeInstanceOf(MalformedCrudListResponseError);
      expect((caught as Error).message).toContain(
        "Malformed CRUD list response for Test Entities from /api/test",
      );
      expect(caught).toMatchObject({
        entity: "Test Entities",
        endpoint: "/api/test",
        responseShape: "object keys: unexpected",
      });
    });

    it("throws a typed error when paginated response data is not an array", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        json: () =>
          Promise.resolve({
            data: { id: "1", name: "Not an array" },
            pagination: {
              current_page: 1,
              page_size: 10,
              total_pages: 1,
              total_records: 1,
            },
          }),
        headers: new Headers({
          "content-type": "application/json",
        }),
      });

      const service = createCrudService(mockConfig);

      await expect(service.getList()).rejects.toThrow(
        MalformedCrudListResponseError,
      );
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
      const beforeDelete = vi.fn((_id: string): Promise<boolean> =>
        Promise.resolve(true),
      );
      const afterDelete = vi.fn((_id: string): Promise<void> =>
        Promise.resolve(),
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
      const beforeDelete = vi.fn((_id: string): Promise<boolean> =>
        Promise.resolve(false),
      );

      const configWithHooks: EntityConfig<TestEntity> = {
        ...mockConfig,
        hooks: {
          beforeDelete,
        },
      };

      const service = createCrudService(configWithHooks);

      const result = await service.delete("1");
      expect(result).toBe("Löschen wurde abgebrochen");
      expect(global.fetch).not.toHaveBeenCalled();
    });

    it("returns null on successful delete", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: true,
        status: 204,
        headers: new Headers({ "content-length": "0" }),
      });

      const service = createCrudService(mockConfig);
      const result = await service.delete("1");
      expect(result).toBeNull();
    });

    it("returns error message on 409 Conflict with nested JSON", async () => {
      const backendError = JSON.stringify({
        status: "error",
        error:
          "education: DeleteGroup: Gruppe kann nicht gelöscht werden: Gruppe hat noch zugewiesene Kinder",
      });
      const routeHandlerError = JSON.stringify({
        error: `API error (409): ${backendError}`,
      });

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 409,
        text: () => Promise.resolve(routeHandlerError),
      });

      const service = createCrudService(mockConfig);
      const result = await service.delete("1");
      expect(result).toBe(
        "Gruppe kann nicht gelöscht werden: Gruppe hat noch zugewiesene Kinder",
      );
    });

    it("returns generic German message on 500 server error", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Internal Server Error"),
      });

      const service = createCrudService(mockConfig);
      const result = await service.delete("1");
      expect(result).toBe(
        "Ein unerwarteter Fehler ist aufgetreten. Bitte versuchen Sie es später erneut.",
      );
    });

    it("returns error message on 409 with simple JSON error", async () => {
      const simpleError = JSON.stringify({
        error: "Raum wird noch von Gruppen verwendet",
      });

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 409,
        text: () => Promise.resolve(simpleError),
      });

      const service = createCrudService(mockConfig);
      const result = await service.delete("1");
      expect(result).toBe("Raum wird noch von Gruppen verwendet");
    });
  });

  describe("getDeleteErrorMessage", () => {
    it("extracts message from Error objects", () => {
      expect(getDeleteErrorMessage(new Error("test error"))).toBe("test error");
    });

    it("returns fallback for non-Error objects", () => {
      expect(getDeleteErrorMessage("string error")).toBe(
        "Fehler beim Löschen. Bitte versuchen Sie es erneut.",
      );
    });

    it("returns fallback for null", () => {
      expect(getDeleteErrorMessage(null)).toBe(
        "Fehler beim Löschen. Bitte versuchen Sie es erneut.",
      );
    });
  });

  describe("rejected requests", () => {
    // The companion-plan 409 is the reason the raw body travels: the backend
    // reports WHICH children and weekdays conflict as a top-level `conflicts`
    // array next to `error`. extractErrorMessage keeps only the German
    // sentence, so a caller reading `message` alone can never name what the
    // user confirmed — "Ergänzen und speichern" then re-sends an empty
    // confirmation and earns the identical 409 forever.
    const conflictBody = JSON.stringify({
      error:
        "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
      conflicts: [{ student_id: 42, weekdays: ["mon", "tue"] }],
    });

    it("attaches status and the raw response body to the thrown error", async () => {
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 409,
        text: () => Promise.resolve(conflictBody),
      });

      const service = createCrudService(mockConfig);
      const caught = (await service
        .update("1", { name: "Test" })
        .catch((error: unknown) => error)) as Error & {
        status?: number;
        body?: string;
      };

      expect(caught).toBeInstanceOf(Error);
      expect(caught.status).toBe(409);
      expect(caught.body).toBe(conflictBody);
      // The human sentence stays exactly what it was — the body is additive,
      // existing consumers keep reading `message`.
      expect(caught.message).toBe(
        "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
      );
      // ...and it is precisely why the body is needed: the structured part of
      // the answer never survives into the message.
      expect(caught.message).not.toContain("conflicts");
    });

    it("keeps the raw body when the message is dug out of the route-handler wrapping", async () => {
      // The real chain wraps twice (backend → route handler → here). The
      // message unwraps to the innermost German sentence; the body must stay
      // the untouched outer text so the caller can still parse it.
      const routeHandlerError = JSON.stringify({
        error: `API error (409): ${conflictBody}`,
      });

      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 409,
        text: () => Promise.resolve(routeHandlerError),
      });

      const service = createCrudService(mockConfig);
      const caught = (await service
        .create({ name: "Test" })
        .catch((error: unknown) => error)) as Error & {
        status?: number;
        body?: string;
      };

      expect(caught.status).toBe(409);
      expect(caught.body).toBe(routeHandlerError);
      expect(caught.message).toBe(
        "Der Heimweg des verknüpften Kindes erlaubt diese Tage noch nicht.",
      );
    });

    it("attaches the body of a non-JSON server error too", async () => {
      // No parsing, no guessing: whatever the server said is preserved, so a
      // caller that knows the endpoint can still make sense of it while the
      // generic message stays technical noise.
      (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValue({
        ok: false,
        status: 500,
        text: () => Promise.resolve("Internal Server Error"),
      });

      const service = createCrudService(mockConfig);
      const caught = (await service
        .getList()
        .catch((error: unknown) => error)) as Error & {
        status?: number;
        body?: string;
      };

      expect(caught.status).toBe(500);
      expect(caught.body).toBe("Internal Server Error");
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
      labels: {
        createModalTitle: "Test erstellen",
        editModalTitle: "Test bearbeiten",
      },
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
      labels: {
        createModalTitle: "Test erstellen",
        editModalTitle: "Test bearbeiten",
      },
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
