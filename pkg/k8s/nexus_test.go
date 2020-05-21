// +build unit

package k8s

import (
	"testing"
)

func TestNexusIsEqual(t *testing.T) {

	app1 := "test-service"
	app2 := "service"

	dep11 := NewQualifiedName("dep1", "")

	dep21 := NewQualifiedName("dep2", "eeh")
	dep22 := NewQualifiedName("dep2", "ooh")
	dep23 := NewQualifiedName("dep2", "oooh")

	dep31 := NewQualifiedName("dep3", "")

	downstreams := []QualifiedName{}
	inboundPorts := []InboundPort{
		{Name: "http", Port: 8889},
	}

	tests := []struct {
		name  string
		nexus Nexus
		arg   Nexus
		want  bool
	}{
		{
			name:  "equal simple",
			nexus: NewNexus(app1, []QualifiedName{dep11}, downstreams, inboundPorts),
			arg:   NewNexus(app1, []QualifiedName{dep11}, downstreams, inboundPorts),
			want:  true,
		},
		{
			name:  "not equal simple",
			nexus: NewNexus(app1, []QualifiedName{dep11}, downstreams, inboundPorts),
			arg:   NewNexus(app2, []QualifiedName{dep21}, downstreams, inboundPorts),
			want:  false,
		},
		{
			name:  "equal with nil",
			nexus: NewNexus(app1, nil, nil, nil),
			arg:   NewNexus(app1, nil, nil, nil),
			want:  true,
		},
		{
			name:  "not equal with nil",
			nexus: NewNexus(app1, nil, nil, nil),
			arg:   NewNexus(app1, []QualifiedName{dep11}, downstreams, inboundPorts),
			want:  false,
		},
		{
			name: "equal several services",
			nexus: NewNexus(app1, []QualifiedName{
				dep11,
				dep21,
				dep22,
				dep23,
			}, downstreams, inboundPorts),
			arg: NewNexus(app1, []QualifiedName{
				dep11,
				dep21,
				dep22,
				dep23,
			}, downstreams, inboundPorts),
			want: true,
		},
		{
			name: "not equal several services",
			nexus: NewNexus(app1, []QualifiedName{
				dep11,
				dep21,
				dep22,
				dep23,
			}, downstreams, inboundPorts),
			arg: NewNexus(app1, []QualifiedName{
				dep21,
				dep22,
				dep23,
				dep31,
			}, downstreams, inboundPorts),
			want: false,
		},
		{
			name: "not equal disjoint services",
			nexus: NewNexus(app1, []QualifiedName{
				dep11,
				dep21,
			}, downstreams, inboundPorts),
			arg: NewNexus(app1, []QualifiedName{
				dep22,
				dep23,
				dep31,
			}, downstreams, inboundPorts),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nexus.isEqual(tt.arg); got != tt.want {
				t.Errorf("Nexus.isEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
