import { afterEach, describe, expect, it, vi } from "vitest";
import { createCrudService } from "~/lib/database/service-factory";
import type { Student } from "~/lib/student-helpers";
import { studentsConfig } from "./students.config";

// The Kinder database page creates children through this service. The create
// modal's whole Heimweg plan has to reach the BFF: the former legacy-editor
// mapRequest dropped departure_days from the body.
describe("studentsConfig create", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts the create modal's payload unchanged, departure plan included", async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ data: { id: "1" } }), {
        status: 201,
        headers: { "content-type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const payload: Partial<Student> = {
      first_name: "Mia",
      second_name: "Muster",
      school_class: "2b",
      group_id: "5",
      departure_days: { mon: "pickup", tue: "bus" },
      allowed_departure_modes: { mon: ["pickup"], tue: ["bus", "alone"] },
    };

    await createCrudService(studentsConfig).create(payload);

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/students",
      expect.objectContaining({ method: "POST" }),
    );
    const init = fetchMock.mock.calls[0]?.[1] as RequestInit;
    expect(JSON.parse(String(init.body))).toEqual(payload);
  });
});
