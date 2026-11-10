// Ersatz für `src/env.client.ts` in Layouttests (`loadSourceModules`): Sie
// rendern Markup ohne Server und brauchen keine echten Adressen.
export const clientEnv = new Proxy({} as Record<string, string>, {
  get: (_target, key) => `layout-test-${String(key)}`,
});
