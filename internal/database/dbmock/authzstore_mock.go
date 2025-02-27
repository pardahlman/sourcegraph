// Code generated by go-mockgen 1.1.2; DO NOT EDIT.

package dbmock

import (
	"context"
	"sync"

	database "github.com/sourcegraph/sourcegraph/internal/database"
	types "github.com/sourcegraph/sourcegraph/internal/types"
)

// MockAuthzStore is a mock implementation of the AuthzStore interface (from
// the package github.com/sourcegraph/sourcegraph/internal/database) used
// for unit testing.
type MockAuthzStore struct {
	// AuthorizedReposFunc is an instance of a mock function object
	// controlling the behavior of the method AuthorizedRepos.
	AuthorizedReposFunc *AuthzStoreAuthorizedReposFunc
	// GrantPendingPermissionsFunc is an instance of a mock function object
	// controlling the behavior of the method GrantPendingPermissions.
	GrantPendingPermissionsFunc *AuthzStoreGrantPendingPermissionsFunc
	// RevokeUserPermissionsFunc is an instance of a mock function object
	// controlling the behavior of the method RevokeUserPermissions.
	RevokeUserPermissionsFunc *AuthzStoreRevokeUserPermissionsFunc
}

// NewMockAuthzStore creates a new mock of the AuthzStore interface. All
// methods return zero values for all results, unless overwritten.
func NewMockAuthzStore() *MockAuthzStore {
	return &MockAuthzStore{
		AuthorizedReposFunc: &AuthzStoreAuthorizedReposFunc{
			defaultHook: func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error) {
				return nil, nil
			},
		},
		GrantPendingPermissionsFunc: &AuthzStoreGrantPendingPermissionsFunc{
			defaultHook: func(context.Context, *database.GrantPendingPermissionsArgs) error {
				return nil
			},
		},
		RevokeUserPermissionsFunc: &AuthzStoreRevokeUserPermissionsFunc{
			defaultHook: func(context.Context, *database.RevokeUserPermissionsArgs) error {
				return nil
			},
		},
	}
}

// NewStrictMockAuthzStore creates a new mock of the AuthzStore interface.
// All methods panic on invocation, unless overwritten.
func NewStrictMockAuthzStore() *MockAuthzStore {
	return &MockAuthzStore{
		AuthorizedReposFunc: &AuthzStoreAuthorizedReposFunc{
			defaultHook: func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error) {
				panic("unexpected invocation of MockAuthzStore.AuthorizedRepos")
			},
		},
		GrantPendingPermissionsFunc: &AuthzStoreGrantPendingPermissionsFunc{
			defaultHook: func(context.Context, *database.GrantPendingPermissionsArgs) error {
				panic("unexpected invocation of MockAuthzStore.GrantPendingPermissions")
			},
		},
		RevokeUserPermissionsFunc: &AuthzStoreRevokeUserPermissionsFunc{
			defaultHook: func(context.Context, *database.RevokeUserPermissionsArgs) error {
				panic("unexpected invocation of MockAuthzStore.RevokeUserPermissions")
			},
		},
	}
}

// NewMockAuthzStoreFrom creates a new mock of the MockAuthzStore interface.
// All methods delegate to the given implementation, unless overwritten.
func NewMockAuthzStoreFrom(i database.AuthzStore) *MockAuthzStore {
	return &MockAuthzStore{
		AuthorizedReposFunc: &AuthzStoreAuthorizedReposFunc{
			defaultHook: i.AuthorizedRepos,
		},
		GrantPendingPermissionsFunc: &AuthzStoreGrantPendingPermissionsFunc{
			defaultHook: i.GrantPendingPermissions,
		},
		RevokeUserPermissionsFunc: &AuthzStoreRevokeUserPermissionsFunc{
			defaultHook: i.RevokeUserPermissions,
		},
	}
}

// AuthzStoreAuthorizedReposFunc describes the behavior when the
// AuthorizedRepos method of the parent MockAuthzStore instance is invoked.
type AuthzStoreAuthorizedReposFunc struct {
	defaultHook func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error)
	hooks       []func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error)
	history     []AuthzStoreAuthorizedReposFuncCall
	mutex       sync.Mutex
}

