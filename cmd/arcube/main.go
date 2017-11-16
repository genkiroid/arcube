package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/genkiroid/arcube"
)

func main() {
	if len(os.Args[1:]) != 1 {
		log.Fatalln("input eccube zip file path")
	}

	zipPath, err := filepath.Abs(os.Args[1])
	if err != nil {
		log.Fatalf("filepath.Abs failed: %v", err)
	}

	if err := arcube.Run(zipPath); err != nil {
		log.Fatalf("arcube.Run failed: %v", err)
	}
}
