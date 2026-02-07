import { cookies } from "next/headers";

const OPERATOR_TOKEN_COOKIE = "phoenix-operator-token";
const OPERATOR_REFRESH_COOKIE = "phoenix-operator-refresh";

const COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === "production",
  sameSite: "lax" as const,
  path: "/",
};

export async function setOperatorTokens(
  accessToken: string,
  refreshToken: string,
): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.set(OPERATOR_TOKEN_COOKIE, accessToken, {
    ...COOKIE_OPTIONS,
    maxAge: 15 * 60, // 15 minutes
  });
  cookieStore.set(OPERATOR_REFRESH_COOKIE, refreshToken, {
    ...COOKIE_OPTIONS,
    maxAge: 60 * 60, // 1 hour
  });
}

export async function clearOperatorTokens(): Promise<void> {
  const cookieStore = await cookies();
  cookieStore.delete(OPERATOR_TOKEN_COOKIE);
  cookieStore.delete(OPERATOR_REFRESH_COOKIE);
}

export async function getOperatorToken(): Promise<string | undefined> {
  const cookieStore = await cookies();
  return cookieStore.get(OPERATOR_TOKEN_COOKIE)?.value;
}

export async function getOperatorRefreshToken(): Promise<string | undefined> {
  const cookieStore = await cookies();
  return cookieStore.get(OPERATOR_REFRESH_COOKIE)?.value;
}
