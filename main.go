package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/trace"
	"strings"
	"sync"
	"time"
	"unicode"

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
	fmt.Printf("Calculating each word ocurrence count..\n")
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
	defer stopProfiling(profiler)

	// Process files
	start := time.Now()
	if len(flag.Args()) == 0 {
		log.Error("No files to process") // Log an error and exit if no files are passed
		return
	}

	finalResult, err := processFiles(flag.Args(), maxWorkers)
	if err != nil {
		log.Fatal(err)
	}

	// Print the final word count results
	// printResult(finalResult)
	fmt.Printf("Processing took: %v\n Total words: %v\n", time.Since(start), len(finalResult)) // Print elapsed time
}

// stopProfiling stops the profiler if it was started.
func stopProfiling(profiler interface{ Stop() }) {
	if profiler != nil {
		profiler.Stop() // Ensure profiler is stopped when the program exits
	}
}

// processFiles processes the list of files and returns the final word count result.
func processFiles(files []string, maxWorkers int) (map[string]int, error) {
	workersWG := new(sync.WaitGroup)
	partialResults := make(chan map[string]int, maxWorkers)
	workQueue := make(chan string, maxWorkers)
	reducerWG := new(sync.WaitGroup)
	finalResult := make(map[string]int)
	finalResultMutex := new(sync.Mutex)

	// Start the reducer goroutine to aggregate intermediate results
	for i := 0; i < maxWorkers; i++ {
		reducer(reducerWG, finalResult, partialResults, finalResultMutex)
	}
	// Start worker goroutines to process files
	for i := 0; i < maxWorkers; i++ {
		processFile(workersWG, partialResults, workQueue)
	}

	// Enqueue all filenames into the work queue
	for _, fn := range files {
		workQueue <- fn
	}
	close(workQueue)
	workersWG.Wait()      // Wait for all workers to complete their tasks
	close(partialResults) // Signal that no more intermediate results are coming
	reducerWG.Wait()      // Wait for the reducer to finish aggregating results

	return finalResult, nil
}

// processFile waits for file names on the workQueue, processes each file,
// and sends the word count results to the result channel.
func processFile(wg *sync.WaitGroup, result chan<- map[string]int, workQueue <-chan string) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Worker panicked: %v", r)
			}
		}()

		for fn := range workQueue {
			res := make(map[string]int)
			file, err := os.Open(fn)
			if err != nil {
				log.Warnf("Failed to open file %s: %v", fn, err)
				continue
			}
			defer file.Close()

			reader := bufio.NewReader(file)
			buf := make([]byte, 1024*1024) // 1 MB buffer

			for {
				n, err := reader.Read(buf)
				if n > 0 {
					// Convert the chunk to a string and split into words
					words := strings.FieldsFunc(string(buf[:n]), func(r rune) bool {
						// Split by any non-letter characters
						return !unicode.IsLetter(r)
					})

					for _, word := range words {
						// Convert to lowercase for case-insensitive comparison
						word = strings.ToLower(word)
						if word != "" {
							res[word]++
						}
					}
				}
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Warnf("Error reading file %s: %v", fn, err)
					break
				}
			}

			if err := file.Close(); err != nil {
				log.Warnf("Error closing file %s: %v", fn, err)
			}

			result <- res                       // Send results to channel
			log.Infof("Processed file: %s", fn) // Log progress
		}
	}()
}

// printResult prints the final word count results in a tabular format.
func printResult(result map[string]int) {
	fmt.Printf("%-10s%s\n", "Count", "Word")
	fmt.Printf("%-10s%s\n", "-----", "----")

	for w, c := range result {
		fmt.Printf("%-10v%s\n", c, w)
	}
}

// reducer aggregates the intermediate results from workers
// into the final result map and exits when the input channel closes.
func reducer(wg *sync.WaitGroup, finResult map[string]int, in <-chan map[string]int, mutex *sync.Mutex) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for res := range in {
			for k, v := range res {
				mutex.Lock()
				finResult[k] += v
				mutex.Unlock()
			}
		}
	}()
}
