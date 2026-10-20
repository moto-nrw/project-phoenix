package sftp

import "context"

func PreparePortUpload(ctx context.Context, client *Client, target Target, filename string, data []byte) (func() error, func() error, error) {
	pending, err := client.Prepare(ctx, target, filename, data)
	if err != nil {
		return nil, nil, err
	}
	return pending.Commit, pending.Rollback, nil
}
