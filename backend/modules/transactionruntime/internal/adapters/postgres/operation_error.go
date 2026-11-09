package postgres

import (
	"errors"
	"strings"
)

// cleanupDiagnostic marks a cleanup failure attached to an operation error,
// such as the tenant runtime's failed savepoint rollback (#6316). It is
// diagnostics: the classifiers below never read it.
type cleanupDiagnostic interface{ CleanupDiagnostic() }

// walkOperation visits err and its wrapped errors in errors.Is order but
// skips attached cleanup diagnostics, so a classification reads the
// operation's own result only. It stops when visit returns true.
func walkOperation(err error, visit func(error) bool) bool {
	if err == nil {
		return false
	}
	if _, diagnostic := err.(cleanupDiagnostic); diagnostic {
		return false
	}
	if visit(err) {
		return true
	}
	switch wrapper := err.(type) {
	case interface{ Unwrap() error }:
		return walkOperation(wrapper.Unwrap(), visit)
	case interface{ Unwrap() []error }:
		for _, wrapped := range wrapper.Unwrap() {
			if walkOperation(wrapped, visit) {
				return true
			}
		}
	}
	return false
}

// operationAs is errors.As over the operation's own result.
func operationAs[T any](err error) (T, bool) {
	var found T
	ok := walkOperation(err, func(node error) bool {
		if typed, match := node.(T); match {
			found = typed
			return true
		}
		if as, custom := node.(interface{ As(any) bool }); custom && as.As(&found) {
			return true
		}
		return false
	})
	return found, ok
}

// operationIs is errors.Is over the operation's own result.
func operationIs(err, target error) bool {
	return walkOperation(err, func(node error) bool {
		if sameError(node, target) {
			return true
		}
		is, custom := node.(interface{ Is(error) bool })
		return custom && is.Is(target)
	})
}

func sameError(a, b error) (same bool) {
	defer func() {
		if recover() != nil {
			same = false // uncomparable dynamic type
		}
	}()
	return a == b
}

// operationText is the text the textual fallbacks read. Without attached
// diagnostics it is err's text. With them (a failed savepoint rollback or
// outer ROLLBACK, #6316) it is the text of the operation's own leaf errors,
// so no cleanup failure classifies the operation by its wording.
func operationText(err error) string {
	var savepoint cleanupDiagnostic
	var rollback *rollbackDiagnostic
	if !errors.As(err, &savepoint) && !errors.As(err, &rollback) {
		return err.Error()
	}
	var leaves []string
	walkOperation(err, func(node error) bool {
		switch node.(type) {
		case interface{ Unwrap() error }, interface{ Unwrap() []error }:
		default:
			leaves = append(leaves, node.Error())
		}
		return false
	})
	return strings.Join(leaves, "\n")
}
