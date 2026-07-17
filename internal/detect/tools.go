package detect

import (
	"context"

	"github.com/render-oss/render-install-wizard/internal/ids"
)

// DetectTools discovers which supported coding agents are installed on the host.
// It is currently a stub.
func DetectTools(ctx context.Context) ([]ids.ToolID, error) {
	_ = ctx
	// TODO(phase 1B): probe the filesystem/PATH for installed coding agents.
	return nil, nil
}
