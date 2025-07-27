package yathreadsafeset_test

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yathreadsafeset"
)

func TestThreadSafeSet_BasicOps(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()

	set.Set("a")
	set.Set("b")

	if !set.Has("a") || !set.Has("b") {
		t.Fatalf("Set or Has failed")
	}

	if set.Has("c") {
		t.Fatalf("Has returned true for missing element")
	}

	if set.Length() != 2 {
		t.Fatalf("Length failed, got %d", set.Length())
	}

	set.Delete("a")

	if set.Has("a") {
		t.Fatalf("Delete failed")
	}

	set.Delete("b")

	if !set.IsEmpty() {
		t.Fatalf("IsEmpty failed after delete")
	}

	set.Set("z")

	if !set.Pop("z") {
		t.Fatalf("Pop failed")
	}

	if set.Pop("z") {
		t.Fatalf("Pop should fail for non-existent element")
	}

	set.Set("x")
	set.Clear()

	if !set.IsEmpty() {
		t.Fatalf("Clear failed, set should be empty")
	}
}

func TestThreadSafeSet_Iterate(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[int]()

	vals := map[int]struct{}{1: {}, 2: {}, 3: {}}
	for k := range vals {
		set.Set(k)
	}

	collected := map[int]struct{}{}
	set.Iterate(func(x int) {
		collected[x] = struct{}{}
	})

	if !reflect.DeepEqual(collected, vals) {
		t.Fatalf("Iterate did not visit all values, got: %+v", collected)
	}
}

func TestThreadSafeSet_IterateOnCopy(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[int]()
	for i := 1; i <= 5; i++ {
		set.Set(i)
	}

	var (
		mu      sync.Mutex
		visited []int
	)

	set.IterateOnCopy(func(x int) {
		mu.Lock()

		visited = append(visited, x)

		mu.Unlock()
	})

	want := []int{1, 2, 3, 4, 5}
	for _, v := range want {
		found := slices.Contains(visited, v)
		if !found {
			t.Fatalf("IterateOnCopy missed %d", v)
		}
	}
}

func TestThreadSafeSet_IterateWithBreak(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[int]()
	for i := 1; i <= 5; i++ {
		set.Set(i)
	}

	var cnt int
	set.IterateWithBreak(func(_ int) bool {
		cnt++

		return cnt < 3
	})

	if cnt != 3 {
		t.Fatalf("IterateWithBreak did not break after 3")
	}
}

func TestThreadSafeSet_ImportFromMap(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	src := map[string]struct{}{"foo": {}, "bar": {}}
	set.ImportFromMap(src)

	if !set.Has("foo") || !set.Has("bar") {
		t.Fatalf("ImportFromMap failed")
	}
}

func TestThreadSafeSet_CopyRaw(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	set.Set("a")
	set.Set("b")

	m := set.CopyRaw()
	if len(m) != 2 || m["a"] != struct{}{} || m["b"] != struct{}{} {
		t.Fatalf("CopyRaw failed")
	}
}

func TestThreadSafeSet_StringAndMarshalJSON(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	set.Set("foo")
	set.Set("bar")

	s := set.String()
	if !strings.Contains(s, "foo") || !strings.Contains(s, "bar") || strings.Contains(s, "<error>") {
		t.Fatalf("String() failed: %q", s)
	}

	data, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	if !strings.Contains(string(data), "foo") || !strings.Contains(string(data), "bar") {
		t.Fatalf("MarshalJSON output wrong: %q", string(data))
	}
}

func TestThreadSafeSet_Values(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[int]()

	vals := []int{7, 8, 9}
	for _, v := range vals {
		set.Set(v)
	}

	got := set.Values()
	for _, v := range vals {
		found := slices.Contains(got, v)
		if !found {
			t.Fatalf("Values() missing %d", v)
		}
	}
}

func TestThreadSafeSet_DeleteMultiple(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	set.Set("x")
	set.Set("y")
	set.Set("z")
	set.DeleteMultiple([]string{"x", "z"})

	if set.Has("x") || set.Has("z") || !set.Has("y") {
		t.Fatalf("DeleteMultiple failed")
	}
}

func TestThreadSafeSet_IsEqual(t *testing.T) {
	a := yathreadsafeset.NewThreadSafeSet[int]()

	b := yathreadsafeset.NewThreadSafeSet[int]()
	if !a.IsEqual(b) {
		t.Fatalf("Empty sets should be equal")
	}

	a.Set(1)

	if a.IsEqual(b) {
		t.Fatalf("Should not be equal after add")
	}

	b.Set(1)

	if !a.IsEqual(b) {
		t.Fatalf("Sets with same content should be equal")
	}
}

func TestThreadSafeSet_Concurrency(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[int]()

	var wg sync.WaitGroup

	n := 1000

	for i := range n {
		wg.Add(1)

		go func(x int) {
			set.Set(x)
			wg.Done()
		}(i)
	}

	wg.Wait()

	if set.Length() != n {
		t.Fatalf("Concurrency Set failed, got %d", set.Length())
	}

	for i := range n {
		wg.Add(1)

		go func(x int) {
			set.Delete(x)
			wg.Done()
		}(i)
	}

	wg.Wait()

	if !set.IsEmpty() {
		t.Fatalf("Concurrency Delete failed")
	}
}

func TestThreadSafeSet_IsEmpty(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	if !set.IsEmpty() {
		t.Fatalf("Empty set must be IsEmpty()")
	}

	set.Set("abc")

	if set.IsEmpty() {
		t.Fatalf("Non-empty set is not empty")
	}

	set.Clear()

	if !set.IsEmpty() {
		t.Fatalf("IsEmpty after Clear() should be true")
	}
}

