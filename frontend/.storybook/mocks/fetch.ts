interface StorybookFetchRequest {
  input: RequestInfo | URL;
  init?: RequestInit;
  url: string;
  method: string;
}

export type StorybookFetchHandler = (
  request: StorybookFetchRequest,
  originalFetch: typeof fetch,
) => Response | Promise<Response> | undefined;

export function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  const headers = new Headers(init.headers);
  if (!headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  return new Response(JSON.stringify(body), {
    ...init,
    headers,
  });
}

export function installStorybookFetch(
  handler: StorybookFetchHandler,
): () => void {
  const originalFetch = globalThis.fetch;

  globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
    const request = normalizeRequest(input, init);
    const response = handler(request, originalFetch);

    if (response !== undefined) {
      return Promise.resolve(response);
    }

    return originalFetch(input, init);
  }) as typeof fetch;

  return () => {
    globalThis.fetch = originalFetch;
  };
}

function normalizeRequest(
  input: RequestInfo | URL,
  init?: RequestInit,
): StorybookFetchRequest {
  if (input instanceof Request) {
    return {
      input,
      init,
      url: input.url,
      method: init?.method ?? input.method,
    };
  }

  return {
    input,
    init,
    url: input.toString(),
    method: init?.method ?? "GET",
  };
}
