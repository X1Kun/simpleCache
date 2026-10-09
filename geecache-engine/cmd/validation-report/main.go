// validation-report summarizes captured test runs without invoking Git.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/validationreport"
)

func main() {
	dir := flag.String("dir", "", "Run artifact directory")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "-dir is required")
		os.Exit(2)
	}
	r, err := validationreport.Generate(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	fmt.Println(string(b))
	if r.Status != "passed" {
		os.Exit(1)
	}
}
