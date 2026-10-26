import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST, PUT } from "./route";
import { ApiResponseError } from "~/lib/api-helpers.server";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiGet, mockApiPost, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
}));

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

function studentListResponse<T>(data: T[]) {
  return {
    data,
    pagination: {
      current_page: 1,
      page_size: 50,
      total_pages: data.length === 0 ? 0 : 1,
      total_records: data.length,
    },
  };
}

describe("GET /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches enrolled students for activity", async () => {
    const mockEnrollments = [
      { id: 1, first_name: "Alice", last_name: "Smith", school_class: "3A" },
      { id: 2, first_name: "Bob", last_name: "Jones", school_class: "3B" },
    ];

    mockApiGet.mockResolvedValueOnce({ data: mockEnrollments });

    const request = createMockRequest("/api/activities/5/students");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/activities/5/students",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; student_id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("1");
    expect(json.data[0]!.student_id).toBe("1");
    expect(json.data[0]!.name).toBe("Alice Smith");
  });

  it("fetches available students when available=true query param", async () => {
    const mockEnrolled = [{ id: 1, first_name: "Alice", last_name: "Smith" }];
    const mockAllStudents = [
      { id: 1, first_name: "Alice", last_name: "Smith", school_class: "3A" },
      { id: 2, first_name: "Bob", last_name: "Jones", school_class: "3B" },
      { id: 3, first_name: "Carol", last_name: "White", school_class: "3C" },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: mockEnrolled })
      .mockResolvedValueOnce(studentListResponse(mockAllStudents));

    const request = createMockRequest(
      "/api/activities/5/students?available=true",
    );
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenCalledTimes(2);
    expect(mockApiGet).toHaveBeenNthCalledWith(
      1,
      "/api/activities/5/students",
      "test-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?",
      "test-token",
    );

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("2");
    expect(json.data[0]!.name).toBe("Bob Jones");
    expect(json.data[1]!.id).toBe("3");
    expect(json.data[1]!.name).toBe("Carol White");
  });

  it("filters available students by search param", async () => {
    const mockEnrolled = [{ id: 1, first_name: "Alice", last_name: "Smith" }];
    const mockAllStudents = [
      { id: 2, first_name: "Bob", last_name: "Jones", school_class: "3B" },
    ];

    mockApiGet
      .mockResolvedValueOnce({ data: mockEnrolled })
      .mockResolvedValueOnce(studentListResponse(mockAllStudents));

    const request = createMockRequest(
      "/api/activities/5/students?available=true&search=Bob",
    );
    await GET(request, createMockContext({ id: "5" }));

    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students?search=Bob",
      "test-token",
    );
  });

  it.each([
    { total: 60, available: 55, pages: 2 },
    { total: 160, available: 155, pages: 4 },
  ])(
    "returns all $available available students out of $total across $pages backend pages",
    async ({ total, available, pages }) => {
      const students = Array.from({ length: total }, (_, index) => ({
        id: index + 1,
        first_name: "Kind",
        last_name: String(index + 1),
        school_class: "3A",
      }));
      const enrolled = [1, 17, 49, 52, total].map((id) => ({ id }));

      mockApiGet.mockImplementation(async (endpoint: string, token: string) => {
        expect(token).toBe("test-token");
        if (endpoint === "/api/activities/5/students") {
          return { data: enrolled };
        }
        const url = new URL(endpoint, "http://localhost");
        expect(url.pathname).toBe("/api/students");
        expect(url.searchParams.get("search")).toBe("Kind");
        expect(url.searchParams.get("group_id")).toBe("7");
        const page = Number(url.searchParams.get("page") ?? 1);
        return {
          data: students.slice((page - 1) * 50, page * 50),
          pagination: {
            current_page: page,
            page_size: 50,
            total_pages: pages,
            total_records: total,
          },
        };
      });

      const response = await GET(
        createMockRequest(
          "/api/activities/5/students?available=true&search=Kind&group_id=7",
        ),
        createMockContext({ id: "5" }),
      );

      expect(response.status).toBe(200);
      const json = await parseJsonResponse<{
        data: Array<{ id: string; name: string; school_class: string }>;
      }>(response);
      expect(json.data).toHaveLength(available);
      expect(mockApiGet).toHaveBeenCalledTimes(pages + 1);
      expect(json.data).toContainEqual({
        id: "59",
        name: "Kind 59",
        school_class: "3A",
      });
      expect(json.data.at(-1)?.id).toBe(String(total - 1));
      for (const id of ["1", "17", "49", "52", String(total)]) {
        expect(json.data.map((student) => student.id)).not.toContain(id);
      }
    },
  );

  it("starts roster and student requests before either response resolves", async () => {
    const enrolled = deferred<{ data: Array<{ id: number }> }>();
    const students = deferred<ReturnType<typeof studentListResponse>>();
    mockApiGet.mockImplementation((endpoint: string) =>
      endpoint === "/api/activities/5/students"
        ? enrolled.promise
        : students.promise,
    );

    const pendingResponse = GET(
      createMockRequest("/api/activities/5/students?available=true"),
      createMockContext({ id: "5" }),
    );

    try {
      await vi.waitFor(() => {
        expect(mockApiGet).toHaveBeenCalledWith(
          "/api/activities/5/students",
          "test-token",
        );
        expect(mockApiGet).toHaveBeenCalledWith("/api/students?", "test-token");
      });
    } finally {
      enrolled.resolve({ data: [{ id: 1 }] });
      students.resolve(
        studentListResponse([
          { id: 1, first_name: "Kind", last_name: "Eins" },
          { id: 2, first_name: "Kind", last_name: "Zwei" },
        ]),
      );
      await pendingResponse;
    }

    const response = await pendingResponse;
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      data: [{ id: "2", name: "Kind Zwei", school_class: "" }],
    });
  });

  it.each([
    { scenario: "empty intermediate page", pages: [[1], [], [3]], maxCalls: 3 },
    { scenario: "repeated page contents", pages: [[1], [1], [3]], maxCalls: 3 },
    {
      scenario: "repeated page number",
      pages: [[1], [2], [3]],
      repeatPage: true,
      maxCalls: 3,
    },
    {
      scenario: "growing total_pages",
      pages: [[1], [2], [3]],
      growPages: true,
      maxCalls: 3,
    },
    {
      scenario: "empty pages with growing total_pages",
      pages: [[1], [], []],
      growPages: true,
      maxCalls: 3,
    },
    {
      scenario: "changed total_records",
      pages: [[1], [2], [3]],
      changeTotal: true,
    },
    { scenario: "missing final student", pages: [[1], [2], []] },
    {
      scenario: "duplicate IDs within one page",
      pages: [[1, 1]],
      pageSize: 2,
      total: 2,
      maxCalls: 2,
    },
    {
      scenario: "missing pagination",
      pages: [[1]],
      omitPagination: true,
      maxCalls: 2,
    },
    {
      scenario: "excessive page count",
      pages: [[1]],
      total: 1001,
      maxCalls: 2,
    },
  ])(
    "rejects $scenario instead of returning a partial or duplicated success",
    async ({
      pages,
      repeatPage,
      growPages,
      changeTotal,
      pageSize = 1,
      total = 3,
      omitPagination,
      maxCalls = 4,
    }) => {
      mockApiGet.mockImplementation(async (endpoint: string) => {
        if (endpoint === "/api/activities/5/students") return { data: [] };
        const page = Number(
          new URL(endpoint, "http://localhost").searchParams.get("page") ?? 1,
        );
        // Bound the broken implementation too, so this regression cannot hang.
        if (page > 5) throw new Error("Pagination did not terminate");
        return {
          data: (pages[page - 1] ?? []).map((id) => ({
            id,
            first_name: "Kind",
            last_name: String(id),
          })),
          ...(omitPagination
            ? {}
            : {
                pagination: {
                  current_page: repeatPage && page === 2 ? 1 : page,
                  page_size: pageSize,
                  total_pages: growPages
                    ? 2 + page
                    : Math.ceil(total / pageSize),
                  total_records: changeTotal && page > 1 ? total + 1 : total,
                },
              }),
        };
      });

      const response = await GET(
        createMockRequest("/api/activities/5/students?available=true"),
        createMockContext({ id: "5" }),
      );

      expect(response.status).toBe(502);
      expect(response.headers.get("Content-Type")).toBe(
        "application/problem+json",
      );
      expect(await response.json()).toMatchObject({
        status: 502,
        code: "general.unavailable",
      });
      expect(mockApiGet.mock.calls.length).toBeLessThanOrEqual(maxCalls);
    },
  );

  it.each(["roster", "first page", "later page"])(
    "forwards a backend failure in the %s without returning a partial list",
    async (failure) => {
      const body = JSON.stringify({
        type: "about:blank",
        title: "Service Unavailable",
        status: 503,
        code: "general.unavailable",
        detail: "Temporarily unavailable",
      });
      mockApiGet.mockImplementation(async (endpoint: string) => {
        const isRoster = endpoint === "/api/activities/5/students";
        const isLaterPage = endpoint.includes("page=2");
        if (
          (failure === "roster" && isRoster) ||
          (failure === "first page" && !isRoster && !isLaterPage) ||
          (failure === "later page" && isLaterPage)
        ) {
          throw new ApiResponseError(503, body, {
            contentType: "application/problem+json",
            retryAfter: "13",
          });
        }
        return {
          data: [
            { id: isLaterPage ? 2 : 1, first_name: "Kind", last_name: "Eins" },
          ],
          pagination: {
            current_page: isLaterPage ? 2 : 1,
            page_size: 1,
            total_pages: 2,
            total_records: 2,
          },
        };
      });

      const response = await GET(
        createMockRequest("/api/activities/5/students?available=true"),
        createMockContext({ id: "5" }),
      );

      expect(response.status).toBe(503);
      expect(response.headers.get("Content-Type")).toBe(
        "application/problem+json",
      );
      expect(response.headers.get("Retry-After")).toBe("13");
      expect(await response.json()).toEqual({
        type: "about:blank",
        title: "Service Unavailable",
        status: 503,
        code: "general.unavailable",
        detail: "Service Unavailable",
      });
    },
  );

  it("returns an empty available list when the backend has no matching students", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] }).mockResolvedValueOnce({
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0,
      },
    });

    const response = await GET(
      createMockRequest("/api/activities/5/students?available=true"),
      createMockContext({ id: "5" }),
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({ data: [] });
    expect(mockApiGet).toHaveBeenCalledTimes(2);
  });

  it("does not turn a failed backend request into an empty enrollment list", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/activities/5/students");
    const response = await GET(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Backend error");
  });
});

