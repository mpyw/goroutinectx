package spawner

import "testing"

func TestIsSpawnerComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "canonical", text: "//goroutinectx:spawner", want: true},
		{name: "spaced", text: "// goroutinectx:spawner", want: false},
		{name: "block comment", text: "/* goroutinectx:spawner */", want: false},
		{name: "trailing comment", text: "//goroutinectx:spawner //vt:helper", want: true},
		{name: "longer name", text: "//goroutinectx:spawnerX", want: false},
		{name: "spawnerlabel", text: "//goroutinectx:spawnerlabel", want: false},
		{name: "longer tool", text: "//goroutinectxx:spawner", want: false},
		{name: "other directive", text: "//goroutinectx:ignore", want: false},
		{name: "plain comment", text: "// spawner", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isSpawnerComment(tt.text); got != tt.want {
				t.Errorf("isSpawnerComment(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}
