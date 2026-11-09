package analytics

import (
	"fmt"
	"net/http"
)

// traegerDocumentCoreActions classifies the writing routes of the documents
// a Träger shares (#4692) and their deletion (#5993). They belong to the Träger, not to a school, so
// like the other Träger-Portal routes they count for no school yet. init
// merges the table into coreActions.
var traegerDocumentCoreActions = map[RouteKey]CoreAction{
	{http.MethodPost, "/traeger/documents/"}:                            notCaptured,
	{http.MethodPost, "/traeger/documents/{id}/releases"}:               notCaptured,
	{http.MethodDelete, "/traeger/documents/{id}/releases/{releaseId}"}: notCaptured,
	{http.MethodDelete, "/traeger/documents/{id}"}:                      notCaptured,
}

func init() {
	for key, action := range traegerDocumentCoreActions {
		if _, ok := coreActions[key]; ok {
			panic(fmt.Sprintf("analytics: route %s %s classified twice", key.Method, key.Pattern))
		}
		coreActions[key] = action
	}
}
