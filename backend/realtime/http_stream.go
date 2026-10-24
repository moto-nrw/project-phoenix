package realtime

// SSEType exposes the existing wire event name to the HTTP stream port.
func (event Event) SSEType() string { return string(event.Type) }

// SSEEvents exposes the same channel used by the hub, without a bridge.
func (client *Client) SSEEvents() <-chan Event { return client.Channel }

// NewSSEClient builds the client selected by the root's HTTP composition.
func NewSSEClient(events chan Event, userID, accountID, tenantID int64, admin bool, groups map[string]bool) *Client {
	return &Client{Channel: events, UserID: userID, AccountID: accountID, TenantID: tenantID, IsAdmin: admin, SubscribedGroups: groups}
}
