package graphqlbackend

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/graph-gophers/graphql-go/errors"
	gqlerrors "github.com/graph-gophers/graphql-go/errors"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmock"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestOrganization(t *testing.T) {
	users := dbmock.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1}, nil)

	orgMembers := dbmock.NewMockOrgMemberStore()
	orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, nil)

	orgs := dbmock.NewMockOrgStore()
	orgs.GetByNameFunc.SetDefaultReturn(&types.Org{ID: 1, Name: "acme"}, nil)

	db := dbmock.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)

	t.Run("anyone can access by default", func(t *testing.T) {
		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": {
						"name": "acme"
					}
				}
			`,
			},
		})
	})

	t.Run("users not invited or not a member cannot access on Sourcegraph.com", func(t *testing.T) {
		orig := envvar.SourcegraphDotComMode()
		envvar.MockSourcegraphDotComMode(true)
		defer envvar.MockSourcegraphDotComMode(orig)

		RunTests(t, []*Test{
			{
				Schema: mustParseGraphQLSchema(t, db),
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": null
				}
				`,
				ExpectedErrors: []*errors.QueryError{
					{
						Message: "org not found: name acme",
						Path:    []interface{}{"organization"},
					},
				},
			},
		})
	})

	t.Run("org members can access on Sourcegraph.com", func(t *testing.T) {
		orig := envvar.SourcegraphDotComMode()
		envvar.MockSourcegraphDotComMode(true)
		defer envvar.MockSourcegraphDotComMode(orig)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		users := dbmock.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		orgMembers := dbmock.NewMockOrgMemberStore()
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(&types.OrgMembership{OrgID: 1, UserID: 1}, nil)

		db := dbmock.NewMockDBFrom(db)
		db.UsersFunc.SetDefaultReturn(users)
		db.OrgMembersFunc.SetDefaultReturn(orgMembers)

		RunTests(t, []*Test{
			{
				Schema:  mustParseGraphQLSchema(t, db),
				Context: ctx,
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": {
						"name": "acme"
					}
				}
				`,
			},
		})
	})

	t.Run("invited users can access on Sourcegraph.com", func(t *testing.T) {
		orig := envvar.SourcegraphDotComMode()
		envvar.MockSourcegraphDotComMode(true)
		defer envvar.MockSourcegraphDotComMode(orig)

		ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

		users := dbmock.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: false}, nil)

		orgMembers := dbmock.NewMockOrgMemberStore()
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})

		orgInvites := dbmock.NewMockOrgInvitationStore()
		orgInvites.GetPendingFunc.SetDefaultReturn(nil, nil)

		db := dbmock.NewMockDBFrom(db)
		db.UsersFunc.SetDefaultReturn(users)
		db.OrgMembersFunc.SetDefaultReturn(orgMembers)
		db.OrgInvitationsFunc.SetDefaultReturn(orgInvites)

		RunTests(t, []*Test{
			{
				Schema:  mustParseGraphQLSchema(t, db),
				Context: ctx,
				Query: `
				{
					organization(name: "acme") {
						name
					}
				}
			`,
				ExpectedResult: `
				{
					"organization": {
						"name": "acme"
					}
				}
				`,
			},
		})
	})
}

func TestAddOrganizationMember(t *testing.T) {
	userID := int32(2)
	userName := "add-org-member"
	orgID := int32(1)
	orgIDString := string(MarshalOrgID(orgID))

	orgs := dbmock.NewMockOrgStore()
	orgs.GetByNameFunc.SetDefaultReturn(&types.Org{ID: orgID, Name: "acme"}, nil)

	users := dbmock.NewMockUserStore()
	users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
	users.GetByUsernameFunc.SetDefaultReturn(&types.User{ID: 2, Username: userName}, nil)

	orgMembers := dbmock.NewMockOrgMemberStore()
	orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})
	orgMembers.CreateFunc.SetDefaultReturn(&types.OrgMembership{OrgID: orgID, UserID: userID}, nil)

	featureFlags := dbmock.NewMockFeatureFlagStore()
	featureFlags.GetOrgFeatureFlagFunc.SetDefaultReturn(true, nil)

	// tests below depend on config being there
	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{AuthProviders: []schema.AuthProviders{{Builtin: &schema.BuiltinAuthProvider{}}}, EmailSmtp: nil}})

	// mock repo updater http client
	oldClient := repoupdater.DefaultClient.HTTPClient
	repoupdater.DefaultClient.HTTPClient = &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte{'{', '}'})),
			}, nil
		}),
	}

	defer func() {
		repoupdater.DefaultClient.HTTPClient = oldClient
	}()

	db := dbmock.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)
	db.UsersFunc.SetDefaultReturn(users)
	db.OrgMembersFunc.SetDefaultReturn(orgMembers)
	db.FeatureFlagsFunc.SetDefaultReturn(featureFlags)

	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	t.Run("Works for site admin if not on Cloud", func(t *testing.T) {
		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation AddUserToOrganization($organization: ID!, $username: String!) {
				addUserToOrganization(organization: $organization, username: $username) {
					alwaysNil
				}
			}`,
			ExpectedResult: `{
				"addUserToOrganization": {
					"alwaysNil": null
				}
			}`,
			Variables: map[string]interface{}{
				"organization": orgIDString,
				"username":     userName,
			},
		})
	})

	t.Run("Does not work for site admin on Cloud", func(t *testing.T) {
		envvar.MockSourcegraphDotComMode(true)
		defer envvar.MockSourcegraphDotComMode(false)

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation AddUserToOrganization($organization: ID!, $username: String!) {
				addUserToOrganization(organization: $organization, username: $username) {
					alwaysNil
				}
			}`,
			ExpectedResult: "null",
			ExpectedErrors: []*gqlerrors.QueryError{
				{
					Message: "Must be a member of the organization to add members%!(EXTRA *withstack.withStack=current user is not an org member)",
					Path:    []interface{}{string("addUserToOrganization")},
				},
			},
			Variables: map[string]interface{}{
				"organization": orgIDString,
				"username":     userName,
			},
		})
	})

	t.Run("Works on Cloud if site admin is org member", func(t *testing.T) {
		envvar.MockSourcegraphDotComMode(true)
		orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultHook(func(ctx context.Context, orgID int32, userID int32) (*types.OrgMembership, error) {
			if userID == 1 {
				return &types.OrgMembership{OrgID: orgID, UserID: 1}, nil
			} else if userID == 2 {
				return nil, &database.ErrOrgMemberNotFound{}
			}
			t.Fatalf("Unexpected user ID received for OrgMembers.GetByOrgIDAndUserID: %d", userID)
			return nil, nil
		})

		defer func() {
			envvar.MockSourcegraphDotComMode(false)
			orgMembers.GetByOrgIDAndUserIDFunc.SetDefaultReturn(nil, &database.ErrOrgMemberNotFound{})
		}()

		RunTest(t, &Test{
			Schema:  mustParseGraphQLSchema(t, db),
			Context: ctx,
			Query: `mutation AddUserToOrganization($organization: ID!, $username: String!) {
				addUserToOrganization(organization: $organization, username: $username) {
					alwaysNil
				}
			}`,
			ExpectedResult: `{
				"addUserToOrganization": {
					"alwaysNil": null
				}
			}`,
			Variables: map[string]interface{}{
				"organization": orgIDString,
				"username":     userName,
			},
		})
	})
}

func TestOrganizationRepositories_OSS(t *testing.T) {
	db := dbmock.NewMockDB()
	ctx := actor.WithActor(context.Background(), &actor.Actor{UID: 1})

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					organization(name: "acme") {
						name,
						repositories {
							nodes {
								name
							}
						}
					}
				}
			`,
			ExpectedErrors: []*errors.QueryError{{
				Message:   `Cannot query field "repositories" on type "Org".`,
				Locations: []errors.Location{{Line: 5, Column: 7}},
				Rule:      "FieldsOnCorrectType",
			}},
			Context: ctx,
		},
	})
}

