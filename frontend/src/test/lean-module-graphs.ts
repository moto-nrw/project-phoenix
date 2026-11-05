import type { Plugin } from "vite";
import type { TestModule, TestSpecification } from "vitest/node";

/**
 * Keeps the Vitest main process's module graphs small in one-off runs.
 *
 * Every project has its own Vite server, and each one keeps the transform
 * result of every module it served until the run ends. A result holds the
 * transformed code, its source map and, appended to the code, the same map
 * once more as base64. Vite fills the map's `sourcesContent` with the whole
 * original file, so each module's source is held three times. On the full
 * suite (1547 files, 11/2026) the graphs held about 240 MB of a 400 MB live
 * main heap.
 *
 * 1. `sourcesContent` is dropped before Vite caches a result. Stack traces
 *    are mapped through `mappings` and `sources`, code frames read the file
 *    from disk.
 * 2. Once the last test file of a project has passed, its graph is
 *    invalidated. Projects run one group after another (`groupOrder`), so the
 *    graphs of the earlier groups are free before `app-dom` runs. A project
 *    with a failed file keeps its graph for the error report.
 *
 * Coverage and watch mode keep Vite's default: coverage maps through the
 * cached results, a rerun would transform every module again.
 */
export function leanModuleGraphs(): Plugin {
  return {
    name: "lean-module-graphs",
    configureVitest({ vitest, project }) {
      if (vitest.config.watch || vitest.config.coverage.enabled) return;
      const environments = Object.values(project.vite.environments);

      for (const { moduleGraph } of environments) {
        const update =
          moduleGraph.updateModuleTransformResult.bind(moduleGraph);
        moduleGraph.updateModuleTransformResult = (mod, result) => {
          // Fetching inlines this same map object into the code afterwards.
          if (result?.map && "sourcesContent" in result.map) {
            Reflect.deleteProperty(result.map, "sourcesContent");
          }
          update(mod, result);
        };
      }

      let pending = 0;
      let failed = false;
      // Reporters are created from the resolved config after this hook.
      vitest.config.reporters.push({
        onTestRunStart(specifications: readonly TestSpecification[]) {
          pending = specifications.filter(
            (specification) => specification.project === project,
          ).length;
          failed = false;
        },
        onTestModuleEnd(testModule: TestModule) {
          if (testModule.project !== project) return;
          failed ||= !testModule.ok();
          pending -= 1;
          if (pending > 0 || failed) return;
          for (const { moduleGraph } of environments) {
            moduleGraph.invalidateAll();
          }
        },
      });
    },
  } as Plugin;
}
