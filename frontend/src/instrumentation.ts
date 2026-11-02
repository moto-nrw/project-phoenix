import * as Sentry from "@sentry/nextjs";

export async function register() {
  if (process.env.NEXT_RUNTIME === "nodejs") {
    const { validateServerRuntimeEnv } =
      await import("./lib/server-runtime-env");
    validateServerRuntimeEnv();
    // No response leaves this server with a session cookie that
    // gateSessionCookieWrites did not approve (#5520).
    const { installSessionCookieExit } =
      await import("./server/auth/session-cookie-exit");
    installSessionCookieExit();
    await import("./sentry.server.config");
  }

  if (process.env.NEXT_RUNTIME === "edge") {
    await import("./sentry.edge.config");
  }
}

export const onRequestError = Sentry.captureRequestError;
