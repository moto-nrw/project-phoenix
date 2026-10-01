package observability

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// Outcomes of TagSentryEnvelope.
var (
	// ErrSentryEnvelopeInvalid rejects a body that is no Sentry envelope.
	ErrSentryEnvelopeInvalid = errors.New("invalid sentry envelope")
	// ErrSentryProjectNotAllowed rejects an envelope addressed to another
	// project than the one it may go to.
	ErrSentryProjectNotAllowed = errors.New("sentry project not allowed")
)

// SentryTarget is the Sentry project a DSN names and the endpoint that takes
// its envelopes.
type SentryTarget struct {
	ProjectID   string
	EnvelopeURL string
}

// ParseSentryDSN reads a DSN of the form
// scheme://public_key@host[:port][/path]/project_id.
func ParseSentryDSN(dsn string) (SentryTarget, error) {
	u, err := url.Parse(strings.TrimSpace(dsn))
	if err != nil {
		return SentryTarget{}, errors.New("sentry DSN is not a URL")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return SentryTarget{}, errors.New("sentry DSN scheme must be https or http")
	}
	if u.Host == "" || u.User == nil || u.User.Username() == "" {
		return SentryTarget{}, errors.New("sentry DSN needs a host and a public key")
	}
	path := strings.TrimSuffix(u.Path, "/")
	slash := strings.LastIndex(path, "/")
	projectID := path[slash+1:]
	if _, err := strconv.ParseUint(projectID, 10, 64); err != nil {
		return SentryTarget{}, errors.New("sentry DSN project ID must be numeric")
	}
	endpoint := url.URL{Scheme: u.Scheme, Host: u.Host, Path: path[:slash] + "/api/" + projectID + "/envelope/"}
	return SentryTarget{ProjectID: projectID, EnvelopeURL: endpoint.String()}, nil
}

// SentryReporter is the device an envelope came from, as its device key
// proved it.
type SentryReporter struct {
	DeviceID string
	SchoolID int64
}

// TagSentryEnvelope checks that the envelope is addressed to projectID and
// writes the reporter's device_id and school_id tags into every event item
// (#3645). The reporter's values replace what the device sent under these
// keys, so a device cannot report as another one. Other items pass
// unchanged.
//
// Envelope format: a header line, then items, each an item header line and a
// payload. A payload is `length` bytes when the item header gives a length,
// otherwise it runs to the next newline.
func TagSentryEnvelope(envelope []byte, projectID string, reporter SentryReporter) ([]byte, error) {
	headerLine, rest, _ := bytes.Cut(envelope, []byte("\n"))
	var header struct {
		DSN string `json:"dsn"`
	}
	if err := json.Unmarshal(headerLine, &header); err != nil {
		return nil, ErrSentryEnvelopeInvalid
	}
	if target, err := ParseSentryDSN(header.DSN); err != nil || target.ProjectID != projectID {
		return nil, ErrSentryProjectNotAllowed
	}

	var out bytes.Buffer
	out.Grow(len(envelope) + 128)
	out.Write(headerLine)
	out.WriteByte('\n')
	for len(rest) > 0 {
		itemHeaderLine, payload, remaining, err := nextSentryItem(rest)
		if err != nil {
			return nil, err
		}
		rest = remaining
		if itemHeaderLine == nil {
			continue
		}
		if isSentryEvent(itemHeaderLine) {
			if payload, err = tagSentryEvent(payload, reporter); err != nil {
				return nil, err
			}
			if itemHeaderLine, err = withSentryItemLength(itemHeaderLine, len(payload)); err != nil {
				return nil, err
			}
		}
		out.Write(itemHeaderLine)
		out.WriteByte('\n')
		out.Write(payload)
		out.WriteByte('\n')
	}
	return out.Bytes(), nil
}

// nextSentryItem splits the next item off rest. A blank line yields a nil
// header.
func nextSentryItem(rest []byte) (itemHeaderLine, payload, remaining []byte, err error) {
	itemHeaderLine, after, _ := bytes.Cut(rest, []byte("\n"))
	if len(bytes.TrimSpace(itemHeaderLine)) == 0 {
		return nil, nil, after, nil
	}
	var itemHeader struct {
		Length *int `json:"length"`
	}
	if err := json.Unmarshal(itemHeaderLine, &itemHeader); err != nil {
		return nil, nil, nil, ErrSentryEnvelopeInvalid
	}
	if itemHeader.Length == nil {
		payload, remaining, _ = bytes.Cut(after, []byte("\n"))
		return itemHeaderLine, payload, remaining, nil
	}
	length := *itemHeader.Length
	if length < 0 || length > len(after) {
		return nil, nil, nil, ErrSentryEnvelopeInvalid
	}
	return itemHeaderLine, after[:length], bytes.TrimPrefix(after[length:], []byte("\n")), nil
}

func isSentryEvent(itemHeaderLine []byte) bool {
	var itemHeader struct {
		Type string `json:"type"`
	}
	return json.Unmarshal(itemHeaderLine, &itemHeader) == nil && itemHeader.Type == "event"
}

// tagSentryEvent sets the device_id and school_id tags of an event payload.
func tagSentryEvent(payload []byte, reporter SentryReporter) ([]byte, error) {
	event, err := decodeSentryObject(payload)
	if err != nil {
		return nil, err
	}
	deviceID, schoolID := reporter.DeviceID, strconv.FormatInt(reporter.SchoolID, 10)
	switch tags := event["tags"].(type) {
	case map[string]any:
		tags["device_id"] = deviceID
		tags["school_id"] = schoolID
	case []any:
		// Sentry also accepts tags as a list of [key, value] pairs.
		kept := make([]any, 0, len(tags)+2)
		for _, pair := range tags {
			if p, ok := pair.([]any); ok && len(p) > 0 && (p[0] == "device_id" || p[0] == "school_id") {
				continue
			}
			kept = append(kept, pair)
		}
		event["tags"] = append(kept, []any{"device_id", deviceID}, []any{"school_id", schoolID})
	default:
		event["tags"] = map[string]any{"device_id": deviceID, "school_id": schoolID}
	}
	return json.Marshal(event)
}

// withSentryItemLength rewrites an item header with the new payload length.
func withSentryItemLength(itemHeaderLine []byte, length int) ([]byte, error) {
	itemHeader, err := decodeSentryObject(itemHeaderLine)
	if err != nil {
		return nil, err
	}
	itemHeader["length"] = length
	return json.Marshal(itemHeader)
}

// decodeSentryObject decodes a JSON object and keeps its numbers as written.
func decodeSentryObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, ErrSentryEnvelopeInvalid
	}
	return object, nil
}
