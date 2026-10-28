# Message sources

Edit `<locale>/<namespace>.json`, not the generated locale facades. Each relative
JSON path owns that exact subtree of the message object. Sources are discovered
from the filesystem; there is no hand-maintained namespace/import list.

Error copy has smaller ownership boundaries:

- `<locale>/errorCatalog/classes.json`: shared error-class fallbacks.
- `<locale>/errorCatalog/actions.json`: shared action labels.
- `<locale>/errorCatalog/codes/<domain>.json`: one canonical error-code domain.

A new error code changes only its domain file in each supported language.
Duplicate JSON properties, duplicate mounts and overlapping parent/child source
paths fail validation. Do not combine `parentChild.json` and
`parentChild/consents.json`; split the parent completely before using child files.
Every supported language must still define exactly the same global leaf keys.
The backend locale registry remains authoritative for supported languages.

After adding/removing source files, run:

```sh
pnpm generate:messages
pnpm generate:error-catalog
pnpm run check
```

`<locale>.generated.ts` contains only static JSON imports and object composition.
It retains nested TypeScript inference. Server request configuration loads the
selected locale lazily; German-only clients still pass only their existing
namespace subsets. Do not import the Node-only source reader into runtime code
or replace these static imports with runtime filesystem access/client globbing.
Generated message facades and the runtime error catalog must pass freshness
checks, but remain derivatives, not a second source of translation text.