func TestThreadSafeSet_TypeParamSupport(t *testing.T) {
	type custom struct{ v int }

	set := yathreadsafeset.NewThreadSafeSet[custom]()
	val := custom{42}
	set.Set(val)

	if !set.Has(val) {
		t.Fatalf("Set/Has failed for custom type")
	}
}

func TestThreadSafeSet_MarshalUnmarshal(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	set.Set("one")
	set.Set("two")

	b, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var vals []string
	if err := json.Unmarshal(b, &vals); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(vals) != 2 || (vals[0] != "one" && vals[1] != "one") {
		t.Fatalf("Marshal/Unmarshal output wrong: %+v", vals)
	}
}

func NewThreadSafeSetFromSlice[T comparable](slice []T) *yathreadsafeset.ThreadSafeSet[T] {
	s := yathreadsafeset.NewThreadSafeSet[T]()
	for _, v := range slice {
		s.Set(v)
	}

	return s
}

func TestThreadSafeSet_Stress(_ *testing.T) {
	const (
		goroutines = 64
		opsPerG    = 5000
	)

	set := yathreadsafeset.NewThreadSafeSet[int]()

	var wg sync.WaitGroup

	for range goroutines {
		wg.Add(1)

		go func() {
			for range opsPerG {
				op := rand.Intn(4)
				val := rand.Intn(1000)

				switch op {
				case 0:
					set.Set(val)
				case 1:
					set.Delete(val)
				case 2:
					set.Has(val)
				case 3:
					set.Length()
				}
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

func TestThreadSafeSet_Copy(t *testing.T) {
	set := yathreadsafeset.NewThreadSafeSet[string]()
	set.Set("a")
	set.Set("b")

	copySet := set.Copy()

	if !copySet.Has("a") || !copySet.Has("b") {
		t.Fatalf("Copy failed, missing elements")
	}

	copySet.Delete("a")

	if !set.Has("a") {
		t.Fatalf("Original set should not be affected by copy modification")
	}

	if copySet.Has("b") {
		t.Logf("Copy still has 'b': %v", copySet)
	} else {
		t.Fatalf("Copy should still have 'b'")
	}
}

func TestThreadSafeSet_TestSafety(t *testing.T) {
	set := yathreadsafeset.ThreadSafeSet[int]{}

	set.Set(1)

	if !set.Has(1) {
		t.Fatalf("Set/Has failed for single element")
	}
}

func TestThreadSafeSet_Intersect(t *testing.T) {
	setA := yathreadsafeset.NewThreadSafeSet[int]()
	setB := yathreadsafeset.NewThreadSafeSet[int]()

	for i := 1; i <= 5; i++ {
		setA.Set(i)
	}

	for i := 3; i <= 7; i++ {
		setB.Set(i)
	}

	intersection := setA.Intersect(setB)

	expected := []int{3, 4, 5}
	for _, v := range expected {
		if !intersection.Has(v) {
			t.Fatalf("Intersection missing %d", v)
		}
	}

	if intersection.Length() != len(expected) {
		t.Fatalf("Intersection length mismatch, got %d, want %d", intersection.Length(), len(expected))
	}
}

func TestThreadSafeSet_Union(t *testing.T) {
	setA := yathreadsafeset.NewThreadSafeSet[int]()
	setB := yathreadsafeset.NewThreadSafeSet[int]()

	for i := 1; i <= 5; i++ {
		setA.Set(i)
	}

	for i := 4; i <= 8; i++ {
		setB.Set(i)
	}

	union := setA.Union(setB)

	expected := []int{1, 2, 3, 4, 5, 6, 7, 8}
	for _, v := range expected {
		if !union.Has(v) {
			t.Fatalf("Union missing %d", v)
		}
	}

	if union.Length() != len(expected) {
		t.Fatalf("Union length mismatch, got %d, want %d", union.Length(), len(expected))
	}
}

func TestThreadSafeSet_Difference(t *testing.T) {
	setA := yathreadsafeset.NewThreadSafeSet[int]()
	setB := yathreadsafeset.NewThreadSafeSet[int]()

	for i := 1; i <= 5; i++ {
		setA.Set(i)
	}

	for i := 4; i <= 8; i++ {
		setB.Set(i)
	}

	diff := setA.Difference(setB)

	expected := []int{1, 2, 3}
	for _, v := range expected {
		if !diff.Has(v) {
			t.Fatalf("Difference missing %d", v)
		}
	}

	if diff.Length() != len(expected) {
		t.Fatalf("Difference length mismatch, got %d, want %d", diff.Length(), len(expected))
	}
}

func TestThreadSafeSet_SymmetricDifference(t *testing.T) {
	setA := yathreadsafeset.NewThreadSafeSet[int]()
	setB := yathreadsafeset.NewThreadSafeSet[int]()

	for i := 1; i <= 5; i++ {
		setA.Set(i)
	}

	for i := 4; i <= 8; i++ {
		setB.Set(i)
	}

	diff := setA.SymmetricDifference(setB)

	expected := []int{1, 2, 3, 6, 7, 8}
	for _, v := range expected {
		if !diff.Has(v) {
			t.Fatalf("SymmetricDifference missing %d", v)
		}
	}

	if diff.Length() != len(expected) {
		t.Fatalf("SymmetricDifference length mismatch, got %d, want %d", diff.Length(), len(expected))
	}
}
