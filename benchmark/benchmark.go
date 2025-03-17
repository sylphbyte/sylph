package benchmark

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sylphbyte/sylph"
)

// 为基准测试定义的端点
const BenchmarkEndpoint sylph.Endpoint = "benchmark"

// BenchmarkOptions 性能测试选项
type BenchmarkOptions struct {
	// 并发数
	Concurrency int
	// 每个并发执行的次数
	Iterations int
	// 预热迭代次数
	WarmupIterations int
	// 是否启用CPU Profile
	EnableCPUProfile bool
	// CPU Profile文件路径
	CPUProfilePath string
	// 是否启用内存Profile
	EnableMemProfile bool
	// 内存Profile文件路径
	MemProfilePath string
	// 测试名称
	Name string
}

// DefaultBenchmarkOptions 默认的性能测试选项
var DefaultBenchmarkOptions = BenchmarkOptions{
	Concurrency:      100,
	Iterations:       10000,
	WarmupIterations: 1000,
	EnableCPUProfile: false,
	CPUProfilePath:   "cpu.pprof",
	EnableMemProfile: false,
	MemProfilePath:   "mem.pprof",
	Name:             "default",
}

// BenchmarkResult 性能测试结果
type BenchmarkResult struct {
	// 测试名称
	Name string
	// 总耗时
	TotalDuration time.Duration
	// 每秒请求数
	RPS float64
	// 平均耗时
	AverageDuration time.Duration
	// P50耗时
	P50 time.Duration
	// P90耗时
	P90 time.Duration
	// P99耗时
	P99 time.Duration
	// 最小耗时
	Min time.Duration
	// 最大耗时
	Max time.Duration
	// 并发数
	Concurrency int
	// 总请求数
	TotalRequests int
	// 失败请求数
	FailedRequests int
	// 内存分配统计
	Allocs uint64
	// 总内存分配
	TotalAlloc uint64
}

// 创建基准测试上下文
func createBenchContext() sylph.Context {
	return sylph.NewDefaultContext(BenchmarkEndpoint, "/benchmark")
}

// RunBenchmark 运行基准测试
func RunBenchmark(fn func(ctx sylph.Context) error, opts BenchmarkOptions) (*BenchmarkResult, error) {
	if opts.Concurrency <= 0 {
		opts.Concurrency = DefaultBenchmarkOptions.Concurrency
	}
	if opts.Iterations <= 0 {
		opts.Iterations = DefaultBenchmarkOptions.Iterations
	}
	if opts.Name == "" {
		opts.Name = DefaultBenchmarkOptions.Name
	}

	// 输出测试信息
	log.Printf("开始性能测试: %s\n", opts.Name)
	log.Printf("并发数: %d, 迭代次数: %d\n", opts.Concurrency, opts.Iterations)

	// 预热
	if opts.WarmupIterations > 0 {
		log.Printf("预热中 (%d 次迭代)...\n", opts.WarmupIterations)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opts.WarmupIterations; i++ {
				ctx := createBenchContext()
				_ = fn(ctx)
				// 使用完毕后释放上下文，减少内存压力
				if releaser, ok := ctx.(*sylph.DefaultContext); ok {
					releaser.Release()
				}
			}
		}()
		wg.Wait()
		log.Println("预热完成")
	}

	// 内存统计
	var memStatsBefore, memStatsAfter runtime.MemStats
	runtime.ReadMemStats(&memStatsBefore)

	// CPU Profile
	if opts.EnableCPUProfile {
		f, err := os.Create(opts.CPUProfilePath)
		if err != nil {
			return nil, fmt.Errorf("创建CPU profile文件失败: %v", err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			return nil, fmt.Errorf("启动CPU profile失败: %v", err)
		}
		defer pprof.StopCPUProfile()
	}

	// 执行测试
	log.Println("开始执行测试...")
	startTime := time.Now()

	var wg sync.WaitGroup
	durations := make([]time.Duration, 0, opts.Concurrency*opts.Iterations)
	durationCh := make(chan time.Duration, opts.Concurrency*opts.Iterations)
	var errorCount int64 = 0

	// 并发执行
	wg.Add(opts.Concurrency)
	for i := 0; i < opts.Concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opts.Iterations; j++ {
				ctx := createBenchContext()
				start := time.Now()
				err := fn(ctx)
				duration := time.Since(start)

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				}

				durationCh <- duration

				// 使用完毕后释放上下文，减少内存压力
				if releaser, ok := ctx.(*sylph.DefaultContext); ok {
					releaser.Release()
				}
			}
		}(i)
	}

	// 收集结果
	go func() {
		for d := range durationCh {
			durations = append(durations, d)
		}
	}()

	wg.Wait()
	close(durationCh)

	// 等待所有结果收集完毕
	for len(durations) < opts.Concurrency*opts.Iterations {
		time.Sleep(10 * time.Millisecond)
	}

	endTime := time.Now()
	totalDuration := endTime.Sub(startTime)

	// 内存Profile
	if opts.EnableMemProfile {
		f, err := os.Create(opts.MemProfilePath)
		if err != nil {
			return nil, fmt.Errorf("创建内存profile文件失败: %v", err)
		}
		defer f.Close()
		if err := pprof.WriteHeapProfile(f); err != nil {
			return nil, fmt.Errorf("写入内存profile失败: %v", err)
		}
	}

	// 内存统计
	runtime.ReadMemStats(&memStatsAfter)

	// 计算结果
	totalRequests := opts.Concurrency * opts.Iterations
	rps := float64(totalRequests) / totalDuration.Seconds()

	// 计算耗时分布
	durationsSorted := make([]time.Duration, len(durations))
	copy(durationsSorted, durations)
	sort.Slice(durationsSorted, func(i, j int) bool {
		return durationsSorted[i] < durationsSorted[j]
	})

	p50Idx := int(float64(len(durationsSorted)) * 0.5)
	p90Idx := int(float64(len(durationsSorted)) * 0.9)
	p99Idx := int(float64(len(durationsSorted)) * 0.99)

	// 结果汇总
	result := &BenchmarkResult{
		Name:            opts.Name,
		TotalDuration:   totalDuration,
		RPS:             rps,
		AverageDuration: totalDuration / time.Duration(totalRequests),
		P50:             durationsSorted[p50Idx],
		P90:             durationsSorted[p90Idx],
		P99:             durationsSorted[p99Idx],
		Min:             durationsSorted[0],
		Max:             durationsSorted[len(durationsSorted)-1],
		Concurrency:     opts.Concurrency,
		TotalRequests:   totalRequests,
		FailedRequests:  int(errorCount),
		Allocs:          memStatsAfter.Mallocs - memStatsBefore.Mallocs,
		TotalAlloc:      memStatsAfter.TotalAlloc - memStatsBefore.TotalAlloc,
	}

	// 输出结果
	log.Printf("测试完成: %s\n", opts.Name)
	log.Printf("总请求数: %d, 失败请求: %d\n", result.TotalRequests, result.FailedRequests)
	log.Printf("总耗时: %v\n", result.TotalDuration)
	log.Printf("RPS: %.2f\n", result.RPS)
	log.Printf("平均耗时: %v\n", result.AverageDuration)
	log.Printf("P50: %v, P90: %v, P99: %v\n", result.P50, result.P90, result.P99)
	log.Printf("Min: %v, Max: %v\n", result.Min, result.Max)
	log.Printf("内存分配次数: %d, 总内存分配: %d bytes\n", result.Allocs, result.TotalAlloc)

	return result, nil
}
