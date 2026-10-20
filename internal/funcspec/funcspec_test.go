package funcspec

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in      string
		want    Spec
		wantErr bool
	}{
		{in: "github.com/org/apm.NewGoroutineContext", want: Spec{PkgPath: "github.com/org/apm", FuncName: "NewGoroutineContext"}},
		{in: "github.com/newrelic/go-agent/v3/newrelic.Transaction.NewGoroutine", want: Spec{PkgPath: "github.com/newrelic/go-agent/v3/newrelic", TypeName: "Transaction", FuncName: "NewGoroutine"}},
		{in: "context.WithCancel", want: Spec{PkgPath: "context", FuncName: "WithCancel"}},
		{in: "sync.WaitGroup.Go", want: Spec{PkgPath: "sync", TypeName: "WaitGroup", FuncName: "Go"}},
		{in: "apm.NewGoroutineContext", wantErr: true},
		{in: "apm.Tracer.Start", wantErr: true},
		{in: "NewGoroutineContext", wantErr: true},
		{in: "github.com/org/apm.", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			got, err := Parse(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}