describe("POST /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("enrolls single student successfully", async () => {
    const mockUpdatedEnrollments = [
      { id: 10, first_name: "Eve", last_name: "Brown", school_class: "3D" },
    ];
    mockApiPost.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_id: "10" },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/5/enroll/10",
      "test-token",
      {},
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; student_id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]!.id).toBe("10");
    expect(json.data[0]!.student_id).toBe("10");
    expect(json.data[0]!.name).toBe("Eve Brown");
  });

  it("batch updates multiple students", async () => {
    const mockUpdatedEnrollments = [
      { id: 10, first_name: "Eve", last_name: "Brown", school_class: "3D" },
      { id: 11, first_name: "Frank", last_name: "Green", school_class: "3E" },
    ];
    mockApiPut.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: { student_ids: ["10", "11"] },
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/5/students",
      "test-token",
      { student_ids: [10, 11] },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("10");
    expect(json.data[1]!.id).toBe("11");
  });

  it("throws error when neither student_id nor student_ids provided", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "POST",
      body: {},
    });
    const response = await POST(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "must provide student_id or student_ids",
    );
  });
});

describe("PUT /api/activities/[id]/students", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("replaces enrolled students successfully", async () => {
    const mockUpdatedEnrollments = [
      { id: 20, first_name: "Grace", last_name: "Lee", school_class: "3F" },
      { id: 21, first_name: "Henry", last_name: "King", school_class: "3G" },
    ];

    mockApiPut.mockResolvedValueOnce(undefined);
    mockApiGet.mockResolvedValueOnce({ data: mockUpdatedEnrollments });

    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: { student_ids: ["20", "21"] },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/activities/5/students",
      "test-token",
      { student_ids: [20, 21] },
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: Array<{ id: string; student_id: string; name: string }>;
    }>(response);
    expect(json.data).toHaveLength(2);
    expect(json.data[0]!.id).toBe("20");
    expect(json.data[0]!.student_id).toBe("20");
    expect(json.data[0]!.name).toBe("Grace Lee");
  });

  it("throws error when student_ids not provided", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: {},
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("student_ids array is required");
  });

  it("throws error when student_ids is not an array", async () => {
    const request = createMockRequest("/api/activities/5/students", {
      method: "PUT",
      body: { student_ids: "not-an-array" },
    });
    const response = await PUT(request, createMockContext({ id: "5" }));

    expect(response.status).toBe(500);
  });
});
