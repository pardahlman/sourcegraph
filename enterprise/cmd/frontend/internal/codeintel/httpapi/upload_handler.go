package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/cockroachdb/errors"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	store "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/uploadstore"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

type UploadHandler struct {
	db          dbutil.DB
	dbStore     DBStore
	uploadStore uploadstore.Store
	validators  AuthValidatorMap
	internal    bool
	operations  *Operations
}

func NewUploadHandler(
	db dbutil.DB,
	dbStore DBStore,
	uploadStore uploadstore.Store,
	internal bool,
	authValidators AuthValidatorMap,
	operations *Operations,
) http.Handler {
	handler := &UploadHandler{
		db:          db,
		dbStore:     dbStore,
		uploadStore: uploadStore,
		internal:    internal,
		validators:  authValidators,
		operations:  operations,
	}

	return http.HandlerFunc(handler.handleEnqueue)
}

var revhashPattern = lazyregexp.New(`^[a-z0-9]{40}$`)

// POST /upload
func (h *UploadHandler) handleEnqueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query()
	body := r.Body

	uploadArgs := uploadArgs{
		uploadID:          getQueryInt(r, "uploadId"),
		repository:        getQuery(r, "repository"),
		commit:            getQuery(r, "commit"),
		root:              sanitizeRoot(getQuery(r, "root")),
		indexer:           getQuery(r, "indexerName"),
		associatedIndexID: getQueryInt(r, "associatedIndexId"),
		suppliedIndex:     hasQuery(r, "index"),
		index:             getQueryInt(r, "index"),
		multipart:         hasQuery(r, "multiPart"),
		numParts:          getQueryInt(r, "numParts"),
		done:              hasQuery(r, "done"),
	}

	validation := func() (int, error) {
		if uploadArgs.commit != "" && !revhashPattern.Match([]byte(uploadArgs.commit)) {
			return http.StatusBadRequest, errors.Errorf("Commit must be a 40-character revhash")
		}

		if uploadArgs.uploadID == 0 {
			// 🚨 SECURITY: Ensure we return before proxying to the precise-code-intel-api-server upload
			// endpoint. This endpoint is unprotected, so we need to make sure the user provides a valid
			// token proving contributor access to the repository.
			if !h.internal && conf.Get().LsifEnforceAuth && !isSiteAdmin(ctx, h.db) {
				if statusCode, err := enforceAuth(ctx, query, uploadArgs.repository, h.validators); err != nil {
					return statusCode, err
				}
			}
		}

		//
		// TODO - why is this condition necessary?
		//

		if uploadArgs.uploadID == 0 {
			if repo, statusCode, err := ensureRepoAndCommitExist(ctx, database.NewDB(h.db), uploadArgs.repository, uploadArgs.commit); err != nil {
				return statusCode, err
			} else {
				uploadArgs.repositoryID = int(repo.ID)
			}
		}

		return 0, nil
	}

	if statusCode, err := validation(); err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	payload, err := h.handleEnqueueErr(ctx, w, uploadArgs, body)
	if err != nil {
		var e *ClientError
		if errors.As(err, &e) {
			http.Error(w, e.Error(), http.StatusBadRequest)
			return
		}

		log15.Error("Failed to enqueue payload", "error", err)
		http.Error(w, fmt.Sprintf("failed to enqueue payload: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	if payload != nil {
		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, payload)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

type uploadArgs struct {
	uploadID          int
	repository        string
	repositoryID      int
	commit            string
	root              string
	indexer           string
	associatedIndexID int
	suppliedIndex     bool
	multipart         bool
	numParts          int
	index             int
	done              bool
}

type enqueuePayload struct {
	ID string `json:"id"`
}

// handleEnqueueErr dispatches to the correct handler function based on query args. Running the
// `src lsif upload` command will cause one of two sequences of requests to occur. For uploads that
// are small enough repos (that can be uploaded in one-shot), only one request will be made:
//
//    - POST `/upload?repositoryId,commit,root,indexerName`
//
// For larger uploads, the requests are broken up into a setup request, a serires of upload requests,
// and a finalization request:
//
//   - POST `/upload?repositoryId,commit,root,indexerName,multiPart=true,numParts={n}`
//   - POST `/upload?uploadId={id},index={i}`
//   - POST `/upload?uploadId={id},done=true`
//
// See the functions the following functions for details on how each request is handled:
//
//   - handleEnqueueSinglePayload
//   - handleEnqueueMultipartSetup
//   - handleEnqueueMultipartUpload
//   - handleEnqueueMultipartFinalize
func (h *UploadHandler) handleEnqueueErr(ctx context.Context, w http.ResponseWriter, uploadArgs uploadArgs, payload io.Reader) (interface{}, error) {
	if !uploadArgs.multipart && uploadArgs.uploadID == 0 {
		return h.handleEnqueueSinglePayload(ctx, payload, uploadArgs)
	}

	if uploadArgs.multipart {
		return h.handleEnqueueMultipartSetup(ctx, uploadArgs)
	}

	if uploadArgs.uploadID == 0 {
		return nil, clientError("no uploadId supplied")
	}

	upload, exists, err := h.dbStore.GetUploadByID(ctx, uploadArgs.uploadID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, clientError("upload not found")
	}

	if uploadArgs.suppliedIndex {
		return h.handleEnqueueMultipartUpload(ctx, payload, upload, uploadArgs)
	}

	if uploadArgs.done {
		return h.handleEnqueueMultipartFinalize(ctx, upload)
	}

	return nil, clientError("no index supplied")
}

// handleEnqueueSinglePayload handles a non-multipart upload. This creates an upload record
// with state 'queued', proxies the data to the bundle manager, and returns the generated ID.
func (h *UploadHandler) handleEnqueueSinglePayload(ctx context.Context, payload io.Reader, uploadArgs uploadArgs) (interface{}, error) {
	tx, err := h.dbStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	id, err := tx.InsertUpload(ctx, store.Upload{
		Commit:            uploadArgs.commit,
		Root:              uploadArgs.root,
		RepositoryID:      uploadArgs.repositoryID,
		Indexer:           uploadArgs.indexer,
		AssociatedIndexID: &uploadArgs.associatedIndexID,
		State:             "uploading",
		NumParts:          1,
		UploadedParts:     []int{0},
	})
	if err != nil {
		return nil, err
	}

	size, err := h.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.lsif.gz", id), payload)
	if err != nil {
		return nil, err
	}

	if err := tx.MarkQueued(ctx, id, &size); err != nil {
		return nil, err
	}

	log15.Info(
		"Enqueued upload",
		"id", id,
		"repository_id", uploadArgs.repositoryID,
		"commit", uploadArgs.commit,
	)

	// older versions of src-cli expect a string
	return enqueuePayload{strconv.Itoa(id)}, nil
}

// handleEnqueueMultipartSetup handles the first request in a multipart upload. This creates a
// new upload record with state 'uploading' and returns the generated ID to be used in subsequent
// requests for the same upload.
func (h *UploadHandler) handleEnqueueMultipartSetup(ctx context.Context, uploadArgs uploadArgs) (interface{}, error) {
	if uploadArgs.numParts <= 0 {
		return nil, clientError("illegal number of parts: %d", uploadArgs.numParts)
	}

	id, err := h.dbStore.InsertUpload(ctx, store.Upload{
		Commit:            uploadArgs.commit,
		Root:              uploadArgs.root,
		RepositoryID:      uploadArgs.repositoryID,
		Indexer:           uploadArgs.indexer,
		AssociatedIndexID: &uploadArgs.associatedIndexID,
		State:             "uploading",
		NumParts:          uploadArgs.numParts,
		UploadedParts:     nil,
	})
	if err != nil {
		return nil, err
	}

	log15.Info(
		"Enqueued upload",
		"id", id,
		"repository_id", uploadArgs.repositoryID,
		"commit", uploadArgs.commit,
	)

	// older versions of src-cli expect a string
	return enqueuePayload{strconv.Itoa(id)}, nil
}

// handleEnqueueMultipartUpload handles a partial upload in a multipart upload. This proxies the
// data to the bundle manager and marks the part index in the upload record.
func (h *UploadHandler) handleEnqueueMultipartUpload(ctx context.Context, payload io.Reader, upload store.Upload, uploadArgs uploadArgs) (interface{}, error) {
	if uploadArgs.index < 0 || uploadArgs.index >= upload.NumParts {
		return nil, clientError("illegal part index: index %d is outside the range [0, %d)", uploadArgs.index, upload.NumParts)
	}

	if _, err := h.uploadStore.Upload(ctx, fmt.Sprintf("upload-%d.%d.lsif.gz", upload.ID, uploadArgs.index), payload); err != nil {
		h.markUploadAsFailed(context.Background(), h.dbStore, upload.ID, err)
		return nil, err
	}

	if err := h.dbStore.AddUploadPart(ctx, upload.ID, uploadArgs.index); err != nil {
		return nil, err
	}

	return nil, nil
}

// handleEnqueueMultipartFinalize handles the final request of a multipart upload. This transitions the
// upload from 'uploading' to 'queued', then instructs the bundle manager to concatenate all of the part
// files together.
func (h *UploadHandler) handleEnqueueMultipartFinalize(ctx context.Context, upload store.Upload) (interface{}, error) {
	if len(upload.UploadedParts) != upload.NumParts {
		return nil, clientError("upload is missing %d parts", upload.NumParts-len(upload.UploadedParts))
	}

	tx, err := h.dbStore.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = tx.Done(err)
	}()

	var sources []string
	for partNumber := 0; partNumber < upload.NumParts; partNumber++ {
		sources = append(sources, fmt.Sprintf("upload-%d.%d.lsif.gz", upload.ID, partNumber))
	}

	size, err := h.uploadStore.Compose(ctx, fmt.Sprintf("upload-%d.lsif.gz", upload.ID), sources...)
	if err != nil {
		h.markUploadAsFailed(context.Background(), tx, upload.ID, err)
		return nil, err
	}

	if err := tx.MarkQueued(ctx, upload.ID, &size); err != nil {
		return nil, err
	}

	return nil, nil
}

// markUploadAsFailed attempts to mark the given upload as failed, extracting a human-meaningful
// error message from the given error. We assume this method to whenever an error occurs when
// interacting with the upload store so that the status of the upload is accurately reflected in
// the UI.
//
// This method does not return an error as it's best-effort cleanup. If an error occurs when
// trying to modify the record, it will be logged but will not be directly visible to the user.
func (h *UploadHandler) markUploadAsFailed(ctx context.Context, tx DBStore, uploadID int, err error) {
	var reason string
	if errors.HasType(err, &ClientError{}) {
		reason = fmt.Sprintf("client misbehaving:\n* %s", err)
	} else if awsErr := formatAWSError(err); awsErr != "" {
		reason = fmt.Sprintf("object store error:\n* %s", awsErr)
	} else {
		reason = fmt.Sprintf("unknown error:\n* %s", err)
	}

	if markErr := tx.MarkFailed(ctx, uploadID, reason); markErr != nil {
		log15.Error("Failed to mark upload as failed", "error", markErr)
	}
}

// 🚨 SECURITY: It is critical to call this function after necessary authz check
// because this function would bypass authz to for testing if the repository and
// commit exists in Sourcegraph.
func ensureRepoAndCommitExist(ctx context.Context, db database.DB, repoName, commit string) (*types.Repo, int, error) {
	// This function won't be able to see all repositories without bypassing authz.
	ctx = actor.WithInternalActor(ctx)

	repo, err := backend.NewRepos(db.Repos()).GetByName(ctx, api.RepoName(repoName))
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, http.StatusNotFound, errors.Errorf("unknown repository %q", repoName)
		}

		return nil, http.StatusInternalServerError, err
	}

	if _, err := backend.NewRepos(db.Repos()).ResolveRev(ctx, repo, commit); err != nil {
		if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
			return nil, http.StatusNotFound, errors.Errorf("unknown commit %q", commit)
		}

		// If the repository is currently being cloned (which is most likely to happen on dotcom),
		// then we want to continue to queue the LSIF upload record to unblock the client, then have
		// the worker wait until the rev is resolvable before starting to process.
		if !gitdomain.IsCloneInProgress(err) {
			return nil, http.StatusInternalServerError, err
		}
	}

	return repo, 0, nil
}

// formatAWSError returns the unwrapped, root AWS/S3 error. This method returns
// an empty string when the given error value is neither an AWS nor an S3 error.
func formatAWSError(err error) string {
	var e manager.MultiUploadFailure
	if errors.As(err, &e) {
		return e.Error()
	}

	return ""
}
