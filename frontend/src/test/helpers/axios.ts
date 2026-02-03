import type { AxiosResponse } from "axios";

/**
 * Creates a typed AxiosResponse with sensible defaults.
 * Overrides can customize status, statusText, headers, and config.
 */
export function createAxiosResponse<T>(
  data: T,
  overrides?: Partial<Omit<AxiosResponse<T>, "data">>,
): AxiosResponse<T> {
  return {
    data,
    status: 200,
    statusText: "OK",
    headers: {},
    config: {} as AxiosResponse["config"],
    ...overrides,
  };
}
