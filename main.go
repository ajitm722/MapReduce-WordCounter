package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/pkg/profile"
	log "github.com/sirupsen/logrus"
)

var (
	profileType string // Flag to specify the type of profiling (cpu/mem/block/trace)
	maxWorkers  int    // Number of workers for processing files
)

func main() {
	// Parse command-line flags
	flag.StringVar(&profileType, "profile", "", "type of profiling: cpu, mem, block, or trace")
	flag.Parse()

	// Set maxWorkers to the number of CPUs available on the system
	maxWorkers = runtime.NumCPU()

	// Start profiling based on the profileType flag
	var profiler interface{ Stop() }
	switch profileType {
	case "cpu":
		profiler = profile.Start(profile.CPUProfile)
	case "mem":
		profiler = profile.Start(profile.MemProfile)
	case "block":
		profiler = profile.Start(profile.BlockProfile)
	case "trace":
		traceFile, err := os.Create("trace.out")
		if err != nil {
			log.Fatal("Could not create trace file: ", err)
		}
		defer traceFile.Close()
		if err := trace.Start(traceFile); err != nil {
			log.Fatal("Could not start trace: ", err)
		}
		defer trace.Stop() // Ensure tracing stops when the program exits
	default:
		if profileType != "" {
			log.Warn("Invalid profile type. Valid options are: cpu, mem, block, trace")
		}
	}
	if profiler != nil {
		defer profiler.Stop() // Ensure profiler is stopped when the program exits
	}

	// Check if there are input files to process
	if len(flag.Args()) == 0 {
		log.Error("No files to process") // Log an error and exit if no files are passed
		return
	}

	// Synchronization primitives for managing worker and reducer tasks
	workersWG := new(sync.WaitGroup)                        // Wait group for worker goroutines
	partialResults := make(chan map[string]int, maxWorkers) // Channel for intermediate results from workers
	workQueue := make(chan string, maxWorkers)              // Channel for file names to process
	reducerWG := new(sync.WaitGroup)                        // Wait group for reducer goroutine
	finalResult := make(map[string]int)                     // Final word count result map

	start := time.Now() // Record start time for measuring performance

	// Start the reducer goroutine to aggregate intermediate results
	reducer(reducerWG, finalResult, partialResults)

	// Start worker goroutines to process files
	for i := 0; i < maxWorkers; i++ {
		processFile(workersWG, partialResults, workQueue)
	}

	// Enqueue all filenames into the work queue
	for _, fn := range flag.Args() {
		workQueue <- fn // Send file name to the work queue
	}
	close(workQueue)      // No more files to process, close the work queue
	workersWG.Wait()      // Wait for all workers to complete their tasks
	close(partialResults) // Signal that no more intermediate results are coming
	reducerWG.Wait()      // Wait for the reducer to finish aggregating results

	// Print the final word count results
	printResult(finalResult)
	fmt.Printf("Processing took: %v\n", time.Since(start)) // Print elapsed time
}

// processFile waits for file names on the workQueue, processes each file,
// and sends the word count results to the result channel.
func processFile(wg *sync.WaitGroup, result chan<- map[string]int, workQueue <-chan string) {
	wg.Add(1) // Increment the wait group counter
	go func() {
		defer wg.Done() // Decrement the counter when the goroutine completes
		var w string
		for fn := range workQueue { // Get file names from the work queue
			res := make(map[string]int) // Intermediate result map for word counts
			r, err := os.Open(fn)       // Open the file
			if nil != err {
				log.Warn(err) // Log a warning if the file cannot be opened
				return
			}
			defer r.Close() // Ensure the file is closed after processing

			sc := bufio.NewScanner(r) // Create a scanner to read the file
			sc.Split(bufio.ScanWords) // Configure the scanner to split by words

			// Count the occurrence of each word in the file
			for sc.Scan() {
				w = strings.ToLower(sc.Text()) // Convert word to lowercase for case-insensitive comparison
				res[w] = res[w] + 1            // Increment the count for the word
			}
			result <- res // Send the intermediate result to the result channel
		}
	}()
}

// printResult prints the final word count results in a tabular format.
func printResult(result map[string]int) {
	fmt.Printf("%-10s%s\n", "Count", "Word") // Table header
	fmt.Printf("%-10s%s\n", "-----", "----")

	for w, c := range result { // Iterate through the result map
		fmt.Printf("%-10v%s\n", c, w) // Print word and its count
	}
}

// reducer aggregates the intermediate results from workers
// into the final result map and exits when the input channel closes.
func reducer(wg *sync.WaitGroup, finResult map[string]int, in <-chan map[string]int) {
	wg.Add(1) // Increment the wait group counter
	go func() {
		defer wg.Done()       // Decrement the counter when the goroutine completes
		for res := range in { // Read intermediate results from the input channel
			for k, v := range res { // Merge intermediate results into the final result map
				finResult[k] += v
			}
		}
	}()
}