// AuthorizedRepos delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockAuthzStore) AuthorizedRepos(v0 context.Context, v1 *database.AuthorizedReposArgs) ([]*types.Repo, error) {
	r0, r1 := m.AuthorizedReposFunc.nextHook()(v0, v1)
	m.AuthorizedReposFunc.appendCall(AuthzStoreAuthorizedReposFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the AuthorizedRepos
// method of the parent MockAuthzStore instance is invoked and the hook
// queue is empty.
func (f *AuthzStoreAuthorizedReposFunc) SetDefaultHook(hook func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// AuthorizedRepos method of the parent MockAuthzStore instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *AuthzStoreAuthorizedReposFunc) PushHook(hook func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *AuthzStoreAuthorizedReposFunc) SetDefaultReturn(r0 []*types.Repo, r1 error) {
	f.SetDefaultHook(func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *AuthzStoreAuthorizedReposFunc) PushReturn(r0 []*types.Repo, r1 error) {
	f.PushHook(func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error) {
		return r0, r1
	})
}

func (f *AuthzStoreAuthorizedReposFunc) nextHook() func(context.Context, *database.AuthorizedReposArgs) ([]*types.Repo, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *AuthzStoreAuthorizedReposFunc) appendCall(r0 AuthzStoreAuthorizedReposFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of AuthzStoreAuthorizedReposFuncCall objects
// describing the invocations of this function.
func (f *AuthzStoreAuthorizedReposFunc) History() []AuthzStoreAuthorizedReposFuncCall {
	f.mutex.Lock()
	history := make([]AuthzStoreAuthorizedReposFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// AuthzStoreAuthorizedReposFuncCall is an object that describes an
// invocation of method AuthorizedRepos on an instance of MockAuthzStore.
type AuthzStoreAuthorizedReposFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 *database.AuthorizedReposArgs
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []*types.Repo
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c AuthzStoreAuthorizedReposFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c AuthzStoreAuthorizedReposFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// AuthzStoreGrantPendingPermissionsFunc describes the behavior when the
// GrantPendingPermissions method of the parent MockAuthzStore instance is
// invoked.
type AuthzStoreGrantPendingPermissionsFunc struct {
	defaultHook func(context.Context, *database.GrantPendingPermissionsArgs) error
	hooks       []func(context.Context, *database.GrantPendingPermissionsArgs) error
	history     []AuthzStoreGrantPendingPermissionsFuncCall
	mutex       sync.Mutex
}

// GrantPendingPermissions delegates to the next hook function in the queue
// and stores the parameter and result values of this invocation.
func (m *MockAuthzStore) GrantPendingPermissions(v0 context.Context, v1 *database.GrantPendingPermissionsArgs) error {
	r0 := m.GrantPendingPermissionsFunc.nextHook()(v0, v1)
	m.GrantPendingPermissionsFunc.appendCall(AuthzStoreGrantPendingPermissionsFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the
// GrantPendingPermissions method of the parent MockAuthzStore instance is
// invoked and the hook queue is empty.
func (f *AuthzStoreGrantPendingPermissionsFunc) SetDefaultHook(hook func(context.Context, *database.GrantPendingPermissionsArgs) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// GrantPendingPermissions method of the parent MockAuthzStore instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *AuthzStoreGrantPendingPermissionsFunc) PushHook(hook func(context.Context, *database.GrantPendingPermissionsArgs) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *AuthzStoreGrantPendingPermissionsFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, *database.GrantPendingPermissionsArgs) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *AuthzStoreGrantPendingPermissionsFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, *database.GrantPendingPermissionsArgs) error {
		return r0
	})
}

func (f *AuthzStoreGrantPendingPermissionsFunc) nextHook() func(context.Context, *database.GrantPendingPermissionsArgs) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *AuthzStoreGrantPendingPermissionsFunc) appendCall(r0 AuthzStoreGrantPendingPermissionsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of AuthzStoreGrantPendingPermissionsFuncCall
// objects describing the invocations of this function.
func (f *AuthzStoreGrantPendingPermissionsFunc) History() []AuthzStoreGrantPendingPermissionsFuncCall {
	f.mutex.Lock()
	history := make([]AuthzStoreGrantPendingPermissionsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// AuthzStoreGrantPendingPermissionsFuncCall is an object that describes an
// invocation of method GrantPendingPermissions on an instance of
// MockAuthzStore.
type AuthzStoreGrantPendingPermissionsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 *database.GrantPendingPermissionsArgs
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c AuthzStoreGrantPendingPermissionsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c AuthzStoreGrantPendingPermissionsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// AuthzStoreRevokeUserPermissionsFunc describes the behavior when the
// RevokeUserPermissions method of the parent MockAuthzStore instance is
// invoked.
type AuthzStoreRevokeUserPermissionsFunc struct {
	defaultHook func(context.Context, *database.RevokeUserPermissionsArgs) error
	hooks       []func(context.Context, *database.RevokeUserPermissionsArgs) error
	history     []AuthzStoreRevokeUserPermissionsFuncCall
	mutex       sync.Mutex
}

// RevokeUserPermissions delegates to the next hook function in the queue
// and stores the parameter and result values of this invocation.
func (m *MockAuthzStore) RevokeUserPermissions(v0 context.Context, v1 *database.RevokeUserPermissionsArgs) error {
	r0 := m.RevokeUserPermissionsFunc.nextHook()(v0, v1)
	m.RevokeUserPermissionsFunc.appendCall(AuthzStoreRevokeUserPermissionsFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the
// RevokeUserPermissions method of the parent MockAuthzStore instance is
// invoked and the hook queue is empty.
func (f *AuthzStoreRevokeUserPermissionsFunc) SetDefaultHook(hook func(context.Context, *database.RevokeUserPermissionsArgs) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// RevokeUserPermissions method of the parent MockAuthzStore instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *AuthzStoreRevokeUserPermissionsFunc) PushHook(hook func(context.Context, *database.RevokeUserPermissionsArgs) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *AuthzStoreRevokeUserPermissionsFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, *database.RevokeUserPermissionsArgs) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *AuthzStoreRevokeUserPermissionsFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, *database.RevokeUserPermissionsArgs) error {
		return r0
	})
}

func (f *AuthzStoreRevokeUserPermissionsFunc) nextHook() func(context.Context, *database.RevokeUserPermissionsArgs) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *AuthzStoreRevokeUserPermissionsFunc) appendCall(r0 AuthzStoreRevokeUserPermissionsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of AuthzStoreRevokeUserPermissionsFuncCall
// objects describing the invocations of this function.
func (f *AuthzStoreRevokeUserPermissionsFunc) History() []AuthzStoreRevokeUserPermissionsFuncCall {
	f.mutex.Lock()
	history := make([]AuthzStoreRevokeUserPermissionsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// AuthzStoreRevokeUserPermissionsFuncCall is an object that describes an
// invocation of method RevokeUserPermissions on an instance of
// MockAuthzStore.
type AuthzStoreRevokeUserPermissionsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 *database.RevokeUserPermissionsArgs
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c AuthzStoreRevokeUserPermissionsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c AuthzStoreRevokeUserPermissionsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
