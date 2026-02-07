interface OperatorFetchOptions {
  method?: "GET" | "POST" | "PUT" | "DELETE";
  body?: unknown;
}

export class OperatorApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "OperatorApiError";
    this.status = status;
  }
}

export async function operatorFetch<T>(
  endpoint: string,
  options: OperatorFetchOptions = {},
): Promise<T> {
  const { method = "GET", body } = options;

  const response = await fetch(endpoint, {
    method,
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    ...(body !== undefined && { body: JSON.stringify(body) }),
  });

  if (!response.ok) {
    // Expired/invalid token → redirect to login immediately
    if (
      response.status === 401 &&
      typeof window !== "undefined" &&
      !window.location.pathname.startsWith("/operator/login")
    ) {
      window.location.href = "/operator/login";
      throw new OperatorApiError("Session expired", 401);
    }

    let errorMessage = response.statusText;
    try {
      const errorData = (await response.json()) as { error?: string };
      errorMessage = errorData.error ?? errorMessage;
    } catch {
      // use statusText
    }
    throw new OperatorApiError(errorMessage, response.status);
  }

  if (response.status === 204) {
    return {} as T;
  }

  const json: unknown = await response.json();

  // Unwrap proxy response envelope { success, data, message }
  if (
    typeof json === "object" &&
    json !== null &&
    "data" in json &&
    "success" in json
  ) {
    return (json as { data: T }).data;
  }

  return json as T;
}

export function isOperatorApiError(error: unknown): error is OperatorApiError {
  return error instanceof OperatorApiError;
}
