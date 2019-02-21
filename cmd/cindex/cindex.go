// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"codesearch/index"
	"strings"
)

var usageMessage = `usage: cindex [-d indexdb|indexdb_dir] path [path...]
usage: cindex [-d indexdb] -list

indexfile is specified, or search in curdir to / for name .csearchindex or
$CSEARCHINDEX or $HOME/.csearchindex
`

func usage() {
	_,_ = fmt.Fprintf(os.Stderr, usageMessage)
	os.Exit(2)
}

var (
	listFlag    = flag.Bool("list", false, "list indexed paths and exit")
	resetFlag   = flag.Bool("reset", false, "discard existing index")
	verboseFlag = flag.Bool("verbose", false, "print extra information")
	cpuProfile  = flag.String("cpuprofile", "", "write cpu profile to this file")
	indfile = flag.String("d", "", "the index db filename")
	filetypes = flag.String("ft", "c|cpp|cxx|cc|inc|asm|s|h|hh|hxx|hpp|def|hdr|y|lex|yy", "file types")
)

func keepElem(elem string, isdir bool) bool{
	if elem[0] == '.' || elem[0] == '#' || elem[0] == '~' || elem[len(elem)-1] == '~' {
		return false
	}
	if isdir {
		if elem == "test" || elem == "tests" || elem == "testsuite" || elem == "testsuites"{
			return false
		}
		if elem == "unittests" || elem == "unittest" {
			return false
		}
		return true
	}
	// skip foo_test.c
	if strings.Index(elem, "_test.") >= 0{
		return false
	}
	// skip test_foo.c
	if strings.HasPrefix(elem, "test_") {
		return false
	}
	pos := strings.LastIndex(elem, ".")
	if pos < 0{
		return false
	}
	fext := strings.ToLower(elem[pos+1:])

	arr := strings.Split(*filetypes, "|")
	for _,o := range arr{
		if o == fext{
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) <= 1{
		usage()
		return
	}
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	index.SetFile(*indfile)

	if *listFlag {
		ix := index.Open(index.File())
		for _, arg := range ix.Paths() {
			fmt.Printf("%s\n", arg)
		}
		return
	}

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if *resetFlag && len(args) == 0 {
		os.Remove(index.File())
		return
	}
	if *indfile == "" {
		index.SetFile("./.csearchindex")
	}
	if len(args) == 0 {
		ix := index.Open(index.File())
		for _, arg := range ix.Paths() {
			args = append(args, arg)
		}
	}

	// Translate paths to absolute paths so that we can
	// generate the file list in sorted order.
	for i, arg := range args {
		a, err := filepath.Abs(arg)
		if err != nil {
			log.Printf("%s: %s", arg, err)
			args[i] = ""
			continue
		}
		args[i] = a
	}
	sort.Strings(args)

	for len(args) > 0 && args[0] == "" {
		args = args[1:]
	}

	master := index.File()
	if _, err := os.Stat(master); err != nil {
		// Does not exist.
		*resetFlag = true
	}
	file := master
	if !*resetFlag {
		file += "~"
	}

	ix := index.Create(file)
	ix.Verbose = *verboseFlag
	ix.AddPaths(args)
	for _, arg := range args {
		log.Printf("index %s", arg)
		_ = filepath.Walk(arg, func(path string, info os.FileInfo, err error) error {
			if err!=nil{
				_,_ = fmt.Fprintf(os.Stderr, "%s err: %s", path, err)
				return nil
			}
			if _, elem := filepath.Split(path); elem != "" {
				if ! keepElem(elem, info.IsDir()){
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			if err != nil {
				log.Printf("%s: %s", path, err)
				return nil
			}
			if info != nil && info.Mode()&os.ModeType == 0 {
				ix.AddFile(path)
			}
			return nil
		})
	}
	log.Printf("flush index")
	ix.Flush()

	if !*resetFlag {
		log.Printf("merge %s %s", master, file)
		index.Merge(file+"~", master, file)
		os.Remove(file)
		os.Rename(file+"~", master)
	}
	log.Printf("done")
	return
}
