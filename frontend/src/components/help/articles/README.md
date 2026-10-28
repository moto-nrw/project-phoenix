# Help article sources

Each file exports one named `<name>Topic` factory returning `HelpTopic`. Keep the
article's text and settings-dependent variants together. Import types from
`../help-types`, never from the public facade; shared article helpers live in
`../help-article-shared`. This keeps runtime dependencies acyclic.

For a new article:

1. Register its ID and audience in `lib/help-topics.ts` and
   `lib/help-topic-availability.ts`.
2. Add its factory here and run `pnpm generate:help-index`.
3. Add `articles.<name>Topic(...)` to the appropriate ordered list in
   `../help-content.ts`, after its prerequisites.
4. Run the help behavior suites and `pnpm run check`.

`help-articles.generated.ts` is a deterministic runtime derivative of the
discovered factories. It inlines their exact implementations, avoiding one shipped
module wrapper per article. Named type-only factory imports preserve actual
export reachability and public factory types; runtime code never imports the authored
article modules separately. Regenerate after any article change. It does not
choose product order. The public `help-content.ts` facade still owns role lists,
filtering, groups and order; its public API and existing article IDs remain
stable. Do not alphabetize product lists while regenerating the index.

The help-guide sync rule and German-copy skill continue to apply to actual text
or flow changes. Splitting files does not exempt articles from completeness,
audience, related-link, role/settings and screenshot checks.
