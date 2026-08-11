import { createEnv } from "@t3-oss/env-nextjs";
import {
  clientEnvSchema,
  isSkipEnvValidationEnabled,
  serverEnvSchema,
} from "./lib/env-validation.js";

export const env = createEnv({
  /**
   * Specify your server-side environment variables schema here. This way you can ensure the app
   * isn't built with invalid env vars.
   */
  server: serverEnvSchema,

  /**
   * Specify your client-side environment variables schema here. This way you can ensure the app
   * isn't built with invalid env vars. To expose them to the client, prefix them with
   * `NEXT_PUBLIC_`.
   */
  client: clientEnvSchema,

  /**
   * You can't destruct `process.env` as a regular object in the Next.js edge runtimes (e.g.
   * middlewares) or client-side so we need to destruct manually.
   */
  runtimeEnv: {
    API_URL: process.env.API_URL,
    AUTH_JWT_EXPIRY: process.env.AUTH_JWT_EXPIRY,
    AUTH_JWT_REFRESH_EXPIRY: process.env.AUTH_JWT_REFRESH_EXPIRY,
    NODE_ENV: process.env.NODE_ENV,
    NEXTAUTH_URL: process.env.NEXTAUTH_URL,
    NEXTAUTH_SECRET: process.env.NEXTAUTH_SECRET,
    TENANT_DOMAIN: process.env.TENANT_DOMAIN,
    NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL,
    NEXT_PUBLIC_LOG_LEVEL: process.env.NEXT_PUBLIC_LOG_LEVEL,
    NEXT_PUBLIC_TENANT_DOMAIN: process.env.NEXT_PUBLIC_TENANT_DOMAIN,
    NEXT_PUBLIC_POSTHOG_KEY: process.env.NEXT_PUBLIC_POSTHOG_KEY,
    NEXT_PUBLIC_POSTHOG_HOST: process.env.NEXT_PUBLIC_POSTHOG_HOST,
    NEXT_PUBLIC_OPERATOR_HOSTNAME: process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME,
    NEXT_PUBLIC_PARENTS_HOSTNAME: process.env.NEXT_PUBLIC_PARENTS_HOSTNAME,
    NEXT_PUBLIC_SCHOOL_HOSTNAME: process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME,
    NEXT_PUBLIC_SENTRY_DSN: process.env.NEXT_PUBLIC_SENTRY_DSN,
    NEXT_PUBLIC_SENTRY_ENVIRONMENT: process.env.NEXT_PUBLIC_SENTRY_ENVIRONMENT,
  },
  /**
   * Run `build` or `dev` with `SKIP_ENV_VALIDATION` to skip env validation. This is especially
   * useful for Docker builds.
   */
  skipValidation: isSkipEnvValidationEnabled(),
  /**
   * Makes it so that empty strings are treated as undefined. `SOME_VAR: z.string()` and
   * `SOME_VAR=''` will throw an error.
   */
  emptyStringAsUndefined: true,
});
