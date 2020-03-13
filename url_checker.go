package main

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

type urlChecker struct {
	timeout         time.Duration
	documentRoot    string
	excludedPattern *regexp.Regexp
	semaphore       semaphore
}

func newURLChecker(t time.Duration, d string, r *regexp.Regexp, s semaphore) urlChecker {
	return urlChecker{t, d, r, s}
}

func (c urlChecker) GetHTTPResponse(u string, timeout time.Duration) (error, int) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()

	req.SetRequestURI(u)
	req.Header.SetMethod("HEAD")

	var err error

	if timeout == 0 {
		err = fasthttp.Do(req, resp)
	} else {
		err = fasthttp.DoTimeout(req, resp, timeout)
	}

	if err != nil {
		return err, 0
	}

	return nil, resp.StatusCode()
}

func (c urlChecker) Check(u string, f string) error {
	u, frag, local, err := c.resolveURLWithFragment(u, f)
	if err != nil {
		return err
	}

	if c.excludedPattern != nil && c.excludedPattern.MatchString(u) {
		return nil
	}

	if local {
		if u == c.documentRoot || u == filepath.Dir(f) {
			// same file
			if len(frag) > 0 {
				err = mdAnchors.CheckAnchor(f, frag)
			}
		} else {
			// different file
			_, err = os.Stat(u)
			if len(frag) > 0 {
				err = mdAnchors.CheckAnchor(u, frag)
			}
		}
		return err
	}

	c.semaphore.Request()
	defer c.semaphore.Release()

	err, sc := c.GetHTTPResponse(u, c.timeout)
	if err != nil {
		// Skip if small header.
		if strings.HasPrefix(err.Error(), "error when reading response headers: small read buffer. Increase ReadBufferSize.") {
			return nil
		}
		return err
	}
	if sc >= fasthttp.StatusBadRequest && sc != fasthttp.StatusUnsupportedMediaType &&
		sc != fasthttp.StatusMethodNotAllowed {
		return fmt.Errorf("%s (HTTP error %d)", fasthttp.StatusMessage(sc), sc)
	}
	// Ignore errors from fasthttp about small buffer for URL headers,
	// the content is discarded anyway.
	if _, ok := err.(*fasthttp.ErrSmallBuffer); ok {
		err = nil
	}
	return err
}

func (c urlChecker) CheckMany(us []string, f string, rc chan<- urlResult) {
	wg := sync.WaitGroup{}

	for _, s := range us {
		wg.Add(1)

		go func(s string) {
			rc <- urlResult{s, c.Check(s, f)}
			wg.Done()
		}(s)
	}

	wg.Wait()
	close(rc)
}

func (c urlChecker) resolveURLWithFragment(u string, f string) (string, string, bool, error) {
	uu, err := url.Parse(u)

	if err != nil {
		return "", "", false, err
	}

	if uu.Scheme != "" {
		return u, "", false, nil
	}

	if !path.IsAbs(uu.Path) {
		return path.Join(filepath.Dir(f), uu.Path), uu.Fragment, true, nil
	}

	if c.documentRoot == "" {
		return "", "", false, fmt.Errorf("document root directory is not specified")
	}

	return path.Join(c.documentRoot, uu.Path), uu.Fragment, true, nil
}

func (c urlChecker) resolveURL(u string, f string) (string, bool, error) {
	uu, err := url.Parse(u)

	if err != nil {
		return "", false, err
	}

	if uu.Scheme != "" {
		return u, false, nil
	}

	if !path.IsAbs(uu.Path) {
		return path.Join(filepath.Dir(f), uu.Path), true, nil
	}

	if c.documentRoot == "" {
		return "", false, fmt.Errorf("document root directory is not specified")
	}

	return path.Join(c.documentRoot, uu.Path), true, nil
}
