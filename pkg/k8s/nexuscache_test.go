// +build unit

package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	navigatorV1 "github.com/alxegox/navigator/pkg/apis/navigator/v1"
)

func TestNexusCache(t *testing.T) {
	assertForwardCache := func(t *testing.T, expected, actual map[string]Nexus) {
		// if used for maps pretty print
		if len(expected) != len(actual) {
			assert.Equal(t, expected, actual)
			return
		}
		for expKey, expVal := range expected {
			ok := assert.Contains(t, actual, expKey)
			if !ok {
				continue
			}
			actVal := actual[expKey]
			assert.Equal(t, expVal.AppName, actVal.AppName)
			assert.ElementsMatch(t, expVal.Services, actVal.Services)
		}
	}

	assertReverseCache := func(t *testing.T, expected, actual map[QualifiedName][]string) {
		// if used for maps pretty print
		if len(expected) != len(actual) {
			assert.Equal(t, expected, actual)
			return
		}
		for expKey, expVal := range expected {
			ok := assert.Contains(t, actual, expKey)
			if !ok {
				continue
			}
			actVal := actual[expKey]
			assert.ElementsMatch(t, expVal, actVal)
		}
	}

	type args struct {
		cluster string
		nx      *navigatorV1.Nexus
	}
	type snapshot struct {
		forward map[string]Nexus
		reverse map[QualifiedName][]string
	}
	type operation string
	const (
		update operation = "update"
		delete operation = "delete"
	)
	cache := NewNexusCache()

	app1Name := "test-service"
	app2Name := "service"

	app1Service := NewQualifiedName(app1Name, "")
	app2Service := NewQualifiedName(app2Name, "")

	testDep1 := NewQualifiedName("test-dep-1", "")
	testDep21 := NewQualifiedName("test-dep-2", "ooh")
	testDep22 := NewQualifiedName("test-dep-2", "oooh")
	testDep23 := NewQualifiedName("test-dep-2", "oooooh")

	dep1 := NewQualifiedName("dep-1", "oooooh")
	dep2 := NewQualifiedName("dep-2", "eeh")

	cluster1 := "alpha"
	cluster2 := "beta"
	cluster3 := "gamma"

	tests := []struct {
		name         string
		op           operation
		cache        NexusCache
		args         args
		want         []string
		wantSnapshot snapshot
	}{
		{
			name:  "update empty cache",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: []string{app1Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
				},
			},
		},
		{
			name:  "update cache with new part",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v2",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep23,
						),
					},
				},
			},
			want: []string{"test-service"},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
							testDep23,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
					testDep23:   {app1Name},
				},
			},
		},
		{
			name:  "update part",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
						),
					},
				},
			},
			want: []string{"test-service"},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep23,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep23:   {app1Name},
				},
			},
		},
		{
			name:  "update cache with new appID",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app2Service.Namespace,
						Name:      app2Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app2Name,
						Services: toResource(
							app2Service,
							dep1,
							dep2,
						),
					},
				},
			},
			want: []string{app2Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep23,
						},
					},
					app2Name: {
						AppName: app2Name,
						Services: []QualifiedName{
							app2Service,
							dep1,
							dep2,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep23:   {app1Name},
					app2Service: {app2Name},
					dep1:        {app2Name},
					dep2:        {app2Name},
				},
			},
		},
		{
			name:  "update cache with used nexuses",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app2Service.Namespace,
						Name:      app2Name + "-v2",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app2Name,
						Services: toResource(
							app2Service,
							dep1,
							testDep1,
							testDep23,
						),
					},
				},
			},
			want: []string{app2Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep23,
						},
					},
					app2Name: {
						AppName: app2Name,
						Services: []QualifiedName{
							app2Service,
							dep1,
							dep2,
							testDep1,
							testDep23,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name, app2Name},
					testDep23:   {app1Name, app2Name},
					app2Service: {app2Name},
					dep1:        {app2Name},
					dep2:        {app2Name},
				},
			},
		},
		{
			name:  "delete part",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app2Service.Namespace,
						Name:      app2Name + "-v2",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app2Name,
						Services: toResource(
							app2Service,
							dep1,
							testDep1,
							testDep23,
						),
					},
				},
			},
			want: []string{app2Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep23,
						},
					},
					app2Name: {
						AppName: app2Name,
						Services: []QualifiedName{
							app2Service,
							dep1,
							dep2,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep23:   {app1Name},
					app2Service: {app2Name},
					dep1:        {app2Name},
					dep2:        {app2Name},
				},
			},
		},
		{
			name:  "delete deleted part",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app2Service.Namespace,
						Name:      app2Name + "-v2",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app2Name,
						Services: toResource(
							app2Service,
							dep1,
							testDep1,
							testDep23,
						),
					},
				},
			},
			want: nil,
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep23,
						},
					},
					app2Name: {
						AppName: app2Name,
						Services: []QualifiedName{
							app2Service,
							dep1,
							dep2,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep23:   {app1Name},
					app2Service: {app2Name},
					dep1:        {app2Name},
					dep2:        {app2Name},
				},
			},
		},
		{
			name:  "delete last part in appID",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app2Service.Namespace,
						Name:      app2Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app2Name,
						Services: toResource(
							app2Service,
							dep1,
							dep2,
						),
					},
				},
			},
			want: []string{app2Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep23,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep23:   {app1Name},
				},
			},
		},
		{
			name:  "delete part 2",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v2",
					},
					Spec: navigatorV1.NexusSpec{
						AppName:  app1Name,
						Services: toResource(),
					},
				},
			},
			want: []string{app1Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
				},
			},
		},
		{
			name:  "delete all",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName:  app1Name,
						Services: toResource(),
					},
				},
			},
			want: []string{app1Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{},
				reverse: map[QualifiedName][]string{},
			},
		},
		{
			name:  "delete part in empty cashe",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName:  app1Name,
						Services: toResource(),
					},
				},
			},
			want: nil,
			wantSnapshot: snapshot{
				forward: map[string]Nexus{},
				reverse: map[QualifiedName][]string{},
			},
		},
		{
			name:  "update empty cache from cluster 1",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: []string{app1Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
				},
			},
		},
		{
			name:  "update cache from cluster 2",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster2,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: nil,
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
				},
			},
		},
		{
			name:  "update cache from cluster 3",
			op:    update,
			cache: cache,
			args: args{
				cluster: cluster3,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: nil,
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
				},
			},
		},
		{
			name:  "delete cache from cluster 2",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster2,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: nil,
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
				},
			},
		},
		{
			name:  "delete cache from cluster 1",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster1,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: nil,
			wantSnapshot: snapshot{
				forward: map[string]Nexus{
					app1Name: {
						AppName: app1Name,
						Services: []QualifiedName{
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						},
					},
				},
				reverse: map[QualifiedName][]string{
					app1Service: {app1Name},
					testDep1:    {app1Name},
					testDep21:   {app1Name},
					testDep22:   {app1Name},
				},
			},
		},
		{
			name:  "delete cache from cluster 3",
			op:    delete,
			cache: cache,
			args: args{
				cluster: cluster3,
				nx: &navigatorV1.Nexus{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: app1Service.Namespace,
						Name:      app1Name + "-v1",
					},
					Spec: navigatorV1.NexusSpec{
						AppName: app1Name,
						Services: toResource(
							app1Service,
							testDep1,
							testDep21,
							testDep22,
						),
					},
				},
			},
			want: []string{app1Name},
			wantSnapshot: snapshot{
				forward: map[string]Nexus{},
				reverse: map[QualifiedName][]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			if tt.op == update {
				got = tt.cache.Update(tt.args.cluster, tt.args.nx)
			} else {
				got = tt.cache.Delete(tt.args.cluster, tt.args.nx)
			}
			assert.Equal(t, tt.want, got)
			forwardKeys := make([]string, 0, len(tt.wantSnapshot.forward))
			for k := range tt.wantSnapshot.forward {
				forwardKeys = append(forwardKeys, k)
			}
			forwardSnap := tt.cache.GetSnapshotByAppName(forwardKeys)
			assertForwardCache(t, tt.wantSnapshot.forward, forwardSnap)

			reverseKeys := make([]QualifiedName, 0, len(tt.wantSnapshot.reverse))
			for k := range tt.wantSnapshot.reverse {
				reverseKeys = append(reverseKeys, k)
			}
			reverseSnap := tt.cache.GetAppNamesSnapshotByService(reverseKeys)
			assertReverseCache(t, tt.wantSnapshot.reverse, reverseSnap)
		})
	}
}

func toResource(services ...QualifiedName) []navigatorV1.Service {
	result := []navigatorV1.Service{}
	for _, service := range services {
		result = append(result, navigatorV1.Service{
			Name:      service.Name,
			Namespace: service.Namespace,
		})
	}
	return result
}
