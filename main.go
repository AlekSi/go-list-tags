package main

import (
	"flag"
	"fmt"
	"go/build"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
)

func main() {
	log.SetFlags(log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix("go-list-tags: ")
	flag.Parse()

	cmd := exec.Command("go", "tool", "dist", "list")
	cmd.Stderr = os.Stderr
	b, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}

	allGOOS := make(map[string]struct{})
	allGOARCH := make(map[string]struct{})
	for _, pair := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		parts := strings.Split(pair, "/")
		if len(parts) != 2 {
			log.Fatal(parts)
		}
		allGOOS[parts[0]] = struct{}{}
		allGOARCH[parts[1]] = struct{}{}
	}

	allRelease := make(map[string]struct{})
	for _, tag := range build.Default.ReleaseTags {
		allRelease[tag] = struct{}{}
	}

	args := flag.Args()
	if len(args) == 0 {
		log.Printf("Empty list of packages, assuming `all`.")
		args = []string{"all"}
	}

	// make package list
	packagesM := make(map[string]struct{})
	for _, arg := range args {
		cmd = exec.Command("go", "list", "-e", "-find", arg)
		cmd.Stderr = os.Stderr
		b, err = cmd.Output()
		if err != nil {
			log.Fatal(err)
		}

		for _, path := range strings.Split(strings.TrimSpace(string(b)), "\n") {
			packagesM[path] = struct{}{}
		}
	}

	packages := make([]string, 0, len(packagesM))
	for path := range packagesM {
		packages = append(packages, path)
	}
	sort.Strings(packages)

	log.Printf("Expanded packages list to %d packages.", len(packages))

	goroutines := runtime.GOMAXPROCS(-1)
	pathsCh := make(chan string, goroutines)
	packagesCh := make(chan *build.Package, goroutines)
	var wg sync.WaitGroup

	// receive paths from pathsCh until it is closed,
	// import package and send it to packagesCh
	context := build.Default
	context.UseAllFiles = true
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for path := range pathsCh {
				p, err := context.Import(path, ".", 0)
				if err != nil {
					log.Print(err)
					continue
				}

				packagesCh <- p
			}
		}()
	}

	// close packagesCh when all importing goroutines are done
	go func() {
		wg.Wait()
		close(packagesCh)
	}()

	// send all packages to pathsCh and close it
	go func() {
		for i, path := range packages {
			pathsCh <- path
			log.Printf("%4d/%4d %s", i+1, len(packages), path)
		}
		close(pathsCh)
	}()

	// receive packages from packagesCh until it is closed
	tagsM := make(map[string][]string) // tag -> packages
	for p := range packagesCh {
		for _, tag := range p.AllTags {
			s := tagsM[tag]
			s = append(s, p.ImportPath)
			tagsM[tag] = s
		}
	}

	tags := make([]string, 0, len(tagsM))
	for tag := range tagsM {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	fmt.Println("All tags: ", tags)

	var goos, goarch, release, other []string
	for _, tag := range tags {
		if _, ok := allGOOS[tag]; ok {
			goos = append(goos, tag)
			continue
		}

		if _, ok := allGOARCH[tag]; ok {
			goarch = append(goarch, tag)
			continue
		}

		if _, ok := allRelease[tag]; ok {
			release = append(release, tag)
			continue
		}

		other = append(other, tag)
	}

	sort.Strings(goos)
	sort.Strings(goarch)
	sort.Strings(other)

	fmt.Println("GOOS tags: ", goos)
	fmt.Println("GOARCH tags: ", goarch)
	fmt.Println("Release tags: ", release)
	fmt.Println("Other tags: ", other)

	for _, tag := range tags {
		fmt.Printf("%s:\n", tag)
		s := tagsM[tag]
		sort.Strings(s)
		for _, path := range s {
			fmt.Printf("\t%s\n", path)
		}
	}
}
