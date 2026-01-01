package metrics

import (
	"sync"
	"testing"
)

func TestIncRateLimitDrop(t *testing.T) {
	// 重置全局状态
	rl = rateLimitStats{}

	tests := []struct {
		name   string
		prefix string
	}{
		{
			name:   "increment with prefix",
			prefix: "test",
		},
		{
			name:   "increment with empty prefix (defaults to global)",
			prefix: "",
		},
		{
			name:   "increment global",
			prefix: "global",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 获取初始状态
			initialTotal, _ := RateLimitSnapshot()

			// 调用递增函数
			IncRateLimitDrop(tt.prefix)

			// 验证总数增加了
			newTotal, byPrefix := RateLimitSnapshot()
			if newTotal != initialTotal+1 {
				t.Errorf("total = %d, want %d", newTotal, initialTotal+1)
			}

			// 验证前缀计数器
			expectedPrefix := tt.prefix
			if expectedPrefix == "" {
				expectedPrefix = "global"
			}
			if byPrefix[expectedPrefix] == 0 {
				t.Errorf("prefix %s not incremented", expectedPrefix)
			}
		})
	}
}

func TestIncRateLimitDrop_Concurrent(t *testing.T) {
	// 重置全局状态
	rl = rateLimitStats{}

	const goroutines = 100
	const incrementsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			for j := 0; j < incrementsPerGoroutine; j++ {
				IncRateLimitDrop("concurrent")
			}
		}(i)
	}

	wg.Wait()

	total, byPrefix := RateLimitSnapshot()
	expectedTotal := uint64(goroutines * incrementsPerGoroutine)

	if total != expectedTotal {
		t.Errorf("total = %d, want %d", total, expectedTotal)
	}

	if byPrefix["concurrent"] != expectedTotal {
		t.Errorf("concurrent prefix = %d, want %d", byPrefix["concurrent"], expectedTotal)
	}
}

func TestRateLimitSnapshot(t *testing.T) {
	// 重置全局状态
	rl = rateLimitStats{}

	// 初始状态应该是空的
	total, byPrefix := RateLimitSnapshot()
	if total != 0 {
		t.Errorf("initial total = %d, want 0", total)
	}
	if len(byPrefix) != 0 {
		t.Errorf("initial byPrefix length = %d, want 0", len(byPrefix))
	}

	// 添加一些计数
	IncRateLimitDrop("prefix1")
	IncRateLimitDrop("prefix1")
	IncRateLimitDrop("prefix2")

	total, byPrefix = RateLimitSnapshot()

	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	if byPrefix["prefix1"] != 2 {
		t.Errorf("prefix1 = %d, want 2", byPrefix["prefix1"])
	}

	if byPrefix["prefix2"] != 1 {
		t.Errorf("prefix2 = %d, want 1", byPrefix["prefix2"])
	}
}

func TestRateLimitSnapshot_Isolation(t *testing.T) {
	// 重置全局状态
	rl = rateLimitStats{}

	IncRateLimitDrop("test")

	// 获取快照
	snapshot1, _ := RateLimitSnapshot()

	// 修改原始数据
	IncRateLimitDrop("test")

	// 快照不应该改变
	snapshot2, _ := RateLimitSnapshot()

	if snapshot2 != snapshot1+1 {
		t.Errorf("snapshot isolation failed: snapshot1=%d, snapshot2=%d", snapshot1, snapshot2)
	}
}

func TestIncRateLimitDrop_MultiplePrefixes(t *testing.T) {
	// 重置全局状态
	rl = rateLimitStats{}

	prefixes := []string{"api", "web", "mobile", "desktop"}
	expectedCounts := make(map[string]uint64)

	for i, prefix := range prefixes {
		count := uint64(i + 1) * 10
		expectedCounts[prefix] = count

		for j := uint64(0); j < count; j++ {
			IncRateLimitDrop(prefix)
		}
	}

	total, byPrefix := RateLimitSnapshot()

	var expectedTotal uint64
	for _, count := range expectedCounts {
		expectedTotal += count
	}

	if total != expectedTotal {
		t.Errorf("total = %d, want %d", total, expectedTotal)
	}

	for prefix, expectedCount := range expectedCounts {
		if byPrefix[prefix] != expectedCount {
			t.Errorf("prefix %s = %d, want %d", prefix, byPrefix[prefix], expectedCount)
		}
	}
}