func TestNode_Org(t *testing.T) {
	orgs := dbmock.NewMockOrgStore()
	orgs.GetByIDFunc.SetDefaultReturn(&types.Org{ID: 1, Name: "acme"}, nil)

	db := dbmock.NewMockDB()
	db.OrgsFunc.SetDefaultReturn(orgs)

	RunTests(t, []*Test{
		{
			Schema: mustParseGraphQLSchema(t, db),
			Query: `
				{
					node(id: "T3JnOjE=") {
						id
						... on Org {
							name
						}
					}
				}
			`,
			ExpectedResult: `
				{
					"node": {
						"id": "T3JnOjE=",
						"name": "acme"
					}
				}
			`,
		},
	})
}

func TestUnmarshalOrgID(t *testing.T) {
	t.Run("Valid org ID is parsed correctly", func(t *testing.T) {
		const id = int32(1)
		namespaceOrgID := relay.MarshalID("Org", id)
		orgID, err := UnmarshalOrgID(namespaceOrgID)
		assert.NoError(t, err)
		assert.Equal(t, id, orgID)
	})

	t.Run("Returns error for invalid org ID", func(t *testing.T) {
		const id = 1
		namespaceOrgID := relay.MarshalID("User", id)
		_, err := UnmarshalOrgID(namespaceOrgID)
		assert.Error(t, err)
	})
}
