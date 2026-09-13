// tiho-backend serves one native UI over JSON lines on stdin/stdout.
// Closing stdin ends the process. stdout is reserved for protocol responses.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type request struct {
	ID     int    `json:"id"`
	Method string `json:"method"`
}

type response struct {
	ID     int    `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type status struct {
	Version      string `json:"version"`
	BackendReady bool   `json:"backendReady"`
	CoreReady    bool   `json:"coreReady"`
	Connected    bool   `json:"connected"`
}

func serve(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 64*1024)
	encoder := json.NewEncoder(output)
	for scanner.Scan() {
		var req request
		res := response{}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			res.Error = "invalid_request"
		} else {
			res.ID = req.ID
			switch req.Method {
			case "status":
				res.Result = status{"0.2.0", true, false, false}
			default:
				res.Error = "method_not_found"
			}
		}
		if err := encoder.Encode(res); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func main() {
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
