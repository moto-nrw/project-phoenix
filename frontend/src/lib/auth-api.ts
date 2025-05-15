// lib/auth-api.ts
import { signIn, signOut } from "next-auth/react";
import { authService } from "./auth-service";

/**
 * Function to refresh the authentication token
 * @returns Promise with the new tokens or null if refresh failed
 */
export async function refreshToken(): Promise<{
  access_token: string;
  refresh_token: string;
} | null> {
  try {
    const response = await fetch("/api/auth/token", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      credentials: "include", // Important to include cookies
    });

    if (!response.ok) {
      console.error("Token refresh failed:", response.status);
      return null;
    }

    const data = (await response.json()) as {
      access_token: string;
      refresh_token: string;
    };
    return data;
  } catch (error) {
    console.error("Error refreshing token:", error);
    return null;
  }
}

/**
 * Handle a failed authentication by attempting to refresh the token
 * or signing out if that fails
 */
export async function handleAuthFailure(): Promise<boolean> {
  try {
    // Try to refresh the token
    const newTokens = await refreshToken();

    if (newTokens) {
      // Token refresh successful
      console.log("Token refreshed successfully");
      // Force a session reload to update the token in the session
      const result = await signIn("credentials", {
        redirect: false,
        internalRefresh: true, // Special flag to indicate this is just a token refresh
        token: newTokens.access_token,
        refreshToken: newTokens.refresh_token,
      });

      if (result?.error) {
        console.error(
            "Session update failed after token refresh:",
            result.error,
        );
        await signOut({ redirect: false });
        return false;
      }

      console.log("Session updated with new tokens");
      return true;
    }

    // If refresh failed, sign out
    console.log("Token refresh failed, signing out");
    await signOut({ redirect: false });

    // If we're in the browser, redirect to login
    if (typeof window !== "undefined") {
      window.location.href = "/";
    }

    return false;
  } catch (error) {
    console.error("Auth failure handling error:", error);
    await signOut({ redirect: false });
    return false;
  }
}

/**
 * Export the auth service for use throughout the application
 */
export { authService };