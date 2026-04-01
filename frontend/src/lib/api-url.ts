import { env } from "~/env";

export function isBrowserContext(): boolean {
  return globalThis.window !== undefined;
}

export function resolveApiUrl(
  proxyPath: string,
  backendPath: string = proxyPath,
): string {
  return isBrowserContext() ? proxyPath : `${env.API_URL}${backendPath}`;
}
