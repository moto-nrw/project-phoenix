import { auth, signIn } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";

type TokenPair = {
  accessToken: string;
  refreshToken: string;
};

let refreshPromise: Promise<TokenPair | null> | null = null;

/**
 * Refresh the user session on the server by exchanging the refresh token for a new access token.
 * Ensures only a single refresh is in-flight at a time to avoid invalidating rotating refresh tokens.
 */
export async function refreshSessionTokensOnServer(): Promise<TokenPair | null> {
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    try {
      const session = await auth();

      const refreshToken = session?.user?.refreshToken;
      if (!refreshToken) {
        console.warn(
          "Server-side refresh requested without a refresh token available",
        );
        return null;
      }

      const response = await fetch(`${getServerApiUrl()}/auth/refresh`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${refreshToken}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        const errorText = await response.text().catch(() => "");
        console.error(
          `Server-side token refresh failed: ${response.status} ${errorText}`,
        );
        return null;
      }

      const tokens = (await response.json()) as {
        access_token: string;
        refresh_token: string;
      };

      try {
        await signIn("credentials", {
          redirect: false,
          internalRefresh: "true",
          token: tokens.access_token,
          refreshToken: tokens.refresh_token,
        });
      } catch (signInError) {
        console.error(
          "Failed to persist refreshed tokens into session",
          signInError,
        );
        return null;
      }

      return {
        accessToken: tokens.access_token,
        refreshToken: tokens.refresh_token,
      } satisfies TokenPair;
    } catch (error) {
      console.error("Unexpected error during server-side token refresh", error);
      return null;
    }
  })();

  try {
    return await refreshPromise;
  } finally {
    refreshPromise = null;
  }
}
