package fswatch

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/watch"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestGitManager(t *testing.T) {
	f := newGMFixture(t)
	defer f.TearDown()

	head := f.JoinPath(".git", "HEAD")
	f.WriteFile(head, "ref: refs/heads/nicks/branch")

	f.store.WithState(func(state *store.EngineState) {
		repo := model.LocalGitRepo{LocalPath: f.Path()}
		m := manifestbuilder.New(f, "fe").
			WithK8sYAML(testyaml.SanchoYAML).
			WithImageTarget(
				model.ImageTarget{}.
					WithRepos([]model.LocalGitRepo{repo})).
			Build()
		state.UpsertManifestTarget(store.NewManifestTarget(m))
	})

	f.gm.OnChange(f.ctx, f.store)
	assert.Equal(t, "ref: refs/heads/nicks/branch",
		f.NextGitBranchStatusAction().Head)

	f.store.ClearActions()
	f.WriteFile(head, "ref: refs/heads/nicks/branch2")
	f.fakeMultiWatcher.Events <- watch.NewFileEvent(head)
	assert.Equal(t, "ref: refs/heads/nicks/branch2",
		f.NextGitBranchStatusAction().Head)
}

type gmFixture struct {
	ctx              context.Context
	cancel           func()
	store            *store.TestingStore
	gm               *GitManager
	fakeMultiWatcher *FakeMultiWatcher
	*tempdir.TempDirFixture
}

func newGMFixture(t *testing.T) *gmFixture {
	st := store.NewTestingStore()
	fakeMultiWatcher := NewFakeMultiWatcher()
	gm := NewGitManager(fakeMultiWatcher.NewSub)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	f := tempdir.NewTempDirFixture(t)
	f.Chdir()

	return &gmFixture{
		ctx:              ctx,
		cancel:           cancel,
		store:            st,
		gm:               gm,
		fakeMultiWatcher: fakeMultiWatcher,
		TempDirFixture:   f,
	}
}

func (f *gmFixture) TearDown() {
	f.TempDirFixture.TearDown()
	f.cancel()
	f.store.AssertNoErrorActions(f.T())
}

func (f *gmFixture) NextGitBranchStatusAction() GitBranchStatusAction {
	return f.store.WaitForAction(f.T(), reflect.TypeOf(GitBranchStatusAction{})).(GitBranchStatusAction)
}
