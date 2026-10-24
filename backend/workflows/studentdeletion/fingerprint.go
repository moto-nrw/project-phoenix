package studentdeletion

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// fingerprintOf binds a confirmation to the student and person versions and
// the counts the preview showed.
func fingerprintOf(studentUpdatedAt, personUpdatedAt time.Time, counts Counts) (string, error) {
	payload := struct {
		StudentUpdatedAt int64  `json:"student_updated_at"`
		PersonUpdatedAt  int64  `json:"person_updated_at"`
		Counts           Counts `json:"counts"`
	}{StudentUpdatedAt: studentUpdatedAt.UnixNano(), PersonUpdatedAt: personUpdatedAt.UnixNano(), Counts: counts}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func equalFingerprint(actual, expected string) bool {
	actualBytes, actualErr := hex.DecodeString(actual)
	expectedBytes, expectedErr := hex.DecodeString(strings.TrimSpace(expected))
	if actualErr != nil || expectedErr != nil || len(actualBytes) != len(expectedBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(actualBytes, expectedBytes) == 1
}
