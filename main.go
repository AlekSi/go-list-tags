package main

import (
	"flag"
	"go/build"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func main() {
	log.SetFlags(log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix("go-list-tags: ")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Printf("Empty list of packages, assuming `all`.")
		args = []string{"all"}
	}

	packagesM := make(map[string]struct{})
	for _, arg := range args {
		cmd := exec.Command("go", "list", arg)
		cmd.Stderr = os.Stderr
		b, err := cmd.Output()
		if err != nil {
			continue
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

	context := build.Default
	context.UseAllFiles = true

	tagsM := make(map[string][]string) // tag -> packages
	for _, path := range packages {
		p, err := context.Import(path, ".", 0)
		if err != nil {
			log.Print(err)
			continue
		}

		for _, tag := range p.AllTags {
			s := tagsM[tag]
			s = append(s, path)
			tagsM[tag] = s
		}
	}

	tags := make([]string, 0, len(tagsM))
	for tag := range tagsM {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		log.Printf("%s:\n", tag)
		s := tagsM[tag]
		sort.Strings(s)
		for _, path := range s {
			log.Printf("\t%s\n", path)
		}
	}
}
