package resolver

import (
	"testing"
)

func TestSubstituteInputs(t *testing.T) {
	cases := []struct {
		name   string
		data   string
		inputs map[string]any
		want   string
	}{
		{
			name:   "simple substitution",
			data:   `stage: $[[ inputs.STAGE ]]`,
			inputs: map[string]any{"STAGE": "deploy"},
			want:   `stage: deploy`,
		},
		{
			name:   "default string used when key absent",
			data:   `image: $[[ inputs.IMAGE | default('ubuntu:22.04') ]]`,
			inputs: map[string]any{},
			want:   `image: ubuntu:22.04`,
		},
		{
			name:   "input overrides default",
			data:   `image: $[[ inputs.IMAGE | default('ubuntu:22.04') ]]`,
			inputs: map[string]any{"IMAGE": "alpine:3.18"},
			want:   `image: alpine:3.18`,
		},
		{
			name:   "integer input",
			data:   `variables:\n  RETRIES: $[[ inputs.RETRY_COUNT ]]`,
			inputs: map[string]any{"RETRY_COUNT": 3},
			want:   `variables:\n  RETRIES: 3`,
		},
		{
			name:   "boolean default",
			data:   `variables:\n  ENABLED: $[[ inputs.ENABLE | default(true) ]]`,
			inputs: map[string]any{},
			want:   `variables:\n  ENABLED: true`,
		},
		{
			name:   "numeric default",
			data:   `variables:\n  COUNT: $[[ inputs.COUNT | default(5) ]]`,
			inputs: map[string]any{},
			want:   `variables:\n  COUNT: 5`,
		},
		{
			name:   "missing key no default becomes empty",
			data:   `script: $[[ inputs.CMD ]]`,
			inputs: map[string]any{},
			want:   `script: `,
		},
		{
			name:   "no placeholders unchanged",
			data:   `stage: build`,
			inputs: nil,
			want:   `stage: build`,
		},
		{
			name:   "multiple placeholders in one document",
			data:   "stage: $[[ inputs.STAGE ]]\nimage: $[[ inputs.IMAGE | default('alpine') ]]",
			inputs: map[string]any{"STAGE": "test"},
			want:   "stage: test\nimage: alpine",
		},
		{
			name:   "double-quoted default",
			data:   `image: $[[ inputs.IMAGE | default("debian:12") ]]`,
			inputs: map[string]any{},
			want:   `image: debian:12`,
		},
		{
			name:   "whitespace inside brackets",
			data:   `stage: $[[ inputs.STAGE  ]]`,
			inputs: map[string]any{"STAGE": "build"},
			want:   `stage: build`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(substituteInputs([]byte(tc.data), tc.inputs))
			if got != tc.want {
				t.Errorf("substituteInputs:\n got  %q\n want %q", got, tc.want)
			}
		})
	}
}
