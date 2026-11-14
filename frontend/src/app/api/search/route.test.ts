import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
  }),
);

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiPost: mockApiPost,
}));

function searchRequest(body: unknown): NextRequest {
  return new NextRequest(new URL("/api/search", "http://localhost:3000"), {
    method: "POST",
    body: JSON.stringify(body),
    headers: { "Content-Type": "application/json" },
  });
}

describe("POST /api/search", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("sends the query in the body and maps every kind to camelCase", async () => {
    mockApiPost.mockResolvedValueOnce({
      data: {
        children: [
          {
            id: "11",
            first_name: "Mia",
            last_name: "Sonnenberg",
            school_class: "3a",
            group_name: "Füchse",
            match: {
              field: "enrollment_answer",
              label: "Seepferdchen",
              text: "Ja",
            },
          },
        ],
        contacts: [
          {
            id: "21",
            first_name: "Sabine",
            last_name: "Sonnenberg",
            match: { field: "phone", text: "+49 170 1234567" },
            children: [
              {
                id: "11",
                first_name: "Mia",
                last_name: "Sonnenberg",
                school_class: "3a",
                relationship_type: "parent",
              },
            ],
          },
        ],
        staff: [{ id: "31", first_name: "Greta", last_name: "Wiesenthal" }],
        rooms: [{ id: "41", name: "Atelier", building: "Altbau" }],
        groups: [{ id: "51", name: "Füchse" }],
        scope: {
          children: true,
          child_details: true,
          contacts: true,
          staff: false,
          rooms: true,
          groups: true,
        },
      },
    });

    const response = await POST(
      searchRequest({ query: "0170 1234567" }),
      createMockContext(),
    );

    expect(mockApiPost).toHaveBeenCalledWith("/api/search", "test-token", {
      query: "0170 1234567",
    });
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: unknown };
    expect(json.data).toEqual({
      children: [
        {
          id: "11",
          firstName: "Mia",
          lastName: "Sonnenberg",
          schoolClass: "3a",
          groupName: "Füchse",
          match: {
            field: "enrollment_answer",
            label: "Seepferdchen",
            text: "Ja",
          },
        },
      ],
      contacts: [
        {
          id: "21",
          firstName: "Sabine",
          lastName: "Sonnenberg",
          match: { field: "phone", text: "+49 170 1234567" },
          children: [
            {
              id: "11",
              firstName: "Mia",
              lastName: "Sonnenberg",
              schoolClass: "3a",
              relationshipType: "parent",
            },
          ],
        },
      ],
      staff: [{ id: "31", firstName: "Greta", lastName: "Wiesenthal" }],
      rooms: [{ id: "41", name: "Atelier", building: "Altbau" }],
      groups: [{ id: "51", name: "Füchse" }],
      scope: {
        children: true,
        childDetails: true,
        contacts: true,
        staff: false,
        rooms: true,
        groups: true,
      },
    });
  });

  it("grants nothing the answer does not state", async () => {
    mockApiPost.mockResolvedValueOnce({
      data: { children: [], contacts: [], staff: [], rooms: [], groups: [] },
    });
    const response = await POST(
      searchRequest({ query: "Mia" }),
      createMockContext(),
    );
    const json = (await response.json()) as {
      data: { scope: Record<string, boolean> };
    };
    expect(json.data.scope).toEqual({
      children: false,
      childDetails: false,
      contacts: false,
      staff: false,
      rooms: false,
      groups: false,
    });
  });

  it("forwards a missing query as empty text for the backend to refuse", async () => {
    mockApiPost.mockResolvedValueOnce({
      data: { children: [], contacts: [], staff: [], rooms: [], groups: [] },
    });
    await POST(searchRequest({}), createMockContext());
    expect(mockApiPost).toHaveBeenCalledWith("/api/search", "test-token", {
      query: "",
    });
  });
});
