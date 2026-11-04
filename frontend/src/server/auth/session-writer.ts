import "server-only";
import type { NextAuthResult } from "next-auth";
import type { createResponseAwareAuth } from "./route-handler";

/** Every portal keeps its own Auth.js instance and response context. */
export function createSessionWriter(
  mutate: ReturnType<typeof createResponseAwareAuth>["mutateSession"],
  signIn: NextAuthResult["signIn"],
  provider: string,
) {
  return async (token: string, refreshToken: string): Promise<void> => {
    await mutate(
      () =>
        signIn(provider, {
          redirect: false,
          internalRefresh: "true",
          token,
          refreshToken,
        }),
      // Refused Credentials throw. The callback URL is not an error payload.
      (result) => typeof result === "string",
    );
  };
}
