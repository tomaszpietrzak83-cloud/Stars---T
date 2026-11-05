package cs

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPatrolJobCacheTracksTagsWithoutAliasing(t *testing.T) {
	f := &Fleet{}
	savePatrolJob(f, patrolJob{Target: 41, SupportDesigns: map[int]int{3: 4}})
	first := readPatrolJob(f)
	first.SupportDesigns[3] = 99
	require.Equal(t, 4, readPatrolJob(f).SupportDesigns[3])
	savePatrolJob(f, patrolJob{Target: 42})
	require.Equal(t, 42, readPatrolJob(f).Target)
	f.SetTag(patrolTag, "")
	require.Zero(t, readPatrolJob(f).Target)
	f.SetTag(patrolTag, `{"target":43}`)
	require.Equal(t, 43, readPatrolJob(f).Target)
}
