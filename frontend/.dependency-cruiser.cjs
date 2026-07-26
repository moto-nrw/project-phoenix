const SOURCE = "^src";
const TEST_OR_SUPPORT =
  "\\.(test|spec|stories)\\.[cm]?[jt]sx?$|/__tests__/|^src/test/";
const NEXT_APP_ENTRYPOINT =
  "^src/app/.*/(page|layout|template|loading|error|global-error|not-found|global-not-found|route|default)\\.[cm]?[jt]sx?$";
const PUBLIC_ROOT_ENTRYPOINT =
  "^src/(proxy|instrumentation|instrumentation-client|sentry\\.(client|server|edge|shared)\\.config)\\.[cm]?[jt]sx?$|^src/app/manifest\\.ts$";
const PRODUCTION_SOURCE = `${TEST_OR_SUPPORT}|${NEXT_APP_ENTRYPOINT}|${PUBLIC_ROOT_ENTRYPOINT}`;

module.exports = {
  forbidden: [
    {
      name: "no-circular",
      severity: "error",
      from: { path: SOURCE, pathNot: TEST_OR_SUPPORT },
      to: { circular: true },
    },
    {
      name: "not-to-unresolvable",
      severity: "error",
      from: { path: SOURCE, pathNot: TEST_OR_SUPPORT },
      to: { couldNotResolve: true, pathNot: "\\.css$" },
    },
    {
      name: "not-to-non-package-json",
      severity: "error",
      from: { path: SOURCE, pathNot: TEST_OR_SUPPORT },
      to: { dependencyTypes: ["npm-no-pkg"] },
    },
    {
      name: "not-to-dev-dep-from-prod-src",
      severity: "error",
      from: { path: SOURCE, pathNot: PRODUCTION_SOURCE },
      to: { dependencyTypes: ["npm-dev"] },
    },
    {
      name: "no-orphans",
      severity: "error",
      from: { orphan: true, path: SOURCE, pathNot: PRODUCTION_SOURCE },
      to: {},
    },
    {
      name: "lib-not-to-app-or-components",
      severity: "error",
      from: { path: "^src/lib/", pathNot: TEST_OR_SUPPORT },
      to: { path: "^src/(app|components)/" },
    },
    {
      name: "ui-not-to-domain-components-or-app",
      severity: "error",
      from: { path: "^src/components/ui/", pathNot: TEST_OR_SUPPORT },
      to: { path: "^src/(app|components/(?!ui/)|hooks|contexts|server)/" },
    },
    {
      name: "components-not-to-app-or-api-routes",
      severity: "error",
      from: { path: "^src/components/", pathNot: TEST_OR_SUPPORT },
      to: { path: "^src/app/" },
    },
    {
      name: "api-routes-not-to-ui-client-layers",
      severity: "error",
      from: { path: "^src/app/api/", pathNot: TEST_OR_SUPPORT },
      to: { path: "^src/(components|hooks|contexts)/" },
    },
    {
      name: "server-not-to-client-layers",
      severity: "error",
      from: { path: "^src/server/", pathNot: TEST_OR_SUPPORT },
      to: { path: "^src/(components|hooks|contexts)/" },
    },
  ],
  options: {
    tsConfig: { fileName: "tsconfig.json" },
    tsPreCompilationDeps: true,
    doNotFollow: { path: "node_modules" },
    exclude: { path: TEST_OR_SUPPORT },
    enhancedResolveOptions: {
      exportsFields: ["exports"],
      conditionNames: ["import", "require", "node", "default", "types"],
      extensions: [".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json"],
    },
  },
};
