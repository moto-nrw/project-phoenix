package students

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Refuse retired fields even when empty or null, rather than acknowledging a
// contact update that was not applied. Other request fields keep their contract.
func rejectRetiredStudentContacts(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	for key := range fields {
		switch strings.ToLower(key) {
		case "guardian_name", "guardian_contact", "guardian_email", "guardian_phone":
			return fmt.Errorf("%s is no longer supported on students; use guardian contacts", key)
		}
	}
	return nil
}

func (req *StudentRequest) UnmarshalJSON(data []byte) error {
	if err := rejectRetiredStudentContacts(data); err != nil {
		return err
	}
	type request StudentRequest
	return json.Unmarshal(data, (*request)(req))
}

func (req *UpdateStudentRequest) UnmarshalJSON(data []byte) error {
	if err := rejectRetiredStudentContacts(data); err != nil {
		return err
	}
	type request UpdateStudentRequest
	return json.Unmarshal(data, (*request)(req))
}
