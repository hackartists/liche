package main

import (
	"bytes"
	"errors"
	"github.com/russross/blackfriday/v2"
	"golang.org/x/net/html"
	"io/ioutil"
	"sync"
)

var mdAnchors *MdAnchors = &MdAnchors{anchors: make(map[string]map[string]bool)}

type MdAnchors struct {
	anchors map[string]map[string]bool
	lock sync.Mutex
}

func (m *MdAnchors) CheckAnchor(filename, anchor string) error {
	err := m.parseFile(filename)
	if err != nil {
		return err
	}

	if _, ok := m.anchors[filename][anchor]; !ok {
		return errors.New("anchor not found")
	}

	return nil
}

func (m *MdAnchors) parseFile(filename string) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Skip if already parsed.
	if _, ok := m.anchors[filename]; ok {
		return nil
	}

	bs, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if !isMarkupFile(filename) {
		return errors.New("not md file")
	}

	bs = blackfriday.Run(bs, blackfriday.WithExtensions(blackfriday.CommonExtensions), blackfriday.WithExtensions(blackfriday.AutoHeadingIDs))

	n, err := html.Parse(bytes.NewReader(bs))
	if err != nil {
		return err
	}

	anchors, err := m.extractAnchors(n)
	if err != nil {
		return err
	}
	m.anchors[filename] = anchors

	return nil
}

func (m *MdAnchors) extractAnchors(n *html.Node) (map[string]bool, error) {
	us := make(map[string]bool)
	ns := []*html.Node{n}

	for len(ns) > 0 {
		i := len(ns) - 1
		n := ns[i]
		ns = ns[:i]

		if n.Type == html.ElementNode {
			for _, attr := range n.Attr {
				if attr.Key == "id" {
					us[attr.Val] = true
				}
			}
		}

		for n := n.FirstChild; n != nil; n = n.NextSibling {
			ns = append(ns, n)
		}
	}

	return us, nil
}

