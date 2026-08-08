import type { ReactNode } from "react";

/**
 * Renders plain text with http(s) URLs as clickable links — as React elements,
 * never via innerHTML, so nothing in the text can inject markup. Only absolute
 * http/https URLs are linked; trailing sentence punctuation stays outside the
 * link. Whitespace/line breaks are preserved by the caller (e.g. the
 * `whitespace-pre-line` utility on the wrapping element).
 */

const URL_PATTERN = /https?:\/\/[^\s<>"']+/g;
// Punctuation that usually belongs to the surrounding sentence, not the URL.
const TRAILING_PUNCTUATION = /[.,;:!?)\]']+$/;

export function LinkifiedText({
  text,
  linkClassName = "text-moto-blue-strong hover:text-moto-blue-hover font-medium underline underline-offset-2",
}: Readonly<{
  text: string;
  linkClassName?: string;
}>) {
  const nodes: ReactNode[] = [];
  let cursor = 0;
  for (const match of text.matchAll(URL_PATTERN)) {
    const start = match.index;
    let url = match[0];
    const trailing = TRAILING_PUNCTUATION.exec(url);
    if (trailing) url = url.slice(0, -trailing[0].length);
    if (start > cursor) nodes.push(text.slice(cursor, start));
    nodes.push(
      <a
        key={`${start}-${url}`}
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className={linkClassName}
      >
        {url}
      </a>,
    );
    cursor = start + url.length;
  }
  if (cursor < text.length) nodes.push(text.slice(cursor));
  return <>{nodes}</>;
}
