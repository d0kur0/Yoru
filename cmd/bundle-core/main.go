package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/d0kur0/Yoru/internal/core"
	"os"
	"runtime"
	"time"
)

func main() {
	platform := flag.String("os", runtime.GOOS, "target OS")
	arch := flag.String("arch", runtime.GOARCH, "target architecture")
	dir := flag.String("out", "internal/bundled/payload", "payload directory")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if e := core.PrepareBundle(ctx, *dir, *platform, *arch); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Printf("Bundled Mihomo %s: %s/%s\n", core.BundledVersion, *platform, *arch)
}
