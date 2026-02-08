import { auth } from "~/server/auth";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TokenRefresh" });

type TokenPair = {
  accessToken: string;
  refreshToken: string;
};

let refreshPromise: Promise<TokenPair | null> | null = null;

/**
 * Refresh the user session on the server by triggering the JWT callback via auth().
 * The JWT callback handles the actual backend refresh call and token rotation.
 * Ensures only a single auth() call is in-flight at a time to avoid redundant work.
 */
export async function refreshSessionTokensOnServer(): Promise<TokenPair | null> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    try {
      const session = await auth();

      if (!session?.user?.token || !session?.user?.refreshToken) {
        logger.warn("server_side_refresh_no_valid_tokens", {});
        return null;
      }

      return {
        accessToken: session.user.token,
        refreshToken: session.user.refreshToken,
      } satisfies TokenPair;
    } catch (error) {
      logger.error("unexpected_error_during_token_refresh", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    }
  })();

  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}
