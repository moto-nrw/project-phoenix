// Package authtest provides shared func-field mocks for the
// services/auth interfaces (auth.MFAService, auth.InvitationService),
// replacing the hand-rolled full-interface stubs duplicated across
// api/auth, api/operator, services/platform, and services/import tests.
//
// Each method delegates to its Fn field; a nil field returns zero values.
// Tests configure only the methods they exercise:
//
//	mfa := &authtest.MFAServiceMock{
//		IsRequiredFn: func(ctx context.Context, account *authModels.Account, tenantID int64) (bool, error) {
//			return true, nil
//		},
//	}
//
// Note: services/auth's own internal tests cannot import this package —
// authtest imports services/auth, so that would be an import cycle.
package authtest
