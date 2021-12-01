package httpapi

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
)

func isSiteAdmin(ctx context.Context, db dbutil.DB) bool {
	user, err := database.Users(db).GetByCurrentAuthUser(ctx)
	if err != nil {
		if errcode.IsNotFound(err) || err == database.ErrNoCurrentUser {
			return false
		}

		log15.Error("precise-code-intel proxy: failed to get up current user", "error", err)
		return false
	}

	return user != nil && user.SiteAdmin
}

var DefaultValidatorByCodeHost = map[string]func(context.Context, url.Values, string) (int, error){
	"github.com": enforceAuthViaGitHub,
}

type AuthValidatorMap = map[string]func(context.Context, url.Values, string) (int, error)

func enforceAuth(ctx context.Context, query url.Values, repoName string, validators AuthValidatorMap) (int, error) {
	for codeHost, validator := range validators {
		if !strings.HasPrefix(repoName, codeHost) {
			continue
		}

		if status, err := validator(ctx, query, repoName); err != nil {
			return status, err
		}

		return 0, nil
	}

	return http.StatusUnprocessableEntity, errors.Errorf("verification not supported for code host - see https://github.com/sourcegraph/sourcegraph/issues/4967")
}
