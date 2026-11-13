// Ersatz für `src/env.client.ts` in Layouttests (`loadSourceModules`): Sie
// rendern Markup ohne Server und brauchen keine echten Adressen.
/** @public Loaded through the Vite alias in `loadSourceModules`, not imported. */
export const clientEnv = new Proxy({} as Record<string, string>, {
  get: (_target, key) => `layout-test-${String(key)}`,
});
