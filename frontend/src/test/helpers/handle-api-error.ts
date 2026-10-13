import { handleApiError } from "~/lib/api-response-error.server";

/**
 * Stand-in for `handleApiError` in route tests that mock
 * `~/lib/api-helpers.server`: it is the real implementation, loaded from the
 * module the tests do not mock. A backend envelope is forwarded with its
 * code, details and field errors; a frontend error gets its own envelope.
 * The status comes from the thrown `ApiResponseError` or `ApiError`, never
 * from its message.
 *
 * vi.mock("~/lib/api-helpers.server", async () => {
 *   const { mockHandleApiError } = await import("~/test/helpers/handle-api-error");
 *   return { apiGet: mockApiGet, handleApiError: vi.fn(mockHandleApiError) };
 * });
 */
export function mockHandleApiError(
  error: unknown,
  request?: Request,
): Response {
  return handleApiError(error, request);
}
