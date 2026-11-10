package index

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlanReport(t *testing.T) {
	tests := []struct {
		name string
		plan Plan
		want string
	}{
		{
			name: "nothing to do",
			plan: Plan{},
			want: "nothing to change\n",
		},
		{
			name: "versions to add",
			plan: Plan{Add: []Ref{{Chart: "fleet", Version: "1"}, {Chart: "fleet", Version: "2"}}},
			want: "add fleet 1\nadd fleet 2\n",
		},
		{
			name: "versions to hide",
			plan: Plan{Hide: []Ref{{Chart: "fleet", Version: "1"}}},
			want: "hide fleet 1\n",
		},
		{
			name: "both, the additions first",
			plan: Plan{
				Add:  []Ref{{Chart: "alerting", Version: "9"}},
				Hide: []Ref{{Chart: "fleet", Version: "1"}},
			},
			want: "add alerting 9\nhide fleet 1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.plan.Report())
		})
	}
}
