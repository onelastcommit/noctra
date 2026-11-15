package cleanup

import (
	"errors"
	"reflect"
	"testing"
)

func TestSplitByOpenPRKeepsBranchOnLookupError(t *testing.T) {
	lookup := map[string]error{"noctra/a": nil, "noctra/b": errors.New("gh: not logged in")}
	open := map[string]bool{"noctra/c": true}
	withPR, safe := splitByOpenPR([]string{"noctra/a", "noctra/b", "noctra/c"}, func(b string) (bool, error) {
		return open[b], lookup[b]
	})
	if want := []string{"noctra/b", "noctra/c"}; !reflect.DeepEqual(withPR, want) {
		t.Errorf("withPR = %v, want %v", withPR, want)
	}
	if want := []string{"noctra/a"}; !reflect.DeepEqual(safe, want) {
		t.Errorf("safe = %v, want %v", safe, want)
	}
}
