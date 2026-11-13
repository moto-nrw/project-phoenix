package analytics

import (
	"fmt"
	"net/http"
)

// traegerCatalogCoreActions classifies the writing routes of the Träger
// catalog (#4704, ADR 0195). The catalog belongs to the Träger, not to a
// school, and a Träger session names no school: no school to count it for.
var traegerCatalogCoreActions = map[RouteKey]CoreAction{
	{http.MethodPost, "/traeger/catalog/phases"}:                  notCaptured,
	{http.MethodPut, "/traeger/catalog/phases/{id}"}:              notCaptured,
	{http.MethodPost, "/traeger/catalog/phases/{id}/withdraw"}:    notCaptured,
	{http.MethodPost, "/traeger/catalog/phases/{id}/offerings"}:   notCaptured,
	{http.MethodPut, "/traeger/catalog/offerings/{id}"}:           notCaptured,
	{http.MethodPost, "/traeger/catalog/offerings/{id}/withdraw"}: notCaptured,
	{http.MethodPost, "/traeger/catalog/phases/{id}/preview"}:     notCaptured,
	{http.MethodPost, "/traeger/catalog/phases/{id}/rollout"}:     notCaptured,
}

func init() {
	for key, action := range traegerCatalogCoreActions {
		if _, ok := coreActions[key]; ok {
			panic(fmt.Sprintf("analytics: route %s %s classified twice", key.Method, key.Pattern))
		}
		coreActions[key] = action
	}
}
