const storybookProcessEnv = Object.freeze({
  API_URL: "http://storybook-api.localhost",
  AUTH_JWT_EXPIRY: "15m",
  AUTH_JWT_REFRESH_EXPIRY: "168h",
  NEXTAUTH_URL: "http://storybook.localhost:6006",
  NEXTAUTH_SECRET: "storybook-development-secret",
  TENANT_DOMAIN: "localhost",
  NEXT_PUBLIC_API_URL: "http://storybook-api.localhost",
  NEXT_PUBLIC_LOG_LEVEL: "info",
  NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
  NEXT_PUBLIC_POSTHOG_KEY: "",
  NEXT_PUBLIC_POSTHOG_HOST: "",
  NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.localhost:6006",
  NEXT_PUBLIC_PARENTS_HOSTNAME: "parents.localhost:6006",
  NEXT_PUBLIC_SENTRY_DSN: "",
  NEXT_PUBLIC_SENTRY_ENVIRONMENT: "",
});

const storybookPublicEnvDefines = Object.freeze(
  Object.fromEntries(
    Object.entries(storybookProcessEnv)
      .filter(([key]) => key.startsWith("NEXT_PUBLIC_"))
      .map(([key, value]) => [`process.env.${key}`, JSON.stringify(value)]),
  ),
);

const env = Object.freeze({
  ...storybookProcessEnv,
  NODE_ENV: "development",
});

export { env, storybookProcessEnv, storybookPublicEnvDefines };
