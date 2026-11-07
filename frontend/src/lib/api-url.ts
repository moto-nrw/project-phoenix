export function isBrowserContext(): boolean {
  return globalThis.window !== undefined;
}

async function resolveServerApiUrl(backendPath: string): Promise<string> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  return `${getServerApiUrl()}${backendPath}`;
}

export async function resolveApiUrl(
  proxyPath: string,
  backendPath: string = proxyPath,
): Promise<string> {
  if (isBrowserContext()) return proxyPath;
  return resolveServerApiUrl(backendPath);
}
