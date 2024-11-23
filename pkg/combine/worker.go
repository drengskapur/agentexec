// File: pkg/combine/worker.go
package combine

import (
	"runtime"
	"sync"

	"go.uber.org/zap"
)

// ProcessFilesConcurrently processes files using a worker pool and returns the combined contents.
func ProcessFilesConcurrently(files []string, maxWorkers int, parentDir string, logger *zap.Logger) ([]FileContent, error) {
	jobs := make(chan string, len(files))
	results := make(chan FileContent, len(files))
	var wg sync.WaitGroup

	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
		logger.Debug("Adjusted worker count", zap.Int("workers", maxWorkers))
	}

	logger.Debug("Initializing worker pool", zap.Int("workers", maxWorkers))
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		workerLogger := logger.With(zap.Int("workerID", w))
		go worker(w, jobs, results, parentDir, &wg, workerLogger)
	}

	logger.Debug("Distributing files to workers")
	for _, file := range files {
		jobs <- file
	}
	close(jobs)
	logger.Debug("All files distributed to workers")

	// Collect results concurrently
	go func() {
		wg.Wait()
		close(results)
	}()

	var combinedContents []FileContent
	for content := range results {
		logger.Debug("Received processed file", zap.String("file", content.Path))
		combinedContents = append(combinedContents, content)
	}

	logger.Debug("All files processed", zap.Int("processedFiles", len(combinedContents)))
	return combinedContents, nil
}

// worker is a goroutine that processes files from the jobs channel.
func worker(id int, jobs <-chan string, results chan<- FileContent, parentDir string, wg *sync.WaitGroup, logger *zap.Logger) {
	defer wg.Done()
	logger.Debug("Worker started", zap.Int("workerID", id))

	for file := range jobs {
		logger.Debug("Worker received file to process",
			zap.Int("workerID", id),
			zap.String("filePath", file))

		content, err := ProcessSingleFile(file, parentDir, logger)
		if err != nil {
			logger.Error("Worker failed to process file",
				zap.Int("workerID", id),
				zap.String("filePath", file),
				zap.Error(err))
			continue // Decide whether to skip or halt on error
		}

		results <- content
		logger.Debug("Worker successfully processed file",
			zap.Int("workerID", id),
			zap.String("filePath", file))
	}

	logger.Debug("Worker finished processing", zap.Int("workerID", id))
}
