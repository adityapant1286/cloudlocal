package utils

import (
	"bufio"
	"io"
	"os"
	"time"
)

func StreamLogFile(filePath string, output chan<- string, stop <-chan struct{}) {
	file, err := os.Open(filePath)
	if err != nil {
		output <- "Error opening log: " + err.Error()
		return
	}
	defer file.Close()

	// Move to the end of the file initially if you only want new logs
	// Or stay at 0 to show recent history
	reader := bufio.NewReader(file)
	for {
		select {
		case <-stop:
			return
		default:
			line, err := reader.ReadString('\n')
			if err == nil {
				output <- line
			} else if err == io.EOF {
				time.Sleep(500 * time.Millisecond) // Wait for new lines
			} else {
				return
			}
		}
	}
}
