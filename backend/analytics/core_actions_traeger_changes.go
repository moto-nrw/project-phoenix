package analytics

import (
	"fmt"
	"net/http"
)

// traegerChangeCoreActions classifies the Träger-Büro's decision of the
// parents' changes of the booked care (#5979). It belongs to the Träger
// Portal, which counts for no school yet, like the other Träger routes;
// the same decision by the school counts as offering_change_decided. init
// merges the table into coreActions.
var traegerChangeCoreActions = map[RouteKey]CoreAction{
	{http.MethodPost, "/traeger/contract-changes/{id}/requests/{requestId}/decide"}: notCaptured,
}

func init() {
	for key, action := range traegerChangeCoreActions {
		if _, ok := coreActions[key]; ok {
			panic(fmt.Sprintf("analytics: route %s %s classified twice", key.Method, key.Pattern))
		}
		coreActions[key] = action
	}
}
