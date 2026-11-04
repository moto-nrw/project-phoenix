// Markdown of a legal text (AGB, Datenschutz, Hinweise) in the enrollment
// forms and the Träger's view of a registration. Kept apart from
// enrollment-form-consent.tsx so a page that only shows a text does not load
// the public form client with it.

import ReactMarkdown, { type Components } from "react-markdown";

// Explicit element styling for the legal-document Markdown. Defined at
// module scope (not inline in the `components` prop) so the renderers
// keep a stable identity across re-renders. Tailwind v4 + Preflight
// strips default heading/list styling, and the project has no
// @tailwindcss/typography plugin, so each tag is styled by hand.
// Links open in a new tab; react-markdown does not emit raw HTML by
// default, so authored Markdown stays XSS-safe.
//
// Every renderer strips react-markdown's internal `node` prop before
// spreading the rest onto the DOM element — forwarding `node` produces
// an invalid HTML attribute and a React warning.
const LEGAL_MARKDOWN_COMPONENTS: Components = {
  h1: ({ node: _node, children, ...props }) => (
    <h1 className="mt-4 mb-2 text-lg font-bold text-gray-900" {...props}>
      {children}
    </h1>
  ),
  h2: ({ node: _node, children, ...props }) => (
    <h2 className="mt-4 mb-2 text-base font-bold text-gray-900" {...props}>
      {children}
    </h2>
  ),
  h3: ({ node: _node, children, ...props }) => (
    <h3 className="mt-3 mb-1.5 text-sm font-bold text-gray-900" {...props}>
      {children}
    </h3>
  ),
  p: ({ node: _node, children, ...props }) => (
    <p className="mb-3 leading-6" {...props}>
      {children}
    </p>
  ),
  ul: ({ node: _node, children, ...props }) => (
    <ul className="mb-3 list-disc space-y-1 pl-5" {...props}>
      {children}
    </ul>
  ),
  ol: ({ node: _node, children, ...props }) => (
    <ol className="mb-3 list-decimal space-y-1 pl-5" {...props}>
      {children}
    </ol>
  ),
  li: ({ node: _node, children, ...props }) => (
    <li className="leading-6" {...props}>
      {children}
    </li>
  ),
  a: ({ node: _node, children, ...props }) => (
    <a
      className="text-moto-blue hover:text-moto-blue-hover font-medium underline underline-offset-2"
      target="_blank"
      rel="noopener noreferrer"
      {...props}
    >
      {children}
    </a>
  ),
  strong: ({ node: _node, children, ...props }) => (
    <strong className="font-semibold text-gray-900" {...props}>
      {children}
    </strong>
  ),
  em: ({ node: _node, children, ...props }) => (
    <em className="italic" {...props}>
      {children}
    </em>
  ),
};

export function LegalMarkdown({ text }: { readonly text: string }) {
  return (
    <ReactMarkdown components={LEGAL_MARKDOWN_COMPONENTS}>{text}</ReactMarkdown>
  );
}
