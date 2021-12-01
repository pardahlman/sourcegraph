package codeintel

import (
	"context"
	"net/http"

	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel/httpapi"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/uploadstore"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

// NewCodeIntelUploadHandler creates a new code intel LSIF upload HTTP handler. This is used
// by both the enterprise frontend codeintel init code to install handlers in the frontend API
// as well as the the enterprise frontend executor init code to install handlers in the proxy.
func NewCodeIntelUploadHandler(
	ctx context.Context,
	conf conftypes.SiteConfigQuerier,
	db database.DB,
	internal bool,
	dbStore *dbstore.Store,
	uploadStore uploadstore.Store,
	operations *httpapi.Operations,
) (http.Handler, error) {
	return httpapi.NewUploadHandler(
		db,
		&httpapi.DBStoreShim{dbStore},
		uploadStore,
		internal,
		httpapi.DefaultValidatorByCodeHost,
		operations,
	), nil
}
