/**
 * Diese Route leitet nur auf /planung weiter — es gibt keinen Inhalt, dessen
 * Form ein Skelett nachbilden könnte. Ohne eigene Hülle würde das Skelett der
 * Mitarbeiterliste eine Seite andeuten, die hier nie erscheint (#2828).
 */
export default function DienstplanRedirectLoading() {
  return null;
}
