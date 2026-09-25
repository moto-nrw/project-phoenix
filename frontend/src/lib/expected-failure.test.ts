import { describe, expect, it } from "vitest";
import { errorStatus, expectedFailure } from "./expected-failure";

describe("expectedFailure", () => {
  it.each([
    ["Failed to fetch"],
    ["Load failed"],
    ["TypeError: Load failed"],
    ["NetworkError when attempting to fetch resource."],
    ["Error fetching students: Failed to fetch"],
  ])("treats the dropped connection %j as a network failure", (error) => {
    expect(expectedFailure({ error })).toBe("network");
  });

  it("treats 401 and 409 as expected, from the status field or the API error text", () => {
    expect(expectedFailure({ status: 401, error: "unauthorized" })).toBe(
      "unauthorized",
    );
    expect(expectedFailure({ status: 409 })).toBe("conflict");
    expect(
      expectedFailure({
        error:
          'API error (409): {"error":"Raum kann nicht gelöscht werden: Raum wird noch von Gruppen verwendet"}',
      }),
    ).toBe("conflict");
  });

  it.each([
    ["a 403", { status: 403, error: "timetable operation forbidden" }],
    ["a 5xx", { status: 503, error: "API error (503): unavailable" }],
    ["a 5xx with network wording", { status: 500, error: "Failed to fetch" }],
    ["another 4xx", { status: 400, error: "invalid request" }],
    ["an exception", { error: "Cannot read properties of undefined" }],
    [
      "an unreadable response",
      { error: "SyntaxError: The string did not match the expected pattern." },
    ],
    [
      "a message that only starts like one",
      { error: "Failed to fetch profile" },
    ],
    ["no context", undefined],
  ])("keeps %s a defect", (_label, context) => {
    expect(expectedFailure(context)).toBeNull();
  });
});

describe("errorStatus", () => {
  it("reads status and httpStatus, and nothing else", () => {
    expect(errorStatus(Object.assign(new Error("x"), { status: 401 }))).toBe(
      401,
    );
    expect(
      errorStatus(Object.assign(new Error("x"), { httpStatus: 409 })),
    ).toBe(409);
    expect(errorStatus(new Error("x"))).toBeUndefined();
    expect(errorStatus("x")).toBeUndefined();
  });
});
