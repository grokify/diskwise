package scan

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/grokify/diskwise/platform"
)

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	data := bytes.Repeat([]byte{0xCD}, size)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func findChild(node *Node, name string) *Node {
	for _, c := range node.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestWalk_ComputesSubtreeTotals(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub1", "sub2"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "a.txt"), 10_000)
	writeFile(t, filepath.Join(root, "sub1", "b.txt"), 20_000)
	writeFile(t, filepath.Join(root, "sub1", "sub2", "c.txt"), 30_000)

	rootNode, stats, err := Walk(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if rootNode.State != StateComplete {
		t.Errorf("root State = %v, want StateComplete", rootNode.State)
	}
	if rootNode.SubtreeLogicalSize != 60_000 {
		t.Errorf("root SubtreeLogicalSize = %d, want 60000", rootNode.SubtreeLogicalSize)
	}

	sub1 := findChild(rootNode, "sub1")
	if sub1 == nil {
		t.Fatal("sub1 not found")
	}
	if sub1.SubtreeLogicalSize != 50_000 {
		t.Errorf("sub1 SubtreeLogicalSize = %d, want 50000", sub1.SubtreeLogicalSize)
	}

	sub2 := findChild(sub1, "sub2")
	if sub2 == nil {
		t.Fatal("sub2 not found")
	}
	if sub2.SubtreeLogicalSize != 30_000 {
		t.Errorf("sub2 SubtreeLogicalSize = %d, want 30000", sub2.SubtreeLogicalSize)
	}

	if stats.DirsScanned != 3 {
		t.Errorf("DirsScanned = %d, want 3", stats.DirsScanned)
	}
	if stats.FilesScanned != 3 {
		t.Errorf("FilesScanned = %d, want 3", stats.FilesScanned)
	}
}

func TestWalk_EmptyDirectory(t *testing.T) {
	root := t.TempDir()

	rootNode, stats, err := Walk(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if rootNode.State != StateComplete {
		t.Errorf("State = %v, want StateComplete", rootNode.State)
	}
	if rootNode.SubtreeLogicalSize != 0 {
		t.Errorf("SubtreeLogicalSize = %d, want 0", rootNode.SubtreeLogicalSize)
	}
	if stats.DirsScanned != 1 {
		t.Errorf("DirsScanned = %d, want 1", stats.DirsScanned)
	}
}

// TestWalk_CallerSuppliedStats verifies a caller-provided Stats
// pointer is the one Walk mutates, so progress can be polled from
// another goroutine while the walk is still running.
func TestWalk_CallerSuppliedStats(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), 100)
	writeFile(t, filepath.Join(root, "b.txt"), 200)

	stats := &Stats{}
	_, returnedStats, err := Walk(context.Background(), root, Options{Stats: stats})
	if err != nil {
		t.Fatal(err)
	}
	if returnedStats != stats {
		t.Fatal("Walk returned a different Stats than the one supplied via Options")
	}
	if stats.FilesScanned != 2 {
		t.Errorf("FilesScanned = %d, want 2", stats.FilesScanned)
	}
}

// TestWalk_HardLinkDedup verifies the exact behavior the whole product
// depends on for accurate savings estimates: a hard-linked file's
// bytes are attributed once, not once per path that references it.
func TestWalk_HardLinkDedup(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(root, "orig.txt")
	linked := filepath.Join(root, "sub", "linked.txt")
	writeFile(t, original, 50_000)
	if err := os.Link(original, linked); err != nil {
		t.Fatal(err)
	}

	rootNode, stats, err := Walk(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}

	if stats.DuplicateFiles != 1 {
		t.Errorf("DuplicateFiles = %d, want 1", stats.DuplicateFiles)
	}
	if stats.DuplicateBytesSaved <= 0 {
		t.Errorf("DuplicateBytesSaved = %d, want > 0", stats.DuplicateBytesSaved)
	}

	origNode := findChild(rootNode, "orig.txt")
	if origNode == nil {
		t.Fatal("orig.txt not found")
	}
	if origNode.Duplicate {
		t.Error("the first-seen path should not be marked Duplicate")
	}

	sub := findChild(rootNode, "sub")
	if sub == nil {
		t.Fatal("sub not found")
	}
	linkedNode := findChild(sub, "linked.txt")
	if linkedNode == nil {
		t.Fatal("linked.txt not found")
	}
	if !linkedNode.Duplicate {
		t.Error("the second-seen hard link should be marked Duplicate")
	}

	if rootNode.SubtreeLogicalSize != 50_000 {
		t.Errorf("root SubtreeLogicalSize = %d, want 50000 (linked.txt's bytes should not be double-counted)", rootNode.SubtreeLogicalSize)
	}
}

func TestWalk_DeniedDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}

	root := t.TempDir()
	blocked := filepath.Join(root, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(blocked, "secret.txt"), 1_000)
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(blocked, 0o755)
	})

	rootNode, stats, err := Walk(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}

	blockedNode := findChild(rootNode, "blocked")
	if blockedNode == nil {
		t.Fatal("blocked not found")
	}
	if blockedNode.State != StateDenied {
		t.Errorf("blocked State = %v, want StateDenied", blockedNode.State)
	}
	if stats.DeniedPaths < 1 {
		t.Errorf("DeniedPaths = %d, want >= 1", stats.DeniedPaths)
	}
	if rootNode.State != StatePartial {
		t.Errorf("root State = %v, want StatePartial (has a denied descendant)", rootNode.State)
	}
}

