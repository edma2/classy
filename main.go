package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"9fans.net/go/plan9"
	"9fans.net/go/plumb"

	"github.com/edma2/navigator/index"
	"github.com/edma2/navigator/zinc"
	"github.com/edma2/navigator/zinc/fsevents"
)

func plumbDir(idx *index.Index, children []string, w io.Writer) error {
	dirs := make(map[string]int)
	for _, c := range children {
		if get := idx.Get(c); get != nil {
			if get.Path != "" && !strings.Contains(get.Path, "/src/test/") {
				dirs[filepath.Dir(get.Path)]++
			}
		}
	}
	// guess dir to plumb by most common dir
	max := 0
	dir := ""
	for d, n := range dirs {
		if n > max {
			max = n
			dir = d
		}
	}
	log.Println(dirs)
	log.Println(dir)
	if dir == "" {
		return nil
	}
	return plumbDir1(dir, w)
}

func plumbDir1(dir string, w io.Writer) error {
	m := plumb.Message{
		Src:  "navigator",
		Dst:  "edit",
		Type: "text",
		Data: []byte(dir),
	}
	log.Printf("Sending to plumber: %s\n", m)
	return m.Send(w)
}

func leafOf(name string) string {
	if i := strings.LastIndexByte(name, '.'); i != -1 && i+1 <= len(name) {
		return name[i+1:]
	}
	return ""
}

func candidatesOf(name string) []string {
	candidates := []string{}
	elems := strings.Split(name, ".")
	for i, _ := range elems {
		candidates = append(candidates, strings.Join(elems[0:i+1], "."))
	}
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	return candidates
}

func plumbFile(m *plumb.Message, w io.Writer, name, path string) error {
	m.Src = "navigator"
	m.Dst = ""
	m.Data = []byte(path)
	var attr *plumb.Attribute
	for attr = m.Attr; attr != nil; attr = attr.Next {
		if attr.Name == "addr" {
			break
		}
	}
	if attr == nil {
		if leafName := leafOf(name); leafName != "" {
			addr := fmt.Sprintf("/(trait|class|object|interface)[ 	]*%s/", leafName)
			m.Attr = &plumb.Attribute{Name: "addr", Value: addr, Next: m.Attr}
		}
	}
	log.Printf("Sending to plumber: %s\n", m)
	return m.Send(w)
}

func serve(idx *index.Index) error {
	fid, err := plumb.Open("editclass", plan9.OREAD)
	if err != nil {
		return err
	}
	defer fid.Close()
	r := bufio.NewReader(fid)
	w, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		return err
	}
	defer w.Close()
	for {
		m := plumb.Message{}
		err := m.Recv(r)
		if err != nil {
			return err
		}
		log.Printf("Received from plumber: %s\n", m)
		name := string(m.Data)
		var get *index.GetResult
		for _, c := range candidatesOf(name) {
			if get = idx.Get(c); get != nil {
				break
			}
		}
		if get == nil {
			log.Printf("Found no results for: %s\n", name)
			continue
		}
		if get.Path != "" {
			if err := plumbFile(&m, w, name, get.Path); err != nil {
				log.Printf("%s: %s\n", get.Path, err)
			}
		} else if get.Children != nil {
			if err := plumbDir(idx, get.Children, w); err != nil {
				log.Printf("error opening dir: %s\n", err)
			}
		} else {
			log.Printf("Result was empty: %s\n", name)
		}
	}
	return nil
}

func Main() error {
	flag.Parse()
	paths := flag.Args()
	if len(paths) == 0 {
		return nil
	}
	for _, path := range paths {
		log.Println("Watching " + path)
	}
	idx := index.NewIndex()
	for _, path := range paths {
		idx.Watch(zinc.Watch(fsevents.Watch(path)))
	}
	return serve(idx)
}

func main() {
	if err := Main(); err != nil {
		log.Fatal(err)
	}
}
