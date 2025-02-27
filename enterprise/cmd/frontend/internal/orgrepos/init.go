package orgrepos

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/enterprise"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/orgrepos/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
)

func Init(ctx context.Context, db database.DB, _ conftypes.UnifiedWatchable, _ *oobmigration.Runner, enterpriseServices *enterprise.Services, observationContext *observation.Context) error {
	enterpriseServices.OrgRepositoryResolver = resolvers.NewResolver(db)
	return nil
}