func TestWalk_DepthCap(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(nested, "deep.txt"), 5_000)

	rootNode, stats, err := Walk(context.Background(), root, Options{Depth: 1})
	if err != nil {
		t.Fatal(err)
	}

	a := findChild(rootNode, "a")
	if a == nil {
		t.Fatal("a not found")
	}
	if a.State != StateDeferred {
		t.Errorf("a State = %v, want StateDeferred", a.State)
	}
	if a.SubtreeLogicalSize != 0 {
		t.Errorf("a SubtreeLogicalSize = %d, want 0 (unexpanded)", a.SubtreeLogicalSize)
	}
	if rootNode.State != StatePartial {
		t.Errorf("root State = %v, want StatePartial (has a deferred child)", rootNode.State)
	}
	if stats.DirsScanned != 1 {
		t.Errorf("DirsScanned = %d, want 1 (only root expanded)", stats.DirsScanned)
	}
}

func TestWalk_ContextCancelled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "f.txt"), 1_000)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rootNode, _, err := Walk(ctx, root, Options{})
	if err == nil {
		t.Fatal("expected an error from a pre-cancelled context")
	}
	if rootNode.State != StatePartial {
		t.Errorf("root State = %v, want StatePartial", rootNode.State)
	}
}

func TestWalk_RootNotFound(t *testing.T) {
	_, _, err := Walk(context.Background(), filepath.Join(t.TempDir(), "missing"), Options{})
	if err == nil {
		t.Fatal("expected an error for a missing root")
	}
}

func TestWalk_RootNotDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "f.txt")
	writeFile(t, file, 100)

	_, _, err := Walk(context.Background(), file, Options{})
	if err == nil {
		t.Fatal("expected an error when root is not a directory")
	}
}

func TestShouldDescend(t *testing.T) {
	cases := []struct {
		name                    string
		childDevice, rootDevice int32
		crossDevice             bool
		want                    bool
	}{
		{"same device", 1, 1, false, true},
		{"different device, disabled", 2, 1, false, false},
		{"different device, enabled", 2, 1, true, true},
		{"same device, enabled flag irrelevant", 1, 1, true, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldDescend(c.childDevice, c.rootDevice, c.crossDevice)
			if got != c.want {
				t.Errorf("shouldDescend(%d, %d, %v) = %v, want %v", c.childDevice, c.rootDevice, c.crossDevice, got, c.want)
			}
		})
	}
}

func TestIsPriorityPath(t *testing.T) {
	priorityPaths := []string{"/a/b", "/c/d"}
	if !isPriorityPath("/a/b", priorityPaths) {
		t.Error("expected an exact match to be a priority path")
	}
	if isPriorityPath("/a/b/child", priorityPaths) {
		t.Error("a descendant of a priority path should not itself match (exact match only)")
	}
	if isPriorityPath("/x/y", priorityPaths) {
		t.Error("an unrelated path should not match")
	}
}

// TestExpandDir_PriorityPathBoost calls expandDir directly (a
// white-box test) to observe the exact priority value it pushes for
// each discovered subdirectory — end-to-end Walk() concurrency makes
// expansion order inherently racy to assert on, but the queue push
// itself is a single, deterministic decision worth pinning down.
func TestExpandDir_PriorityPathBoost(t *testing.T) {
	root := t.TempDir()
	priorityDir := filepath.Join(root, "priority")
	normalDir := filepath.Join(root, "normal")
	if err := os.Mkdir(priorityDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(normalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(priorityDir, "f.txt"), 100)
	writeFile(t, filepath.Join(normalDir, "f.txt"), 100)

	rootStat, err := platform.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	rootNode := &Node{Path: root, Kind: KindDir}

	pq := newPriorityQueue()
	var pending sync.WaitGroup
	stats := &Stats{}
	dedup := newDedupSet()
	opts := Options{PriorityPaths: []string{priorityDir}}

	item := &queueItem{node: rootNode, priority: 0, depth: 0}
	pending.Add(1)
	expandDir(context.Background(), item, rootStat.Device, opts, pq, &pending, stats, dedup)
	pending.Done()
	pq.Close()

	priorities := map[string]int64{}
	for {
		qi, ok := pq.Pop()
		if !ok {
			break
		}
		priorities[qi.node.Name] = qi.priority
		pending.Done()
	}

	if priorities["priority"] != math.MaxInt64 {
		t.Errorf("priority dir priority = %d, want MaxInt64", priorities["priority"])
	}
	if priorities["normal"] == math.MaxInt64 {
		t.Error("normal dir should not receive the priority-path boost")
	}
}
